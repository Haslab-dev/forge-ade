package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	corestudio "github.com/hasdev/forge-ade/core/studio"
)

// activeStudioWorkflow returns the active customizable WorkflowPipeline for the
// current Studio project (Image or Video), lazily seeding a built-in preset.
func (a *AppUI) activeStudioWorkflow(project *corestudio.StudioProject) *corestudio.WorkflowPipeline {
	isVideo := project != nil && project.Type == corestudio.ArtifactTypeVideo
	if isVideo {
		if a.StudioVideoWorkflow == nil {
			wf := corestudio.PresetTextToVideoDirect()
			a.StudioVideoWorkflow = &wf
		}
		return a.StudioVideoWorkflow
	}
	if a.StudioImageWorkflow == nil {
		wf := corestudio.PresetTextToImage()
		for i := range wf.Steps {
			if wf.Steps[i].Type == corestudio.StepTypePrompt {
				if a.StudioPositivePrompt != "" {
					wf.Steps[i].SetParam("positive", a.StudioPositivePrompt)
				}
				if a.StudioNegativePrompt != "" {
					wf.Steps[i].SetParam("negative", a.StudioNegativePrompt)
				}
			}
		}
		a.StudioImageWorkflow = &wf
	}
	return a.StudioImageWorkflow
}

func (a *AppUI) setActiveStudioWorkflow(project *corestudio.StudioProject, wf *corestudio.WorkflowPipeline) {
	if wf == nil {
		return
	}
	isVideo := project != nil && project.Type == corestudio.ArtifactTypeVideo
	if isVideo {
		a.StudioVideoWorkflow = wf
	} else {
		a.StudioImageWorkflow = wf
	}
	a.StudioWorkflowStepDrafts = nil
	a.StudioWorkflowError = ""
	if wf.Mode == "template" {
		a.StudioWorkflowJSONDraft = wf.TemplateJSON
	}
}

// bindStepString synchronizes a step param with a persistent draft map so MyGo
// TextInput / TextArea widgets can edit map[string]interface{} values smoothly.
func (a *AppUI) bindStepString(step *corestudio.WorkflowStep, paramKey, fallback string, render func(val *string)) {
	if a.StudioWorkflowStepDrafts == nil {
		a.StudioWorkflowStepDrafts = map[string]string{}
	}
	draftKey := step.ID + ":" + paramKey
	cur, ok := a.StudioWorkflowStepDrafts[draftKey]
	if !ok {
		cur = step.GetString(paramKey, fallback)
	}
	render(&cur)
	a.StudioWorkflowStepDrafts[draftKey] = cur
	step.SetParam(paramKey, cur)
	if step.Type == corestudio.StepTypePrompt {
		if paramKey == "positive" {
			a.StudioPositivePrompt = cur
		} else if paramKey == "negative" {
			a.StudioNegativePrompt = cur
		}
	}
}

// ---------------------------------------------------------------------------
// Studio Native Design System Primitives (matching ForgeADE controls.go)
// ---------------------------------------------------------------------------

func wfPrimaryBtn(c *ui.Context, tc theme.ThemeColors, key string, icon *ui.SVG, label, kbd string, loading bool) ui.Element {
	btn := ui.Row(c.Key(key)).Height(30).Padding(0, 14).Radius(8).Gap(6).
		AlignItems(ui.Center).Justify(ui.Center).Background(tc.Accent).Cursor(ui.CursorPointer)
	if btn.Hovered() {
		btn.Background(tc.Accent.Alpha(0.88))
	}
	btn.Children(func() {
		if loading {
			ui.Spinner(c).Size(12, 12).Label("")
		} else if icon != nil {
			ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.AccentForeground)
		}
		ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(tc.AccentForeground).SingleLine()
		if kbd != "" && !loading {
			ui.Text(c, kbd).FontSize(10.5).FontWeight(500).TextColor(tc.AccentForeground.Alpha(0.72)).SingleLine()
		}
	})
	return btn
}

func wfSecondaryBtn(c *ui.Context, tc theme.ThemeColors, key string, icon *ui.SVG, label string) ui.Element {
	btn := ui.Row(c.Key(key)).Height(28).Padding(0, 10).Radius(7).Gap(6).
		AlignItems(ui.Center).Justify(ui.Center).
		Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).Cursor(ui.CursorPointer)
	if btn.Hovered() {
		btn.Background(tc.ToolbarControlHover)
	}
	btn.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.IconMuted)
		}
		ui.Text(c, label).FontSize(11.5).FontWeight(500).TextColor(tc.Text).SingleLine()
	})
	return btn
}

func wfGhostIconBtn(c *ui.Context, tc theme.ThemeColors, key string, icon *ui.SVG, tooltip string, danger bool) ui.Element {
	btn := ui.Row(c.Key(key)).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
	hovered := btn.Hovered()
	if hovered {
		if danger {
			btn.Background(tc.Error.Alpha(0.12))
		} else {
			btn.Background(tc.Text.Alpha(0.07))
		}
	}
	if tooltip != "" {
		btn.Tooltip(tooltip)
	}
	btn.Children(func() {
		col := tc.IconMuted
		if hovered {
			if danger {
				col = tc.Error
			} else {
				col = tc.Text
			}
		}
		ui.Icon(c, icon).Width(13).Height(13).TextColor(col)
	})
	return btn
}

func wfFieldLabel(c *ui.Context, tc theme.ThemeColors, text string) {
	ui.Text(c, text).FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
}

func wfInputWrap(c *ui.Context, tc theme.ThemeColors, content func()) {
	wrap := ui.Row(c).Height(36).Radius(9).Padding(0, 12).Gap(8).AlignItems(ui.Center).
		Background(tc.Input).BorderWidth(1).BorderColor(tc.Border)
	if wrap.Hovered() {
		wrap.BorderColor(tc.Accent.Alpha(0.45))
	}
	wrap.Children(content)
}

func wfTextAreaWrap(c *ui.Context, tc theme.ThemeColors, content func()) {
	wrap := ui.Column(c).Radius(10).Padding(10, 12).Gap(6).
		Background(tc.Input).BorderWidth(1).BorderColor(tc.Border)
	if wrap.Hovered() {
		wrap.BorderColor(tc.Accent.Alpha(0.45))
	}
	wrap.Children(content)
}

func wfValueBadge(c *ui.Context, tc theme.ThemeColors, text string) {
	ui.Row(c).Height(24).Padding(0, 8).Radius(6).Background(tc.ToolbarControl).
		BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(11).FontWeight(600).TextColor(tc.Text).SingleLine()
	})
}

func wfChoiceGroup(c *ui.Context, tc theme.ThemeColors, keyPrefix string, options []string, current string, onSelect func(string)) {
	ui.Row(c.Key(keyPrefix+"-grp")).Height(30).Padding(3, 3).Gap(2).Radius(8).
		Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).AlignItems(ui.Center).Children(func() {
		for _, opt := range options {
			active := current == opt
			item := ui.Row(c.Key(keyPrefix+"-"+opt)).Height(22).Padding(0, 10).Radius(5).
				AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
			if active {
				item.Background(tc.Accent.Alpha(0.16)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.4))
				item.Children(func() {
					ui.Text(c, opt).FontSize(11.5).FontWeight(600).TextColor(tc.Accent).SingleLine()
				})
			} else {
				if item.Hovered() {
					item.Background(tc.Text.Alpha(0.05))
				}
				item.Children(func() {
					ui.Text(c, opt).FontSize(11.5).FontWeight(500).TextColor(tc.TextMuted).SingleLine()
				})
			}
			if item.Clicked() && !active {
				onSelect(opt)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Main 3-Pane Full-Native Workflow Studio Surface
// ---------------------------------------------------------------------------

// renderStudioNativeWorkflow renders the 100% Pure-Go MyGo 3-pane Workflow Studio
// for Image & Video generation (Pipeline Builder + Injectable JSON Templates).
func (a *AppUI) renderStudioNativeWorkflow(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject) {
	a.closeStudioNodeWebView()

	cfg := corestudio.LoadStudioConfig()
	wf := a.activeStudioWorkflow(project)
	if a.StudioWorkflowRightTab == "" {
		a.StudioWorkflowRightTab = "preview"
	}

	if c.Shortcut(ui.Cmd, ui.KeyEnter) && !a.StudioIsGenerating {
		a.studioQueuePrompt()
	}

	winW, _ := c.Size()
	compact := winW > 0 && winW < 1120

	ui.Column(c.Key("native-workflow-studio")).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
		// -----------------------------------------------------------------
		// 1. TOP TOOLBAR (Clean, aligned 40px bar)
		// -----------------------------------------------------------------
		ui.Row(c.Key("native-wf-topbar")).Height(40).Padding(0, 14).Gap(8).AlignItems(ui.Center).
			Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {

			// Collapse / Expand Left Presets Rail
			sideBtn := ToggleIconButton(c, tc, icons.TablerPanelLeft, 14, !a.StudioWorkflowLeftCollapsed)
			sideBtn.Tooltip("Toggle Workflows & Stages Sidebar")
			if sideBtn.Clicked() {
				a.StudioWorkflowLeftCollapsed = !a.StudioWorkflowLeftCollapsed
			}

			// Segmented Mode Switch: Pipeline | JSON Template
			ui.Row(c.Key("wf-mode-switcher")).Height(28).Padding(2, 2).Gap(2).Radius(7).
				Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).AlignItems(ui.Center).Children(func() {
				modeTab := func(key, label, mode string) {
					active := wf.Mode == mode
					btn := ui.Row(c.Key(key)).Height(22).Padding(0, 10).Radius(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					if active {
						btn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						btn.Children(func() {
							ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
						})
					} else {
						if btn.Hovered() {
							btn.Background(tc.Text.Alpha(0.05))
						}
						btn.Children(func() {
							ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(tc.TextMuted).SingleLine()
						})
					}
					if btn.Clicked() && wf.Mode != mode {
						wf.Mode = mode
						if mode == "template" {
							if strings.TrimSpace(wf.TemplateJSON) == "" {
								var preset corestudio.WorkflowPipeline
								if project != nil && project.Type == corestudio.ArtifactTypeVideo {
									preset = corestudio.PresetVideoJSONTemplate()
								} else {
									preset = corestudio.PresetImageJSONTemplate()
								}
								wf.TemplateJSON = preset.TemplateJSON
								wf.Variables = preset.Variables
							}
							a.StudioWorkflowJSONDraft = wf.TemplateJSON
						}
					}
				}
				modeTab("wf-mode-pipeline", "Pipeline", "pipeline")
				modeTab("wf-mode-template", "JSON Template", "template")
			})

			// Quiet Endpoint Indicator (opens Studio Settings)
			endpoint := cfg.ImageEndpoint
			if project != nil && project.Type == corestudio.ArtifactTypeVideo {
				endpoint = cfg.VideoEndpoint
			}
			if strings.TrimSpace(endpoint) == "" {
				endpoint = "Local Preview"
			}
			apiBtn := ui.Row(c.Key("wf-api-chip")).Height(30).Padding(0, 10).Radius(7).Gap(7).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			if apiBtn.Hovered() {
				apiBtn.Background(tc.Text.Alpha(0.05))
			}
			apiBtn.Tooltip("Configure Image & Video endpoints in Studio Settings")
			apiBtn.Children(func() {
				ui.Box(c).Size(7, 7).Radius(4).Background(tc.Success)
				ui.Text(c, endpoint).FontSize(12).TextColor(tc.TextMuted).SingleLine()
			})
			if apiBtn.Clicked() {
				a.openSettings(settings.SettingsTabStudio)
			}

			ui.Box(c).Grow(1)

			// Secondary Actions: Import / Export
			if wfSecondaryBtn(c, tc, "wf-btn-import", icons.TablerFolderOpen, "Import").Clicked() {
				go a.studioImportWorkflowJSONFile(project)
			}
			if wfSecondaryBtn(c, tc, "wf-btn-export-json", icons.TablerDeviceFloppy, "Export").Clicked() {
				go a.studioExportWorkflowJSONFile(wf)
			}

			// Primary CTA: Generate
			genLabel := "Generate"
			if a.StudioIsGenerating {
				genLabel = "Running..."
			}
			if wfPrimaryBtn(c, tc, "btn-queue-prompt", icons.TablerSparkles, genLabel, "⌘↵", a.StudioIsGenerating).Clicked() && !a.StudioIsGenerating {
				a.studioQueuePrompt()
			}
		})

		// Subtle Progress Strip
		if a.StudioIsGenerating {
			ui.Row(c.Key("native-wf-progress")).Height(30).Padding(0, 16).Gap(8).AlignItems(ui.Center).
				Background(tc.Accent.Alpha(0.08)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Accent.Alpha(0.2)).Children(func() {
				ui.Spinner(c).Size(12, 12).Label("")
				ui.Text(c, a.StudioGenerationStatus).FontSize(12).FontWeight(500).TextColor(tc.Text).SingleLine()
				ui.Box(c).Grow(1)
				pct := max(0, min(100, int(a.StudioGenerationProgress*100)))
				ui.Text(c, fmt.Sprintf("%d%%", pct)).FontSize(12).FontWeight(600).TextColor(tc.Accent)
			})
		}

		// Clean Error Banner
		if a.StudioWorkflowError != "" {
			ui.Row(c.Key("native-wf-error")).Padding(8, 16).Gap(10).AlignItems(ui.Center).
				Background(tc.Error.Alpha(0.1)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Error.Alpha(0.25)).Children(func() {
				ui.Text(c, a.StudioWorkflowError).FontSize(12).FontWeight(500).TextColor(tc.Error).Grow(1)
				if wfGhostIconBtn(c, tc, "wf-err-dismiss", icons.TablerX, "Dismiss", false).Clicked() {
					a.StudioWorkflowError = ""
				}
			})
		}

		// -----------------------------------------------------------------
		// 2. 3-PANE WORKSPACE
		// -----------------------------------------------------------------
		ui.Row(c.Key("native-wf-body")).Grow(1).FillHeight().Children(func() {
			// LEFT SIDEBAR: Flat native list of Workflows & Stages (collapsible)
			if !a.StudioWorkflowLeftCollapsed {
				leftW := float32(220)
				if compact {
					leftW = 192
				}
				a.renderStudioWorkflowLeftPane(c, tc, project, wf, leftW)
			}

			// CENTER PANE: Workflow Stage Editor / JSON Template Injector
			centerW := float32(450)
			if compact {
				centerW = 370
			}
			ui.Column(c.Key("native-wf-center")).Width(centerW).FillHeight().
				Background(tc.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
				if wf.Mode == "template" {
					a.renderStudioTemplateInjectorPane(c, tc, project, wf)
				} else {
					a.renderStudioPipelineBuilderPane(c, tc, wf)
				}
			})

			// RIGHT HERO PANE: Preview Canvas, Gallery & Logs
			a.renderStudioWorkflowRightPane(c, tc, project, wf)
		})
	})
}

func (a *AppUI) renderStudioWorkflowLeftPane(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject, wf *corestudio.WorkflowPipeline, width float32) {
	isVideo := project != nil && project.Type == corestudio.ArtifactTypeVideo
	domainType := "image"
	sectionTitle := "IMAGE WORKFLOWS"
	if isVideo {
		domainType = "video"
		sectionTitle = "VIDEO WORKFLOWS"
	}

	ui.Column(c.Key("native-wf-left")).Width(width).FillHeight().
		Background(tc.Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).Children(func() {

		ui.Scroll(c.Key("native-wf-left-scroll")).Grow(1).Children(func() {
			ui.Column(c).Padding(16, 10, 20, 10).Gap(18).Children(func() {
				// Section 1: Domain-specific Workflows ONLY (no mixing Image & Video)
				ui.Column(c).Gap(3).Children(func() {
					ui.Row(c).Padding(0, 10, 6, 10).Children(func() {
						SectionLabel(c, tc, sectionTitle)
					})

					presets := corestudio.PresetsForPipelineType(domainType)
					for idx := range presets {
						preset := presets[idx]
						active := wf.ID == preset.ID
						ic := icons.TablerPhoto
						if isVideo {
							ic = icons.TablerMovie
						}
						if preset.Mode == "template" {
							ic = icons.TablerBraces
						}

						row := ui.Row(c.Key("wf-preset-"+preset.ID)).Height(32).Radius(6).Padding(0, 10).Gap(9).
							AlignItems(ui.Center).Cursor(ui.CursorPointer)
						if active {
							row.Background(tc.Text.Alpha(0.08))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.05))
						}
						row.Tooltip(preset.Description)
						row.Children(func() {
							iconCol := tc.IconMuted
							txtCol := tc.TextMuted
							fw := 500
							if active {
								iconCol = tc.Text
								txtCol = tc.Text
								fw = 600
							}
							ui.Icon(c, ic).Width(15).Height(15).TextColor(iconCol)
							ui.Text(c, preset.Name).FontSize(13).FontWeight(fw).TextColor(txtCol).SingleLine().Grow(1)
						})
						if row.Clicked() {
							cloned := preset
							a.setActiveStudioWorkflow(project, &cloned)
						}
					}
				})

				// Section 2: Domain-specific Add Stage (only in Pipeline mode)
				if wf.Mode == "pipeline" {
					ui.Column(c).Gap(3).Children(func() {
						ui.Row(c).Padding(0, 10, 6, 10).Children(func() {
							SectionLabel(c, tc, "ADD STAGE")
						})

						addStageRow := func(key, title string, ic *ui.SVG, st corestudio.WorkflowStepType) {
							row := ui.Row(c.Key("add-stage-"+key)).Height(30).Radius(6).Padding(0, 10).Gap(9).
								AlignItems(ui.Center).Cursor(ui.CursorPointer)
							hovered := row.Hovered()
							if hovered {
								row.Background(tc.Text.Alpha(0.05))
							}
							row.Children(func() {
								ui.Icon(c, ic).Width(14).Height(14).TextColor(tc.IconMuted)
								ui.Text(c, title).FontSize(12.5).FontWeight(500).TextColor(tc.Text).SingleLine().Grow(1)
								if hovered {
									ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.TextMuted)
								}
							})
							if row.Clicked() {
								wf.AddStep(corestudio.NewDefaultStep(st, len(wf.Steps)+1))
							}
						}

						if isVideo {
							addStageRow("ref", "Keyframe / Reference", icons.TablerPhoto, corestudio.StepTypeReference)
							addStageRow("prompt", "Prompt", icons.TablerPencil, corestudio.StepTypePrompt)
							addStageRow("story", "Storyboard & Camera", icons.TablerColumns, corestudio.StepTypeStoryboard)
							addStageRow("vidgen", "Video Generator", icons.TablerMovie, corestudio.StepTypeVideoGen)
							addStageRow("output", "Video Output", icons.TablerDeviceFloppy, corestudio.StepTypeOutput)
						} else {
							addStageRow("ref", "Reference Image", icons.TablerPhoto, corestudio.StepTypeReference)
							addStageRow("prompt", "Prompt", icons.TablerPencil, corestudio.StepTypePrompt)
							addStageRow("imggen", "Image Generator", icons.TablerSparkles, corestudio.StepTypeImageGen)
							addStageRow("upscale", "Upscaler", icons.TablerWand, corestudio.StepTypeUpscale)
							addStageRow("output", "Image Output", icons.TablerDeviceFloppy, corestudio.StepTypeOutput)
						}
					})
				}
			})
		})
	})
}

func (a *AppUI) renderStudioPipelineBuilderPane(c *ui.Context, tc theme.ThemeColors, wf *corestudio.WorkflowPipeline) {
	ui.Scroll(c.Key("native-wf-pipeline-scroll")).Grow(1).Children(func() {
		ui.Column(c.Key("native-wf-steps-col")).Padding(20, 20, 28, 20).Gap(0).Children(func() {
			// Clean Header
			ui.Column(c).Padding(0, 0, 16, 0).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, wf.Name).FontSize(15).FontWeight(600).TextColor(tc.Text)
					ui.Box(c).Grow(1)
					ui.Text(c, fmt.Sprintf("%d stages", len(wf.Steps))).FontSize(12).TextColor(tc.TextMuted)
				})
				ui.Text(c, wf.Description).FontSize(12.5).TextColor(tc.TextMuted)
			})

			if len(wf.Steps) == 0 {
				ui.Column(c).Padding(28, 20).Radius(12).Background(tc.Surface).
					BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, "No stages yet").FontSize(13.5).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Pick a stage from the left sidebar or load a workflow preset.").FontSize(12).TextColor(tc.TextMuted)
				})
				return
			}

			for i := 0; i < len(wf.Steps); i++ {
				step := &wf.Steps[i]
				a.renderStudioPipelineStepCard(c, tc, wf, step, i, len(wf.Steps))

				// Quiet vertical connector between stages
				if i < len(wf.Steps)-1 {
					ui.Row(c.Key("conn-"+step.ID)).Height(24).Padding(0, 0, 0, 22).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Box(c).Size(1, 24).Background(tc.Border)
						ui.Text(c, strings.ToLower(step.OutputType)).FontSize(11).TextColor(tc.TextMuted)
					})
				}
			}
		})
	})
}

func wfStepIcon(st corestudio.WorkflowStepType) *ui.SVG {
	switch st {
	case corestudio.StepTypeReference:
		return icons.TablerPhoto
	case corestudio.StepTypePrompt:
		return icons.TablerPencil
	case corestudio.StepTypeStoryboard:
		return icons.TablerColumns
	case corestudio.StepTypeImageGen:
		return icons.TablerSparkles
	case corestudio.StepTypeVideoGen:
		return icons.TablerMovie
	case corestudio.StepTypeUpscale:
		return icons.TablerWand
	case corestudio.StepTypeOutput:
		return icons.TablerDeviceFloppy
	default:
		return icons.TablerBraces
	}
}

func (a *AppUI) renderStudioPipelineStepCard(c *ui.Context, tc theme.ThemeColors, wf *corestudio.WorkflowPipeline, step *corestudio.WorkflowStep, idx, total int) {
	bgCol := tc.Surface
	if !step.Enabled {
		bgCol = tc.Surface.Alpha(0.5)
	}

	ui.Column(c.Key("step-card-"+step.ID)).Radius(12).Background(bgCol).
		BorderWidth(1).BorderColor(tc.Border).Padding(14, 16).Gap(12).Children(func() {

		// Clean Card Header with domain icon badge
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Row(c).Size(24, 24).Radius(6).Background(tc.Accent.Alpha(0.12)).
				AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Icon(c, wfStepIcon(step.Type)).Width(13).Height(13).TextColor(tc.Accent)
			})
			ui.Text(c, fmt.Sprintf("%02d", idx+1)).FontSize(11).FontWeight(700).TextColor(tc.TextMuted)
			titleCol := tc.Text
			if !step.Enabled {
				titleCol = tc.TextMuted
			}
			ui.Text(c, step.Title).FontSize(13).FontWeight(600).TextColor(titleCol).SingleLine()

			if !step.Enabled {
				Badge(c, tc, "Bypassed", tc.Warning)
			}

			ui.Box(c).Grow(1)

			// Native MyGo Switch for enable/bypass + ghost move/delete icons
			enabled := step.Enabled
			if ui.Switch(c.Key("sw-"+step.ID), &enabled).Clicked() {
				step.Enabled = enabled
			}
			if idx > 0 {
				if wfGhostIconBtn(c, tc, "up-"+step.ID, icons.TablerChevronUp, "Move up", false).Clicked() {
					wf.MoveStep(step.ID, -1)
				}
			}
			if idx < total-1 {
				if wfGhostIconBtn(c, tc, "down-"+step.ID, icons.TablerChevronDown, "Move down", false).Clicked() {
					wf.MoveStep(step.ID, 1)
				}
			}
			if wfGhostIconBtn(c, tc, "del-"+step.ID, icons.TablerTrash, "Remove stage", true).Clicked() {
				wf.RemoveStep(step.ID)
			}
		})

		if !step.Enabled {
			return
		}

		// Step Controls
		switch step.Type {
		case corestudio.StepTypeReference:
			ui.Column(c).Gap(10).Children(func() {
				ui.Column(c).Gap(5).Children(func() {
					wfFieldLabel(c, tc, "Reference Image")
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Grow(1).Height(36).Radius(9).Padding(0, 12).AlignItems(ui.Center).
							Background(tc.Input).BorderWidth(1).BorderColor(tc.Border).Children(func() {
							a.bindStepString(step, "path", "", func(v *string) {
								ui.TextInput(c, v).Placeholder("Choose an image file...").Grow(1)
							})
						})
						if wfSecondaryBtn(c, tc, "browse-ref-"+step.ID, icons.TablerFolderOpen, "Browse").Clicked() {
							stepID := step.ID
							go a.studioPickReferenceImage(wf, stepID, "")
						}
					})
				})
				ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
					wfFieldLabel(c, tc, "Influence")
					strengthVal := step.GetFloat("strength", 0.65)
					ui.Slider(c.Key("sl-str-"+step.ID), &strengthVal, 0.1, 1.0).Grow(1)
					step.SetParam("strength", strengthVal)
					wfValueBadge(c, tc, fmt.Sprintf("%.2f", strengthVal))
				})
			})

		case corestudio.StepTypePrompt:
			ui.Column(c).Gap(10).Children(func() {
				ui.Column(c).Gap(5).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						wfFieldLabel(c, tc, "Prompt")
						ui.Box(c).Grow(1)
						if strings.TrimSpace(step.GetString("positive", "")) != "" {
							if wfGhostIconBtn(c, tc, "clr-pos-"+step.ID, icons.TablerX, "Clear prompt", false).Clicked() {
								step.SetParam("positive", "")
								if a.StudioWorkflowStepDrafts != nil {
									a.StudioWorkflowStepDrafts[step.ID+":positive"] = ""
								}
								a.StudioPositivePrompt = ""
							}
						}
					})
					wfTextAreaWrap(c, tc, func() {
						a.bindStepString(step, "positive", a.StudioPositivePrompt, func(v *string) {
							ui.TextAreaBase(c, v).Lines(3, 6).Placeholder("Describe the scene, subject, lighting, and style...").FillWidth()
						})
					})
				})
				ui.Column(c).Gap(5).Children(func() {
					wfFieldLabel(c, tc, "Negative Prompt")
					wfInputWrap(c, tc, func() {
						a.bindStepString(step, "negative", a.StudioNegativePrompt, func(v *string) {
							ui.TextInput(c, v).Placeholder("What to avoid (blurry, watermark, artifacts...)").Grow(1)
						})
					})
				})
				// Interactive style preset chips (highlighted when present in prompt)
				ui.Row(c).Wrap().Gap(6).Children(func() {
					curPos := step.GetString("positive", "")
					chip := func(label, token string) {
						active := strings.Contains(strings.ToLower(curPos), strings.ToLower(token))
						btn := ui.Row(c.Key("chip-"+step.ID+"-"+label)).Height(24).Padding(0, 9).Radius(999).
							AlignItems(ui.Center).Cursor(ui.CursorPointer)
						if active {
							btn.Background(tc.Accent.Alpha(0.16)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.45))
							btn.Children(func() {
								ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(tc.Accent)
							})
						} else {
							btn.Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
							if btn.Hovered() {
								btn.Background(tc.ToolbarControlHover)
							}
							btn.Children(func() {
								ui.Text(c, "+ "+label).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
							})
						}
						if btn.Clicked() && !active {
							next := curPos
							if strings.TrimSpace(next) == "" {
								next = token
							} else {
								next += ", " + token
							}
							step.SetParam("positive", next)
							if a.StudioWorkflowStepDrafts != nil {
								a.StudioWorkflowStepDrafts[step.ID+":positive"] = next
							}
							a.StudioPositivePrompt = next
						}
					}
					chip("Cinematic", "cinematic volumetric lighting, anamorphic lens, 8k")
					chip("Cyberpunk", "neon rain reflections, cyberpunk atmosphere")
					chip("Photoreal", "photorealistic 35mm photography, crisp depth of field")
					chip("Anime", "makoto shinkai anime aesthetic, vibrant sky")
					chip("Minimalist", "swiss minimalist composition, clean studio lighting")
				})
			})

		case corestudio.StepTypeStoryboard:
			ui.Column(c).Gap(10).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
					wfFieldLabel(c, tc, "Camera Motion")
					ui.Box(c).Grow(1)
					cam := step.GetString("camera", "Slow Dolly In")
					ui.Select(c.Key("sel-cam-"+step.ID), &cam, []string{"Slow Dolly In", "Tracking Pan", "Orbit 360", "Crane Pullback", "Handheld"})
					step.SetParam("camera", cam)
				})
				ui.Column(c).Gap(5).Children(func() {
					wfFieldLabel(c, tc, "Shot Sequence")
					wfInputWrap(c, tc, func() {
						a.bindStepString(step, "shots", "", func(v *string) {
							ui.TextInput(c, v).Placeholder("0-3s establishing; 3-6s action; 6-8s close-up").Grow(1)
						})
					})
				})
			})

		case corestudio.StepTypeImageGen:
			ui.Column(c).Gap(12).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					wfFieldLabel(c, tc, "Model")
					wfInputWrap(c, tc, func() {
						a.bindStepString(step, "model", "flux-1-schnell", func(v *string) {
							ui.TextInput(c, v).Placeholder("flux-1-schnell, sdxl, dall-e-3").Grow(1)
						})
					})
					wfChoiceGroup(c, tc, "mdl-img-"+step.ID, []string{"flux-1-schnell", "flux-1-dev", "sdxl"}, step.GetString("model", "flux-1-schnell"), func(v string) {
						step.SetParam("model", v)
						if a.StudioWorkflowStepDrafts != nil {
							a.StudioWorkflowStepDrafts[step.ID+":model"] = v
						}
						a.StudioModel = v
					})
				})

				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					wfFieldLabel(c, tc, "Aspect Ratio")
					ui.Box(c).Grow(1)
					wfChoiceGroup(c, tc, "ar-"+step.ID, []string{"16:9", "1:1", "9:16", "4:3"}, step.GetString("aspect_ratio", "16:9"), func(v string) {
						step.SetParam("aspect_ratio", v)
						a.StudioAspectRatio = v
					})
				})

				ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
					wfFieldLabel(c, tc, "Sampler")
					ui.Box(c).Grow(1)
					smp := step.GetString("sampler", "Euler a")
					ui.Select(c.Key("sel-smp-"+step.ID), &smp, []string{"Euler a", "DPM++ 2M", "DDIM", "UniPC"})
					step.SetParam("sampler", smp)
					a.StudioSampler = smp
				})

				ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
					wfFieldLabel(c, tc, "Steps")
					stepsVal := float64(step.GetInt("steps", 25))
					ui.Slider(c.Key("sl-steps-"+step.ID), &stepsVal, 5, 80).Grow(1)
					step.SetParam("steps", int(stepsVal))
					a.StudioSteps = int(stepsVal)
					wfValueBadge(c, tc, strconv.Itoa(int(stepsVal)))
				})

				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					wfFieldLabel(c, tc, "Seed")
					ui.Box(c).Grow(1)
					ui.Row(c).Width(170).Height(32).Padding(0, 8).Gap(6).Radius(8).
						Background(tc.Input).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Children(func() {
						seedStr := strconv.Itoa(step.GetInt("seed", 4928172))
						ui.TextInput(c.Key("seed-in-"+step.ID), &seedStr).Placeholder("Seed").Grow(1)
						if parsed, err := strconv.Atoi(strings.TrimSpace(seedStr)); err == nil {
							step.SetParam("seed", parsed)
							a.StudioSeed = int64(parsed)
						}
						if wfGhostIconBtn(c, tc, "seed-rnd-"+step.ID, icons.TablerRefresh, "Randomize seed", false).Clicked() {
							ns := int(time.Now().UnixNano() % 9999999)
							step.SetParam("seed", ns)
							a.StudioSeed = int64(ns)
						}
					})
				})
			})

		case corestudio.StepTypeVideoGen:
			ui.Column(c).Gap(12).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					wfFieldLabel(c, tc, "Video Model")
					wfInputWrap(c, tc, func() {
						a.bindStepString(step, "model", "veo-2-ai", func(v *string) {
							ui.TextInput(c, v).Placeholder("veo-2-ai, luma-ray, kling-1.6").Grow(1)
						})
					})
					wfChoiceGroup(c, tc, "mdl-vid-"+step.ID, []string{"veo-2-ai", "luma-ray", "kling-1.6"}, step.GetString("model", "veo-2-ai"), func(v string) {
						step.SetParam("model", v)
						if a.StudioWorkflowStepDrafts != nil {
							a.StudioWorkflowStepDrafts[step.ID+":model"] = v
						}
						a.StudioModel = v
					})
				})
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					wfFieldLabel(c, tc, "Duration")
					ui.Box(c).Grow(1)
					curDur := fmt.Sprintf("%ds", step.GetInt("duration", 6))
					wfChoiceGroup(c, tc, "dur-"+step.ID, []string{"4s", "6s", "8s", "10s"}, curDur, func(v string) {
						if d, err := strconv.Atoi(strings.TrimSuffix(v, "s")); err == nil {
							step.SetParam("duration", d)
						}
					})
				})
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					wfFieldLabel(c, tc, "Frame Rate")
					ui.Box(c).Grow(1)
					wfChoiceGroup(c, tc, "fps-"+step.ID, []string{"24", "30", "60"}, strconv.Itoa(step.GetInt("fps", 24)), func(v string) {
						if fps, err := strconv.Atoi(v); err == nil {
							step.SetParam("fps", fps)
						}
					})
				})
				ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
					wfFieldLabel(c, tc, "Motion")
					mbVal := float64(step.GetInt("motion_bucket", 127))
					ui.Slider(c.Key("sl-mb-"+step.ID), &mbVal, 20, 255).Grow(1)
					step.SetParam("motion_bucket", int(mbVal))
					wfValueBadge(c, tc, strconv.Itoa(int(mbVal)))
				})
			})

		case corestudio.StepTypeUpscale:
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				wfFieldLabel(c, tc, "Scale Factor")
				ui.Box(c).Grow(1)
				curScale := fmt.Sprintf("%dx", step.GetInt("scale", 2))
				wfChoiceGroup(c, tc, "sc-"+step.ID, []string{"2x", "4x"}, curScale, func(v string) {
					if sc, err := strconv.Atoi(strings.TrimSuffix(v, "x")); err == nil {
						step.SetParam("scale", sc)
					}
				})
			})

		case corestudio.StepTypeOutput:
			ui.Column(c).Gap(10).Children(func() {
				ui.Column(c).Gap(5).Children(func() {
					wfFieldLabel(c, tc, "Filename Prefix")
					wfInputWrap(c, tc, func() {
						a.bindStepString(step, "prefix", "forge_studio", func(v *string) {
							ui.TextInput(c, v).Placeholder("forge_studio").Grow(1)
						})
					})
				})
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					wfFieldLabel(c, tc, "Format")
					ui.Box(c).Grow(1)
					wfChoiceGroup(c, tc, "fmt-"+step.ID, []string{"PNG", "SVG", "MP4"}, step.GetString("format", "PNG"), func(v string) {
						step.SetParam("format", v)
					})
				})
			})
		}
	})
}

func (a *AppUI) renderStudioTemplateInjectorPane(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject, wf *corestudio.WorkflowPipeline) {
	if a.StudioWorkflowJSONDraft == "" && wf.TemplateJSON != "" {
		a.StudioWorkflowJSONDraft = wf.TemplateJSON
	}

	ui.Scroll(c.Key("native-tpl-scroll")).Grow(1).Children(func() {
		ui.Column(c).Padding(20, 20, 28, 20).Gap(16).Children(func() {
			// Clean Header
			ui.Column(c).Gap(10).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "JSON Workflow Template").FontSize(15).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, fmt.Sprintf("%d injectable variables detected", len(wf.Variables))).FontSize(12.5).TextColor(tc.TextMuted)
				})
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					toggleLabel := "Edit JSON"
					if a.StudioWorkflowShowJSON {
						toggleLabel = "Form Inputs"
					}
					if wfSecondaryBtn(c, tc, "tpl-toggle-raw", icons.TablerCode, toggleLabel).Clicked() {
						a.StudioWorkflowShowJSON = !a.StudioWorkflowShowJSON
						if a.StudioWorkflowShowJSON {
							a.StudioWorkflowJSONDraft = wf.TemplateJSON
						}
					}
					if wfSecondaryBtn(c, tc, "tpl-materialize-btn", icons.TablerGitBranch, "Open in Pipeline").Clicked() {
						_, resolved, err := corestudio.InjectWorkflowJSON(wf.TemplateJSON, wf.Variables)
						if err != nil {
							a.StudioWorkflowError = "Template injection error: " + err.Error()
						} else {
							resolved.Mode = "pipeline"
							a.setActiveStudioWorkflow(project, resolved)
							c.AddToast(ui.Toast{Title: "Workflow Materialized", Description: "Opened resolved stages in Pipeline Builder", Type: "success"})
						}
					}
				})
			})

			if a.StudioWorkflowShowJSON {
				ui.Column(c).Radius(12).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(12).Children(func() {
					wfFieldLabel(c, tc, "Workflow JSON (use {{variable}} placeholders)")
					wfTextAreaWrap(c, tc, func() {
						ui.TextAreaBase(c, &a.StudioWorkflowJSONDraft).Lines(14, 24).Placeholder(`{"name": "My Workflow", "steps": [...]}`).FillWidth()
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						if wfPrimaryBtn(c, tc, "tpl-apply-json", icons.TablerCheck, "Parse Variables", "", false).Clicked() {
							vars, err := corestudio.ParseWorkflowVariables(a.StudioWorkflowJSONDraft)
							if err != nil {
								a.StudioWorkflowError = "Workflow JSON parse error: " + err.Error()
							} else {
								wf.TemplateJSON = a.StudioWorkflowJSONDraft
								wf.Variables = vars
								a.StudioWorkflowError = ""
								a.StudioWorkflowShowJSON = false
								c.AddToast(ui.Toast{Title: "Template Parsed", Description: fmt.Sprintf("Detected %d injectable variables", len(vars)), Type: "success"})
							}
						}
						if wfSecondaryBtn(c, tc, "tpl-load-file", icons.TablerFolderOpen, "Load File").Clicked() {
							go a.studioImportWorkflowJSONFile(project)
						}
					})
				})
				return
			}

			if len(wf.Variables) == 0 {
				ui.Column(c).Radius(12).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Padding(24, 18).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "No variables found").FontSize(13.5).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Click Edit JSON to add {{variable}} placeholders or import a .json file.").FontSize(12).TextColor(tc.TextMuted)
				})
				return
			}

			// Single clean form card for all template variables (avoiding nested-card clutter)
			ui.Column(c).Radius(12).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(14).Children(func() {
				for i := range wf.Variables {
					v := &wf.Variables[i]
					ui.Column(c.Key("tpl-var-" + v.Key)).Gap(6).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
							ui.Text(c, v.Label).FontSize(12.5).FontWeight(600).TextColor(tc.Text)
							ui.Box(c).Grow(1)
							ui.Text(c, "{{"+v.Key+"}}").FontSize(11).TextColor(tc.TextMuted)
						})

						switch v.ValueType {
						case "multiline":
							wfTextAreaWrap(c, tc, func() {
								ui.TextAreaBase(c, &v.Value).Lines(2, 4).Placeholder("Enter " + v.Label + "...").FillWidth()
							})
						case "file":
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Row(c).Grow(1).Height(34).Radius(8).Padding(0, 10).AlignItems(ui.Center).
									Background(tc.Input).BorderWidth(1).BorderColor(tc.Border).Children(func() {
									ui.TextInput(c, &v.Value).Placeholder("Reference image path...").Grow(1)
								})
								if wfSecondaryBtn(c, tc, "tpl-browse-"+v.Key, icons.TablerFolderOpen, "Browse").Clicked() {
									varKey := v.Key
									go a.studioPickReferenceImage(wf, "", varKey)
								}
							})
						default:
							wfInputWrap(c, tc, func() {
								ui.TextInput(c, &v.Value).Placeholder(v.Key).Grow(1)
								if strings.Contains(strings.ToLower(v.Key), "seed") {
									if wfGhostIconBtn(c, tc, "tpl-rnd-"+v.Key, icons.TablerRefresh, "Randomize seed", false).Clicked() {
										v.Value = strconv.FormatInt(time.Now().UnixNano()%99999999, 10)
									}
								}
							})
						}
					})
				}
			})
		})
	})
}

func (a *AppUI) renderStudioWorkflowRightPane(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject, wf *corestudio.WorkflowPipeline) {
	ui.Column(c.Key("native-wf-right")).Grow(1).MinWidth(320).FillHeight().Background(tc.Canvas).Children(func() {
		// Right Pane Sub-Header: Segmented Tabs + Output Metadata (aligned 40px)
		ui.Row(c.Key("wf-right-tabs")).Height(40).Padding(0, 16).Gap(10).AlignItems(ui.Center).
			Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Row(c).Height(28).Padding(2, 2).Gap(2).Radius(7).
				Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).AlignItems(ui.Center).Children(func() {
				rTab := func(id, label string) {
					active := a.StudioWorkflowRightTab == id
					btn := ui.Row(c.Key("wf-rtab-"+id)).Height(22).Padding(0, 10).Radius(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					if active {
						btn.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						btn.Children(func() {
							ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(tc.Text)
						})
					} else {
						if btn.Hovered() {
							btn.Background(tc.Text.Alpha(0.05))
						}
						btn.Children(func() {
							ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(tc.TextMuted)
						})
					}
					if btn.Clicked() {
						a.StudioWorkflowRightTab = id
					}
				}
				rTab("preview", "Preview")
				rTab("gallery", "Gallery")
				rTab("logs", fmt.Sprintf("Logs (%d)", len(a.StudioWorkflowLogs)))
			})

			ui.Box(c).Grow(1)

			if a.StudioGeneratedImagePath != "" && a.StudioWorkflowRightTab == "preview" {
				meta := a.StudioGeneratedFormat
				if a.StudioGeneratedWidth > 0 {
					meta = fmt.Sprintf("%dx%d  ·  %s", a.StudioGeneratedWidth, a.StudioGeneratedHeight, a.StudioGeneratedFormat)
				}
				ui.Text(c, meta).FontSize(12).TextColor(tc.TextMuted).SingleLine()
			}
		})

		switch a.StudioWorkflowRightTab {
		case "gallery":
			ui.Scroll(c.Key("wf-gallery-scroll")).Grow(1).Children(func() {
				ui.Column(c).Padding(16, 18).Gap(8).Children(func() {
					assets := a.scanStudioMediaLibrary()
					if len(assets) == 0 {
						ui.Column(c).Padding(48, 20).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Icon(c, icons.TablerPhoto).Width(32).Height(32).TextColor(tc.IconMuted)
							ui.Text(c, "No outputs yet").FontSize(14).FontWeight(600).TextColor(tc.Text)
							ui.Text(c, "Run a workflow to see generated assets here.").FontSize(12.5).TextColor(tc.TextMuted)
						})
						return
					}
					for i, asset := range assets {
						if i >= 30 {
							break
						}
						isActive := asset.FilePath != "" && asset.FilePath == a.StudioGeneratedImagePath
						row := ui.Row(c.Key("wf-gal-"+asset.ID)).Height(46).Padding(0, 12).Radius(9).Gap(10).
							AlignItems(ui.Center).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
						if isActive {
							row.Background(tc.Accent.Alpha(0.12)).BorderColor(tc.Accent.Alpha(0.4))
						} else if row.Hovered() {
							row.Background(tc.ToolbarControlHover)
						}
						row.Children(func() {
							ic := icons.TablerPhoto
							if asset.Type == "video" {
								ic = icons.TablerMovie
							}
							ui.Icon(c, ic).Width(16).Height(16).TextColor(tc.IconMuted)
							ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
								ui.Text(c, asset.Title).FontSize(12.5).FontWeight(500).TextColor(tc.Text).SingleLine()
								ui.Text(c, strings.ToUpper(asset.Type)).FontSize(10.5).TextColor(tc.TextMuted)
							})
							if isActive {
								Badge(c, tc, "Active", tc.Accent)
							}
							if wfSecondaryBtn(c, tc, "wf-gal-ref-"+asset.ID, nil, "Use as Ref").Clicked() && asset.FilePath != "" {
								a.studioInjectReferencePath(wf, asset.FilePath)
								c.AddToast(ui.Toast{Title: "Reference Applied", Description: filepath.Base(asset.FilePath), Type: "success"})
							}
						})
						if row.Clicked() && asset.FilePath != "" {
							a.StudioGeneratedImagePath = asset.FilePath
							a.StudioWorkflowIsVideoOutput = asset.Type == "video"
							a.StudioWorkflowRightTab = "preview"
						}
					}
				})
			})

		case "logs":
			ui.Scroll(c.Key("wf-logs-scroll")).Grow(1).Children(func() {
				ui.Column(c).Padding(16, 18).Gap(8).Children(func() {
					if len(a.StudioWorkflowLogs) == 0 {
						ui.Text(c, "No execution logs yet. Click Generate to trace each workflow stage.").FontSize(12.5).TextColor(tc.TextMuted)
						return
					}
					for i, line := range a.StudioWorkflowLogs {
						ui.Row(c.Key(fmt.Sprintf("wf-log-%d", i))).Padding(10, 12).Radius(8).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Gap(10).AlignItems(ui.Start).Children(func() {
							ui.Text(c, fmt.Sprintf("%02d", i+1)).FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
							ui.Text(c, line).FontSize(12.5).TextColor(tc.Text).Grow(1)
						})
					}
				})
			})

		default: // "preview" — Hero Canvas
			ui.Column(c.Key("wf-preview-hero")).Grow(1).Padding(20, 20).Gap(14).Children(func() {
				// Main Stage Viewport
				ui.Column(c.Key("wf-preview-box")).Grow(1).FillWidth().Radius(12).Background(tc.Surface).
					BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center).Clip().Children(func() {
					if a.StudioIsGenerating {
						ui.Spinner(c).Size(28, 28).Label("")
						ui.Box(c).Size(1, 10)
						ui.Text(c, a.StudioGenerationStatus).FontSize(13).FontWeight(500).TextColor(tc.TextMuted)
					} else if img, _, _, _ := a.getStudioPreviewImage(); img != nil {
						ui.Image(c, img).Fit(ui.Contain).Grow(1)
					} else {
						ic := icons.TablerPhoto
						if project != nil && project.Type == corestudio.ArtifactTypeVideo {
							ic = icons.TablerMovie
						}
						ui.Icon(c, ic).Width(42).Height(42).TextColor(tc.IconMuted)
						ui.Box(c).Size(1, 10)
						ui.Text(c, "Ready to render").FontSize(15).FontWeight(600).TextColor(tc.Text)
						ui.Box(c).Size(1, 4)
						ui.Text(c, "Customize your stages on the left and press Generate.").FontSize(12.5).TextColor(tc.TextMuted)
					}
				})

				// Clean Bottom Action Bar when an output exists
				if a.StudioGeneratedImagePath != "" {
					ui.Row(c.Key("wf-preview-actions")).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, filepath.Base(a.StudioGeneratedImagePath)).FontSize(12.5).FontWeight(500).TextColor(tc.TextMuted).SingleLine().Grow(1)

						if a.StudioWorkflowIsVideoOutput {
							if wfSecondaryBtn(c, tc, "wf-btn-play-vid", icons.TablerPlayerPlay, "Play").Clicked() {
								_ = mygo.Shell.OpenPath(a.StudioGeneratedImagePath)
							}
						} else {
							if wfSecondaryBtn(c, tc, "wf-btn-send-video", icons.TablerMovie, "Animate in Video").Clicked() {
								a.studioSendImageToVideo(a.StudioGeneratedImagePath)
								c.AddToast(ui.Toast{Title: "Sent to Video Studio", Description: "Loaded as keyframe in Image → Video", Type: "success"})
							}
						}
						if wfSecondaryBtn(c, tc, "wf-btn-use-ref", icons.TablerPhoto, "Use as Ref").Clicked() {
							a.studioInjectReferencePath(wf, a.StudioGeneratedImagePath)
							c.AddToast(ui.Toast{Title: "Chained as Reference", Description: filepath.Base(a.StudioGeneratedImagePath), Type: "success"})
						}
						if wfSecondaryBtn(c, tc, "wf-btn-finder", icons.TablerFolderOpen, "Finder").Clicked() {
							mygo.Shell.ShowItemInFolder(a.StudioGeneratedImagePath)
						}
						if wfPrimaryBtn(c, tc, "wf-btn-send-editor", icons.TablerExternalLink, "Send to Editor", "", false).Clicked() {
							a.studioExportActive()
						}
					})
				}
			})
		}
	})
}

// studioSendImageToVideo switches to Video Studio, loads the Image -> Video
// workflow, and injects imgPath as the keyframe reference in one click.
func (a *AppUI) studioSendImageToVideo(imgPath string) {
	if imgPath == "" {
		return
	}
	a.studioEnsureProjectForType(corestudio.ArtifactTypeVideo)
	vidWf := corestudio.PresetImageToVideo()
	a.studioInjectReferencePath(&vidWf, imgPath)
	a.StudioVideoWorkflow = &vidWf
	a.StudioWorkflowStepDrafts = nil
	a.StudioWorkflowError = ""
}

func (a *AppUI) studioInjectReferencePath(wf *corestudio.WorkflowPipeline, imgPath string) {
	if wf == nil || imgPath == "" {
		return
	}
	if wf.Mode == "template" {
		for i := range wf.Variables {
			if wf.Variables[i].ValueType == "file" {
				wf.Variables[i].Value = imgPath
				return
			}
		}
		return
	}
	for i := range wf.Steps {
		if wf.Steps[i].Type == corestudio.StepTypeReference {
			wf.Steps[i].SetParam("path", imgPath)
			if a.StudioWorkflowStepDrafts != nil {
				a.StudioWorkflowStepDrafts[wf.Steps[i].ID+":path"] = imgPath
			}
			return
		}
	}
	ref := corestudio.NewDefaultStep(corestudio.StepTypeReference, len(wf.Steps)+1)
	ref.SetParam("path", imgPath)
	wf.Steps = append([]corestudio.WorkflowStep{ref}, wf.Steps...)
}

func (a *AppUI) studioPickReferenceImage(wf *corestudio.WorkflowPipeline, stepID, varKey string) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: a.Window,
		Title:  "Select Reference Image",
		Filters: []mygo.FileFilter{
			{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "webp", "svg"}},
		},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	chosen := paths[0]
	a.runOnUI(func() {
		if varKey != "" {
			for i := range wf.Variables {
				if wf.Variables[i].Key == varKey {
					wf.Variables[i].Value = chosen
					break
				}
			}
		} else if stepID != "" {
			for i := range wf.Steps {
				if wf.Steps[i].ID == stepID {
					wf.Steps[i].SetParam("path", chosen)
					if a.StudioWorkflowStepDrafts != nil {
						a.StudioWorkflowStepDrafts[stepID+":path"] = chosen
					}
					break
				}
			}
		}
		a.invalidateUI()
	})
}

func (a *AppUI) studioImportWorkflowJSONFile(project *corestudio.StudioProject) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: a.Window,
		Title:  "Import ForgeADE Workflow JSON",
		Filters: []mygo.FileFilter{
			{Name: "Workflow JSON", Extensions: []string{"json"}},
		},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		a.runOnUI(func() {
			a.StudioWorkflowError = "Failed to read workflow file: " + err.Error()
			a.invalidateUI()
		})
		return
	}
	defType := "image"
	if project != nil && project.Type == corestudio.ArtifactTypeVideo {
		defType = "video"
	}
	imported, err := corestudio.ImportWorkflowJSON(raw, defType)
	if err != nil {
		a.runOnUI(func() {
			a.StudioWorkflowError = "Invalid workflow JSON: " + err.Error()
			a.invalidateUI()
		})
		return
	}
	if imported.Name == "" || imported.Name == "Imported Workflow Template" {
		imported.Name = strings.TrimSuffix(filepath.Base(paths[0]), filepath.Ext(paths[0]))
	}
	a.runOnUI(func() {
		a.setActiveStudioWorkflow(project, imported)
		a.StudioWorkflowShowJSON = false
		a.queueToast(ui.Toast{
			Title:       "Workflow Imported",
			Description: imported.Name,
			Type:        "success",
		})
		a.invalidateUI()
	})
}

func (a *AppUI) studioExportWorkflowJSONFile(wf *corestudio.WorkflowPipeline) {
	if wf == nil {
		return
	}
	var data []byte
	var err error
	suggested := "forge_workflow.json"
	if wf.Mode == "template" {
		data, _, err = corestudio.InjectWorkflowJSON(wf.TemplateJSON, wf.Variables)
		suggested = "forge_workflow_injected.json"
	} else {
		data, err = wf.ExportJSON()
		suggested = strings.ToLower(strings.ReplaceAll(wf.ID, " ", "_")) + ".json"
	}
	if err != nil {
		a.runOnUI(func() {
			a.StudioWorkflowError = "Export failed: " + err.Error()
			a.invalidateUI()
		})
		return
	}
	home, _ := os.UserHomeDir()
	destPath, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.Window,
		Title:       "Export Workflow JSON",
		DefaultPath: filepath.Join(home, "Downloads", suggested),
		ButtonLabel: "Save JSON",
	})
	if err != nil || destPath == "" {
		return
	}
	if !strings.HasSuffix(strings.ToLower(destPath), ".json") {
		destPath += ".json"
	}
	if err := os.WriteFile(destPath, data, 0644); err == nil {
		mygo.Shell.ShowItemInFolder(destPath)
	}
}
