package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/hasdev/forge-ade/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

const devLoadingHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>ForgeADE - Starting Dev Server...</title>
  <style>
    body {
      background: #121214;
      color: #a1a1aa;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100vh;
      margin: 0;
      user-select: none;
    }
    .spinner {
      width: 32px;
      height: 32px;
      border: 3px solid rgba(255,255,255,0.1);
      border-top-color: #3b82f6;
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
      margin-bottom: 16px;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    h2 { font-size: 16px; font-weight: 500; color: #f4f4f5; margin: 0 0 8px 0; }
    p { font-size: 13px; margin: 0; color: #71717a; }
  </style>
  <script>
    setTimeout(function() {
      location.reload();
    }, 1000);
  </script>
</head>
<body>
  <div class="spinner"></div>
  <h2>Starting Development Server…</h2>
  <p>Connecting to Vite frontend. This window will refresh automatically.</p>
</body>
</html>`

type devFallbackResponseWriter struct {
	http.ResponseWriter
	req         *http.Request
	status      int
	headerSent  bool
	intercepted bool
}

func (w *devFallbackResponseWriter) WriteHeader(statusCode int) {
	if w.headerSent {
		return
	}
	w.status = statusCode
	if statusCode == http.StatusBadGateway && (w.req.URL.Path == "/" || w.req.URL.Path == "/index.html" || strings.Contains(w.req.Header.Get("Accept"), "text/html")) {
		w.intercepted = true
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Del("Content-Length")
		w.ResponseWriter.WriteHeader(http.StatusOK)
		w.headerSent = true
		_, _ = w.ResponseWriter.Write([]byte(devLoadingHTML))
		return
	}
	w.headerSent = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *devFallbackResponseWriter) Write(b []byte) (int, error) {
	if w.intercepted {
		return len(b), nil
	}
	if !w.headerSent {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func main() {
	// Tune Go runtime memory footprint for desktop app on macOS:
	// 1. Lower GC target from 100% to 50% so heap does not bloat before collecting.
	// 2. Set soft memory limit (96 MiB) to trigger GC before claiming excess OS memory.
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(96 * 1024 * 1024)

	a := app.NewApp()

	// When launched from "Open New Window" (or the CLI), open the passed
	// folder or .workspace file instead of the previously persisted workspace.
	if len(os.Args) > 1 && os.Args[1] != "" {
		argPath := os.Args[1]
		if resolved, err := app.ResolvePath(argPath); err == nil {
			if info, err := os.Stat(resolved); err == nil {
				if info.IsDir() {
					if _, err := a.OpenFolder(resolved); err != nil {
						log.Printf("new window: open folder %s: %v", resolved, err)
					}
				} else if _, err := a.OpenWorkspace(resolved); err != nil {
					log.Printf("new window: open workspace %s: %v", resolved, err)
				}
			}
		}
	}

	wailsApp := application.New(application.Options{
		Name:        "ForgeADE",
		Description: "Native AI Development Workspace",
		Services: []application.Service{
			application.NewService(a),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/wails/custom.js" {
						w.Header().Set("Content-Type", "application/javascript")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("// custom.js\n"))
						return
					}
					rec := &devFallbackResponseWriter{
						ResponseWriter: w,
						req:            r,
					}
					next.ServeHTTP(rec, r)
				})
			},
		},
	})

	wailsApp.Menu.Set(application.DefaultApplicationMenu())

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:                      "ForgeADE",
		URL:                        "/",
		Width:                      1280,
		Height:                     800,
		MinWidth:                   800,
		MinHeight:                  600,
		BackgroundType:             application.BackgroundTypeSolid,
		EnableFileDrop:             true,
		DevToolsEnabled:            true,
		DefaultContextMenuDisabled: false,
		KeyBindings: map[string]func(window application.Window){
			"F12": func(window application.Window) {
				window.OpenDevTools()
			},
			"CmdOrCtrl+Shift+I": func(window application.Window) {
				window.OpenDevTools()
			},
			"CmdOrCtrl+Alt+I": func(window application.Window) {
				window.OpenDevTools()
			},
			"F5": func(window application.Window) {
				window.Reload()
			},
			"CmdOrCtrl+R": func(window application.Window) {
				window.Reload()
			},
		},
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarHidden,
			Appearance: application.NSAppearanceNameDarkAqua,
		},
	})
	a.SetMainWindow(mainWindow)

	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if wailsApp.Env.Info().Debug {
			mainWindow.OpenDevTools()
		}
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
