package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	agentsession "github.com/hasdev/forge-ade/core/agentsession"
)

// ProjectIconConfig stores a user-selected custom icon or image for a sidebar project.
type ProjectIconConfig struct {
	Kind      string `json:"kind"`                 // "icon" | "image"
	IconID    string `json:"icon_id,omitempty"`    // key in projectIconCatalog
	ColorKey  string `json:"color_key,omitempty"`  // "accent", "success", "info", "warning", "error", "muted"
	ImagePath string `json:"image_path,omitempty"` // local PNG/JPG/WebP path
}

// CustomProject groups multiple workspace folders under a single sidebar project name.
type CustomProject struct {
	Name    string   `json:"name"`
	Folders []string `json:"folders"`
}

type projectIconChoice struct {
	ID    string
	Label string
	Icon  *ui.SVG
}

var projectIconCatalog = []projectIconChoice{
	{ID: "code", Label: "Code", Icon: icons.TablerCode},
	{ID: "terminal", Label: "Terminal", Icon: icons.TablerTerminal},
	{ID: "globe", Label: "Web", Icon: icons.TablerGlobe},
	{ID: "database", Label: "Database", Icon: icons.TablerDatabase},
	{ID: "cpu", Label: "Compute", Icon: icons.TablerCpu},
	{ID: "server", Label: "Server", Icon: icons.TablerServer},
	{ID: "sparkles", Label: "AI", Icon: icons.TablerSparkles},
	{ID: "bolt", Label: "Fast", Icon: icons.TablerBolt},
	{ID: "flame", Label: "Hot", Icon: icons.TablerFlame},
	{ID: "shield", Label: "Security", Icon: icons.TablerShield},
	{ID: "compass", Label: "Explore", Icon: icons.TablerCompass},
	{ID: "palette", Label: "Design", Icon: icons.TablerPalette},
	{ID: "puzzle", Label: "Plugin", Icon: icons.TablerPuzzle},
	{ID: "star", Label: "Star", Icon: icons.TablerStar},
	{ID: "go", Label: "Go", Icon: icons.TablerBrandGolang},
	{ID: "ts", Label: "TypeScript", Icon: icons.TablerBrandTypescript},
	{ID: "py", Label: "Python", Icon: icons.TablerBrandPython},
	{ID: "folder", Label: "Folder", Icon: icons.TablerFolder},
}

func lookupProjectIconChoice(id string) *ui.SVG {
	for _, ch := range projectIconCatalog {
		if ch.ID == id {
			return ch.Icon
		}
	}
	return nil
}

func resolveProjectIconColor(tc theme.ThemeColors, key string) ui.Color {
	switch key {
	case "success":
		return tc.Success
	case "info":
		return tc.Info
	case "warning":
		return tc.Warning
	case "error":
		return tc.Error
	case "muted":
		return tc.IconMuted
	default:
		return tc.Accent
	}
}

// renderProjectGroupIcon renders the project's custom image, custom icon, or default folder/server icon.
func (a *AppUI) renderProjectGroupIcon(c *ui.Context, tc theme.ThemeColors, group *sidebarProjectGroup, isCollapsed bool) {
	if group != nil && a.ProjectIcons != nil {
		if cfg, ok := a.ProjectIcons[group.ProjectName]; ok {
			if cfg.Kind == "image" && cfg.ImagePath != "" {
				if ci := a.resolveBackgroundImage(cfg.ImagePath); ci != nil && ci.src != nil {
					ui.Image(c, ci.src).Size(20, 20).Radius(5).Fit(ui.Cover).Shrink(0)
					return
				}
			}
			if cfg.Kind == "icon" && cfg.IconID != "" {
				if svg := lookupProjectIconChoice(cfg.IconID); svg != nil {
					col := resolveProjectIconColor(tc, cfg.ColorKey)
					ui.Icon(c, svg).Width(18).Height(18).TextColor(col)
					return
				}
			}
		}
	}

	isRemote := group != nil && strings.HasPrefix(group.FolderPath, "ssh://")
	if group != nil && group.IsCustom {
		if isCollapsed {
			ui.Icon(c, icons.TablerPuzzle).Width(16).Height(16).TextColor(tc.IconMuted)
		} else {
			ui.Icon(c, icons.TablerPuzzle).Width(16).Height(16).TextColor(tc.Accent)
		}
		return
	}
	if isCollapsed {
		if isRemote {
			ui.Icon(c, icons.TablerServer).Width(16).Height(16).TextColor(tc.IconMuted)
		} else {
			ui.Icon(c, icons.TablerFolder).Width(16).Height(16).TextColor(tc.IconMuted)
		}
	} else {
		if isRemote {
			ui.Icon(c, icons.TablerServer).Width(16).Height(16).TextColor(tc.Accent)
		} else {
			ui.Icon(c, icons.TablerFolderOpen).Width(16).Height(16).TextColor(tc.Accent)
		}
	}
}

func (a *AppUI) openProjectIconModal(projectName string) {
	if projectName == "" || strings.EqualFold(projectName, "General") {
		return
	}
	a.ProjectIconTarget = projectName
	a.ShowProjectIconModal = true
	a.invalidateUI()
}

func (a *AppUI) setProjectIcon(projectName string, cfg ProjectIconConfig) {
	if projectName == "" {
		return
	}
	if a.ProjectIcons == nil {
		a.ProjectIcons = make(map[string]ProjectIconConfig)
	}
	a.ProjectIcons[projectName] = cfg
	a.prefsDirty.Store(true)
	a.SaveUIPrefs()
	a.invalidateUI()
}

func (a *AppUI) clearProjectIcon(projectName string) {
	if a.ProjectIcons == nil {
		return
	}
	delete(a.ProjectIcons, projectName)
	a.prefsDirty.Store(true)
	a.SaveUIPrefs()
	a.invalidateUI()
}

func (a *AppUI) pickProjectIconImage(projectName string) {
	if a.Window == nil || projectName == "" {
		return
	}
	win := a.Window
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win,
			Title:  "Choose Project Icon Image",
			Filters: []mygo.FileFilter{
				{
					Name:       "Images",
					Extensions: []string{"png", "jpg", "jpeg", "webp", "gif", "bmp", "tiff", "tif", "avif", "svg"},
					MIMETypes:  []string{"image/*"},
				},
			},
		})
		if err != nil || len(paths) == 0 || paths[0] == "" {
			return
		}
		path := paths[0]
		a.runOnUI(func() {
			a.setProjectIcon(projectName, ProjectIconConfig{
				Kind:      "image",
				ImagePath: path,
			})
			a.ShowProjectIconModal = false
		})
	}()
}

func (a *AppUI) renderProjectIconModal(c *ui.Context) {
	if !a.ShowProjectIconModal {
		return
	}
	tc := a.ThemeMgr.Colors()
	target := a.ProjectIconTarget
	cur := ProjectIconConfig{Kind: "icon", IconID: "code", ColorKey: "accent"}
	if a.ProjectIcons != nil {
		if existing, ok := a.ProjectIcons[target]; ok {
			cur = existing
		}
	}
	if cur.ColorKey == "" {
		cur.ColorKey = "accent"
	}

	a.modal(c, &a.ShowProjectIconModal, func() {
		ui.Column(c).Width(390).Radius(12).Padding(16, 18).Gap(12).
			Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, fmt.Sprintf("Project Icon — %s", target)).FontSize(14).FontWeight(600).TextColor(tc.Text)
				closeBtn := IconButton(c, tc, icons.TablerX, 14)
				if closeBtn.Clicked() {
					a.ShowProjectIconModal = false
				}
			})

			// Color Swatches
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Icon Color").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					colorKeys := []string{"accent", "success", "info", "warning", "error", "muted"}
					for _, ck := range colorKeys {
						key := ck
						col := resolveProjectIconColor(tc, key)
						sw := ui.Box(c.Key("picon-col-"+key)).Size(22, 22).Radius(11).Background(col).Cursor(ui.CursorPointer)
						if cur.ColorKey == key {
							sw.BorderWidth(2).BorderColor(tc.Text)
						}
						if sw.Clicked() {
							iconID := cur.IconID
							if iconID == "" {
								iconID = "code"
							}
							a.setProjectIcon(target, ProjectIconConfig{
								Kind:     "icon",
								IconID:   iconID,
								ColorKey: key,
							})
						}
					}
				})
			})

			// Icon Grid
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Choose Icon").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, ch := range projectIconCatalog {
						choice := ch
						selected := cur.Kind == "icon" && cur.IconID == choice.ID
						btn := ui.Row(c.Key("picon-"+choice.ID)).Size(34, 34).Radius(8).
							AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).
							Tooltip(choice.Label)
						if selected {
							btn.Background(tc.Accent.Alpha(0.16)).BorderWidth(1).BorderColor(tc.Accent)
						} else {
							btn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						}
						btn.Children(func() {
							col := resolveProjectIconColor(tc, cur.ColorKey)
							ui.Icon(c, choice.Icon).Width(16).Height(16).TextColor(col)
						})
						if btn.Hovered() && !selected {
							btn.Background(tc.SurfaceRaised)
						}
						if btn.Clicked() {
							a.setProjectIcon(target, ProjectIconConfig{
								Kind:     "icon",
								IconID:   choice.ID,
								ColorKey: cur.ColorKey,
							})
							a.ShowProjectIconModal = false
						}
					}
				})
			})

			// Custom Image + Reset Footer
			ui.Row(c).FillWidth().Padding(4, 0, 0, 0).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				imgBtn := ui.Row(c.Key("picon-upload")).Height(30).Radius(6).Padding(0, 12).Gap(6).
					AlignItems(ui.Center).Background(tc.Accent).Cursor(ui.CursorPointer)
				imgBtn.Children(func() {
					ui.Icon(c, icons.TablerPhoto).Width(13).Height(13).TextColor(tc.AccentForeground)
					ui.Text(c, "Choose Image…").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
				})
				if imgBtn.Hovered() {
					imgBtn.Background(tc.Accent.Alpha(0.85))
				}
				if imgBtn.Clicked() {
					a.pickProjectIconImage(target)
				}

				resetBtn := ui.Button(c, "Reset")
				if resetBtn.Clicked() {
					a.clearProjectIcon(target)
					a.ShowProjectIconModal = false
				}
			})
		})
	})
}

func normalizeCustomFolder(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "ssh://") {
		return agentsession.NormalizeWorkspacePath(p)
	}
	return agentsession.NormalizeWorkspacePath(p)
}

func (a *AppUI) customProjectByName(name string) (CustomProject, int) {
	for i, cp := range a.CustomProjects {
		if strings.EqualFold(cp.Name, name) {
			return cp, i
		}
	}
	return CustomProject{}, -1
}

func (a *AppUI) openNewCustomProjectModal() {
	a.CustomProjectEditOriginalName = ""
	a.CustomProjectNameInput = ""
	a.CustomProjectFoldersInput = nil
	if a.Workspace != nil && a.Workspace.RootDir != "" {
		norm := normalizeCustomFolder(a.Workspace.RootDir)
		if norm != "" {
			a.CustomProjectFoldersInput = []string{norm}
		}
	}
	a.CustomProjectError = ""
	a.ShowCustomProjectModal = true
	a.invalidateUI()
}

func (a *AppUI) openEditCustomProjectModal(name string) {
	cp, idx := a.customProjectByName(name)
	if idx < 0 {
		return
	}
	a.CustomProjectEditOriginalName = cp.Name
	a.CustomProjectNameInput = cp.Name
	a.CustomProjectFoldersInput = append([]string(nil), cp.Folders...)
	a.CustomProjectError = ""
	a.ShowCustomProjectModal = true
	a.invalidateUI()
}

func (a *AppUI) addFolderToCustomProjectDraft() {
	if a.Window == nil {
		return
	}
	win := a.Window
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    win,
			Title:     "Add Folder to Custom Project",
			Directory: true,
		})
		if err != nil || len(paths) == 0 || paths[0] == "" {
			return
		}
		norm := normalizeCustomFolder(paths[0])
		if norm == "" {
			return
		}
		a.runOnUI(func() {
			for _, existing := range a.CustomProjectFoldersInput {
				if existing == norm {
					return
				}
			}
			a.CustomProjectFoldersInput = append(a.CustomProjectFoldersInput, norm)
			a.CustomProjectError = ""
			a.invalidateUI()
		})
	}()
}

func (a *AppUI) saveCustomProjectDraft() {
	name := strings.TrimSpace(a.CustomProjectNameInput)
	if name == "" || strings.EqualFold(name, "General") {
		a.CustomProjectError = "Please enter a valid project name (cannot be General)."
		return
	}
	if len(a.CustomProjectFoldersInput) == 0 {
		a.CustomProjectError = "Add at least one folder to this custom project."
		return
	}

	cp := CustomProject{
		Name:    name,
		Folders: append([]string(nil), a.CustomProjectFoldersInput...),
	}
	if a.CustomProjectEditOriginalName != "" {
		if _, idx := a.customProjectByName(a.CustomProjectEditOriginalName); idx >= 0 {
			a.CustomProjects[idx] = cp
		} else {
			a.CustomProjects = append(a.CustomProjects, cp)
		}
	} else {
		if _, idx := a.customProjectByName(name); idx >= 0 {
			a.CustomProjectError = fmt.Sprintf("A custom project named %q already exists.", name)
			return
		}
		a.CustomProjects = append(a.CustomProjects, cp)
	}
	for _, f := range cp.Folders {
		a.recordRecentWorkspace(f)
	}
	a.prefsDirty.Store(true)
	a.SaveUIPrefs()
	a.ShowCustomProjectModal = false
	a.CustomProjectError = ""
	a.invalidateUI()
}

func (a *AppUI) deleteCustomProject(name string) {
	kept := make([]CustomProject, 0, len(a.CustomProjects))
	for _, cp := range a.CustomProjects {
		if !strings.EqualFold(cp.Name, name) {
			kept = append(kept, cp)
		}
	}
	a.CustomProjects = kept
	a.prefsDirty.Store(true)
	a.SaveUIPrefs()
	a.ShowCustomProjectModal = false
	a.invalidateUI()
}

func (a *AppUI) renderCustomProjectModal(c *ui.Context) {
	if !a.ShowCustomProjectModal {
		return
	}
	tc := a.ThemeMgr.Colors()
	isEdit := a.CustomProjectEditOriginalName != ""

	a.modal(c, &a.ShowCustomProjectModal, func() {
		ui.Column(c).Width(440).Radius(12).Padding(18, 20).Gap(12).
			Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				title := "New Custom Project"
				if isEdit {
					title = "Manage Custom Project"
				}
				ui.Text(c, title).FontSize(14).FontWeight(600).TextColor(tc.Text)
				closeBtn := IconButton(c, tc, icons.TablerX, 14)
				if closeBtn.Clicked() {
					a.ShowCustomProjectModal = false
				}
			})

			ui.Text(c, "Group multiple folders under one project in the sidebar (e.g. MyProject with folder a, folder b, folder c).").
				FontSize(11.5).TextColor(tc.TextMuted)

			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Project Name").FontSize(11).FontWeight(500).TextColor(tc.Text)
				ui.Row(c).Height(32).Radius(6).Padding(0, 8).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
					ui.TextInput(c, &a.CustomProjectNameInput).Placeholder("e.g. MyProject").Grow(1)
				})
			})

			ui.Column(c).Gap(6).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, fmt.Sprintf("Folders (%d)", len(a.CustomProjectFoldersInput))).FontSize(11).FontWeight(500).TextColor(tc.Text)
					addFolderBtn := ui.Row(c.Key("cp-add-folder")).Height(24).Radius(6).Padding(0, 8).Gap(4).
						AlignItems(ui.Center).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
					addFolderBtn.Children(func() {
						ui.Icon(c, icons.TablerFolderPlus).Width(12).Height(12).TextColor(tc.Accent)
						ui.Text(c, "Add Folder…").FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
					if addFolderBtn.Hovered() {
						addFolderBtn.Background(tc.SurfaceRaised)
					}
					if addFolderBtn.Clicked() {
						a.addFolderToCustomProjectDraft()
					}
				})

				if len(a.CustomProjectFoldersInput) == 0 {
					ui.Row(c).FillWidth().Padding(10, 12).Radius(6).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
						ui.Text(c, "No folders added yet. Click \"Add Folder…\" to pick folders.").FontSize(11).TextColor(tc.TextMuted)
					})
				} else {
					ui.Scroll(c).MaxHeight(160).Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							for i, fp := range a.CustomProjectFoldersInput {
								idx := i
								folder := fp
								ui.Row(c.Key("cp-f-"+folder)).Height(28).Radius(6).Padding(0, 8).Gap(6).
									AlignItems(ui.Center).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
									ui.Icon(c, icons.TablerFolder).Width(12).Height(12).TextColor(tc.IconMuted)
									ui.Text(c, filepath.Base(folder)).FontSize(11.5).FontWeight(500).TextColor(tc.Text).SingleLine()
									ui.Text(c, folder).FontSize(10.5).TextColor(tc.TextMuted).Grow(1).MinWidth(0).SingleLine()
									rmBtn := ui.Row(c.Key("cp-rm-"+folder)).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
									rmBtn.Children(func() {
										ui.Icon(c, icons.TablerX).Width(11).Height(11).TextColor(tc.IconMuted)
									})
									if rmBtn.Hovered() {
										rmBtn.Background(tc.Error.Alpha(0.15))
									}
									if rmBtn.Clicked() {
										a.CustomProjectFoldersInput = append(a.CustomProjectFoldersInput[:idx], a.CustomProjectFoldersInput[idx+1:]...)
									}
								})
							}
						})
					})
				}
			})

			if a.CustomProjectError != "" {
				ui.Row(c).Padding(6, 10).Radius(6).Background(tc.Error.Alpha(0.1)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.25)).Children(func() {
					ui.Text(c, a.CustomProjectError).FontSize(11).TextColor(tc.Error)
				})
			}

			ui.Row(c).FillWidth().Padding(4, 0, 0, 0).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				if isEdit {
					delBtn := ui.Row(c.Key("cp-delete")).Height(28).Radius(6).Padding(0, 10).AlignItems(ui.Center).
						Background(tc.Error.Alpha(0.12)).Cursor(ui.CursorPointer)
					delBtn.Children(func() {
						ui.Text(c, "Delete Project").FontSize(11.5).FontWeight(500).TextColor(tc.Error)
					})
					if delBtn.Clicked() {
						a.deleteCustomProject(a.CustomProjectEditOriginalName)
					}
				} else {
					ui.Box(c)
				}

				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					cancelBtn := ui.Button(c, "Cancel")
					if cancelBtn.Clicked() {
						a.ShowCustomProjectModal = false
					}
					saveBtn := ui.Row(c.Key("cp-save")).Height(28).Radius(6).Padding(0, 14).AlignItems(ui.Center).
						Background(tc.Accent).Cursor(ui.CursorPointer)
					saveBtn.Children(func() {
						ui.Text(c, "Save").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
					})
					if saveBtn.Hovered() {
						saveBtn.Background(tc.Accent.Alpha(0.85))
					}
					if saveBtn.Clicked() {
						a.saveCustomProjectDraft()
					}
				})
			})
		})
	})
}
