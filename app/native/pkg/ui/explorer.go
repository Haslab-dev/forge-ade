package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// Shared project explorer: the hierarchical tree, its context menus, and the
// right-drawer Files pane. The agent sidebar's Files view and the drawer pane
// render the same tree, mirroring forge-ade's SidebarFileExplorer.

// renderExplorerTree renders the hierarchical workspace tree with context
// menus, shared by the sidebar Files view and the drawer Files pane.
func (a *AppUI) renderExplorerTree(c *ui.Context, tc theme.ThemeColors) {
	// Lazy per-directory loading (webview GetDirectoryTree parity): when an
	// expanded folder was never listed, read it live before building the
	// tree so expanding always reveals real children.
	var ensureLoaded func(nodes []*FileTreeNode)
	if a.Workspace.FilterQuery == "" {
		ensureLoaded = func(nodes []*FileTreeNode) {
			for _, n := range nodes {
				if !n.IsDir {
					continue
				}
				if a.isFolderExpanded(n.RelativePath, 0) || a.isFolderExpanded(n.RelativePath, 1) {
					if a.Workspace.NeedsLoad(n.RelativePath) {
						a.Workspace.LoadDir(n.RelativePath)
					}
				}
				ensureLoaded(n.Children)
			}
		}
	} else {
		ensureLoaded = func(nodes []*FileTreeNode) {}
	}
	filtered := a.Workspace.FilteredFiles()
	treeNodes := buildFileTree(filtered, a.Workspace.RootDir)
	for pass := 0; pass < 3; pass++ {
		before := len(filtered)
		ensureLoaded(treeNodes)
		filtered = a.Workspace.FilteredFiles()
		treeNodes = buildFileTree(filtered, a.Workspace.RootDir)
		if len(filtered) == before {
			break
		}
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(2, 6).Gap(0).Children(func() {
			// No workspace opened yet — invite the user to open a folder.
			if a.Workspace.RootDir == "" {
				ui.Column(c).Padding(16, 12).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "No folder opened").FontSize(12).TextColor(tc.TextMuted)
					openBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 12).Gap(6).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					openBtn.Children(func() {
						ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.Accent)
						ui.Text(c, "Open Folder…").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if openBtn.Clicked() {
						a.openWorkspaceFolder()
					}
				})
				return
			}
			var renderNode func(node *FileTreeNode, depth int)
			renderNode = func(node *FileTreeNode, depth int) {
				indent := float32(4 + depth*14)

				if node.IsDir {
					isOpen := a.isFolderExpanded(node.RelativePath, depth)
					row := ui.Row(c.Key("fdir-"+node.RelativePath)).Height(FileTreeRowHeight).Radius(4).
						Padding(0, 6, 0, indent).Gap(4).AlignItems(ui.Center)
					row.Children(func() {
						if isOpen {
							ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.IconMuted)
							ui.Icon(c, icons.TablerFolderOpen).Width(14).Height(14).TextColor(tc.Accent)
						} else {
							ui.Icon(c, icons.TablerChevronRight).Width(11).Height(11).TextColor(tc.IconMuted)
							ui.Icon(c, icons.TablerFolder).Width(14).Height(14).TextColor(tc.IconMuted)
						}
						ui.Text(c, node.Name).FontSize(12).FontWeight(500).Grow(1).SingleLine()
					})
					if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() {
						a.ExpandedFileTreeFolders[node.RelativePath] = !isOpen
					}
					a.attachExplorerMenu(c, row, node, tc)

					if isOpen {
						for _, child := range node.Children {
							renderNode(child, depth+1)
						}
					}
				} else {
					// File node
					selected := (a.SelectedFile == node.RelativePath)
					dirty := false
					if doc := a.Editor.GetDoc(node.FullPath); doc != nil && doc.Editor.Buffer.Modified() {
						dirty = true
					}

					row := ui.Row(c.Key("ffile-"+node.RelativePath)).Height(FileTreeRowHeight).Radius(4).
						Padding(0, 6, 0, indent+15).Gap(6).AlignItems(ui.Center)
					row.Children(func() {
						ui.Icon(c, icons.FileIconSVG(node.RelativePath)).Width(14).Height(14).TextColor(tc.IconMuted)
						ui.Text(c, node.Name).FontSize(12).Grow(1).SingleLine()
						if dirty {
							ui.Box(c).Size(6, 6).Radius(3).Background(tc.Warning)
						}
					})
					if selected {
						row.Background(tc.Accent.Alpha(0.12))
					} else if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() {
						if a.LeftSidebarView == "files" {
							// The agent sidebar's explorer previews files in the
							// right sidebar, as forge-ade's side preview does.
							a.openSidePreviewFile(c, node.RelativePath)
						} else {
							a.openExplorerNode(node)
						}
					}
					a.attachExplorerMenu(c, row, node, tc)
				}
			}

			if len(treeNodes) == 0 {
				if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
					ui.Column(c).Padding(16, 12).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "No remote files found").FontSize(11).TextColor(tc.TextMuted)
						refreshBtn := ui.Row(c).Height(26).Radius(5).Padding(0, 10).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
							AlignItems(ui.Center).Justify(ui.Center).Gap(6)
						refreshBtn.Children(func() {
							ui.Icon(c, icons.TablerRefresh).Width(12).Height(12).TextColor(tc.Accent)
							ui.Text(c, "Refresh Remote Explorer").FontSize(10).FontWeight(500).TextColor(tc.Text)
						})
						if refreshBtn.Clicked() {
							a.Workspace.Refresh()
						}
					})
				} else {
					ui.Column(c).Padding(16, 12).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "No files matching filter").FontSize(11).TextColor(tc.TextMuted)
						connectSSHBtn := ui.Row(c).Height(26).Radius(5).Padding(0, 10).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
							AlignItems(ui.Center).Justify(ui.Center).Gap(6)
						connectSSHBtn.Children(func() {
							ui.Icon(c, icons.TablerServer).Width(12).Height(12).TextColor(tc.Accent)
							ui.Text(c, "Connect via SSH Remote").FontSize(10).FontWeight(500).TextColor(tc.Text)
						})
						if connectSSHBtn.Clicked() {
							a.OpenSSHModal()
						}
					})
				}
			} else {
				for _, rootNode := range treeNodes {
					renderNode(rootNode, 0)
				}
			}
		})
	})
}

// openExplorerNode opens a file from the explorer: database files inspect in
// the DB pane, everything else opens in the editor, as forge-ade does.
func (a *AppUI) openExplorerNode(node *FileTreeNode) {
	a.SelectedFile = node.RelativePath
	ext := filepath.Ext(node.RelativePath)
	if ext == ".sqlite" || ext == ".db" || ext == ".duckdb" {
		if a.Database != nil {
			a.Database.Inspect(node.FullPath)
			a.EditorActivity = ActivityDatabase
		}
		return
	}
	_, _ = a.Editor.OpenFile(node.FullPath, node.RelativePath)
}

// attachExplorerMenu gives an explorer row forge-ade's file context menu:
// new file/folder, open, rename, delete, clipboard paths, reveal, terminal.
func (a *AppUI) attachExplorerMenu(c *ui.Context, row ui.Element, node *FileTreeNode, tc theme.ThemeColors) {
	row.ContextMenu(func(m *ui.Menu) {
		if !node.IsDir {
			if a.LeftSidebarView == "files" {
				if m.Item("Open in Side Preview").Chosen() {
					a.openSidePreviewFile(c, node.RelativePath)
				}
				if m.Item("Open in Editor").Chosen() {
					_, _ = a.Editor.OpenFile(node.FullPath, node.RelativePath)
					a.Mode = ModeEditor
				}
				m.Separator()
			} else {
				if m.Item("Open").Chosen() {
					a.openExplorerNode(node)
				}
				if m.Item("Open in Editor").Chosen() {
					_, _ = a.Editor.OpenFile(node.FullPath, node.RelativePath)
					a.Mode = ModeEditor
				}
				m.Separator()
			}
		}
		if m.Item("New File...").Chosen() {
			a.ShowNewFileModal = true
			if node.IsDir {
				a.NewFileName = node.RelativePath + "/"
			} else {
				a.NewFileName = filepath.Dir(node.RelativePath) + "/"
				if a.NewFileName == "./" || a.NewFileName == "/." {
					a.NewFileName = ""
				}
			}
		}
		if m.Item("New Folder...").Chosen() {
			a.ShowNewFolderModal = true
			if node.IsDir {
				a.NewFolderName = node.RelativePath + "/"
			} else {
				a.NewFolderName = filepath.Dir(node.RelativePath) + "/"
				if a.NewFolderName == "./" || a.NewFolderName == "/." {
					a.NewFolderName = ""
				}
			}
		}
		m.Separator()
		if m.Item("Copy Path").Chosen() {
			mygo.Clipboard.WriteText(node.FullPath)
			c.AddToast(ui.Toast{Title: "Path Copied", Description: node.FullPath, Type: "success"})
		}
		if m.Item("Copy Relative Path").Chosen() {
			mygo.Clipboard.WriteText(node.RelativePath)
			c.AddToast(ui.Toast{Title: "Relative Path Copied", Description: node.RelativePath, Type: "success"})
		}
		if !strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
			if m.Item("Reveal in Finder").Chosen() {
				mygo.Shell.ShowItemInFolder(node.FullPath)
			}
			if m.Item("Open in Terminal").Chosen() {
				dir := node.FullPath
				if !node.IsDir {
					dir = filepath.Dir(dir)
				}
				a.ShowBottomTerminal = true
				a.Terminal.Cwd = dir
				_ = a.Terminal.Restart()
			}
			m.Separator()
		}
		if m.Item("Rename...").Chosen() {
			a.ShowRenameFileModal = true
			a.RenameFileOldPath = node.RelativePath
			a.RenameFilePath = node.RelativePath
		}
		if m.Item("Delete").Danger().Chosen() {
			a.deleteExplorerNode(c, node)
		}
	})
}

// deleteExplorerNode deletes a file or folder (to the system trash locally,
// over SFTP remotely) and refreshes the workspace.
func (a *AppUI) deleteExplorerNode(c *ui.Context, node *FileTreeNode) {
	rel := node.RelativePath
	if !node.IsDir && rel == a.SelectedFile {
		if doc := a.Editor.GetDoc(node.FullPath); doc != nil {
			a.Editor.CloseTab(node.FullPath)
		}
	}
	go func() {
		if err := a.Workspace.DeleteFile(rel); err != nil {
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return
		}
		if a.Git != nil {
			a.Git.Refresh()
		}
		if a.Window != nil {
			a.Window.Invalidate()
		}
	}()
	c.AddToast(ui.Toast{Title: "Deleted", Description: rel, Type: "info"})
}

// renderRenameFileModal renders the file/folder rename dialog.
func (a *AppUI) renderRenameFileModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ui.Modal(c, &a.ShowRenameFileModal, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
			Children(func() {
				ui.Text(c, "Rename").FontSize(14).FontWeight(600)
				ui.Text(c, a.RenameFileOldPath).FontSize(11).TextColor(tc.TextMuted).SingleLine()
				in := ui.TextInput(c, &a.RenameFilePath).Grow(1).AutoFocus()
				save := func() {
					if a.RenameFilePath == "" || a.RenameFilePath == a.RenameFileOldPath {
						a.ShowRenameFileModal = false
						return
					}
					oldDoc := a.Editor.GetDoc(a.Workspace.ResolveFullPath(a.RenameFileOldPath))
					if err := a.Workspace.RenameFile(a.RenameFileOldPath, a.RenameFilePath); err != nil {
						c.AddToast(ui.Toast{Title: "Rename Failed", Description: err.Error(), Type: "error"})
						a.ShowRenameFileModal = false
						return
					}
					// Keep the editor tab following the file, as forge-ade does.
					if oldDoc != nil {
						fullPath := a.Workspace.ResolveFullPath(a.RenameFilePath)
						_, _ = a.Editor.OpenFile(fullPath, a.RenameFilePath)
						a.Editor.CloseTab(a.Workspace.ResolveFullPath(a.RenameFileOldPath))
						a.SelectedFile = a.RenameFilePath
					}
					if a.Git != nil {
						go a.Git.Refresh()
					}
					a.ShowRenameFileModal = false
				}
				if in.Submitted() {
					save()
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowRenameFileModal = false
					}
					if ui.PrimaryButton(c, "Rename").Clicked() {
						save()
					}
				})
			})
	})
}

// renderFilesPane is the right drawer's project explorer tab: browse the
// project, open files into the editor, and act on them via context menus,
// following forge-ade's SidebarFileExplorer interaction model.
func (a *AppUI) renderFilesPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Row(c).Height(40).Padding(0, 12).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Files").FontSize(12).FontWeight(600)
		if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
			Badge(c, tc, "REMOTE", tc.Accent)
		}
		ui.Box(c).Grow(1)
		if IconButton(c, tc, icons.TablerFilePlus, 12).Clicked() {
			a.ShowNewFileModal = true
			a.NewFileName = ""
		}
		if IconButton(c, tc, icons.TablerFolderPlus, 12).Clicked() {
			a.ShowNewFolderModal = true
			a.NewFolderName = ""
		}
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() {
			a.Workspace.Refresh()
			if a.Git != nil {
				go a.Git.Refresh()
			}
		}
		if IconButton(c, tc, icons.TablerChevronDown, 12).Clicked() {
			for k := range a.ExpandedFileTreeFolders {
				a.ExpandedFileTreeFolders[k] = false
			}
		}
	})

	// Filter, as the sidebar explorer has
	ui.Row(c).Padding(4, 10).Children(func() {
		ui.TextInput(c, &a.Workspace.FilterQuery).Placeholder("Filter files...").Grow(1)
	})

	a.renderExplorerTree(c, tc)

	// Opened-files quick list, mirroring forge-ade's side file tabs
	if len(a.Editor.OpenTabPaths) > 0 {
		ui.Row(c).Height(32).Padding(0, 12).Gap(6).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, "OPEN FILES").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
			ui.Box(c).Grow(1)
			ui.Text(c, fmt.Sprintf("%d", len(a.Editor.OpenTabPaths))).FontSize(10).TextColor(tc.TextMuted)
		})
		ui.Scroll(c).Height(minF(120, float32(len(a.Editor.OpenTabPaths))*26+8)).Children(func() {
			ui.Column(c).Padding(2, 8).Gap(1).Children(func() {
				for _, p := range a.Editor.OpenTabPaths {
					path := p
					doc := a.Editor.GetDoc(path)
					if doc == nil {
						continue
					}
					active := a.Editor.ActivePath == path
					row := ui.Row(c.Key("drawer-open-"+path)).Height(26).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
					row.Children(func() {
						ui.Icon(c, icons.FileIconSVG(doc.RelativePath)).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.Text(c, filepath.Base(doc.RelativePath)).FontSize(11).Grow(1).SingleLine()
						if doc.Editor.Buffer.Modified() {
							ui.Box(c).Size(6, 6).Radius(3).Background(tc.Warning)
						}
						closeBtn := ui.Row(c).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
						closeBtn.Children(func() {
							ui.Icon(c, icons.TablerX).Width(10).Height(10).TextColor(tc.IconMuted)
						})
						if closeBtn.Hovered() {
							closeBtn.Background(tc.Text.Alpha(0.06))
						}
						if closeBtn.Clicked() {
							a.Editor.CloseTab(path)
						}
					})
					if active {
						row.Background(tc.Accent.Alpha(0.12))
					} else if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() {
						a.Editor.ActivePath = path
						a.SelectedFile = doc.RelativePath
					}
				}
			})
		})
	}
}
