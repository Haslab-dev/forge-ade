package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// When launched from "Open New Window" (or the CLI), open the passed
	// folder or .workspace file instead of the previously persisted workspace.
	if len(os.Args) > 1 && os.Args[1] != "" {
		argPath := os.Args[1]
		if resolved, err := ResolvePath(argPath); err == nil {
			if info, err := os.Stat(resolved); err == nil {
				if info.IsDir() {
					if _, err := app.OpenFolder(resolved); err != nil {
						log.Printf("new window: open folder %s: %v", resolved, err)
					}
				} else if _, err := app.OpenWorkspace(resolved); err != nil {
					log.Printf("new window: open workspace %s: %v", resolved, err)
				}
			}
		}
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "ForgeADE",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			app.Shutdown()
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			BackdropType:         windows.Mica,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarDefault(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "ForgeADE",
				Message: "Native AI Development Workspace",
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
