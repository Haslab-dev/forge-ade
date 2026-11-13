package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	nativeui "github.com/hasdev/forge-ade/app/native/pkg/ui"
)

func main() {
	// Tune Go runtime for interactive desktop app
	debug.SetGCPercent(80)
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(512 * 1024 * 1024)
	}

	// Resolve target workspace directory. Bundles launched via Finder /
	// `open` start with CWD=/ — that resolves to NO workspace: the app opens
	// in an empty editor mode and the user picks a folder (recents offered).
	// Running the binary from a project directory keeps that directory, and
	// an explicit path argument always wins.
	targetDir, err := os.Getwd()
	if err != nil || targetDir == "/" {
		targetDir = ""
	}
	if len(os.Args) > 1 && os.Args[1] != "" {
		if abs, err := filepath.Abs(os.Args[1]); err == nil {
			targetDir = abs
		}
	}

	log.Printf("Starting ForgeADE Native for workspace: %s", targetDir)

	// Initialize native CGO-free domain engine (RakitKode + Git + DBViewer)
	eng := engine.NewEngine("")

	// Launch MyGo GPU Application
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleFileMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu},
			{Role: mygo.RoleWindowMenu},
		}))

		appUI := nativeui.NewAppUI(eng, targetDir)

		win := mygo.NewWindow(mygo.WindowOptions{
			Title:          "ForgeADE Native",
			TitleBarStyle:  mygo.TitleBarHiddenInset,
			TitleBarHeight: int(nativeui.WorkspaceTopBarHeight),
			Width:          1280,
			Height:         800,
			MinWidth:       760,
			MinHeight:      500,
			Content:        ui.View(appUI.View),
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
