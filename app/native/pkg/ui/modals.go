package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/git"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/palette"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// modal shows a dialog via ui.Modal, resetting default panel padding, background,
// and shadow so the custom-styled child card renders as the single crisp dialog overlay.
func (a *AppUI) modal(c *ui.Context, open *bool, fn func()) ui.Element {
	return ui.Modal(c, open, fn).Padding(0).Background(ui.Transparent).Shadow(0, 0, 0, 0, ui.Transparent)
}

// renderModals orchestrates all popup modals and overlay dialogs.
func (a *AppUI) renderModals(c *ui.Context) {
	if a.ShowProjectSwitcherModal {
		a.renderProjectSwitcherModal(c)
	}
	if a.Palette != nil && a.Palette.IsOpen {
		a.renderPaletteModal(c)
	}
	if a.ThemeMgr != nil && a.ThemeMgr.IsOpen {
		a.renderThemePickerModal(c)
	}
	if a.ACPMgr != nil && a.ACPMgr.ActiveReq != nil {
		a.renderPermissionModal(c)
	}
	if a.ShowNewFileModal {
		a.renderNewFileModal(c)
	}
	if a.ShowNewFolderModal {
		a.renderNewFolderModal(c)
	}
	if a.ShowNewTaskModal {
		a.renderNewTaskModal(c)
	}
	if a.ShowTaskMenuModal {
		a.renderTaskActionsMenu(c)
	}
	if a.ShowRenameModal {
		a.renderRenameModal(c)
	}
	if a.ShowRenameFileModal {
		a.renderRenameFileModal(c)
	}
	if a.ShowBranchModal {
		a.renderBranchModal(c)
	}
	if a.ShowModelPalette {
		a.renderModelProviderPaletteModal(c)
	}
	if a.ShowSSHModal {
		a.renderSSHConnectModal(c)
	}
	if a.ShowSSHQuickMenu {
		a.renderSSHQuickMenuModal(c)
	}
	if a.ShowRemoteUploadModal {
		a.renderRemoteFileUploaderModal(c)
	}
	if a.ShowUsageModal {
		a.renderUsageQuotaModal(c)
	}
	if a.ShowRulesModal {
		a.renderUnifiedRulesModal(c)
	}
	if a.ShowNewBoardTaskModal {
		a.renderNewBoardTaskModal(c)
	}
	if a.ShowTerminalSnippetsModal {
		a.renderTerminalSnippetsModal(c)
	}
	if a.ShowPeekModal {
		a.renderPeekModal(c)
	}
	if a.ShowRenameSymbolModal {
		a.renderRenameSymbolModal(c)
	}
	if a.ShowJSONModal {
		a.renderJSONModal(c)
	}
	if a.ShowPlanModal {
		a.renderPlanModal(c)
	}
	if a.ShowBgShellPopover {
		sessionID := ""
		if a.Agent != nil {
			sessionID = a.Agent.ActiveSessionID
		}
		a.renderBgShellPopover(c, a.ThemeMgr.Colors(), sessionID)
	}
	a.renderConfirmDialog(c)
	a.renderEditorConfirmModal(c)
	a.renderQuestionModal(c)
	a.renderProjectSearchModal(c)
}

// renderConfirmDialog draws the reusable destructive-action confirmation
// (confirmDialog). ui.Modal clears ShowConfirmDialog on Escape / outside
// click, which cancels the staged action.
func (a *AppUI) renderConfirmDialog(c *ui.Context) {
	if !a.ShowConfirmDialog {
		return
	}
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		a.cancelConfirmDialog()
		return
	}

	a.modal(c, &a.ShowConfirmDialog, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerAlertTriangle).Width(16).Height(16).TextColor(tc.Warning)
				ui.Text(c, a.ConfirmDialogTitle).FontSize(14).FontWeight(600)
			})
			ui.Text(c, a.ConfirmDialogDesc).FontSize(12).TextColor(tc.TextMuted)
			ui.Row(c).AlignItems(ui.Center).Gap(6).Justify(ui.End).Children(func() {
				if ui.Button(c, "Cancel").Clicked() {
					a.cancelConfirmDialog()
				}
				if ui.PrimaryButton(c, a.ConfirmDialogLabel).Clicked() {
					a.acceptConfirmDialog()
				}
			})
		})
	})
}

// renderEditorConfirmModal renders the EditorManager data-loss confirmations:
// discard-unsaved-buffers, overwrite-changed-on-disk, and workspace Replace
// All. Resolution is delegated back to the manager (ResolveDiscard /
// ResolveOverwrite / the staged replace decide callback).
func (a *AppUI) renderEditorConfirmModal(c *ui.Context) {
	discard := len(a.EditorPendingDiscard) > 0
	overwrite := a.EditorPendingOverwrite != ""
	replace := a.EditorPendingReplaceN > 0 && a.editorReplaceDecide != nil
	if !discard && !overwrite && !replace {
		return
	}
	tc := a.ThemeMgr.Colors()

	title, desc, confirmLabel := "", "", ""
	var onConfirm, onCancel func()
	switch {
	case discard:
		paths := a.EditorPendingDiscard
		list := make([]string, 0, len(paths))
		for _, p := range paths {
			list = append(list, filepath.Base(p))
		}
		title = "Discard Unsaved Changes?"
		desc = fmt.Sprintf("%d file(s) have unsaved changes that will be lost: %s", len(paths), strings.Join(list, ", "))
		confirmLabel = "Discard"
		onConfirm = func() {
			a.EditorPendingDiscard = nil
			a.Editor.ResolveDiscard(true)
		}
		onCancel = func() {
			a.EditorPendingDiscard = nil
			a.Editor.ResolveDiscard(false)
		}
	case overwrite:
		path := a.EditorPendingOverwrite
		title = "File Changed on Disk"
		desc = fmt.Sprintf("%s was modified outside the editor while it had unsaved changes. Overwrite the on-disk version with your buffer?", filepath.Base(path))
		confirmLabel = "Overwrite"
		onConfirm = func() {
			a.EditorPendingOverwrite = ""
			a.Editor.ResolveOverwrite(true)
		}
		onCancel = func() {
			a.EditorPendingOverwrite = ""
			a.Editor.ResolveOverwrite(false)
		}
	case replace:
		n := a.EditorPendingReplaceN
		decide := a.editorReplaceDecide
		title = "Replace All"
		desc = fmt.Sprintf("Replace %d match(es) of “%s” across the workspace? Open buffers are updated so editor and disk stay in sync.", n, a.FileSearchQuery)
		confirmLabel = "Replace"
		onConfirm = func() {
			a.EditorPendingReplaceN = 0
			a.editorReplaceDecide = nil
			decide(true)
		}
		onCancel = func() {
			a.EditorPendingReplaceN = 0
			a.editorReplaceDecide = nil
			decide(false)
		}
	}

	if c.Shortcut(0, ui.KeyEscape) {
		onCancel()
		return
	}

	a.EditorConfirmModalOpen = discard || overwrite || replace
	a.modal(c, &a.EditorConfirmModalOpen, func() {
		ui.Column(c).Width(440).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerAlertTriangle).Width(16).Height(16).TextColor(tc.Warning)
				ui.Text(c, title).FontSize(14).FontWeight(600)
			})
			ui.Text(c, desc).FontSize(12).TextColor(tc.TextMuted)
			ui.Row(c).AlignItems(ui.Center).Gap(6).Justify(ui.End).Children(func() {
				if ui.Button(c, "Cancel").Clicked() {
					onCancel()
				}
				if ui.PrimaryButton(c, confirmLabel).Clicked() {
					onConfirm()
				}
			})
		})
	})
}

func (a *AppUI) renderPaletteModal(c *ui.Context) {
	pm := a.Palette
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		pm.Close()
	}
	if c.Shortcut(0, ui.KeyDown) {
		pm.MoveSelection(1)
	}
	if c.Shortcut(0, ui.KeyUp) {
		pm.MoveSelection(-1)
	}

	matches := pm.FilteredFiles()
	commands := pm.FilteredCommands()
	symbols := pm.FilteredSymbols()
	mode := pm.EffectiveMode()

	activate := func() {
		switch mode {
		case palette.PaletteFiles:
			if pm.SelectedIndex >= 0 && pm.SelectedIndex < len(matches) {
				selected := matches[pm.SelectedIndex].File.RelativePath
				pm.Close()
				a.openSearchResult(selected, 0, "")
			}
		case palette.PaletteSymbols:
			if pm.SelectedIndex >= 0 && pm.SelectedIndex < len(symbols) {
				hit := symbols[pm.SelectedIndex]
				pm.Close()
				a.openSearchResult(hit.Path, hit.Line, "")
			}
		case palette.PaletteLine:
			if line, col, ok := palette.LineJumpSpec(pm.Query); ok {
				pm.Close()
				a.jumpActiveDocumentToLine(line, col)
			}
		default:
			if pm.SelectedIndex < len(commands) {
				cmd := commands[pm.SelectedIndex]
				pm.Close()
				if cmd.Run != nil {
					cmd.Run()
				}
			}
		}
	}

	a.modal(c, &pm.IsOpen, func() {
		panel := ui.Column(c).Width(576).Radius(16).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).Clip()
		panel.Children(func() {
			// Search row
			ui.Row(c).Padding(14).Gap(8).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				icon := icons.TablerSearch
				if mode == palette.PaletteCommands {
					icon = icons.TablerChevronRight
				} else if mode == palette.PaletteSymbols {
					icon = icons.TablerSymbol
				} else if mode == palette.PaletteLine {
					icon = icons.TablerHash
				}
				ui.Icon(c, icon).Width(15).Height(15).TextColor(tc.IconMuted)
				placeholder := "Search files by name..."
				if mode == palette.PaletteCommands {
					placeholder = "Type a command..."
				} else if mode == palette.PaletteSymbols {
					placeholder = "Go to symbol... (@ prefix)"
				} else if mode == palette.PaletteLine {
					placeholder = "Go to line :42 or :42:7"
				}
				in := ui.TextInput(c, &pm.Query).Placeholder(placeholder).Grow(1).AutoFocus()
				if in.Submitted() {
					activate()
				}
				kbd := ui.Row(c).Radius(4).Padding(1, 6).Background(tc.ToolbarControl)
				kbd.Children(func() {
					ui.Text(c, "ESC").FontSize(12)
				})
			})

			// Result list
			ui.Scroll(c).MaxHeight(320).Children(func() {
				ui.Column(c).Padding(6).Gap(2).Children(func() {
					switch mode {
					case palette.PaletteFiles:
						if len(matches) == 0 {
							ui.Text(c, "No matching files").FontSize(13).TextColor(tc.TextMuted).Padding(8, 12)
							break
						}
						for i, m := range matches {
							i, path := i, m.File.RelativePath
							selected := (i == pm.SelectedIndex)
							row := ui.Row(c.Key("pal-"+path)).Radius(10).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, icons.FileIconSVG(path)).Width(13).Height(13).TextColor(tc.IconMuted)
								ui.Text(c, path).FontSize(13).Grow(1)
								if selected {
									ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.Accent)
								}
							})
							if selected {
								row.Background(tc.Accent.Alpha(0.12))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								pm.SelectedIndex = i
								activate()
							}
						}
					case palette.PaletteSymbols:
						if len(symbols) == 0 {
							msg := "No matching symbols"
							if strings.TrimSpace(strings.TrimPrefix(pm.Query, "@")) == "" {
								msg = "Type a symbol name to search workspace..."
							}
							ui.Text(c, msg).FontSize(13).TextColor(tc.TextMuted).Padding(8, 12)
							break
						}
						for i, sym := range symbols {
							i, hit := i, sym
							selected := (i == pm.SelectedIndex)
							row := ui.Row(c.Key(fmt.Sprintf("palsym-%d-%s-%d", i, hit.Path, hit.Line))).
								Radius(10).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, icons.TablerSymbol).Width(13).Height(13).TextColor(tc.Accent)
								ui.Column(c).Grow(1).Gap(1).Children(func() {
									preview := strings.TrimSpace(hit.Content)
									if preview == "" {
										preview = filepath.Base(hit.Path)
									}
									ui.Text(c, preview).FontSize(12).FontWeight(500).SingleLine()
									ui.Text(c, fmt.Sprintf("%s:%d", hit.Path, hit.Line)).FontSize(10).TextColor(tc.TextMuted).SingleLine()
								})
								if selected {
									ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.Accent)
								}
							})
							if selected {
								row.Background(tc.Accent.Alpha(0.12))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								pm.SelectedIndex = i
								activate()
							}
						}
					case palette.PaletteLine:
						if line, col, ok := palette.LineJumpSpec(pm.Query); ok {
							label := fmt.Sprintf("Go to line %d", line)
							if col > 0 {
								label = fmt.Sprintf("Go to line %d, column %d", line, col)
							}
							row := ui.Row(c.Key("pal-line-go")).Radius(10).Padding(8, 12).Gap(8).AlignItems(ui.Center).
								Background(tc.Accent.Alpha(0.12))
							row.Children(func() {
								ui.Icon(c, icons.TablerHash).Width(13).Height(13).TextColor(tc.Accent)
								ui.Text(c, label).FontSize(13).FontWeight(500).Grow(1)
								ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.Accent)
							})
							if row.Clicked() {
								activate()
							}
						} else {
							ui.Text(c, "Type :line or :line:col (e.g. :42 or :42:7)").FontSize(13).TextColor(tc.TextMuted).Padding(8, 12)
						}
					default:
						if len(commands) == 0 {
							ui.Text(c, "No matching commands").FontSize(13).TextColor(tc.TextMuted).Padding(8, 12)
							break
						}
						for i, cmd := range commands {
							i, thisCmd := i, cmd
							selected := (i == pm.SelectedIndex)
							row := ui.Row(c.Key("palcmd-"+thisCmd.ID)).Radius(10).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Text(c, thisCmd.Label).FontSize(13).Grow(1)
								if thisCmd.Shortcut != "" {
									ui.Text(c, thisCmd.Shortcut).FontSize(11).TextColor(tc.SecondaryLabel)
								}
							})
							if selected {
								row.Background(tc.Accent.Alpha(0.12))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								pm.SelectedIndex = i
								activate()
							}
						}
					}
				})
			})

			// Footer: mode switches + hints
			ui.Row(c).Padding(8, 12).Gap(6).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				modeBtn := func(label string, m int) {
					active := mode == m
					btn := ui.Row(c.Key("palmode-"+label)).Radius(4).Padding(2, 8).AlignItems(ui.Center)
					btn.Children(func() {
						txt := ui.Text(c, label).FontSize(12)
						if active {
							txt.TextColor(tc.Accent).FontWeight(500)
						} else {
							txt.TextColor(tc.TextMuted)
						}
					})
					if active {
						btn.Background(tc.Accent.Alpha(0.1))
					} else if btn.Hovered() {
						btn.Background(tc.Text.Alpha(0.04))
					}
					if btn.Clicked() {
						pm.Mode = m
						pm.Query = strings.TrimLeft(pm.Query, "@>:")
						pm.SelectedIndex = 0
					}
				}
				modeBtn("Files", palette.PaletteFiles)
				modeBtn("Commands", palette.PaletteCommands)
				modeBtn("Symbols", palette.PaletteSymbols)
				modeBtn("Line", palette.PaletteLine)
				ui.Box(c).Grow(1)
				ui.Text(c, "↑↓ navigate · ↵ open").FontSize(12).TextColor(tc.TextMuted)
			})
		})
	})
}

func (a *AppUI) renderThemePickerModal(c *ui.Context) {
	tm := a.ThemeMgr
	if tm == nil {
		return
	}
	tc := tm.Colors()

	a.modal(c, &tm.IsOpen, func() {
		ui.Column(c).Width(420).Radius(14).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16).Gap(12).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).Clip().Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerPalette).Width(16).Height(16).TextColor(tc.Accent)
					ui.Text(c, "Theme & Appearance").FontSize(14).FontWeight(600)
				})
				ui.Box(c).Grow(1)
				if IconButton(c, tc, icons.TablerX, 13).Clicked() {
					tm.IsOpen = false
				}
			})

			// Appearance (Dark / Light)
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "APPEARANCE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
				ui.Row(c).Gap(6).Children(func() {
					for _, opt := range []struct{ ID, Label string }{
						{"dark", "Dark"},
						{"light", "Light"},
					} {
						thisOpt := opt
						sel := tm.Appearance == thisOpt.ID || (thisOpt.ID == "dark" && tm.Appearance == "")
						pill := ui.Row(c.Key("th-app-"+thisOpt.ID)).Height(28).Radius(6).Padding(0, 12).Grow(1).
							AlignItems(ui.Center).Justify(ui.Center).BorderWidth(1).Cursor(ui.CursorPointer)
						pill.Children(func() {
							ui.Text(c, thisOpt.Label).FontSize(12).FontWeight(500)
						})
						if sel {
							pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.5))
						} else {
							pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
							if pill.Hovered() {
								pill.Background(tc.Text.Alpha(0.05))
							}
						}
						if pill.Clicked() {
							tm.Appearance = thisOpt.ID
						}
					}
				})
			})

			// Palette list with color swatches
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "COLOR PALETTE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
				ui.Column(c).Gap(3).Children(func() {
					for _, p := range tm.AvailablePalettes() {
						thisP := p
						isSel := (tm.PaletteID == thisP.ID)
						row := ui.Row(c.Key("pal-pick-"+thisP.ID)).Radius(8).Padding(7, 10).Gap(8).
							AlignItems(ui.Center).Cursor(ui.CursorPointer)
						row.Children(func() {
							ui.Box(c).Size(12, 12).Radius(6).Background(theme.SwatchColor(thisP.SwatchHex))
							ui.Text(c, thisP.Name).FontSize(12).FontWeight(500).Grow(1)
							if isSel {
								ui.Icon(c, icons.TablerCheck).Width(14).Height(14).TextColor(tc.Accent)
							}
						})
						if isSel {
							row.Background(tc.Accent.Alpha(0.12))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.04))
						}
						if row.Clicked() {
							tm.PaletteID = thisP.ID
						}
					}
				})
			})

			// Content Width
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "CHAT LANE WIDTH").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
				ui.Row(c).Gap(6).Children(func() {
					for _, opt := range []struct{ ID, Label string }{
						{"comfortable", "Comfortable"},
						{"wide", "Wide"},
						{"full", "Full"},
					} {
						thisOpt := opt
						sel := tm.ChatWidth == thisOpt.ID
						pill := ui.Row(c.Key("th-w-"+thisOpt.ID)).Height(26).Radius(6).Padding(0, 10).Grow(1).
							AlignItems(ui.Center).Justify(ui.Center).BorderWidth(1).Cursor(ui.CursorPointer)
						pill.Children(func() {
							ui.Text(c, thisOpt.Label).FontSize(11).FontWeight(500)
						})
						if sel {
							pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.5))
						} else {
							pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
							if pill.Hovered() {
								pill.Background(tc.Text.Alpha(0.05))
							}
						}
						if pill.Clicked() {
							tm.ChatWidth = thisOpt.ID
						}
					}
				})
			})

			// Diff Scheme
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "DIFF SCHEME").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
				ui.Row(c).Gap(6).Children(func() {
					for _, opt := range []struct{ ID, Label string }{
						{"classic", "Classic (Green/Red)"},
						{"modern", "Modern (Blue/Orange)"},
					} {
						thisOpt := opt
						sel := tm.DiffScheme == thisOpt.ID
						pill := ui.Row(c.Key("th-diff-"+thisOpt.ID)).Height(26).Radius(6).Padding(0, 10).Grow(1).
							AlignItems(ui.Center).Justify(ui.Center).BorderWidth(1).Cursor(ui.CursorPointer)
						pill.Children(func() {
							ui.Text(c, thisOpt.Label).FontSize(11).FontWeight(500)
						})
						if sel {
							pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.5))
						} else {
							pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
							if pill.Hovered() {
								pill.Background(tc.Text.Alpha(0.05))
							}
						}
						if pill.Clicked() {
							tm.DiffScheme = thisOpt.ID
						}
					}
				})
			})

			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.PrimaryButton(c, "Done").FontSize(12).Clicked() {
					tm.IsOpen = false
				}
			})
		})
	})
}

func (a *AppUI) renderPermissionModal(c *ui.Context) {
	mgr := a.ACPMgr
	if mgr == nil || mgr.ActiveReq == nil {
		return
	}
	req := mgr.ActiveReq
	tc := a.ThemeMgr.Colors()

	// ui.Modal dereferences the open pointer every frame, so hand it a real
	// stable bool (a nil pointer panics on every permission prompt).
	a.PermissionModalOpen = true
	a.modal(c, &a.PermissionModalOpen, func() {
		ui.Column(c).Width(480).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerShieldAlert).Width(18).Height(18).TextColor(tc.Warning)
				ui.Text(c, "Agent Permission Request").FontSize(14).FontWeight(600)
			})
			summary := req.Title
			if summary == "" {
				summary = req.Description
			}
			ui.Text(c, summary).FontSize(13)
			if req.Description != "" && req.Title != "" && req.Description != req.Title {
				ui.Text(c, req.Description).FontSize(12).TextColor(tc.TextMuted)
			}
			if req.Arguments != "" {
				args := req.Arguments
				if len(args) > 600 {
					r := []rune(args)
					if len(r) > 600 {
						r = r[:600]
					}
					args = string(r) + "…"
				}
				ui.Box(c).Padding(8).Radius(6).Background(tc.Canvas).Children(func() {
					ui.Text(c, args).FontSize(11).Selectable()
				})
			}
			ui.Row(c).AlignItems(ui.Center).Gap(6).Justify(ui.End).Children(func() {
				ui.Box(c).Grow(1)
				if ui.Button(c, "Deny").Clicked() {
					_ = mgr.RespondPermission(req.SessionID, req.RequestID, false)
				}
				if ui.PrimaryButton(c, "Allow").Clicked() {
					_ = mgr.RespondPermission(req.SessionID, req.RequestID, true)
				}
			})
		})
	})
}

func (a *AppUI) renderNewFileModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	a.modal(c, &a.ShowNewFileModal, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().
			Children(func() {
				ui.Text(c, "Create New File").FontSize(14).FontWeight(600)
				in := ui.TextInput(c, &a.NewFileName).Placeholder("filename.go (or path/to/file.ext)...").Grow(1).AutoFocus()
				create := func() {
					name := strings.TrimSpace(a.NewFileName)
					if name == "" {
						return
					}
					if err := a.Workspace.CreateFile(name); err != nil {
						c.AddToast(ui.Toast{Title: "Create File Failed", Description: err.Error(), Type: "error"})
						return
					}
					a.ShowNewFileModal = false
					full := a.Workspace.ResolveFullPath(name)
					if _, err := a.Editor.OpenFile(full, name); err != nil {
						c.AddToast(ui.Toast{Title: "Open File Failed", Description: err.Error(), Type: "error"})
						return
					}
					a.SelectedFile = name
					a.Mode = ModeEditor
				}
				if in.Submitted() {
					create()
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Justify(ui.End).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowNewFileModal = false
					}
					if ui.PrimaryButton(c, "Create").Clicked() {
						create()
					}
				})
			})
	})
}

func (a *AppUI) renderNewFolderModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	a.modal(c, &a.ShowNewFolderModal, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().
			Children(func() {
				ui.Text(c, "Create New Folder").FontSize(14).FontWeight(600)
				in := ui.TextInput(c, &a.NewFolderName).Placeholder("folder_name (or path/to/folder)...").Grow(1).AutoFocus()
				createFolder := func() {
					name := strings.TrimSpace(a.NewFolderName)
					if name == "" {
						return
					}
					if err := a.Workspace.CreateFolder(name); err != nil {
						c.AddToast(ui.Toast{Title: "Create Folder Failed", Description: err.Error(), Type: "error"})
						return
					}
					a.ShowNewFolderModal = false
				}
				if in.Submitted() {
					createFolder()
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowNewFolderModal = false
					}
					if ui.PrimaryButton(c, "Create").Clicked() {
						createFolder()
					}
				})
			})
	})
}

func (a *AppUI) renderNewTaskModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	wsTarget := a.NewTaskWorkspacePath
	if wsTarget == "" && a.Workspace != nil {
		wsTarget = a.Workspace.RootDir
	}
	projectName := resolveProjectName(wsTarget)

	// Gather ACP agents
	type acpItem struct {
		ID          string
		Name        string
		Type        string
		Description string
		Command     string
		Model       string
		Available   bool
		IsInternal  bool
	}
	var acpAgents []acpItem
	// ForgeADE Internal always first
	acpAgents = append(acpAgents, acpItem{
		ID:          "agent-internal",
		Name:        "ForgeADE Internal",
		Type:        "internal",
		Description: "Internal autonomous engine — multi-turn reasoning and tool use",
		Available:   true,
		IsInternal:  true,
		Model:       a.SelectedAgentModel,
	})
	if a.ACPMgr != nil {
		for _, ag := range a.ACPMgr.Agents {
			if ag.ID == "agent-internal" {
				continue
			}
			acpAgents = append(acpAgents, acpItem{
				ID:          ag.ID,
				Name:        ag.Name,
				Type:        ag.Type,
				Description: ag.Description,
				Command:     ag.Command,
				Model:       ag.Model,
				Available:   ag.Available,
				IsInternal:  ag.Type == "internal",
			})
		}
	}

	// Gather Agent CLIs
	type cliItem struct {
		ID          string
		Name        string
		Executable  string
		Args        []string
		Description string
		Available   bool
	}
	var cliAgents []cliItem
	if a.CLIMgr != nil {
		for _, ca := range a.CLIMgr.GetAgentCLIs() {
			cliAgents = append(cliAgents, cliItem{
				ID:          ca.ID,
				Name:        ca.Name,
				Executable:  ca.Executable,
				Args:        ca.Args,
				Description: ca.Description,
				Available:   ca.Available,
			})
		}
	} else {
		for _, ca := range agent.DefaultAgentCLIs {
			cliAgents = append(cliAgents, cliItem{
				ID:          ca.ID,
				Name:        ca.Name,
				Executable:  ca.Executable,
				Args:        ca.Args,
				Description: ca.Description,
				Available:   false,
			})
		}
	}

	// Filtering
	q := strings.TrimSpace(strings.ToLower(a.NewTaskSearchQuery))
	var filteredAvailACP []acpItem
	var filteredUninstACP []acpItem
	for _, ag := range acpAgents {
		match := q == "" || strings.Contains(strings.ToLower(ag.Name), q) ||
			strings.Contains(strings.ToLower(ag.Description), q) ||
			strings.Contains(strings.ToLower(ag.Command), q) ||
			strings.Contains(strings.ToLower(ag.ID), q)
		if !match {
			continue
		}
		if ag.Available {
			filteredAvailACP = append(filteredAvailACP, ag)
		} else {
			filteredUninstACP = append(filteredUninstACP, ag)
		}
	}

	var filteredAvailCLI []cliItem
	var filteredUninstCLI []cliItem
	for _, cl := range cliAgents {
		match := q == "" || strings.Contains(strings.ToLower(cl.Name), q) ||
			strings.Contains(strings.ToLower(cl.Description), q) ||
			strings.Contains(strings.ToLower(cl.Executable), q) ||
			strings.Contains(strings.ToLower(cl.ID), q)
		if !match {
			continue
		}
		if cl.Available {
			filteredAvailCLI = append(filteredAvailCLI, cl)
		} else {
			filteredUninstCLI = append(filteredUninstCLI, cl)
		}
	}

	activeTab := a.NewTaskFilterTab
	if activeTab == "" {
		activeTab = "all"
	}

	modalWidth := float32(580)
	if winW, _ := c.Size(); winW > 0 && winW*0.92 < modalWidth {
		modalWidth = winW * 0.92
	}

	a.modal(c, &a.ShowNewTaskModal, func() {
		ui.Column(c).Width(modalWidth).Height(500).Radius(14).
			Background(tc.Sidebar).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).Clip().
			Children(func() {

				// 1. Header
				ui.Row(c).Height(46).Padding(0, 16).AlignItems(ui.Center).
					BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
					ui.Icon(c, icons.TablerTerminal).Width(15).Height(15).TextColor(tc.IconMuted)
					ui.Text(c, "New Task — "+projectName).FontSize(13).FontWeight(600).Grow(1).SingleLine()

					if IconButton(c, tc, icons.TablerX, 13).Clicked() {
						a.ShowNewTaskModal = false
					}
				})

				// 2. Search & Segmented Filter
				ui.Column(c).Background(tc.Surface.Alpha(0.25)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
					ui.Row(c).Height(36).Padding(0, 12).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerSearch).Width(14).Height(14).TextColor(tc.IconMuted)
						in := ui.TextInput(c, &a.NewTaskSearchQuery).Placeholder("Filter agents by name, command, or protocol...").Grow(1).AutoFocus()
						if in.Submitted() {
							wsPath := a.NewTaskWorkspacePath
							if wsPath == "" {
								wsPath = a.Workspace.RootDir
							}
							if len(filteredAvailACP) > 0 {
								first := filteredAvailACP[0]
								a.selectACPAgentForNewTask(first.ID, wsPath)
								return
							} else if len(filteredAvailCLI) > 0 {
								if wsPath != "" && wsPath != a.Workspace.RootDir {
									a.switchWorkspace(wsPath)
								}
								first := filteredAvailCLI[0]
								if a.CLIMgr != nil {
									a.CLIMgr.CreateSessionWithTitle(first.ID, first.Name, first.Executable, first.Args, wsPath, fmt.Sprintf("%s Task", first.Name))
								}
								a.ActiveTaskKind = "cli"
								a.Mode = ModeAgent
								a.AgentViewMode = AgentViewChat
								a.ShowNewTaskModal = false
							}
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}
						if a.NewTaskSearchQuery != "" {
							clearBtn := ui.Row(c).Size(18, 18).Radius(9).AlignItems(ui.Center).Justify(ui.Center)
							clearBtn.Children(func() {
								ui.Icon(c, icons.TablerX).Width(11).Height(11).TextColor(tc.IconMuted)
							})
							if clearBtn.Clicked() {
								a.NewTaskSearchQuery = ""
							}
						}
					})

					// Filter Tabs: All | Agent ACP | Agent CLI (Notice: NO "Interactive"!)
					ui.Row(c).Padding(0, 10, 8, 10).Gap(6).AlignItems(ui.Center).Children(func() {
						tabPill := func(key, label string, count int, onPick func()) {
							isSel := (activeTab == key)
							pill := ui.Row(c.Key("nt-tab-"+key)).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center)
							pill.Children(func() {
								txt := ui.Text(c, label).FontSize(11).FontWeight(500)
								if isSel {
									txt.TextColor(tc.Text)
								} else {
									txt.TextColor(tc.TextMuted)
								}
								cntBadge := ui.Box(c).Radius(4).Padding(1, 5).AlignItems(ui.Center).Justify(ui.Center)
								if isSel {
									cntBadge.Background(tc.Accent.Alpha(0.2))
									ui.Text(c, fmt.Sprintf("%d", count)).FontSize(10).FontWeight(700).TextColor(tc.Accent)
								} else {
									cntBadge.Background(tc.Text.Alpha(0.06))
									ui.Text(c, fmt.Sprintf("%d", count)).FontSize(10).TextColor(tc.TextMuted)
								}
							})
							if isSel {
								pill.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
							} else if pill.Hovered() {
								pill.Background(tc.Text.Alpha(0.04))
							}
							if pill.Clicked() {
								onPick()
							}
						}

						tabPill("all", "All", len(filteredAvailACP)+len(filteredAvailCLI), func() { a.NewTaskFilterTab = "all" })
						tabPill("acp", "Agent ACP", len(filteredAvailACP), func() { a.NewTaskFilterTab = "acp" })
						tabPill("cli", "Agent CLI", len(filteredAvailCLI), func() { a.NewTaskFilterTab = "cli" })
					})
				})

				// 3. Scrollable Agent Groups
				ui.Scroll(c).Grow(1).Padding(8, 12).Children(func() {
					ui.Column(c).Gap(10).Children(func() {

						// Section 1: AGENT ACP
						if activeTab == "all" || activeTab == "acp" {
							ui.Column(c).Gap(4).Children(func() {
								ui.Row(c).Padding(2, 4).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, "AGENT ACP").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.SidebarMutedForeground)
									ui.Box(c).Radius(3).Padding(1, 5).Background(tc.Accent.Alpha(0.12)).Children(func() {
										ui.Text(c, "Agent Client Protocol & Native").FontSize(9).FontWeight(600).TextColor(tc.Accent)
									})
									ui.Box(c).Grow(1)
									ui.Text(c, fmt.Sprintf("%d available", len(filteredAvailACP))).FontSize(10).TextColor(tc.TextMuted)
								})

								for _, ag := range filteredAvailACP {
									thisAg := ag
									card := ui.Row(c.Key("nt-acp-"+thisAg.ID)).Height(48).Radius(8).Padding(0, 10).Gap(10).AlignItems(ui.Center).
										Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
									card.Children(func() {
										// Icon
										ui.Box(c).Size(30, 30).Radius(6).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).
											AlignItems(ui.Center).Justify(ui.Center).Children(func() {
											if thisAg.IsInternal {
												ui.Icon(c, icons.TablerSparkles).Width(16).Height(16).TextColor(tc.Accent)
											} else {
												icons.RenderProviderIcon(c, thisAg.ID, 15)
											}
										})

										// Name & Description
										ui.Column(c).Grow(1).Gap(1).Children(func() {
											ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
												ui.Text(c, thisAg.Name).FontSize(13).FontWeight(600)
												if thisAg.IsInternal {
													ui.Box(c).Radius(3).Padding(1, 4).Background(tc.Text.Alpha(0.08)).Children(func() {
														ui.Text(c, "INTERNAL").FontSize(9).FontWeight(700).TextColor(tc.TextMuted)
													})
												} else {
													ui.Box(c).Radius(3).Padding(1, 4).Background(tc.Accent.Alpha(0.15)).Children(func() {
														ui.Text(c, "ACP").FontSize(9).FontWeight(700).TextColor(tc.Accent)
													})
												}
											})
											ui.Text(c, thisAg.Description).FontSize(11).TextColor(tc.TextMuted).SingleLine()
										})

										// Action / Chevron
										ui.Icon(c, icons.TablerChevronRight).Width(13).Height(13).TextColor(tc.IconMuted)
									})
									if card.Hovered() {
										card.Background(tc.ToolbarControlHover).BorderColor(tc.Accent.Alpha(0.4))
									}
									if card.Clicked() {
										wsPath := a.NewTaskWorkspacePath
										if wsPath == "" {
											wsPath = a.Workspace.RootDir
										}
										a.selectACPAgentForNewTask(thisAg.ID, wsPath)
									}
								}

								if len(filteredAvailACP) == 0 {
									ui.Row(c).Padding(8, 12).Children(func() {
										ui.Text(c, "No ACP agents match filter.").FontSize(12).TextColor(tc.TextMuted)
									})
								}

								// Collapsible uninstalled ACP agents
								if len(filteredUninstACP) > 0 {
									tog := ui.Row(c).Padding(4, 6).Gap(5).AlignItems(ui.Center)
									tog.Children(func() {
										arr := icons.TablerChevronDown
										if a.NewTaskUninstalledOpen {
											arr = icons.TablerChevronUp
										}
										ui.Icon(c, arr).Width(12).Height(12).TextColor(tc.IconMuted)
										ui.Text(c, fmt.Sprintf("Other ACP agents (%d not installed)", len(filteredUninstACP))).FontSize(11).TextColor(tc.TextMuted)
									})
									if tog.Hovered() {
										tog.Background(tc.Text.Alpha(0.04))
									}
									if tog.Clicked() {
										a.NewTaskUninstalledOpen = !a.NewTaskUninstalledOpen
									}

									if a.NewTaskUninstalledOpen {
										for _, ag := range filteredUninstACP {
											thisAg := ag
											row := ui.Row(c.Key("nt-uninst-acp-"+thisAg.ID)).Height(38).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center).
												Background(tc.Surface.Alpha(0.3)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.3))
											row.Children(func() {
												ui.Icon(c, icons.TablerBot).Width(14).Height(14).TextColor(tc.IconMuted)
												ui.Column(c).Grow(1).Gap(1).Children(func() {
													ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
														ui.Text(c, thisAg.Name).FontSize(12).FontWeight(500).TextColor(tc.TextMuted)
														ui.Box(c).Radius(3).Padding(0, 4).Background(tc.Border).Children(func() {
															ui.Text(c, "Not Installed").FontSize(9).TextColor(tc.TextMuted)
														})
													})
												})
											})
										}
									}
								}
							})
						}

						// Divider
						if activeTab == "all" {
							ui.Box(c).Height(1).Background(tc.Border.Alpha(0.5)).Margin(2, 0)
						}

						// Section 2: AGENT CLI (Notice: NO "Interactive"!)
						if activeTab == "all" || activeTab == "cli" {
							ui.Column(c).Gap(4).Children(func() {
								ui.Row(c).Padding(2, 4).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, "AGENT CLI").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.SidebarMutedForeground)
									ui.Box(c).Radius(3).Padding(1, 5).Background(tc.Text.Alpha(0.08)).Children(func() {
										ui.Text(c, "Terminal PTY Session").FontSize(9).FontWeight(600).TextColor(tc.TextMuted)
									})
									ui.Box(c).Grow(1)
									ui.Text(c, fmt.Sprintf("%d available", len(filteredAvailCLI))).FontSize(10).TextColor(tc.TextMuted)
								})

								for _, cl := range filteredAvailCLI {
									thisCl := cl
									card := ui.Row(c.Key("nt-cli-"+thisCl.ID)).Height(48).Radius(8).Padding(0, 10).Gap(10).AlignItems(ui.Center).
										Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
									card.Children(func() {
										// Icon
										ui.Box(c).Size(30, 30).Radius(6).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).
											AlignItems(ui.Center).Justify(ui.Center).Children(func() {
											icons.RenderProviderIcon(c, thisCl.ID, 16)
										})

										// Name & Description
										ui.Column(c).Grow(1).Gap(1).Children(func() {
											ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
												ui.Text(c, thisCl.Name).FontSize(13).FontWeight(600)
												ui.Box(c).Radius(3).Padding(1, 4).Background(tc.Text.Alpha(0.08)).Children(func() {
													ui.Text(c, "CLI").FontSize(9).FontWeight(700).TextColor(tc.TextMuted)
												})
												if thisCl.Executable != "" {
													ui.Box(c).Radius(3).Padding(1, 4).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
														ui.Text(c, thisCl.Executable).FontSize(9).TextColor(tc.TextMuted)
													})
												}
											})
											ui.Text(c, thisCl.Description).FontSize(11).TextColor(tc.TextMuted).SingleLine()
										})

										// Chevron
										ui.Icon(c, icons.TablerChevronRight).Width(13).Height(13).TextColor(tc.IconMuted)
									})
									if card.Hovered() {
										card.Background(tc.ToolbarControlHover).BorderColor(tc.Accent.Alpha(0.4))
									}
									if card.Clicked() {
										wsPath := a.NewTaskWorkspacePath
										if wsPath == "" {
											wsPath = a.Workspace.RootDir
										}
										if wsPath != "" && wsPath != a.Workspace.RootDir {
											a.switchWorkspace(wsPath)
										}
										title := fmt.Sprintf("%s Task", thisCl.Name)
										if a.CLIMgr != nil {
											a.CLIMgr.CreateSessionWithTitle(
												thisCl.ID,
												thisCl.Name,
												thisCl.Executable,
												thisCl.Args,
												wsPath,
												title,
											)
										}
										a.ActiveTaskKind = "cli"
										a.Mode = ModeAgent
										a.AgentViewMode = AgentViewChat
										a.ShowNewTaskModal = false
										if a.Window != nil {
											a.Window.Invalidate()
										}
									}
								}

								if len(filteredAvailCLI) == 0 {
									ui.Row(c).Padding(8, 12).Children(func() {
										ui.Text(c, "No Agent CLIs match filter.").FontSize(12).TextColor(tc.TextMuted)
									})
								}

								// Collapsible uninstalled CLI agents
								if len(filteredUninstCLI) > 0 {
									tog := ui.Row(c).Padding(4, 6).Gap(5).AlignItems(ui.Center)
									tog.Children(func() {
										arr := icons.TablerChevronDown
										if a.NewTaskUninstalledOpen {
											arr = icons.TablerChevronUp
										}
										ui.Icon(c, arr).Width(12).Height(12).TextColor(tc.IconMuted)
										ui.Text(c, fmt.Sprintf("Other Agent CLIs (%d not installed)", len(filteredUninstCLI))).FontSize(11).TextColor(tc.TextMuted)
									})
									if tog.Hovered() {
										tog.Background(tc.Text.Alpha(0.04))
									}
									if tog.Clicked() {
										a.NewTaskUninstalledOpen = !a.NewTaskUninstalledOpen
									}

									if a.NewTaskUninstalledOpen {
										for _, cl := range filteredUninstCLI {
											thisCl := cl
											row := ui.Row(c.Key("nt-uninst-cli-"+thisCl.ID)).Height(38).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center).
												Background(tc.Surface.Alpha(0.3)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.3))
											row.Children(func() {
												ui.Icon(c, icons.TablerTerminal).Width(14).Height(14).TextColor(tc.IconMuted)
												ui.Column(c).Grow(1).Gap(1).Children(func() {
													ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
														ui.Text(c, thisCl.Name).FontSize(12).FontWeight(500).TextColor(tc.TextMuted)
														ui.Box(c).Radius(3).Padding(0, 4).Background(tc.Border).Children(func() {
															ui.Text(c, "Executable not found: "+thisCl.Executable).FontSize(9).TextColor(tc.TextMuted)
														})
													})
												})
											})
										}
									}
								}
							})
						}
					})
				})
			})
	})
}

func (a *AppUI) renderTaskActionsMenu(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	sess := a.Agent.ActiveSession()

	a.modal(c, &a.ShowTaskMenuModal, func() {
		ui.Column(c).Width(280).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(6).Gap(2).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().
			Children(func() {
				menuItem := func(icon *ui.SVG, label string, danger bool, onPick func()) {
					var btn ui.Element
					if danger {
						btn = SidebarDangerActionButton(c, tc, icon, label)
					} else {
						btn = SidebarActionButton(c, tc, icon, label, "", false)
					}
					if btn.Clicked() {
						onPick()
					}
				}
				divider := func() {
					ui.Box(c).Height(1).FillWidth().Background(tc.Border.Alpha(0.6)).Padding(0)
				}

				menuItem(icons.TablerPencil, "Rename task", false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						a.openRenameSessionModal(sess.ID, sess.Title, false)
					} else {
						a.ShowRenameModal = true
					}
				})

				archiveLabel := "Archive task"
				if sess != nil && sess.Archived {
					archiveLabel = "Unarchive task"
				}
				menuItem(icons.TablerArchive, archiveLabel, false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						a.Agent.ToggleArchive(sess.ID)
					}
				})

				menuItem(icons.TablerTerminal, "Toggle terminal", false, func() {
					a.ShowTaskMenuModal = false
					a.ShowBottomTerminal = !a.ShowBottomTerminal
				})

				divider()

				menuItem(icons.TablerExternalLink, "Open in Finder", false, func() {
					a.ShowTaskMenuModal = false
					if a.Workspace != nil && a.Workspace.RootDir != "" {
						mygo.Shell.ShowItemInFolder(a.Workspace.RootDir)
					}
				})

				menuItem(icons.TablerFolder, "Open sandbox folder", false, func() {
					a.ShowTaskMenuModal = false
					sandboxDir := filepath.Join(a.Workspace.RootDir, ".forge", "sandbox")
					if _, err := os.Stat(sandboxDir); err == nil {
						mygo.Shell.ShowItemInFolder(sandboxDir)
					} else {
						mygo.Shell.ShowItemInFolder(a.Workspace.RootDir)
					}
				})

				menuItem(icons.TablerCopy, "Copy sandbox path", false, func() {
					a.ShowTaskMenuModal = false
					sandboxDir := filepath.Join(a.Workspace.RootDir, ".forge", "sandbox")
					mygo.Clipboard.WriteText(sandboxDir)
					c.AddToast(ui.Toast{Title: "Copied!", Description: "Sandbox path copied to clipboard", Type: "success"})
				})

				menuItem(icons.TablerGitFork, "Fork conversation", false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						forked := a.Agent.ForkSession(sess)
						if forked != nil {
							a.Agent.SaveSessionToDisk(forked)
						}
					}
				})

				menuItem(icons.TablerCopy, "Copy path", false, func() {
					a.ShowTaskMenuModal = false
					if a.Workspace != nil {
						mygo.Clipboard.WriteText(a.Workspace.RootDir)
						c.AddToast(ui.Toast{Title: "Copied!", Description: "Workspace path copied to clipboard", Type: "success"})
					}
				})

				menuItem(icons.TablerCopy, "Copy log path", false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil && a.Workspace != nil {
						logPath := filepath.Join(a.Workspace.RootDir, ".forge", "sessions", sess.ID+".json")
						mygo.Clipboard.WriteText(logPath)
						c.AddToast(ui.Toast{Title: "Copied!", Description: "Log path copied to clipboard", Type: "success"})
					}
				})

				menuItem(icons.TablerCopy, "Copy session ID", false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						mygo.Clipboard.WriteText(sess.ID)
						c.AddToast(ui.Toast{Title: "Copied!", Description: "Session ID copied to clipboard", Type: "success"})
					}
				})

				divider()

				menuItem(icons.TablerHelpCircle, "Report issue", false, func() {
					a.ShowTaskMenuModal = false
					mygo.Shell.OpenExternal("https://github.com/hasdev/forge-ade/issues")
				})

				menuItem(icons.TablerTrash, "Delete task", true, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						a.confirmDialogLabeled("Delete Task?", fmt.Sprintf("“%s” and its conversation history will be permanently removed.", sess.Title), "Delete", func() {
							a.Agent.DeleteSession(sess.ID)
						}, nil)
					}
				})
			})
	})
}

func (a *AppUI) renderRenameModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	a.modal(c, &a.ShowRenameModal, func() {
		ui.Column(c).Width(380).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().
			Children(func() {
				ui.Text(c, "Rename Task").FontSize(14).FontWeight(600)
				in := ui.TextInput(c, &a.RenameSessionTitle).Grow(1).AutoFocus()
				save := func() {
					title := strings.TrimSpace(a.RenameSessionTitle)
					if title == "" {
						return
					}
					if a.RenameSessionIsCLI && a.CLIMgr != nil && a.RenameSessionID != "" {
						a.CLIMgr.RenameSession(a.RenameSessionID, title)
					} else if a.Agent != nil {
						targetID := a.RenameSessionID
						if targetID == "" {
							if sess := a.Agent.ActiveSession(); sess != nil {
								targetID = sess.ID
							}
						}
						if targetID != "" {
							a.Agent.RenameSession(targetID, title)
						}
					}
					a.RenameSessionID = ""
					a.RenameSessionIsCLI = false
					a.ShowRenameModal = false
				}
				if in.Submitted() {
					save()
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowRenameModal = false
					}
					if ui.PrimaryButton(c, "Save").Clicked() {
						save()
					}
				})
			})
	})
}

func (a *AppUI) renderBranchModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	a.modal(c, &a.ShowBranchModal, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().
			Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerGitBranch).Width(16).Height(16).TextColor(tc.Accent)
						ui.Text(c, "Switch Git Branch").FontSize(14).FontWeight(600)
					})
					ui.Box(c).Grow(1)
					if IconButton(c, tc, icons.TablerX, 13).Clicked() {
						a.ShowBranchModal = false
					}
				})

				in := ui.TextInput(c, &a.BranchSearchQuery).Placeholder("Search or create branch...").Grow(1).AutoFocus()
				query := strings.TrimSpace(a.BranchSearchQuery)
				var snap git.GitSnapshot
				if a.Git != nil {
					// Read through the snapshot: background Refresh writes the
					// live fields, so direct reads race the render thread.
					snap = a.Git.Snapshot()
				}
				if in.Submitted() && query != "" {
					found := false
					for _, b := range snap.Branches {
						if b == query {
							found = true
							break
						}
					}
					if found {
						a.checkoutBranch(query)
					} else {
						a.createBranch(query)
					}
				}

				currentBranch := snap.Branch
				branches := snap.Branches

				var filtered []string
				for _, b := range branches {
					if query == "" || strings.Contains(strings.ToLower(b), strings.ToLower(query)) {
						filtered = append(filtered, b)
					}
				}

				ui.Scroll(c).Height(minF(220, float32(len(filtered))*32+40)).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						for _, b := range filtered {
							branch := b
							isCurrent := (branch == currentBranch)
							row := ui.Row(c.Key("branch-row-"+branch)).Height(30).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, icons.TablerGitBranch).Width(13).Height(13).TextColor(tc.IconMuted)
								ui.Text(c, branch).FontSize(12).FontWeight(500).Grow(1)
								if isCurrent {
									ui.Icon(c, icons.TablerCheck).Width(14).Height(14).TextColor(tc.Success)
								}
							})
							if isCurrent {
								row.Background(tc.Accent.Alpha(0.12))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() && !isCurrent {
								a.checkoutBranch(branch)
							}
						}

						if query != "" {
							exists := false
							for _, b := range branches {
								if b == query {
									exists = true
									break
								}
							}
							if !exists {
								createRow := ui.Row(c.Key("branch-create-"+query)).Height(32).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center).
									Background(tc.Accent.Alpha(0.08))
								createRow.Children(func() {
									ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.Accent)
									ui.Text(c, fmt.Sprintf("Create branch '%s'", query)).FontSize(12).FontWeight(500).TextColor(tc.Accent)
								})
								if createRow.Hovered() {
									createRow.Background(tc.Accent.Alpha(0.16))
								}
								if createRow.Clicked() {
									a.createBranch(query)
								}
							}
						}
					})
				})
			})
	})
}

// renderNewBoardTaskModal renders the task creation dialog for the Workflow Board.
func (a *AppUI) renderNewBoardTaskModal(c *ui.Context) {
	if !a.ShowNewBoardTaskModal {
		return
	}
	tc := a.ThemeMgr.Colors()

	a.modal(c, &a.ShowNewBoardTaskModal, func() {
		modal := ui.Column(c).Width(460).Radius(16).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).Clip()

		modal.Children(func() {
			// Header
			ui.Row(c).Height(48).Padding(0, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).
				Background(tc.SurfaceRaised.Alpha(0.5)).Children(func() {
				ui.Box(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent.Alpha(0.12)).Children(func() {
					ui.Icon(c, icons.TablerLayoutKanban).Width(14).Height(14).TextColor(tc.Accent)
				})
				ui.Text(c, "New Workflow Task").FontSize(13).FontWeight(600).TextColor(tc.Text)
				ui.Box(c).Grow(1)
				closeBtn := ui.Row(c).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.06))
				}
				if closeBtn.Clicked() {
					a.ShowNewBoardTaskModal = false
				}
			})

			// Form body
			ui.Column(c).Padding(16, 18).Gap(12).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "TITLE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.NewBoardTaskTitle).Placeholder("e.g. Implement user authentication").Grow(1)
				})

				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "DESCRIPTION").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.NewBoardTaskDesc).Placeholder("Details, context, or acceptance criteria...").Grow(1)
				})

				// Priority selector
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "PRIORITY").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.Row(c).Gap(6).Children(func() {
						priorities := []struct {
							id    string
							label string
							color ui.Color
						}{
							{"low", "Low", tc.TextMuted},
							{"medium", "Medium", tc.Accent},
							{"high", "High", tc.Warning},
							{"urgent", "Urgent", tc.Error},
						}
						if a.NewBoardTaskPriority == "" {
							a.NewBoardTaskPriority = "medium"
						}
						for _, p := range priorities {
							thisP := p
							isSel := a.NewBoardTaskPriority == thisP.id
							pill := ui.Row(c.Key("prio-"+thisP.id)).Height(28).Radius(6).Padding(0, 10).AlignItems(ui.Center)
							pill.Children(func() {
								ui.Box(c).Size(6, 6).Radius(3).Background(thisP.color)
								ui.Text(c, thisP.label).FontSize(11).FontWeight(500)
							})
							if isSel {
								pill.Background(thisP.color.Alpha(0.18)).BorderWidth(1).BorderColor(thisP.color.Alpha(0.5))
							} else {
								pill.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
							}
							if pill.Clicked() {
								a.NewBoardTaskPriority = thisP.id
							}
						}
					})
				})

				// Status column selector
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "COLUMN").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.Row(c).Gap(5).Children(func() {
						statuses := []struct {
							id    string
							label string
						}{
							{"todo", "To Do"},
							{"in_progress", "In Progress"},
							{"in_review", "In Review"},
							{"in_testing", "Testing"},
							{"done", "Done"},
						}
						if a.NewBoardTaskStatus == "" {
							a.NewBoardTaskStatus = "todo"
						}
						for _, s := range statuses {
							thisS := s
							isSel := a.NewBoardTaskStatus == thisS.id
							pill := ui.Row(c.Key("col-"+thisS.id)).Height(28).Radius(6).Padding(0, 8).AlignItems(ui.Center)
							pill.Children(func() {
								ui.Text(c, thisS.label).FontSize(11).FontWeight(500)
							})
							if isSel {
								pill.Background(tc.Accent.Alpha(0.18)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.5))
							} else {
								pill.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
							}
							if pill.Clicked() {
								a.NewBoardTaskStatus = thisS.id
							}
						}
					})
				})
			})

			// Footer actions
			ui.Row(c).Padding(12, 18).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.6)).
				Background(tc.SurfaceRaised.Alpha(0.3)).Children(func() {
				ui.Box(c).Grow(1)
				cancelBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				cancelBtn.Children(func() {
					ui.Text(c, "Cancel").FontSize(11).TextColor(tc.TextMuted)
				})
				if cancelBtn.Clicked() {
					a.ShowNewBoardTaskModal = false
				}

				createBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 14).Gap(5).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent).BorderWidth(1).BorderColor(tc.Accent)
				createBtn.Children(func() {
					ui.Icon(c, icons.TablerPlus).Width(12).Height(12).TextColor(ui.RGBA(255, 255, 255, 1.0))
					ui.Text(c, "Create Task").FontSize(11).FontWeight(600).TextColor(ui.RGBA(255, 255, 255, 1.0))
				})
				if createBtn.Hovered() {
					createBtn.Background(tc.Accent.Alpha(0.85))
				}
				if createBtn.Clicked() {
					if strings.TrimSpace(a.NewBoardTaskTitle) != "" {
						if a.Board == nil {
							a.RefreshBoard()
						}
						if a.Board != nil {
							_, err := a.Board.AddTask(a.NewBoardTaskTitle, a.NewBoardTaskDesc, a.NewBoardTaskStatus, a.NewBoardTaskPriority)
							if err != nil {
								c.AddToast(ui.Toast{Title: "Error", Description: err.Error(), Type: "error"})
							} else {
								c.AddToast(ui.Toast{Title: "Task Created", Description: a.NewBoardTaskTitle, Type: "success"})
								a.NewBoardTaskTitle = ""
								a.NewBoardTaskDesc = ""
								a.ShowNewBoardTaskModal = false
							}
						}
					}
				}
			})
		})
	})
}

// jumpDocumentToLine scrolls an open document to a 1-based line:column.
func (a *AppUI) jumpDocumentToLine(doc *editor.OpenDocument, line, col int) {
	if doc == nil || line <= 0 {
		return
	}
	text := doc.Editor.Buffer.String()
	offset := 0
	lines := strings.Split(text, "\n")
	if line-1 < len(lines) {
		for i := 0; i < line-1; i++ {
			offset += len(lines[i]) + 1
		}
	}
	length := 0
	if col > 0 && line-1 < len(lines) && col-1 <= len(lines[line-1]) {
		offset += col - 1
		length = 1
	}
	n := doc.Editor.Buffer.Len()
	if offset > n {
		offset = n
	}
	doc.Editor.Select(offset, offset+length)
	doc.Editor.GoTo(doc.Editor.CursorPos())
}

// jumpActiveDocumentToLine jumps the active editor to line:col.
func (a *AppUI) jumpActiveDocumentToLine(line, col int) {
	doc := a.Editor.ActiveDoc()
	if doc == nil {
		return
	}
	a.jumpDocumentToLine(doc, line, col)
	a.Mode = ModeEditor
}

// renderPeekModal renders the Peek Definition popup when multiple symbol definitions exist.
func (a *AppUI) renderPeekModal(c *ui.Context) {
	if !a.ShowPeekModal || len(a.PeekResults) == 0 {
		return
	}
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		a.ShowPeekModal = false
		return
	}

	a.modal(c, &a.ShowPeekModal, func() {
		modal := ui.Column(c).Width(600).MaxHeight(420).Radius(12).
			Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 16, 40, -10, ui.RGBA(0, 0, 0, 0.6)).Clip()

		modal.Children(func() {
			// Header
			ui.Row(c).Height(40).Padding(0, 14).Gap(8).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerSearch).Width(14).Height(14).TextColor(tc.Accent)
				ui.Text(c, fmt.Sprintf("Peek Definition: %s", a.PeekSymbol)).FontSize(12).FontWeight(600).TextColor(tc.Text)
				ui.Box(c).Grow(1)
				ui.Text(c, fmt.Sprintf("%d matches", len(a.PeekResults))).FontSize(11).TextColor(tc.TextMuted)

				closeBtn := ui.Row(c).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if closeBtn.Clicked() {
					a.ShowPeekModal = false
				}
			})

			// Results list
			ui.Scroll(c).MaxHeight(340).Children(func() {
				ui.Column(c).Padding(8, 12).Gap(4).Children(func() {
					for i, m := range a.PeekResults {
						thisM := m
						row := ui.Row(c.Key(fmt.Sprintf("peek-%d", i))).Radius(6).Padding(8, 10).Gap(8).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))

						row.Children(func() {
							ui.Column(c).Grow(1).Gap(2).Children(func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, filepath.Base(thisM.Path)).FontSize(12).FontWeight(600).TextColor(tc.Text)
									ui.Text(c, fmt.Sprintf("Ln %d, Col %d", thisM.Line, thisM.Col)).FontSize(10).TextColor(tc.Accent)
									ui.Text(c, thisM.Path).FontSize(10).TextColor(tc.TextMuted).Grow(1).SingleLine()
								})
								ui.Text(c, thisM.Preview).FontSize(11).Font("monospace").TextColor(tc.Text).SingleLine()
							})
							jumpBtn := ui.Row(c.Key(fmt.Sprintf("jump-%d", i))).Height(24).Radius(5).Padding(0, 8).AlignItems(ui.Center).
								Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
							jumpBtn.Children(func() {
								ui.Text(c, "Jump").FontSize(10).FontWeight(600).TextColor(tc.Accent)
							})
							if jumpBtn.Clicked() || row.Clicked() {
								a.ShowPeekModal = false
								a.openSearchResult(thisM.Path, thisM.Line, "")
							}
						})
						if row.Hovered() {
							row.Background(tc.Text.Alpha(0.04))
						}
					}
				})
			})
		})
	})
}

func (a *AppUI) renderRenameSymbolModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	a.modal(c, &a.ShowRenameSymbolModal, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(12).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Clip().
			Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, icons.TablerPencil).Width(16).Height(16).TextColor(tc.Accent)
					ui.Text(c, "Rename Symbol").FontSize(14).FontWeight(600).TextColor(tc.Text)
				})
				ui.Text(c, fmt.Sprintf("Rename '%s' across current file", a.RenameSymbolTarget)).FontSize(11).TextColor(tc.TextMuted)
				in := ui.TextInput(c, &a.RenameSymbolNewName).Grow(1).AutoFocus()
				doRename := func() {
					oldSym := strings.TrimSpace(a.RenameSymbolTarget)
					newSym := strings.TrimSpace(a.RenameSymbolNewName)
					if oldSym == "" || newSym == "" || oldSym == newSym {
						a.ShowRenameSymbolModal = false
						return
					}
					count := 0
					if doc := a.Editor.ActiveDoc(); doc != nil && doc.Editor != nil && doc.Editor.Buffer != nil {
						text := doc.Editor.Buffer.String()
						if strings.Contains(text, oldSym) {
							count = strings.Count(text, oldSym)
							newText := strings.ReplaceAll(text, oldSym, newSym)
							doc.Editor.Buffer.Edit(mygoeditor.Edit{Start: 0, End: doc.Editor.Buffer.Len(), Text: newText})
						}
					}
					c.AddToast(ui.Toast{
						Title:       "Symbol Renamed",
						Description: fmt.Sprintf("Renamed %d occurrences of '%s' to '%s'", count, oldSym, newSym),
						Type:        "success",
					})
					a.ShowRenameSymbolModal = false
				}
				if in.Submitted() {
					doRename()
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowRenameSymbolModal = false
					}
					if ui.PrimaryButton(c, "Rename").Clicked() {
						doRename()
					}
				})
			})
	})
}
