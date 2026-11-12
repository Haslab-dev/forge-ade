package ui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/agent"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/workspace"
)

// renderAgentSidebar renders the full-height task sessions sidebar with filter tabs and profile row.
func (a *AppUI) renderAgentSidebar(c *ui.Context) {
	t := c.Theme()
	tc := a.ThemeMgr.Colors()
	bar := c.TitleBar()
	topPadLeft := float32(14)
	if bar.Left > 0 {
		topPadLeft = max(bar.Left+8, 76)
	} else if runtime.GOOS == "darwin" {
		topPadLeft = 76
	}

	ui.Column(c).Grow(1).FillHeight().Background(tc.Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).Children(func() {
		// Top window controls strip (48px, macOS traffic-light inset) drags
		// the window, its collapse button stays clickable.
		ui.Row(c).Height(48).Padding(0, 12, 0, topPadLeft).AlignItems(ui.Center).DragWindow().Children(func() {
			if IconButton(c, tc, icons.TablerPanelLeft, 15).Clicked() {
				a.ShowLeftSidebar = false
			}
			ui.Row(c).Padding(0, 6).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.ForgeSparkSVG).Width(15).Height(15).TextColor(tc.Accent)
				ui.Text(c, "ForgeADE").FontSize(13).FontWeight(600).LetterSpacing(-0.2)
			})
			ui.Box(c).Grow(1)
		})

		// Primary Action Buttons: + New Task (⌘N), Search (⌘P)
		ui.Column(c).Padding(6, 10).Gap(2).Children(func() {
			isNewTaskActive := (a.Mode == ModeAgent && a.ActiveTaskKind == "agent" && (a.Agent == nil || a.Agent.ActiveSessionID == ""))
			newTask := SidebarActionButton(c, tc, icons.TablerPlus, "New Task", "⌘N", isNewTaskActive)
			if newTask.Clicked() {
				a.NewAgentTask("")
			}
			search := SidebarActionButton(c, tc, icons.TablerSearch, "Search", "⌘P", false)
			if search.Clicked() {
				a.Palette.Toggle()
			}
		})

		// Sidebar View Switcher: Tasks | Files
		ui.Row(c).Gap(4).Padding(2, 10).Children(func() {
			pill := func(label string, icon *ui.SVG, active bool, onPick func()) {
				btn := ui.Row(c).Height(24).Radius(999).Padding(0, 10).Gap(5).AlignItems(ui.Center)
				btn.Children(func() {
					ui.Icon(c, icon).Width(12).Height(12).TextColor(tc.SidebarForeground)
					ui.Text(c, label).FontSize(12)
				})
				if active {
					btn.Background(tc.Text.Alpha(0.15))
				} else if btn.Hovered() {
					btn.Background(tc.SidebarRowHover)
				}
				if btn.Clicked() {
					onPick()
				}
			}
			pill("Tasks", icons.TablerChecklist, a.AgentSidebarView == AgentSidebarTasks, func() { a.AgentSidebarView = AgentSidebarTasks })
			pill("Files", icons.TablerFiles, a.AgentSidebarView == AgentSidebarFiles, func() { a.AgentSidebarView = AgentSidebarFiles })
		})

		if a.AgentSidebarView == AgentSidebarFiles {
			// Embedded Project File Explorer in Agent Sidebar
			ui.Column(c).Grow(1).Children(func() {
				a.renderFileTree(c)
			})
		} else {
			// Section header: PROJECTS
			ui.Row(c).Padding(10, 14, 4, 14).AlignItems(ui.Center).Children(func() {
				SectionLabel(c, tc, "PROJECTS")
				ui.Box(c).Grow(1)
				if IconButton(c, tc, icons.TablerPlus, 12).Clicked() {
					a.NewAgentTask("")
				}
			})

			// Search Input Filter
			ui.Row(c).Padding(2, 10, 4, 10).Children(func() {
				ui.TextInput(c, &a.Agent.SearchQuery).Placeholder("Search tasks...").Grow(1)
			})

			// Filter Tabs: Active, Pinned, Snoozed, Archived with counts
			activeCount, pinnedCount, snoozedCount, archivedCount := a.Agent.TabCounts()
			tabCounts := map[string]int{
				"active":   activeCount,
				"pinned":   pinnedCount,
				"snoozed":  snoozedCount,
				"archived": archivedCount,
			}
			ui.Row(c).Gap(4).Padding(4, 10).AlignItems(ui.Center).Children(func() {
				capitalize := func(s string) string {
					if s == "" {
						return s
					}
					return strings.ToUpper(s[:1]) + s[1:]
				}
				for _, tab := range []string{"active", "pinned", "snoozed", "archived"} {
					thisTab := tab
					active := a.Agent.ActiveFilterTab == thisTab
					pill := ui.Row(c).Radius(999).Padding(2, 8).AlignItems(ui.Center).Gap(4)
					pill.Children(func() {
						ui.Text(c, capitalize(thisTab)).FontSize(12)
						if n := tabCounts[thisTab]; n > 0 {
							ui.Text(c, fmt.Sprintf("%d", n)).FontSize(10).TextColor(tc.SidebarMutedForeground)
						}
					})
					if active {
						pill.Background(tc.Text.Alpha(0.15))
					} else if pill.Hovered() {
						pill.Background(tc.SidebarRowHover)
					}
					if pill.Clicked() {
						a.Agent.ActiveFilterTab = thisTab
					}
				}
			})

			// Scrollable Task Sessions List grouped by Project/Folder
			type projectTaskGroup struct {
				ProjectName string
				FolderPath  string
				Sessions    []*agent.AgentSession
				CLISessions []*agent.AgentCLISession
			}

			activeWsPath := ""
			if a.Workspace != nil {
				activeWsPath = a.Workspace.RootDir
			}
			activeProjectName := resolveProjectName(activeWsPath)

			groupMap := make(map[string]*projectTaskGroup)
			var groupOrder []string

			getOrCreateGroup := func(name, path string) *projectTaskGroup {
				if g, exists := groupMap[name]; exists {
					if g.FolderPath == "" && path != "" {
						g.FolderPath = path
					}
					return g
				}
				g := &projectTaskGroup{
					ProjectName: name,
					FolderPath:  path,
				}
				groupMap[name] = g
				groupOrder = append(groupOrder, name)
				return g
			}

			// Always seed active workspace first, then General
			getOrCreateGroup(activeProjectName, activeWsPath)
			if activeProjectName != "General" {
				getOrCreateGroup("General", "")
			}

			// 1. Group ACP Sessions
			sessions := a.Agent.FilteredSessions()
			for _, sess := range sessions {
				s := sess
				pName := resolveProjectName(s.WorkspacePath)
				if s.WorkspacePath == activeWsPath || (activeWsPath != "" && strings.HasPrefix(s.WorkspacePath, activeWsPath)) {
					pName = activeProjectName
				}
				g := getOrCreateGroup(pName, s.WorkspacePath)
				g.Sessions = append(g.Sessions, s)
			}

			// 2. Group CLI Sessions
			if a.CLIMgr != nil {
				for _, cliSess := range a.CLIMgr.Sessions {
					cs := cliSess
					if a.Agent.SearchQuery != "" {
						sq := strings.ToLower(a.Agent.SearchQuery)
						if !strings.Contains(strings.ToLower(cs.Title), sq) && !strings.Contains(strings.ToLower(cs.AgentName), sq) {
							continue
						}
					}
					if a.Agent.ActiveFilterTab == "archived" {
						continue
					}
					pName := resolveProjectName(cs.Cwd)
					if cs.Cwd == activeWsPath || (activeWsPath != "" && strings.HasPrefix(cs.Cwd, activeWsPath)) {
						pName = activeProjectName
					}
					g := getOrCreateGroup(pName, cs.Cwd)
					g.CLISessions = append(g.CLISessions, cs)
				}
			}

			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(2, 6).Gap(4).Children(func() {
					for _, name := range groupOrder {
						group := groupMap[name]
						totalCount := len(group.Sessions) + len(group.CLISessions)

						if a.Agent.SearchQuery != "" && totalCount == 0 {
							continue
						}

						isCollapsed := a.CollapsedProjectGroups[group.ProjectName]

						// Project Group Header
						pHeader := ui.Row(c.Key("pheader-"+group.ProjectName)).Height(28).Radius(6).
							Padding(0, 6).Gap(5).AlignItems(ui.Center)
						pHeader.Children(func() {
							if isCollapsed {
								ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Icon(c, icons.TablerFolder).Width(14).Height(14).TextColor(tc.IconMuted)
							} else {
								ui.Icon(c, icons.TablerChevronDown).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Icon(c, icons.TablerFolderOpen).Width(14).Height(14).TextColor(tc.Accent)
							}

							ui.Text(c, group.ProjectName).FontSize(12).FontWeight(600).Grow(1).SingleLine()

							if totalCount > 0 {
								ui.Text(c, fmt.Sprintf("%d", totalCount)).FontSize(10).TextColor(tc.SidebarMutedForeground)
							}

							newBtn := ui.Row(c.Key("pnew-"+group.ProjectName)).Size(18, 18).Radius(4).
								AlignItems(ui.Center).Justify(ui.Center)
							newBtn.Children(func() {
								ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.IconMuted)
							})
							if newBtn.Hovered() {
								newBtn.Background(tc.Text.Alpha(0.08))
							}
							if newBtn.Clicked() {
								a.NewAgentTask(group.FolderPath)
							}
						})

						if pHeader.Hovered() {
							pHeader.Background(tc.SidebarRowHover)
						}
						if pHeader.Clicked() {
							a.CollapsedProjectGroups[group.ProjectName] = !isCollapsed
						}

						// Expanded Project Tasks
						if !isCollapsed {
							ui.Column(c.Key("pbody-"+group.ProjectName)).Padding(0, 0, 4, 8).Gap(2).Children(func() {
								if totalCount == 0 {
									ui.Row(c).Height(24).Padding(0, 8).AlignItems(ui.Center).Children(func() {
										ui.Text(c, "No tasks in this project").FontSize(11).TextColor(tc.SidebarMutedForeground)
									})
								}

								// 1. Agent ACP Sessions
								for _, sess := range group.Sessions {
									s := sess
									isActive := (a.ActiveTaskKind == "agent" && s.ID == a.Agent.ActiveSessionID)
									renderTaskRow(c, tc, taskRowData{
										Key:      "sess-" + s.ID,
										Title:    s.Title,
										Subtitle: s.Model,
										Status:   s.Status,
										Active:   isActive,
										Pinned:   s.Pinned,
										Archived: s.Archived,
										OnPin:    func() { a.Agent.TogglePin(s.ID) },
										OnDelete: func() { a.Agent.DeleteSession(s.ID) },
										OnOpen:   func() { a.ActiveTaskKind = "agent"; a.Agent.SelectSession(s.ID) },
									})
								}

								// 2. Active CLI Sessions
								for _, cliSess := range group.CLISessions {
									cs := cliSess
									isActive := (a.ActiveTaskKind == "cli" && a.CLIMgr.ActiveID == cs.ID)
									renderTaskRow(c, tc, taskRowData{
										Key:      "cli-sess-" + cs.ID,
										Title:    cs.Title,
										Subtitle: cs.AgentName + " · CLI Task",
										Status:   "running",
										Active:   isActive,
										OnOpen:   func() { a.ActiveTaskKind = "cli"; a.CLIMgr.ActiveID = cs.ID },
									})
								}
							})
						}
					}
				})
			})
		}

		// Bots / Automations Section
		ui.Column(c).BorderWidth(1, 0, 0, 0).BorderColor(tc.SidebarBorder).Padding(6, 10).Gap(2).Children(func() {
			ui.Row(c).Padding(2, 4).AlignItems(ui.Center).Children(func() {
				SectionLabel(c, tc, "BOTS")
				ui.Box(c).Grow(1)
				if IconButton(c, tc, icons.TablerPlus, 12).Clicked() {
					a.OpenNewTaskModal()
				}
			})
			botRow := SidebarActionButton(c, tc, icons.TablerBot, "Add a bot or schedule", "", false)
			if botRow.Clicked() {
				a.OpenNewTaskModal()
			}
		})

		// Bottom Utility Dock
		dockBtn := func(icon *ui.SVG, active bool, onPick func()) {
			btn := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
			btn.Children(func() {
				ic := ui.Icon(c, icon).Width(15).Height(15)
				if active {
					ic.TextColor(tc.Accent)
				} else {
					ic.TextColor(tc.IconMuted)
				}
			})
			if active {
				btn.Background(tc.Text.Alpha(0.1))
			} else if btn.Hovered() {
				btn.Background(tc.Text.Alpha(0.05))
			}
			if btn.Clicked() {
				onPick()
			}
		}
		ui.Row(c).Padding(6, 10).Gap(2).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.SidebarBorder).
			Background(tc.Chrome.Alpha(0.4)).
			Children(func() {
				dockBtn(icons.TablerHelp, false, func() { mygo.Shell.OpenExternal("https://github.com/hasdev/forge-ade") })
				dockBtn(icons.TablerChartBar, false, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabTasks
				})
				dockBtn(icons.TablerLayoutKanban, a.RightDrawerTab == RightTabBoard && a.ShowRightDrawer, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabBoard
				})
				dockBtn(icons.TablerGitBranch, a.RightDrawerTab == RightTabGit && a.ShowRightDrawer, func() {
					a.ShowRightDrawer = true
					a.RightDrawerTab = RightTabGit
				})
				dockBtn(icons.TablerBook, false, func() {
					a.ShowNewFileModal = true
					a.NewFileName = "AGENTS.md"
				})
				dockBtn(icons.TablerSettings, a.Settings != nil && a.Settings.IsOpen, func() {
					if a.Settings != nil {
						a.Settings.Toggle()
					}
				})
				ui.Box(c).Grow(1)
				if a.ThemeMgr.Appearance == "dark" {
					dockBtn(icons.TablerMoon, false, a.toggleAppearance)
				} else {
					dockBtn(icons.TablerSun, false, a.toggleAppearance)
				}
			})
		_ = t
	})
}

// resolveProjectName extracts a clean human-readable workspace project name.
func resolveProjectName(folderPath string) string {
	clean := strings.TrimSpace(folderPath)
	if clean == "" || clean == "general" || strings.HasSuffix(clean, "/.forge-ade/general") || strings.HasSuffix(clean, "/.forge-ade/general/") {
		return "General"
	}
	base := filepath.Base(filepath.Clean(clean))
	if base == "." || base == "/" || base == "" {
		return "General"
	}
	return base
}

// FileTreeNode represents a node in the hierarchical file tree.
type FileTreeNode struct {
	Name         string
	RelativePath string
	FullPath     string
	IsDir        bool
	Children     []*FileTreeNode
}

// buildFileTree converts a flat list of workspace files into a hierarchical tree with sorted folders and files.
func buildFileTree(files []workspace.WorkspaceFile, rootDir string) []*FileTreeNode {
	rootMap := make(map[string]*FileTreeNode)
	var rootNodes []*FileTreeNode

	for _, f := range files {
		cleanRel := filepath.Clean(f.RelativePath)
		parts := strings.Split(cleanRel, string(filepath.Separator))
		if len(parts) == 0 {
			continue
		}

		if len(parts) == 1 {
			node := &FileTreeNode{
				Name:         parts[0],
				RelativePath: f.RelativePath,
				FullPath:     f.FullPath,
				IsDir:        f.IsDir,
			}
			rootMap[parts[0]] = node
			rootNodes = append(rootNodes, node)
			continue
		}

		// Navigate or create intermediate directories
		var curNode *FileTreeNode
		currentRel := parts[0]
		if existing, ok := rootMap[parts[0]]; ok {
			curNode = existing
		} else {
			curNode = &FileTreeNode{
				Name:         parts[0],
				RelativePath: currentRel,
				FullPath:     filepath.Join(rootDir, currentRel),
				IsDir:        true,
				Children:     make([]*FileTreeNode, 0),
			}
			rootMap[parts[0]] = curNode
			rootNodes = append(rootNodes, curNode)
		}

		for i := 1; i < len(parts)-1; i++ {
			part := parts[i]
			currentRel = filepath.Join(currentRel, part)
			var nextNode *FileTreeNode
			for _, child := range curNode.Children {
				if child.IsDir && child.Name == part {
					nextNode = child
					break
				}
			}
			if nextNode == nil {
				nextNode = &FileTreeNode{
					Name:         part,
					RelativePath: currentRel,
					FullPath:     filepath.Join(rootDir, currentRel),
					IsDir:        true,
					Children:     make([]*FileTreeNode, 0),
				}
				curNode.Children = append(curNode.Children, nextNode)
			}
			curNode = nextNode
		}

		// Leaf file node
		fileName := parts[len(parts)-1]
		fileNode := &FileTreeNode{
			Name:         fileName,
			RelativePath: f.RelativePath,
			FullPath:     f.FullPath,
			IsDir:        f.IsDir,
		}
		curNode.Children = append(curNode.Children, fileNode)
	}

	// Sort nodes: folders first (alphabetical), then files (alphabetical)
	var sortTree func(nodes []*FileTreeNode)
	sortTree = func(nodes []*FileTreeNode) {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].IsDir != nodes[j].IsDir {
				return nodes[i].IsDir
			}
			return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
		})
		for _, n := range nodes {
			if n.IsDir && len(n.Children) > 0 {
				sortTree(n.Children)
			}
		}
	}
	sortTree(rootNodes)

	return rootNodes
}

func (a *AppUI) isFolderExpanded(relPath string, depth int) bool {
	if a.Workspace.FilterQuery != "" {
		return true // Auto-expand all matching folders during search
	}
	if expanded, exists := a.ExpandedFileTreeFolders[relPath]; exists {
		return expanded
	}
	return depth == 0 // Top-level folders default to expanded
}

// renderFileTree renders the workspace file explorer: 35px header, filter
// input, collapsible folders and file tree with Tabler vector icons and dirty badges.
func (a *AppUI) renderFileTree(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Row(c).Height(35).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "EXPLORER").FontSize(12).FontWeight(600).LetterSpacing(0.4)
		ui.Box(c).Grow(1)
		treeBtn := func(icon *ui.SVG, onPick func()) {
			btn := ui.Row(c).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			btn.Children(func() {
				ui.Icon(c, icon).Width(14).Height(14).TextColor(tc.IconMuted)
			})
			if btn.Hovered() {
				btn.Background(tc.Text.Alpha(0.05))
			}
			if btn.Clicked() {
				onPick()
			}
		}
		treeBtn(icons.TablerFilePlus, func() {
			a.ShowNewFileModal = true
			a.NewFileName = ""
		})
		treeBtn(icons.TablerFolderPlus, func() {
			a.ShowNewFolderModal = true
			a.NewFolderName = ""
		})
		treeBtn(icons.TablerRefresh, func() {
			a.Workspace.Refresh()
			if a.Git != nil {
				go a.Git.Refresh()
			}
			if a.Database != nil {
				go a.Database.ScanDatabases()
			}
		})

		if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
			unplugBtn := ui.Row(c).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
			unplugBtn.Children(func() {
				ui.Icon(c, icons.TablerUnplug).Width(14).Height(14).TextColor(tc.Error)
			})
			if unplugBtn.Hovered() {
				unplugBtn.Background(tc.Error.Alpha(0.15))
			}
			if unplugBtn.Clicked() {
				a.DisconnectSSH()
			}
		}
	})

	// Shared hierarchical explorer with context menus (also rendered by the
	// right drawer's Files pane).
	a.renderExplorerTree(c, tc)
}
