package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/internal/bootstrap"
	nativeui "github.com/hasdev/forge-ade/forge-ade-native/pkg/ui"
)

func main() {
	// Tune Go runtime for interactive desktop app
	debug.SetGCPercent(80)
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(512 * 1024 * 1024)
	}

	// Resolve target workspace directory
	targetDir, err := os.Getwd()
	if err != nil {
		targetDir = "."
	}
	if len(os.Args) > 1 && os.Args[1] != "" {
		if abs, err := filepath.Abs(os.Args[1]); err == nil {
			targetDir = abs
		}
	}

	log.Printf("Starting ForgeADE Native for workspace: %s", targetDir)

	// Initialize ForgeADE core domain engine (RakitKode + subsystems)
	backendApp, err := bootstrap.Boot(bootstrap.BootOptions{})
	if err != nil {
		log.Printf("Warning: failed to boot ForgeADE backend engine: %v. Running in standalone native mode.", err)
	} else {
		// Open the workspace folder in backend
		_, _ = backendApp.OpenFolder(targetDir)
	}

	// Launch MyGo GPU Application
	mygo.App.WhenReady(func() {
		appUI := nativeui.NewAppUI(backendApp, targetDir)

		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "ForgeADE Native",
			Width:     1280,
			Height:    800,
			MinWidth:  760,
			MinHeight: 500,
			Content:   ui.View(appUI.View),
		})

		appUI.SetWindow(win)

		win.OnClosed(func() {
			if appUI.Terminal != nil {
				appUI.Terminal.Close()
			}
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatalf("MyGo runtime error: %v", err)
	}
}
