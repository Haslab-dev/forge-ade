package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/hasdev/forge-ade/app/native/pkg/icons"
)

// isDeckOrMotionFile returns true if the filename matches an Open Design
// presentation or HyperFrames motion composition.
func isDeckOrMotionFile(name string) bool {
	lower := strings.ToLower(name)
	base := filepath.Base(lower)
	return base == "deck.html" ||
		base == "motion.html" ||
		strings.HasSuffix(lower, ".deck.html") ||
		strings.HasSuffix(lower, ".motion.html") ||
		strings.HasSuffix(lower, ".mp4") ||
		strings.HasSuffix(lower, ".pptx")
}

// scanWorkspaceDecks searches the active workspace for presentations and motions.
func (a *AppUI) scanWorkspaceDecks() []string {
	if a.Workspace.RootDir == "" {
		return nil
	}
	var results []string
	root := a.Workspace.RootDir
	// Check common root targets first
	for _, target := range []string{"deck.html", "motion.html", "motion.mp4", "presentation.pptx"} {
		p := filepath.Join(root, target)
		if _, err := os.Stat(p); err == nil {
			results = append(results, p)
		}
	}

	// Shallow scan subdirectories (max 2 levels)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(root, path)
			if strings.Count(rel, string(filepath.Separator)) > 2 {
				return filepath.SkipDir
			}
			return nil
		}
		if isDeckOrMotionFile(info.Name()) {
			dup := false
			for _, r := range results {
				if r == path {
					dup = true
					break
				}
			}
			if !dup {
				results = append(results, path)
			}
		}
		return nil
	})

	return results
}

// openDeckViewer initializes or reloads the Deck & Motion WebView.
func (a *AppUI) openDeckViewer(fullPath string) {
	if fullPath == "" {
		decks := a.scanWorkspaceDecks()
		if len(decks) > 0 {
			fullPath = decks[0]
		} else if a.Workspace.RootDir != "" {
			fullPath = filepath.Join(a.Workspace.RootDir, "deck.html")
		}
	}

	lower := strings.ToLower(fullPath)
	if strings.Contains(lower, "motion") || strings.HasSuffix(lower, ".mp4") {
		a.DeckMode = "motion"
	} else {
		a.DeckMode = "deck"
	}

	if a.DeckWebView != nil && !a.DeckWebView.IsDestroyed() {
		if a.DeckCurrentPath == fullPath {
			return
		}
		a.DeckWebView.Destroy()
		a.DeckWebView = nil
	}

	a.DeckCurrentPath = fullPath
	if a.Window != nil && mygo.WebViewAvailable() && fullPath != "" {
		if _, err := os.Stat(fullPath); err == nil {
			wv, err := a.Window.NewWebView(mygo.WebViewOptions{
				URL: "file://" + fullPath,
			})
			if err == nil {
				a.DeckWebView = wv
			}
		}
	}
}

// closeDeckViewer tears down the active deck webview.
func (a *AppUI) closeDeckViewer() {
	if a.DeckWebView != nil && !a.DeckWebView.IsDestroyed() {
		a.DeckWebView.Destroy()
	}
	a.DeckWebView = nil
	a.DeckCurrentPath = ""
}

// renderDeckPane renders the full Presentation & Motion Deck viewer tab.
func (a *AppUI) renderDeckPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	if a.DeckFiles == nil || len(a.DeckFiles) == 0 {
		a.DeckFiles = a.scanWorkspaceDecks()
	}

	if a.DeckCurrentPath == "" && len(a.DeckFiles) > 0 {
		a.openDeckViewer(a.DeckFiles[0])
	} else if a.DeckWebView == nil && a.DeckCurrentPath != "" {
		a.openDeckViewer(a.DeckCurrentPath)
	}

	isMotion := a.DeckMode == "motion" || strings.Contains(strings.ToLower(a.DeckCurrentPath), "motion") || strings.HasSuffix(strings.ToLower(a.DeckCurrentPath), ".mp4")

	ui.Column(c).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
		// Toolbar
		ui.Row(c).Height(38).Padding(0, 10).Gap(6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {

			tabIcon := icons.TablerPresentation
			modeLabel := "DECK"
			if isMotion {
				tabIcon = icons.TablerMovie
				modeLabel = "MOTION"
			}
			ui.Icon(c, tabIcon).Width(14).Height(14).TextColor(tc.Accent)
			ui.Text(c, "Presentation & Motion").FontSize(12).FontWeight(600)

			// Mode badge
			ui.Row(c).Radius(4).Padding(1, 5).Background(tc.Accent.Alpha(0.15)).Children(func() {
				ui.Text(c, modeLabel).FontSize(9).FontWeight(700).TextColor(tc.Accent)
			})

			// File switcher pill if multiple presentations exist
			if len(a.DeckFiles) > 1 {
				currentLeaf := filepath.Base(a.DeckCurrentPath)
				filePill := ui.Row(c.Key("deck-file-picker")).Height(22).Radius(5).Padding(0, 7).Gap(4).
					AlignItems(ui.Center).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
				filePill.Children(func() {
					ui.Text(c, currentLeaf).FontSize(11).FontWeight(500).SingleLine()
					ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.IconMuted)
				})
				filePill.ContextMenu(func(m *ui.Menu) {
					for _, f := range a.DeckFiles {
						path := f
						leaf := filepath.Base(path)
						if m.Item(leaf).Chosen() {
							a.openDeckViewer(path)
						}
					}
				})
				if filePill.Clicked() {
					// Cycle next presentation
					for i, f := range a.DeckFiles {
						if f == a.DeckCurrentPath {
							next := a.DeckFiles[(i+1)%len(a.DeckFiles)]
							a.openDeckViewer(next)
							break
						}
					}
				}
			}

			ui.Box(c).Grow(1)

			// Slide navigation controls (for deck mode)
			if !isMotion {
				prevBtn := IconButton(c, tc, icons.TablerArrowLeft, 12)
				if prevBtn.Clicked() {
					if a.DeckWebView != nil && !a.DeckWebView.IsDestroyed() {
						_, _ = a.DeckWebView.Page().Eval("window.postMessage({type: 'od:slide', action: 'prev'}, '*');")
					}
					if a.DeckActiveSlide > 0 {
						a.DeckActiveSlide--
					}
				}

				slideLabel := fmt.Sprintf("Slide %02d", a.DeckActiveSlide+1)
				ui.Text(c, slideLabel).FontSize(11).FontWeight(600).TextColor(tc.TextMuted)

				nextBtn := IconButton(c, tc, icons.TablerArrowRight, 12)
				if nextBtn.Clicked() {
					if a.DeckWebView != nil && !a.DeckWebView.IsDestroyed() {
						_, _ = a.DeckWebView.Page().Eval("window.postMessage({type: 'od:slide', action: 'next'}, '*');")
					}
					a.DeckActiveSlide++
				}
			} else {
				// Motion controls
				playIcon := icons.TablerPlayerPlay
				if a.DeckIsPlaying {
					playIcon = icons.TablerPlayerPause
				}
				playBtn := IconButton(c, tc, playIcon, 12)
				if playBtn.Clicked() {
					a.DeckIsPlaying = !a.DeckIsPlaying
					if a.DeckWebView != nil && !a.DeckWebView.IsDestroyed() {
						if a.DeckIsPlaying {
							_, _ = a.DeckWebView.Page().Eval("window.postMessage({type: 'od:motion-control', action: 'play'}, '*');")
						} else {
							_, _ = a.DeckWebView.Page().Eval("window.postMessage({type: 'od:motion-control', action: 'pause'}, '*');")
						}
					}
				}

				restartBtn := IconButton(c, tc, icons.TablerRotate, 12)
				if restartBtn.Clicked() {
					if a.DeckWebView != nil && !a.DeckWebView.IsDestroyed() {
						_, _ = a.DeckWebView.Page().Eval("window.postMessage({type: 'od:motion-control', action: 'restart'}, '*');")
					}
				}
			}

			// Reload
			reloadBtn := IconButton(c, tc, icons.TablerRefresh, 12)
			if reloadBtn.Clicked() {
				if a.DeckWebView != nil && !a.DeckWebView.IsDestroyed() {
					a.DeckWebView.Page().Reload()
				} else {
					a.openDeckViewer(a.DeckCurrentPath)
				}
			}

			// Open in Default App / External Browser
			openExtBtn := IconButton(c, tc, icons.TablerExternalLink, 12)
			if openExtBtn.Clicked() && a.DeckCurrentPath != "" {
				mygo.Shell.OpenPath(a.DeckCurrentPath)
			}

			// Reveal in Finder
			finderBtn := IconButton(c, tc, icons.TablerFolderOpen, 12)
			if finderBtn.Clicked() && a.DeckCurrentPath != "" {
				mygo.Shell.ShowItemInFolder(a.DeckCurrentPath)
			}

			// Close
			closeBtn := IconButton(c, tc, icons.TablerX, 12)
			if closeBtn.Clicked() {
				a.closeDeckViewer()
				a.RightDrawerTab = RightTabChanges
			}
		})

		// Body: Native WebKit preview or Fallback State
		if a.DeckWebView != nil && !a.DeckWebView.IsDestroyed() {
			ui.WebView(c, a.DeckWebView).Grow(1).FillWidth()
		} else {
			// Empty / Placeholder state
			ui.Column(c).Grow(1).Center().Padding(24).Gap(12).Children(func() {
				ui.Box(c).Size(48, 48).Radius(24).Background(tc.Accent.Alpha(0.12)).
					Center().Children(func() {
					ui.Icon(c, icons.TablerPresentation).Width(24).Height(24).TextColor(tc.Accent)
				})

				ui.Text(c, "No Presentation or Motion File Found").FontSize(14).FontWeight(600)
				ui.Text(c, "Create a deck.html or motion.html file in your project or ask your agent to scaffold an Open Design deck.").
					FontSize(11).TextColor(tc.TextMuted).Center()

				scanBtn := ui.Row(c.Key("scan-deck-btn")).Height(28).Radius(6).Padding(0, 12).Gap(6).
					AlignItems(ui.Center).Background(tc.Accent)
				scanBtn.Children(func() {
					ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.AccentForeground)
					ui.Text(c, "Scan Workspace for Decks").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
				})
				if scanBtn.Clicked() {
					a.DeckFiles = a.scanWorkspaceDecks()
					if len(a.DeckFiles) > 0 {
						a.openDeckViewer(a.DeckFiles[0])
					} else {
						c.AddToast(ui.Toast{
							Title:       "Scan Complete",
							Description: "No deck.html, motion.html, or .mp4 files found in workspace.",
							Type:        "info",
						})
					}
				}
			})
		}
	})
}
