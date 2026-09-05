package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
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

	wailsApp := application.New(application.Options{
		Name:        "ForgeADE",
		Description: "Native AI Development Workspace",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "ForgeADE",
		Width:          1280,
		Height:         800,
		MinWidth:       800,
		MinHeight:      600,
		BackgroundType: application.BackgroundTypeSolid,
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarDefault,
			Appearance: application.NSAppearanceNameDarkAqua,
		},
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
