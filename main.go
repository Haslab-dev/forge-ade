package main

import (
	"embed"
	"log"
	"net/http"
	"os"

	"github.com/hasdev/forge-ade/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
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
					next.ServeHTTP(w, r)
				})
			},
		},
	})

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "ForgeADE",
		Width:          1280,
		Height:         800,
		MinWidth:       800,
		MinHeight:      600,
		BackgroundType: application.BackgroundTypeSolid,
		EnableFileDrop: true,
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarHidden,
			Appearance: application.NSAppearanceNameDarkAqua,
		},
	})
	a.SetMainWindow(mainWindow)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
