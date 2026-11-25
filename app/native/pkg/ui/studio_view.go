package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	nativestudio "github.com/hasdev/forge-ade/app/native/pkg/studio"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	corestudio "github.com/hasdev/forge-ade/core/studio"
)

// renderStudioSurface is the top-level native UI surface for Studio Mode.
func (a *AppUI) renderStudioSurface(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	if a.Studio == nil {
		ws := ""
		if a.Workspace != nil {
			ws = a.Workspace.RootDir
		}
		a.initStudioManager(ws)
	}

	project := a.Studio.ActiveProject()

	ui.Column(c).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
		// Main Studio Work Area
		// - Image or Video Generator: 100% Pure Native Workflow Studio (Pipeline Builder + ComfyUI JSON Injection)
		// - Presentation, Slides, Motion, or 2D/3D Sandbox: Pure Native GPU Canvas + Left Chat Agent
		if project != nil && (project.Type == corestudio.ArtifactTypeImage || project.Type == corestudio.ArtifactTypeVideo) {
			a.closeStudioWebView()
			a.renderStudioNativeWorkflow(c, tc, project)
		} else {
			a.closeStudioNodeWebView()
			a.renderStudioSplitAgentViewer(c, tc, project)
		}
	})
}

// renderStudioHeader renders the Studio Top Bar with guaranteed clearance for macOS traffic lights.
func (a *AppUI) renderStudioHeader(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject) {
	bar := c.TitleBar()
	leftPad := float32(14)
	if bar.Left > 0 {
		leftPad = max(bar.Left+10, 78)
	} else if runtime.GOOS == "darwin" {
		leftPad = 78
	}

	// Responsive degradation: collapse chrome labels to icons as the window
	// narrows so the bar never clips (same inline-clamp idiom as the drawer).
	winW, _ := c.Size()
	chromeIcons := winW > 0 && winW < 1000
	domainIcons := winW > 0 && winW < 900

	ui.Row(c.Key("studio-topbar")).Height(WorkspaceTopBarHeight).Padding(0, 14, 0, leftPad).Gap(10).
		AlignItems(ui.Center).Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {

		// Mode Switcher Pill (Agent | Editor | Studio)
		ui.Row(c.Key("studio-mode-switcher")).Height(26).Padding(2, 3).Gap(2).Radius(6).
			Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).AlignItems(ui.Center).Children(func() {

			modeBtn := func(key string, label string, mode int, active bool) {
				btn := ui.Row(c.Key(key)).Height(22).Padding(0, 8).Radius(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				if active {
					btn.Background(tc.Accent)
					ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
				} else {
					if btn.Hovered() {
						btn.Background(tc.ToolbarControlHover)
					}
					ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
				}
				if btn.Clicked() {
					a.setMode(mode)
				}
			}

			modeBtn("mode-agent-btn", "Agent", ModeAgent, a.Mode == ModeAgent)
			modeBtn("mode-editor-btn", "Editor", ModeEditor, a.Mode == ModeEditor)
			modeBtn("mode-studio-btn", "Studio", ModeStudio, a.Mode == ModeStudio)
		})

		ui.Box(c).Size(1, 16).Background(tc.Border)

		// Domain Switcher: Slides, Motion, 3D Sandbox, Image Gen, Video Gen.
		// The pill hugs its content inside a growing horizontal scroll viewport
		// so overflow scrolls instead of clipping (editor tab-strip idiom).
		ui.ScrollHorizontal(c.Key("studio-domain-switcher-scroll")).Grow(1).MinWidth(0).Scrollbars(false).Children(func() {
			ui.Row(c.Key("studio-domain-switcher")).Height(26).Padding(2, 2).Gap(2).Radius(6).
				Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).AlignItems(ui.Center).Children(func() {

				domainTab := func(key, label string, artType corestudio.ArtifactType, icon *ui.SVG) {
					active := project != nil && project.Type == artType
					tab := ui.Row(c.Key(key)).Height(22).Padding(0, 8).Gap(4).Radius(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					if active {
						tab.Background(tc.Surface)
						ui.Icon(c, icon).Width(12).Height(12).TextColor(tc.Accent)
						if !domainIcons {
							ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(tc.Text)
						}
					} else {
						if tab.Hovered() {
							tab.Background(tc.ToolbarControlHover)
						}
						ui.Icon(c, icon).Width(12).Height(12).TextColor(tc.IconMuted)
						if !domainIcons {
							ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
						}
					}
					if domainIcons {
						tab.Tooltip(label)
					}
					if tab.Clicked() {
						a.studioEnsureProjectForType(artType)
					}
				}

				domainTab("tab-dom-slides", "Slides", corestudio.ArtifactTypeSlides, icons.TablerPresentation)
				domainTab("tab-dom-motion", "Motion", corestudio.ArtifactTypeMotion, icons.TablerMovie)
				domainTab("tab-dom-3d", "3D Sandbox", corestudio.ArtifactTypeCodeJS, icons.TablerRotate)
				ui.Box(c).Size(1, 12).Background(tc.Border)
				domainTab("tab-dom-image", "Image Studio", corestudio.ArtifactTypeImage, icons.TablerPhoto)
				domainTab("tab-dom-video", "Video Studio", corestudio.ArtifactTypeVideo, icons.TablerSparkles)
			})
		})

		// API Settings Shortcut Button (icon-only once the window is narrow)
		settingsBtn := ui.Row(c.Key("studio-api-settings-btn")).Height(26).Radius(6).
			Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).
			AlignItems(ui.Center).Cursor(ui.CursorPointer)
		if chromeIcons {
			settingsBtn.Padding(0, 7).Tooltip("API Settings")
		} else {
			settingsBtn.Padding(0, 8).Gap(5)
		}
		settingsBtn.Children(func() {
			ui.Icon(c, icons.TablerSettings).Width(12).Height(12).TextColor(tc.IconMuted)
			if !chromeIcons {
				ui.Text(c, "API Settings").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
			}
		})
		if settingsBtn.Hovered() {
			settingsBtn.Background(tc.ToolbarControlHover)
		}
		if settingsBtn.Clicked() {
			a.openSettings(settings.SettingsTabStudio)
		}

		// Send to Editor (⌘⇧E)
		exportBtn := ui.Row(c.Key("studio-export-btn")).Height(26).Radius(6).
			Background(tc.Accent).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		if chromeIcons {
			exportBtn.Padding(0, 8).Tooltip("Send to Editor (⌘⇧E)")
		} else {
			exportBtn.Padding(0, 10).Gap(5)
		}
		exportBtn.Children(func() {
			ui.Icon(c, icons.TablerExternalLink).Width(12).Height(12).TextColor(tc.AccentForeground)
			if !chromeIcons {
				ui.Text(c, "Send to Editor").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
				ui.Text(c, "⌘⇧E").FontSize(10).FontWeight(500).TextColor(tc.AccentForeground.Alpha(0.8))
			}
		})
		if exportBtn.Hovered() {
			exportBtn.Background(tc.Accent.Alpha(0.9))
		}
		if exportBtn.Clicked() {
			a.studioExportActive()
		}
	})
}

// studioEnsureCanvasProject selects the first active canvas project (slides, motion, 3d, pdf, pptx)
// or seeds a canvas project if none exist.
func (a *AppUI) studioEnsureCanvasProject() {
	if a.Studio == nil {
		return
	}
	for _, p := range a.Studio.Projects() {
		if p.Type != corestudio.ArtifactTypeImage && p.Type != corestudio.ArtifactTypeVideo {
			a.Studio.SelectProject(p.ID)
			return
		}
	}
	a.studioEnsureProjectForType(corestudio.ArtifactTypeSlides)
}

// studioEnsureProjectForType selects the first project of the given type or
// creates a seeded one; shared by the header domain tabs and the empty state.
func (a *AppUI) studioEnsureProjectForType(artType corestudio.ArtifactType) {
	if a.Studio == nil {
		return
	}
	for _, p := range a.Studio.Projects() {
		if p.Type == artType {
			a.Studio.SelectProject(p.ID)
			return
		}
	}
	switch artType {
	case corestudio.ArtifactTypeSlides:
		a.Studio.CreateProject("Presentation Deck", "16:9 Presentation", artType)
	case corestudio.ArtifactTypeMotion:
		a.Studio.CreateProject("Motion Composition", "Kinetic Motion with TTS", artType)
	case corestudio.ArtifactTypeCodeJS:
		a.Studio.CreateProject("3D Torus Knot", "Three.js Realtime Sandbox", artType)
	case corestudio.ArtifactTypeImage:
		a.Studio.CreateProject("Image Node Workflow", "Visual Node Graph Workflow", artType)
	case corestudio.ArtifactTypeVideo:
		a.Studio.CreateProject("Video Node Workflow", "Video Latent Node Pipeline", artType)
	}
}

// studioChatBounds returns the window-aware min/max for the left chat pane so
// the viewer pane always keeps at least ~45% of the window (drawer clamp idiom).
func (a *AppUI) studioChatBounds(c *ui.Context) (float32, float32) {
	winW, _ := c.Size()
	availableW := winW
	if a.ShowLeftSidebar {
		availableW -= a.LeftSidebarWidth
	}
	if a.ShowRightDrawer {
		availableW -= a.RightDrawerWidth
	}
	minW, maxW := float32(320), float32(720)
	if availableW > 0 && availableW*0.55 < maxW {
		maxW = max(minW, availableW*0.55)
	}
	return minW, maxW
}

// studioStageSize fits a 16:9 stage card into the viewer pane's available
// width so the native fallback stage never overflows its pane.
func (a *AppUI) studioStageSize(c *ui.Context) (float32, float32) {
	winW, _ := c.Size()
	used := float32(64) // stage viewport horizontal padding (32 each side)
	if a.ShowLeftSidebar {
		used += a.LeftSidebarWidth
	}
	if a.ShowRightDrawer {
		used += a.RightDrawerWidth + 4 // right drawer and its resize border
	}
	if !a.StudioChatCollapsed {
		minChatW, maxChatW := a.studioChatBounds(c)
		chatW := min(max(a.StudioLeftChatWidth, minChatW), maxChatW)
		used += chatW + 4 // resize handle
	}
	w := float32(720)
	if winW > 0 && winW-used < w {
		w = max(240, winW-used)
	}
	return w, w * 9 / 16
}

// renderStudioSplitAgentViewer renders the requested 2-pane layout for Slides, Motion, and 2D/3D Sandbox:
// Left pane is the real Chat Agent (ACP & internal forgeade), right pane is the Main Viewer.
func (a *AppUI) renderStudioSplitAgentViewer(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject) {
	// Per-frame window-aware clamp keeps the pane sane even right after a resize.
	minChatW, maxChatW := a.studioChatBounds(c)
	a.StudioLeftChatWidth = min(max(a.StudioLeftChatWidth, minChatW), maxChatW)

	ui.Row(c.Key("studio-split-layout")).Grow(1).FillHeight().AlignItems(ui.Stretch).Children(func() {
		if !a.StudioChatCollapsed {
			// -------------------------------------------------------------
			// LEFT PANE: Real Chat Agent (ACP & Internal ForgeADE)
			// -------------------------------------------------------------
			ui.Column(c.Key("studio-agent-left-pane")).Width(a.StudioLeftChatWidth).FillHeight().
				Background(tc.Surface).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {

				// Studio Agent Header Bar (clean, single-line; model & harness live in the composer)
				ui.Row(c.Key("studio-agent-header")).Height(36).Padding(0, 10).Gap(6).AlignItems(ui.Center).
					BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {

					var activeSess *agent.AgentSession
					if a.Agent != nil {
						activeSess = a.Agent.ActiveSession()
					}
					isInternal := (activeSess != nil && activeSess.AgentType == "internal") ||
						(a.Agent != nil && a.Agent.ActiveAgentType == "internal") ||
						(a.SelectedHarnessID == "agent-internal")

					if isInternal {
						ui.Icon(c, icons.ProviderForgeADE).Width(14).Height(14)
						ui.Text(c, "ForgeADE").FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
					} else {
						ui.Icon(c, icons.ProviderAntigravity).Width(14).Height(14).TextColor(tc.Accent)
						ui.Text(c, "Studio Agent").FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
					}

					ui.Box(c).Grow(1)

					// Session History Toggle (icon-only for a clean header)
					historyBtn := ToggleIconButton(c, tc, icons.TablerHistory, 13, a.ShowLeftSidebar)
					historyBtn.Tooltip("Session History")
					if historyBtn.Clicked() {
						a.ShowLeftSidebar = !a.ShowLeftSidebar
					}

					// New Conversation Button
					newTaskBtn := ui.Row(c.Key("studio-new-agent-task")).Height(24).Padding(0, 8).Gap(4).Radius(5).
						Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).
						AlignItems(ui.Center).Cursor(ui.CursorPointer)
					newTaskBtn.Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.IconMuted)
						ui.Text(c, "New").FontSize(11).FontWeight(500).TextColor(tc.Text).SingleLine()
					})
					if newTaskBtn.Hovered() {
						newTaskBtn.Background(tc.ToolbarControlHover)
					}
					if newTaskBtn.Clicked() {
						a.handleNewStudioTask()
					}

					// Collapse the chat pane so the viewer gets full width.
					collapseBtn := IconButton(c, tc, icons.TablerChevronLeft, 13)
					collapseBtn.Tooltip("Hide chat pane")
					if collapseBtn.Clicked() {
						a.StudioChatCollapsed = true
					}
				})

				// Real Agent Chat Engine View (Stream transcript, thinking, tool calls, & composer)
				ui.Column(c.Key("studio-real-agent-chat-container")).Grow(1).FillHeight().Children(func() {
					a.renderAgentChat(c)
				})
			})

			// Draggable Resize Handle
			RightResizeHandle(c, &a.StudioLeftChatWidth, minChatW, maxChatW, 420)
		} else {
			// Collapsed: compact icon rail so the viewer gets maximum width while
			// keeping Studio Agent actions clearly accessible.
			ui.Column(c.Key("studio-chat-reopen")).Width(36).FillHeight().Padding(8, 0).Gap(8).
				Background(tc.Surface).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).
				AlignItems(ui.Center).Children(func() {

				expandBtn := IconButton(c, tc, icons.TablerChevronRight, 14)
				expandBtn.Tooltip("Show Studio Agent")
				if expandBtn.Clicked() {
					a.StudioChatCollapsed = false
				}

				chatBtn := ui.Row(c.Key("studio-chat-reopen-icon")).Size(26, 26).Radius(6).
					Background(tc.Accent.Alpha(0.14)).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				chatBtn.Tooltip("Open Studio Agent Chat")
				chatBtn.Children(func() {
					ui.Icon(c, icons.TablerMessage).Width(14).Height(14).TextColor(tc.Accent)
				})
				if chatBtn.Hovered() {
					chatBtn.Background(tc.Accent.Alpha(0.24))
				}
				if chatBtn.Clicked() {
					a.StudioChatCollapsed = false
				}

				newBtn := IconButton(c, tc, icons.TablerPlus, 14)
				newBtn.Tooltip("New Studio Task")
				if newBtn.Clicked() {
					a.StudioChatCollapsed = false
					a.handleNewStudioTask()
				}

				histBtn := ToggleIconButton(c, tc, icons.TablerHistory, 14, a.ShowLeftSidebar)
				histBtn.Tooltip("Session History")
				if histBtn.Clicked() {
					a.ShowLeftSidebar = !a.ShowLeftSidebar
				}
			})
		}

		// -------------------------------------------------------------
		// RIGHT PANE: Main Pane for Viewer (Slides, Motion, 3D Sandbox)
		// -------------------------------------------------------------
		ui.Column(c.Key("studio-main-viewer-pane")).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
			if project == nil {
				renderStudioEmptyState(c, tc, a)
				return
			}

			activeVer := project.ActiveVersion()
			var data corestudio.ArtifactData
			if activeVer != nil {
				data = activeVer.Data
			}

			a.renderStudioUnifiedCanvasViewer(c, tc, project, data)
		})
	})
}

// renderStudioEmptyState renders the viewer pane's landing state with
// quick-create chips for each creative domain.
func renderStudioEmptyState(c *ui.Context, tc theme.ThemeColors, a *AppUI) {
	ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(16).Padding(24).Children(func() {
		ui.Column(c).Size(64, 64).Radius(16).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Icon(c, icons.TablerSparkles).Width(28).Height(28).TextColor(tc.Accent)
		})

		ui.Column(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, "No Active Project").FontSize(15).FontWeight(600).TextColor(tc.Text)
			ui.Text(c, "Start a generative canvas — the Studio Agent pairs with it automatically.").
				FontSize(12).TextColor(tc.TextMuted)
		})

		ui.Row(c).Wrap().Justify(ui.Center).MaxWidth(560).Gap(8).Children(func() {
			quick := func(key, label string, artType corestudio.ArtifactType, icon *ui.SVG) {
				btn := ui.Row(c.Key("empty-create-"+key)).Height(30).Padding(0, 12).Gap(6).Radius(6).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				btn.Children(func() {
					ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.Accent)
					ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if btn.Hovered() {
					btn.Background(tc.ToolbarControlHover)
				}
				if btn.Clicked() {
					a.studioEnsureProjectForType(artType)
				}
			}
			quick("slides", "Slides", corestudio.ArtifactTypeSlides, icons.TablerPresentation)
			quick("motion", "Motion", corestudio.ArtifactTypeMotion, icons.TablerMovie)
			quick("3d", "3D Sandbox", corestudio.ArtifactTypeCodeJS, icons.TablerRotate)
			quick("image", "Image (Nodes)", corestudio.ArtifactTypeImage, icons.TablerPhoto)
			quick("video", "Video (Nodes)", corestudio.ArtifactTypeVideo, icons.TablerSparkles)
		})
	})
}

// renderStudioAgentHome renders the dedicated Studio Agent landing card when no messages exist yet.
func (a *AppUI) renderStudioAgentHome(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	as := a.Agent
	if as == nil {
		return
	}
	proj := a.Studio.ActiveProject()
	projTitle := "Studio Project"
	if proj != nil {
		projTitle = proj.Name
	}

	ui.Column(c).Grow(1).Fill().Padding(14, 16).Gap(12).Background(tc.Canvas).Children(func() {
		// Context Pairing Card
		ui.Column(c).Radius(8).Padding(12, 14).Gap(6).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Icon(c, icons.TablerSparkles).Width(14).Height(14).TextColor(tc.Accent)
				ui.Text(c, "Studio Agent: Pairing with "+projTitle).FontSize(12).FontWeight(700).TextColor(tc.Text)
			})
			ui.Text(c, "Direct commands given here update your slides, motion timelines, and 3D scenes live on the right pane.").
				FontSize(11).TextColor(tc.TextMuted).LineHeight(1.4)
		})

		// Quick Suggestion Chips
		ui.Row(c).Wrap().Gap(6).Children(func() {
			chip := func(label, prompt string) {
				btn := ui.Row(c.Key("studio-sugg-"+label)).Height(24).Padding(0, 8).Radius(12).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				btn.Children(func() {
					ui.Text(c, label).FontSize(10).FontWeight(500).TextColor(tc.Text)
				})
				if btn.Hovered() {
					btn.Background(tc.ToolbarControlHover)
				}
				if btn.Clicked() {
					as.InputText = prompt
					if as.Invalidate != nil {
						as.Invalidate()
					}
				}
			}

			if proj != nil && proj.Type == corestudio.ArtifactTypeSlides {
				chip("Landing Page HTML", "Create a modern responsive landing page in HTML with hero, features, and pricing")
				chip("+ Add Architecture Slide", "Create a new slide detailing system architecture and data flow")
				chip("Apply Swiss Minimalist", "Refactor slide typography to follow Swiss Minimalist tokens")
			} else if proj != nil && proj.Type == corestudio.ArtifactTypeMotion {
				chip("Kinetic Spring Headline", "Animate main title with spring bounce kinetics")
				chip("ElevenLabs Voice Sync", "Align speech timestamps to kinetic word highlights")
				chip("1080p 60fps Setup", "Configure 1080p 60fps canvas stage viewport")
			} else {
				chip("Torus Knot Mesh", "Render interactive 3D Torus Knot with metallic sheen")
				chip("Orbit Camera Controls", "Inject smooth damping orbit camera controls")
				chip("Catch & Heal Errors", "Audit Three.js scene for zero WebGL runtime errors")
			}
		})

		ui.Box(c).Grow(1)

		// Real multiline composer card
		a.renderComposerCard(c, tc, false)
	})
}

// StudioIntent represents the user's creative target intent detected from prompt.
type StudioIntent string

const (
	IntentSlides  StudioIntent = "slides"
	IntentMotion  StudioIntent = "motion"
	Intent3D      StudioIntent = "3d"
	IntentImage   StudioIntent = "image"
	IntentVideo   StudioIntent = "video"
	IntentGeneral StudioIntent = "general"
)

// detectStudioIntent parses natural language to automatically route to the right creative mode.
func detectStudioIntent(prompt string) StudioIntent {
	p := strings.ToLower(prompt)
	if strings.Contains(p, "motion") || strings.Contains(p, "kinetic") || strings.Contains(p, "animasi") ||
		strings.Contains(p, "remotion") || strings.Contains(p, "hyperframe") || strings.Contains(p, "elevenlabs") ||
		strings.Contains(p, "voice sync") || strings.Contains(p, "tts") || strings.Contains(p, "timeline") ||
		strings.Contains(p, "video text") {
		return IntentMotion
	}
	if strings.Contains(p, "threejs") || strings.Contains(p, "three.js") || strings.Contains(p, "webgl") ||
		strings.Contains(p, "orbit controls") || strings.Contains(p, "orbitcontrols") ||
		strings.Contains(p, "3d scene") || strings.Contains(p, "3d sandbox") || strings.Contains(p, "mesh") ||
		strings.Contains(p, "shader") || strings.Contains(p, "torus") || strings.Contains(p, "p5.js") ||
		strings.Contains(p, "pixijs") || strings.Contains(p, "r3f") {
		return Intent3D
	}
	if strings.Contains(p, "slide") || strings.Contains(p, "deck") || strings.Contains(p, "presentasi") ||
		strings.Contains(p, "presentation") || strings.Contains(p, "powerpoint") || strings.Contains(p, "keynote") ||
		strings.Contains(p, "pptx") || strings.Contains(p, "slidev") || strings.Contains(p, "pptxgenjs") {
		return IntentSlides
	}
	if strings.Contains(p, "generate image") || strings.Contains(p, "bikin gambar") || strings.Contains(p, "buat gambar") ||
		strings.Contains(p, "flux") || strings.Contains(p, "comfyui") || strings.Contains(p, "midjourney") ||
		strings.Contains(p, "text to image") {
		return IntentImage
	}
	if strings.Contains(p, "generate video") || strings.Contains(p, "bikin video") || strings.Contains(p, "buat video") ||
		strings.Contains(p, "veo") || strings.Contains(p, "kling") || strings.Contains(p, "runway") ||
		strings.Contains(p, "sora") || strings.Contains(p, "wan2") {
		return IntentVideo
	}
	return IntentGeneral
}

// detectPathFlavor determines the artifact flavor and corresponding studio project type.
func detectPathFlavor(path string) (string, corestudio.ArtifactType) {
	lower := strings.ToLower(path)
	ext := filepath.Ext(lower)
	if ext == ".pdf" {
		return "pdf", corestudio.ArtifactTypeSlides
	}
	if ext == ".pptx" {
		return "pptx", corestudio.ArtifactTypeSlides
	}
	if strings.Contains(lower, "motion") || ext == ".mp4" {
		return "motion", corestudio.ArtifactTypeMotion
	}
	if strings.Contains(lower, "3d") || strings.Contains(lower, "scene") || ext == ".js" || ext == ".ts" {
		return "3d", corestudio.ArtifactTypeCodeJS
	}
	if strings.Contains(lower, "slide") || strings.Contains(lower, "deck") || ext == ".md" {
		return "slides", corestudio.ArtifactTypeSlides
	}
	return "html", corestudio.ArtifactTypeSlides
}

// detectStudioFlavor checks the active artifact entry and project type to pick the right viewer mode.
func detectStudioFlavor(project *corestudio.StudioProject, data corestudio.ArtifactData) string {
	entry := strings.ToLower(data.EntryHTML)
	switch {
	case strings.HasSuffix(entry, ".pdf"):
		return "pdf"
	case strings.HasSuffix(entry, ".pptx"):
		return "pptx"
	case strings.Contains(entry, "motion") || strings.HasSuffix(entry, ".mp4"):
		return "motion"
	case strings.Contains(entry, "3d") || strings.Contains(entry, "scene") || strings.HasSuffix(entry, ".js"):
		return "3d"
	case strings.Contains(entry, "slide") || strings.Contains(entry, "deck"):
		return "slides"
	case strings.HasSuffix(entry, ".html") || strings.HasSuffix(entry, ".htm"):
		return "html"
	}
	if project != nil {
		switch project.Type {
		case corestudio.ArtifactTypeMotion:
			return "motion"
		case corestudio.ArtifactTypeCodeJS:
			return "3d"
		case corestudio.ArtifactTypeSlides:
			return "slides"
		}
	}
	return "slides"
}

// renderStudioUnifiedCanvasViewer provides a unified interactive canvas and viewer
// for Slides, Motion compositions, 3D Sandbox, PDF, PPTX, and HTML artifacts,
// as well as viewer-only external file previews (StudioPreviewPath).
func (a *AppUI) renderStudioUnifiedCanvasViewer(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject, data corestudio.ArtifactData) {
	isPreview := false
	previewPath := strings.TrimSpace(a.StudioPreviewPath)
	if previewPath != "" {
		if _, err := os.Stat(previewPath); err == nil {
			isPreview = true
		} else {
			a.StudioPreviewPath = ""
		}
	}

	var flavor, assetPath string
	if isPreview {
		assetPath = previewPath
		flavor, _ = detectPathFlavor(previewPath)
	} else {
		flavor = detectStudioFlavor(project, data)
		assetPath = data.EntryHTML
		if assetPath == "" && project != nil {
			assetPath = a.ensureStudioAssetFile(project)
		}
	}

	if assetPath != "" && mygo.WebViewAvailable() {
		bg := colorToCSS(tc.Canvas)
		if flavor == "3d" {
			bg = "#0a0c14"
		}
		a.openStudioWebView(assetPath, bg)
		if !isPreview {
			a.studioSyncWebView(project)
		}
	} else if a.StudioWebView != nil {
		a.closeStudioWebView()
	}

	slides := data.Slides
	agentPage := isPreview || data.EntryHTML != ""
	currIdx := a.StudioSelectedSlideIdx
	var currentSlide corestudio.SlideItem
	if !isPreview {
		if len(slides) > 0 {
			if currIdx < 0 || currIdx >= len(slides) {
				currIdx = 0
			}
			currentSlide = slides[currIdx]
		} else if flavor == "slides" {
			slides = corestudio.DemoSlides()
			if len(slides) > 0 {
				if currIdx < 0 || currIdx >= len(slides) {
					currIdx = 0
				}
				currentSlide = slides[currIdx]
			}
		}
	}

	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// Unified Viewer Toolbar
		ui.Row(c.Key("studio-unified-toolbar")).Height(36).Padding(0, 14).Gap(8).AlignItems(ui.Center).
			Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {

			if isPreview {
				ui.Icon(c, icons.TablerBrowser).Width(14).Height(14).TextColor(tc.Accent)
				ui.Text(c, filepath.Base(assetPath)).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine().Tooltip(assetPath)
				ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Warning.Alpha(0.15)).Children(func() {
					ui.Text(c, "VIEW ONLY").FontSize(9).FontWeight(700).TextColor(tc.Warning)
				})
				ui.Box(c).Grow(1)

				closePrevBtn := ui.Row(c.Key("studio-close-preview")).Height(24).Padding(0, 8).Gap(4).Radius(5).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				closePrevBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, "Close Preview").FontSize(11).FontWeight(500).TextColor(tc.Text).SingleLine()
				})
				if closePrevBtn.Hovered() {
					closePrevBtn.Background(tc.ToolbarControlHover)
				}
				if closePrevBtn.Clicked() {
					a.closeStudioPreview()
				}
			} else {
				switch flavor {
				case "motion":
					ui.Icon(c, icons.TablerMovie).Width(14).Height(14).TextColor(tc.Accent)
					title := "Motion Composition"
					if data.EntryHTML != "" {
						title = "Motion — " + filepath.Base(data.EntryHTML)
					}
					ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()

					fps := data.FPS
					if fps <= 0 {
						fps = 60
					}
					spec := fmt.Sprintf("1080P • %d FPS", fps)
					if data.DurationSec > 0 {
						spec = fmt.Sprintf("%s • %.0fs", spec, data.DurationSec)
					}
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Warning.Alpha(0.15)).Children(func() {
						ui.Text(c, spec).FontSize(9).FontWeight(700).TextColor(tc.Warning)
					})

					ui.Box(c).Grow(1)
					voice := data.VoiceID
					if voice == "" {
						voice = "Default TTS Voice"
					}
					ui.Text(c, "Voice: "+voice).FontSize(11).TextColor(tc.TextMuted).SingleLine()

				case "3d":
					ui.Icon(c, icons.TablerRotate).Width(14).Height(14).TextColor(tc.Success)
					title := "3D WebGL Sandbox"
					if data.EntryHTML != "" {
						title = "3D Scene — " + filepath.Base(data.EntryHTML)
					}
					ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Success.Alpha(0.15)).Children(func() {
						ui.Text(c, "60 FPS").FontSize(9).FontWeight(700).TextColor(tc.Success)
					})
					ui.Box(c).Grow(1)

					if a.StudioWebView != nil && !a.StudioWebView.IsDestroyed() {
						if errCount := len(data.LastErrors); errCount > 0 {
							ui.Row(c).Padding(2, 6).Radius(4).Background(tc.Error.Alpha(0.15)).Gap(4).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons.TablerAlertTriangle).Width(11).Height(11).TextColor(tc.Error)
								ui.Text(c, fmt.Sprintf("%d Error(s)", errCount)).FontSize(10).FontWeight(600).TextColor(tc.Error)
							})
						} else {
							ui.Row(c).Padding(2, 6).Radius(4).Background(tc.Success.Alpha(0.15)).Gap(4).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons.TablerCircleCheck).Width(11).Height(11).TextColor(tc.Success)
								ui.Text(c, "WebGL Ready").FontSize(10).FontWeight(600).TextColor(tc.Success)
							})
						}
					}

				case "pdf":
					ui.Icon(c, icons.TablerFileText).Width(14).Height(14).TextColor(tc.Error)
					title := "PDF Document"
					if data.EntryHTML != "" {
						title = filepath.Base(data.EntryHTML)
					}
					ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Error.Alpha(0.15)).Children(func() {
						ui.Text(c, "PDF").FontSize(9).FontWeight(700).TextColor(tc.Error)
					})
					ui.Box(c).Grow(1)

				case "pptx":
					ui.Icon(c, icons.TablerPresentation).Width(14).Height(14).TextColor(tc.Warning)
					title := "PowerPoint Presentation"
					if data.EntryHTML != "" {
						title = filepath.Base(data.EntryHTML)
					}
					ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Warning.Alpha(0.15)).Children(func() {
						ui.Text(c, "PPTX").FontSize(9).FontWeight(700).TextColor(tc.Warning)
					})
					ui.Box(c).Grow(1)

				case "html":
					ui.Icon(c, icons.TablerBrowser).Width(14).Height(14).TextColor(tc.Accent)
					title := "HTML / Web Preview"
					if data.EntryHTML != "" {
						title = filepath.Base(data.EntryHTML)
					}
					ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Accent.Alpha(0.15)).Children(func() {
						ui.Text(c, "HTML / WEB").FontSize(9).FontWeight(700).TextColor(tc.Accent)
					})
					ui.Box(c).Grow(1)

				default: // slides
					ui.Icon(c, icons.TablerPresentation).Width(14).Height(14).TextColor(tc.Accent)
					if data.EntryHTML != "" {
						ui.Text(c, "Deck — "+filepath.Base(data.EntryHTML)).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
					} else {
						ui.Text(c, fmt.Sprintf("Slide %02d of %02d", currIdx+1, len(slides))).FontSize(12).FontWeight(600).TextColor(tc.Text).SingleLine()
						layoutStr := currentSlide.Layout
						if layoutStr == "" {
							layoutStr = "hero"
						}
						ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Accent.Alpha(0.15)).Children(func() {
							ui.Text(c, strings.ToUpper(layoutStr)).FontSize(9).FontWeight(700).TextColor(tc.Accent)
						})
					}
					ui.Box(c).Grow(1)

					// Navigation Controls (< Prev | Next >) for parsed decks
					if data.EntryHTML == "" && len(slides) > 1 {
						prevBtn := IconButton(c, tc, icons.TablerArrowLeft, 12)
						if prevBtn.Clicked() && currIdx > 0 {
							a.StudioSelectedSlideIdx--
							a.studioWebViewRun(fmt.Sprintf("if (typeof showSlide === 'function') { showSlide(%d); }", a.StudioSelectedSlideIdx))
						}
						nextBtn := IconButton(c, tc, icons.TablerArrowRight, 12)
						if nextBtn.Clicked() && currIdx < len(slides)-1 {
							a.StudioSelectedSlideIdx++
							a.studioWebViewRun(fmt.Sprintf("if (typeof showSlide === 'function') { showSlide(%d); }", a.StudioSelectedSlideIdx))
						}
						ui.Box(c).Size(1, 14).Background(tc.Border)
					}
				}
			}

			// Download / Export Menu (PDF, PPTX for slides, MP4 for motion, HTML)
			if assetPath != "" && !isPreview {
				dlBtn := ui.Row(c.Key("studio-canvas-download")).Height(26).Padding(0, 8).Gap(5).Radius(6).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.ToolbarBorder).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				dlBtn.Children(func() {
					ui.Icon(c, icons.TablerDownload).Width(12).Height(12).TextColor(tc.Accent)
					ui.Text(c, "Download").FontSize(11).FontWeight(600).TextColor(tc.Text)
					ui.Icon(c, icons.TablerChevronDown).Width(10).Height(10).TextColor(tc.IconMuted)
				})
				if dlBtn.Hovered() {
					dlBtn.Background(tc.ToolbarControlHover)
				}
				dlBtn.Menu(func(m *ui.Menu) {
					if m.Item("Download as PDF (.pdf)").Chosen() {
						go a.studioDownloadCanvasPDF(assetPath, project)
					}
					if flavor == "slides" || flavor == "pptx" || len(slides) > 0 {
						if m.Item("Download as PowerPoint (.pptx)").Chosen() {
							go a.studioDownloadCanvasPPTX(project, slides, assetPath)
						}
					}
					if flavor == "motion" {
						if m.Item("Download as MP4 Video (.mp4)").Chosen() {
							go a.studioDownloadCanvasMP4(project, data, assetPath)
						}
					}
					m.Separator()
					if m.Item("Download Source File (" + strings.ToUpper(strings.TrimPrefix(filepath.Ext(assetPath), ".")) + ")").Chosen() {
						go a.studioDownloadMediaFile(assetPath, filepath.Base(assetPath))
					}
					if m.Item("Reveal in Finder").Chosen() {
						mygo.Shell.ShowItemInFolder(assetPath)
					}
				})
			}

			// Open local HTML / Web file in viewer-only preview mode (never added to artifacts)
			openHTMLBtn := IconButton(c, tc, icons.TablerFileCode, 12)
			openHTMLBtn.Tooltip("Preview local HTML/file in Viewer (view only)")
			if openHTMLBtn.Clicked() {
				go a.studioOpenLocalHTMLFile()
			}

			// Reload button
			reloadBtn := IconButton(c, tc, icons.TablerRefresh, 12)
			reloadBtn.Tooltip("Reload viewer")
			if reloadBtn.Clicked() {
				a.closeStudioWebView()
				a.closeStudioNodeWebView()
				a.studioWebviewSynced = false
			}

			// Agent artifact explorer in Right Sidebar
			artActive := a.ShowRightDrawer && a.RightDrawerTab == RightTabArtifacts
			artBtn := ToggleIconButton(c, tc, icons.TablerFolderOpen, 12, artActive)
			artBtn.Tooltip("Artifacts & Explorer (Right sidebar)")
			if artBtn.Clicked() {
				if a.ShowRightDrawer && a.RightDrawerTab == RightTabArtifacts {
					a.ShowRightDrawer = false
				} else {
					a.ShowRightDrawer = true
					a.pickDrawerPaneSingleton(RightTabArtifacts, "Artifacts")
				}
			}

			// Open in external browser / viewer
			if assetPath != "" {
				extBtn := IconButton(c, tc, icons.TablerExternalLink, 12)
				extBtn.Tooltip("Open in External Viewer")
				if extBtn.Clicked() {
					a.openBrowserTab("file://" + assetPath)
				}
			}
		})

		// Main Viewport (WebView interactive runtime with pure-Go GPU fallback)
		if a.StudioWebView != nil && !a.StudioWebView.IsDestroyed() {
			ui.WebView(c, a.StudioWebView).Grow(1).FillWidth()
		} else {
			ui.Column(c.Key("studio-stage-viewport")).Grow(1).Padding(24, 32).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				stageW, stageH := a.studioStageSize(c)
				switch flavor {
				case "motion":
					stage := ui.Column(c.Key("motion-canvas-card")).Width(stageW).Height(stageH).Radius(12).Padding(32, 40).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center)
					stage.Children(func() {
						ui.Text(c, "ForgeADE Studio").FontSize(34).FontWeight(800).TextColor(tc.Text)
						ui.Text(c, "Kinetic Rhythm & Audio Synchronization").FontSize(15).FontWeight(500).TextColor(tc.Accent)
						ui.Box(c).Size(1, 48)
						script := data.ScriptText
						if script == "" {
							script = "ForgeADE Studio introduces a fluid agentic workspace where design skills meet real-time motion."
						}
						ui.Row(c).Padding(8, 20).Radius(20).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
							ui.Text(c, script).FontSize(13).FontWeight(500).TextColor(tc.Text)
						})
					})

				case "3d":
					scene := ui.Column(c.Key("live-3d-scene")).Width(stageW).Height(stageH).Radius(12).
						Background(ui.Hex("#05070d")).BorderWidth(1).BorderColor(tc.Border).Clip().
						AlignItems(ui.Center).Justify(ui.Center)
					scene.Children(func() {
						ui.Text(c, "WebGL 3D Scene Viewport").FontSize(18).FontWeight(700).TextColor(tc.Text)
						ui.Text(c, "OrbitControls active • Three.js canvas initialized").FontSize(12).TextColor(tc.TextMuted)
					})

				case "html":
					webCard := ui.Column(c.Key("live-html-stage")).Width(stageW).Height(stageH).Radius(12).Padding(24, 32).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center).Gap(8)
					webCard.Children(func() {
						ui.Icon(c, icons.TablerBrowser).Width(28).Height(28).TextColor(tc.Accent)
						title := "HTML / Web Content"
						if data.EntryHTML != "" {
							title = filepath.Base(data.EntryHTML)
						}
						ui.Text(c, title).FontSize(16).FontWeight(700).TextColor(tc.Text)
						ui.Text(c, "Interactive HTML & Web preview active in WebView runtime").FontSize(11).TextColor(tc.TextMuted)
					})

				default: // slides
					scale := min(float32(1.0), stageW/720.0)
					cardPadY := max(float32(14), 36*scale)
					cardPadX := max(float32(18), 44*scale)
					titleSize := max(float32(15), 24*scale)
					subSize := max(float32(11), 13*scale)
					bulletSize := max(float32(10), 12*scale)
					slideCard := ui.Column(c.Key("live-slide-stage")).Width(stageW).Height(stageH).Radius(12).
						Padding(cardPadY, cardPadX).Clip().
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Shadow(0, 16, 40, 0, tc.Border.Alpha(0.4))
					slideCard.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("%02d", currIdx+1)).FontSize(max(10, 12*scale)).FontWeight(700).TextColor(tc.Accent)
							ui.Box(c).Grow(1)
							ui.Text(c, "16:9 PRESENTATION").FontSize(max(8, 10*scale)).FontWeight(600).TextColor(tc.TextMuted)
						})
						ui.Box(c).Size(1, max(8, 18*scale))
						title := currentSlide.Title
						if title == "" {
							title = "Presentation Stage"
						}
						ui.Text(c, title).FontSize(titleSize).FontWeight(700).TextColor(tc.Text).LineHeight(1.2)
						ui.Box(c).Size(1, max(4, 6*scale))
						if currentSlide.Subtitle != "" {
							ui.Text(c, currentSlide.Subtitle).FontSize(subSize).FontWeight(500).TextColor(tc.TextMuted)
							ui.Box(c).Size(1, max(8, 14*scale))
						}
						ui.Column(c).Gap(max(4, 8*scale)).Children(func() {
							for _, b := range currentSlide.Bullets {
								ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
									ui.Row(c).Size(5, 5).Radius(3).Background(tc.Accent)
									ui.Text(c, b).FontSize(bulletSize).TextColor(tc.Text)
								})
							}
						})
					})
				}
			})
		}

		// Bottom Dock:
		// If slides: outline thumbnails strip
		if flavor == "slides" && len(slides) > 0 && !agentPage {
			ui.Row(c.Key("slides-thumbnails-strip")).Height(65).Padding(8, 16).Gap(8).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				ui.ScrollHorizontal(c.Key("slides-thumbnails-scroll")).Grow(1).MinWidth(0).Scrollbars(false).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						for i, slideItem := range slides {
							idx := i
							s := slideItem
							isSelected := idx == currIdx

							thumb := ui.Row(c.Key(fmt.Sprintf("thumb-strip-%d", idx))).Height(46).Width(120).Radius(6).
								Padding(4, 8).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)

							if isSelected {
								thumb.Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent)
							} else {
								thumb.Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border)
								if thumb.Hovered() {
									thumb.BorderColor(tc.Border.Alpha(0.8))
								}
							}

							thumb.Children(func() {
								ui.Text(c, fmt.Sprintf("%02d", idx+1)).FontSize(10).FontWeight(700).TextColor(tc.Accent)
								ui.Column(c).Grow(1).Children(func() {
									ui.Text(c, s.Title).FontSize(10).FontWeight(600).SingleLine()
								})
							})

							if thumb.Clicked() {
								a.StudioSelectedSlideIdx = idx
								a.studioWebViewRun(fmt.Sprintf("if (typeof showSlide === 'function') { showSlide(%d); }", idx))
							}
						}
					})
				})
			})
		}
	})
}

// renderStudioSlidesViewer renders the presentation viewer on the right pane (delegates to unified viewer).
func (a *AppUI) renderStudioSlidesViewer(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject, data corestudio.ArtifactData) {
	a.renderStudioUnifiedCanvasViewer(c, tc, project, data)
}

// renderStudioMotionViewer renders the motion video stage and synchronized timeline scrubber (delegates to unified viewer).
func (a *AppUI) renderStudioMotionViewer(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject, data corestudio.ArtifactData) {
	a.renderStudioUnifiedCanvasViewer(c, tc, project, data)
}

// renderStudio3DSandboxViewer renders the interactive WebGL 3D sandbox (delegates to unified viewer).
func (a *AppUI) renderStudio3DSandboxViewer(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject, data corestudio.ArtifactData) {
	a.renderStudioUnifiedCanvasViewer(c, tc, project, data)
}

// renderStudioNodeGraph renders the Visual Node Graph Workflow for Image & Video Generator!
func (a *AppUI) renderStudioNodeGraph(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject) {
	cfg := corestudio.LoadStudioConfig()

	// Ensure the interactive Node Graph WebView is initialized
	a.ensureStudioNodeWebView(project)

	ui.Column(c.Key("node-workflow-stage")).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
		// Node Workflow Top Action Bar
		ui.Row(c.Key("node-workflow-topbar")).Height(42).Padding(0, 16).Gap(10).AlignItems(ui.Center).
			Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {

			ui.Icon(c, icons.TablerGitBranch).Width(14).Height(14).TextColor(tc.Accent)
			ui.Text(c, "Visual Node Workflow").FontSize(12).FontWeight(700).TextColor(tc.Text)

			pipelineLabel := "IMAGE PIPELINE"
			if project != nil && project.Type == corestudio.ArtifactTypeVideo {
				pipelineLabel = "VIDEO PIPELINE"
			}
			ui.Row(c).Padding(1, 6).Radius(4).Background(tc.Accent.Alpha(0.15)).Children(func() {
				ui.Text(c, pipelineLabel).FontSize(9).FontWeight(700).TextColor(tc.Accent)
			})

			// Active API chip
			endpoint := cfg.ImageEndpoint
			if endpoint == "" {
				endpoint = "Custom API"
			}
			apiChip := ui.Row(c).Padding(2, 6).Radius(4).Background(tc.ToolbarControl).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			apiChip.Children(func() {
				ui.Row(c).Size(6, 6).Radius(3).Background(tc.Success)
				ui.Text(c, "API: "+endpoint).FontSize(10).TextColor(tc.TextMuted).SingleLine()
			})
			apiChip.Tooltip("Configure API in Studio Settings")
			if apiChip.Clicked() {
				a.openSettings(settings.SettingsTabStudio)
			}

			ui.Box(c).Grow(1)

			// Settings button
			settingsBtn := ui.Row(c.Key("node-workflow-api-settings")).Height(26).Padding(0, 8).Radius(5).
				Background(tc.ToolbarControl).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			settingsBtn.Children(func() {
				ui.Icon(c, icons.TablerSettings).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "API Settings").FontSize(11).TextColor(tc.TextMuted)
			})
			if settingsBtn.Clicked() {
				a.openSettings(settings.SettingsTabStudio)
			}

			// Primary Action: Queue Generation (⌘↵)
			queueBtn := ui.Row(c.Key("btn-queue-prompt")).Height(28).Padding(0, 14).Radius(6).
				Background(tc.Accent).AlignItems(ui.Center).Gap(6).Cursor(ui.CursorPointer)
			queueBtn.Children(func() {
				if a.StudioIsGenerating {
					ui.Spinner(c).Size(12, 12).Label("")
					ui.Text(c, "Generating...").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
				} else {
					ui.Icon(c, icons.TablerSparkles).Width(13).Height(13).TextColor(tc.AccentForeground)
					ui.Text(c, "Queue Generation (⌘↵)").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
				}
			})
			if queueBtn.Clicked() && !a.StudioIsGenerating {
				a.studioQueuePrompt()
			}
		})

		// Generation Progress Bar (if executing)
		if a.StudioIsGenerating {
			ui.Row(c.Key("node-workflow-prog-bar")).Height(24).Padding(0, 16).Gap(8).AlignItems(ui.Center).
				Background(tc.Accent.Alpha(0.1)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Accent.Alpha(0.2)).Children(func() {
				ui.Spinner(c).Size(10, 10).Label("")
				ui.Text(c, a.StudioGenerationStatus).FontSize(10).FontWeight(600).TextColor(tc.Accent)
			})
		}

		// Node Graph Interactive Viewport
		if a.StudioNodeWebView != nil && !a.StudioNodeWebView.IsDestroyed() {
			ui.WebView(c, a.StudioNodeWebView).Grow(1).FillWidth()
		} else {
			a.renderStudioNodeFallbackStage(c, tc, project)
		}
	})
}

// renderStudioNodeFallbackStage renders the native fallback node canvas for headless environments or when WebView is unavailable.
func (a *AppUI) renderStudioNodeFallbackStage(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject) {
	cfg := corestudio.LoadStudioConfig()
	// Node Graph Canvas with Nodes and Visual Connections
	ui.ScrollBoth(c.Key("node-workflow-viewport")).Grow(1).Padding(24, 28).Children(func() {
		ui.Row(c.Key("node-workflow-row")).Gap(14).AlignItems(ui.Start).Children(func() {

			// -------------------------------------------------------------
			// NODE 1: Text Prompt Node (CLIP Text Encode)
			// -------------------------------------------------------------
			ui.Column(c.Key("node-prompt-box")).Width(270).Radius(10).Background(tc.Surface).
				BorderWidth(1).BorderColor(tc.Border).Padding(12, 14).Gap(8).
				Shadow(0, 8, 20, 0, tc.Border.Alpha(0.4)).Children(func() {

				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "1. CLIP Text Encode").FontSize(11).FontWeight(700).TextColor(tc.Text)
					ui.Box(c).Grow(1)
					// Port output indicator (Amber)
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Warning.Alpha(0.15)).Gap(3).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Size(6, 6).Radius(3).Background(tc.Warning)
						ui.Text(c, "COND").FontSize(8).FontWeight(700).TextColor(tc.Warning)
					})
				})

				ui.Text(c, "Positive Prompt").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
				ui.TextInput(c, &a.StudioPositivePrompt).Placeholder("Describe your visual scene...").Grow(1)

				ui.Text(c, "Negative Prompt").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
				ui.TextInput(c, &a.StudioNegativePrompt).Placeholder("Artifacts to exclude...").Grow(1)

				// Style Preset Chips
				ui.Row(c).Wrap().Gap(4).Padding(4, 0, 0, 0).Children(func() {
					preset := func(label, token string) {
						pBtn := ui.Row(c.Key("preset-"+label)).Height(18).Padding(0, 6).Radius(4).
							Background(tc.ToolbarControl).AlignItems(ui.Center).Cursor(ui.CursorPointer)
						pBtn.Children(func() {
							ui.Text(c, label).FontSize(8).FontWeight(500).TextColor(tc.TextMuted)
						})
						if pBtn.Clicked() {
							if a.StudioPositivePrompt == "" {
								a.StudioPositivePrompt = token
							} else {
								a.StudioPositivePrompt += ", " + token
							}
						}
					}
					preset("Cinematic", "cinematic volumetric lighting 8k")
					preset("Cyberpunk", "neon rain reflections cyberpunk")
					preset("Photoreal", "photorealistic 35mm lens")
					preset("Anime", "makoto shinkai style aesthetic")
				})
			})

			// Cable 1 -> 2
			ui.Column(c).Padding(80, 0, 0, 0).Gap(2).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Size(24, 2).Background(tc.Warning.Alpha(0.8))
				ui.Icon(c, icons.TablerArrowRight).Width(12).Height(12).TextColor(tc.Warning)
			})

			// -------------------------------------------------------------
			// NODE 2: Model & KSampler Parameters Node
			// -------------------------------------------------------------
			ui.Column(c.Key("node-model-box")).Width(250).Radius(10).Background(tc.Surface).
				BorderWidth(1).BorderColor(tc.Border).Padding(12, 14).Gap(8).
				Shadow(0, 8, 20, 0, tc.Border.Alpha(0.4)).Children(func() {

				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "2. KSampler").FontSize(11).FontWeight(700).TextColor(tc.Text)
					ui.Box(c).Grow(1)
					// Port output indicator (Purple)
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Secondary.Alpha(0.15)).Gap(3).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Size(6, 6).Radius(3).Background(tc.Secondary)
						ui.Text(c, "LATENT").FontSize(8).FontWeight(700).TextColor(tc.Secondary)
					})
				})

				// Sampler selector pills
				ui.Text(c, "Sampler").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
				ui.Row(c).Gap(4).Children(func() {
					sPill := func(name string) {
						active := a.StudioSampler == name
						btn := ui.Row(c.Key("sampler-"+name)).Height(20).Padding(0, 6).Radius(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
						if active {
							btn.Background(tc.Accent)
							ui.Text(c, name).FontSize(9).FontWeight(600).TextColor(tc.AccentForeground)
						} else {
							btn.Background(tc.ToolbarControl)
							ui.Text(c, name).FontSize(9).TextColor(tc.TextMuted)
						}
						if btn.Clicked() {
							a.StudioSampler = name
						}
					}
					sPill("Euler a")
					sPill("DPM++ 2M")
					sPill("DDIM")
				})

				// Steps counter
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Steps:").FontSize(10).TextColor(tc.TextMuted).Width(60)
					minusBtn := IconButton(c, tc, icons.TablerMinus, 10)
					if minusBtn.Clicked() && a.StudioSteps > 5 {
						a.StudioSteps -= 5
					}
					ui.Text(c, fmt.Sprintf("%d", a.StudioSteps)).FontSize(11).FontWeight(600).TextColor(tc.Text).Center()
					plusBtn := IconButton(c, tc, icons.TablerPlus, 10)
					if plusBtn.Clicked() && a.StudioSteps < 60 {
						a.StudioSteps += 5
					}
				})

				// Seed with Randomize button
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Seed:").FontSize(10).TextColor(tc.TextMuted).Width(60)
					ui.Text(c, fmt.Sprintf("%d", a.StudioSeed)).FontSize(10).FontWeight(600).TextColor(tc.Text).SingleLine().Grow(1)
					randBtn := ui.Row(c.Key("btn-rand-seed")).Height(18).Padding(0, 5).Radius(3).
						Background(tc.ToolbarControl).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					randBtn.Children(func() {
						ui.Icon(c, icons.TablerRefresh).Width(10).Height(10).TextColor(tc.IconMuted)
					})
					if randBtn.Clicked() {
						a.StudioSeed = time.Now().UnixNano() % 10000000
					}
				})

				// Aspect Ratio
				ui.Text(c, "Aspect Ratio").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
				ui.Row(c).Gap(4).Children(func() {
					aspPill := func(name string) {
						active := a.StudioAspectRatio == name
						btn := ui.Row(c.Key("asp-"+name)).Height(20).Padding(0, 6).Radius(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
						if active {
							btn.Background(tc.Accent)
							ui.Text(c, name).FontSize(9).FontWeight(600).TextColor(tc.AccentForeground)
						} else {
							btn.Background(tc.ToolbarControl)
							ui.Text(c, name).FontSize(9).TextColor(tc.TextMuted)
						}
						if btn.Clicked() {
							a.StudioAspectRatio = name
						}
					}
					aspPill("16:9")
					aspPill("1:1")
					aspPill("9:16")
				})
			})

			// Cable 2 -> 3
			ui.Column(c).Padding(80, 0, 0, 0).Gap(2).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Size(24, 2).Background(tc.Secondary.Alpha(0.8))
				ui.Icon(c, icons.TablerArrowRight).Width(12).Height(12).TextColor(tc.Secondary)
			})

			// -------------------------------------------------------------
			// NODE 3: Custom API Provider Node (From Settings)
			// -------------------------------------------------------------
			ui.Column(c.Key("node-api-box")).Width(250).Radius(10).Background(tc.Surface).
				BorderWidth(1).BorderColor(tc.Border).Padding(12, 14).Gap(8).
				Shadow(0, 8, 20, 0, tc.Border.Alpha(0.4)).Children(func() {

				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "3. Custom API Engine").FontSize(11).FontWeight(700).TextColor(tc.Text)
					ui.Box(c).Grow(1)
					// Port output indicator (Cyan)
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Info.Alpha(0.15)).Gap(3).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Size(6, 6).Radius(3).Background(tc.Info)
						ui.Text(c, "IMAGE").FontSize(8).FontWeight(700).TextColor(tc.Info)
					})
				})

				ui.Text(c, "Endpoint URL").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
				endpoint := cfg.ImageEndpoint
				if endpoint == "" {
					endpoint = "http://localhost:8000/v1/images/generations"
				}
				ui.Text(c, endpoint).FontSize(10).TextColor(tc.Text).SingleLine()

				ui.Text(c, "Model").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
				model := a.StudioModel
				if model == "" {
					model = cfg.ImageModel
				}
				if model == "" {
					model = "flux-1-schnell"
				}
				ui.Text(c, model).FontSize(11).FontWeight(600).TextColor(tc.Accent)

				ui.Row(c).Padding(2, 6).Radius(4).Background(tc.Success.Alpha(0.15)).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerCircleCheck).Width(10).Height(10).TextColor(tc.Success)
					ui.Text(c, "Ready / Connected").FontSize(10).FontWeight(600).TextColor(tc.Success)
				})

				// Configure in settings link
				cfgLink := ui.Row(c.Key("link-comfy-settings")).Height(24).Padding(0, 6).Radius(4).
					Background(tc.ToolbarControl).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				cfgLink.Children(func() {
					ui.Icon(c, icons.TablerSettings).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, "Edit API in Settings").FontSize(10).TextColor(tc.TextMuted)
				})
				if cfgLink.Clicked() {
					a.openSettings(settings.SettingsTabStudio)
				}
			})

			// Cable 3 -> 4
			ui.Column(c).Padding(80, 0, 0, 0).Gap(2).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Size(24, 2).Background(tc.Info.Alpha(0.8))
				ui.Icon(c, icons.TablerArrowRight).Width(12).Height(12).TextColor(tc.Info)
			})

			// -------------------------------------------------------------
			// NODE 4: Result / Preview Node
			// -------------------------------------------------------------
			ui.Column(c.Key("node-preview-box")).Width(360).Radius(10).Background(tc.Surface).
				BorderWidth(1).BorderColor(tc.Border).Padding(12, 14).Gap(10).
				Shadow(0, 8, 20, 0, tc.Border.Alpha(0.4)).Children(func() {

				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "4. Latent Decode & Image Preview").FontSize(11).FontWeight(700).TextColor(tc.Text)
					ui.Box(c).Grow(1)
					statusBadge := "Ready"
					if a.StudioIsGenerating {
						statusBadge = "Generating..."
					}
					ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Accent.Alpha(0.15)).Children(func() {
						ui.Text(c, statusBadge).FontSize(9).FontWeight(700).TextColor(tc.Accent)
					})
				})

				// High-Resolution Preview Screen
				previewBox := ui.Column(c).Height(240).Radius(8).Background(tc.Canvas).
					BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center).Clip()

				previewBox.Children(func() {
					if a.StudioIsGenerating {
						ui.Spinner(c).Size(24, 24).Label("Sampling latent tensor...")
						ui.Box(c).Size(1, 8)
						ui.Text(c, a.StudioGenerationStatus).FontSize(11).TextColor(tc.TextMuted)
					} else if img, _, _, _ := a.getStudioPreviewImage(); img != nil {
						ui.Image(c, img).Fit(ui.Contain).Grow(1)
					} else {
						ui.Icon(c, icons.TablerPhoto).Width(48).Height(48).TextColor(tc.Accent)
						ui.Box(c).Size(1, 8)
						ui.Text(c, "Ready to Render").FontSize(13).FontWeight(600).TextColor(tc.Text)
						ui.Text(c, "Click 'Queue Generation (⌘↵)' to execute node workflow").FontSize(10).TextColor(tc.TextMuted)
					}
				})

				// Action buttons
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					btn := ui.Row(c.Key("btn-preview-send-editor")).Height(26).Padding(0, 10).Radius(5).
						Background(tc.Accent).AlignItems(ui.Center).Gap(4).Cursor(ui.CursorPointer)
					btn.Children(func() {
						ui.Icon(c, icons.TablerExternalLink).Width(11).Height(11).TextColor(tc.AccentForeground)
						ui.Text(c, "Send to Editor (⌘⇧E)").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
					})
					if btn.Clicked() {
						a.studioExportActive()
					}

					if a.StudioGeneratedImagePath != "" {
						finderBtn := ui.Row(c.Key("btn-preview-finder")).Height(26).Padding(0, 8).Radius(5).
							Background(tc.ToolbarControl).AlignItems(ui.Center).Gap(4).Cursor(ui.CursorPointer)
						finderBtn.Children(func() {
							ui.Icon(c, icons.TablerFolderOpen).Width(11).Height(11).TextColor(tc.IconMuted)
							ui.Text(c, "Finder").FontSize(10).TextColor(tc.TextMuted)
						})
						if finderBtn.Clicked() {
							mygo.Shell.ShowItemInFolder(a.StudioGeneratedImagePath)
						}
					}
				})
			})
		})
	})
}

var (
	studioCachedImgPath string
	studioCachedImgSrc  ui.ImageSource
	studioCachedImgW    int
	studioCachedImgH    int
	studioCachedImgFmt  string
)

func (a *AppUI) getStudioPreviewImage() (ui.ImageSource, int, int, string) {
	if a.StudioGeneratedImagePath == "" {
		return nil, 0, 0, ""
	}
	if a.StudioGeneratedImagePath == studioCachedImgPath && studioCachedImgSrc != nil {
		return studioCachedImgSrc, studioCachedImgW, studioCachedImgH, studioCachedImgFmt
	}
	data, err := os.ReadFile(a.StudioGeneratedImagePath)
	if err != nil {
		return nil, 0, 0, ""
	}
	if strings.HasSuffix(strings.ToLower(a.StudioGeneratedImagePath), ".svg") {
		svg, err := ui.ParseSVG(data)
		if err == nil && svg != nil {
			sw, sh := svg.Size()
			studioCachedImgPath = a.StudioGeneratedImagePath
			studioCachedImgSrc = svg
			studioCachedImgW = int(sw)
			studioCachedImgH = int(sh)
			studioCachedImgFmt = "SVG"
			return svg, int(sw), int(sh), "SVG"
		}
	}
	bmp, err := ui.DecodeBitmap(data)
	if err == nil && bmp != nil {
		bw, bh := bmp.Size()
		studioCachedImgPath = a.StudioGeneratedImagePath
		studioCachedImgSrc = bmp
		studioCachedImgW = bw
		studioCachedImgH = bh
		studioCachedImgFmt = "PNG"
		return bmp, bw, bh, "PNG"
	}
	return nil, 0, 0, ""
}

// studioQueuePrompt executes the active Native Studio Workflow (Pipeline Builder
// or ComfyUI JSON Injection) asynchronously and updates preview, history, and logs.
func (a *AppUI) studioQueuePrompt() {
	if a.StudioIsGenerating {
		return
	}
	a.StudioIsGenerating = true
	a.StudioGenerationProgress = 0.05
	a.StudioGenerationStatus = "Starting Native Workflow..."
	a.StudioWorkflowError = ""
	if a.Window != nil {
		a.Window.Invalidate()
	}

	var proj *corestudio.StudioProject
	if a.Studio != nil {
		proj = a.Studio.ActiveProject()
	}
	wf := a.activeStudioWorkflow(proj)

	// Keep prompt step synced if user updated StudioPositivePrompt directly (e.g. tests or shortcuts)
	if wf != nil && wf.Mode == "pipeline" {
		for i := range wf.Steps {
			if wf.Steps[i].Type == corestudio.StepTypePrompt {
				if a.StudioPositivePrompt != "" && wf.Steps[i].GetString("positive", "") == "" {
					wf.Steps[i].SetParam("positive", a.StudioPositivePrompt)
				}
				if a.StudioNegativePrompt != "" && wf.Steps[i].GetString("negative", "") == "" {
					wf.Steps[i].SetParam("negative", a.StudioNegativePrompt)
				}
			}
		}
	}

	cfg := corestudio.LoadStudioConfig()
	outDir := a.studioOutputsDir()
	wfCopy := *wf

	go func() {
		res, err := corestudio.RunWorkflow(context.Background(), &wfCopy, corestudio.WorkflowRunOptions{
			Config:    cfg,
			OutputDir: outDir,
			OnProgress: func(stepIdx, total int, status string, progress float32) {
				a.StudioGenerationProgress = progress
				a.StudioGenerationStatus = status
				a.syncNodeWorkflowProgress(true, status, progress, "")
				if a.Window != nil {
					a.Window.Invalidate()
				}
			},
		})

		a.StudioIsGenerating = false
		if err == nil && res != nil {
			a.StudioGenerationProgress = 1.0
			a.StudioGeneratedImagePath = res.FilePath
			a.StudioGeneratedFormat = res.Format
			a.StudioGeneratedWidth = res.Width
			a.StudioGeneratedHeight = res.Height
			a.StudioWorkflowIsVideoOutput = res.IsVideo
			a.StudioWorkflowLogs = res.StepLogs
			a.StudioWorkflowLastInjectedJSON = res.InjectedJSON
			a.StudioGenerationStatus = fmt.Sprintf("Workflow Complete: %s (%dx%d)", filepath.Base(res.FilePath), res.Width, res.Height)
			a.studioMediaScanAt = time.Time{} // refresh gallery cache
			a.syncNodeWorkflowProgress(false, a.StudioGenerationStatus, 1.0, res.FilePath)

			if a.Studio != nil {
				activeProj := a.Studio.ActiveProject()
				if activeProj != nil {
					activeProj.AddVersion(a.StudioPositivePrompt, fmt.Sprintf("Workflow Run: %s", filepath.Base(res.FilePath)), corestudio.ArtifactData{
						OutputAsset: filepath.Base(res.FilePath),
						AspectRatio: a.StudioAspectRatio,
					}, nil)
				}
			}
		} else {
			a.StudioGenerationProgress = 0
			msg := "Workflow execution failed"
			if err != nil {
				msg = err.Error()
			}
			a.StudioGenerationStatus = msg
			a.StudioWorkflowError = msg
			a.syncNodeWorkflowProgress(false, msg, 0, "")
		}

		if a.Window != nil {
			a.Window.Invalidate()
		}
	}()
}

func colorToCSS(col ui.Color) string {
	if col.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", col.R, col.G, col.B)
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %.3f)", col.R, col.G, col.B, float32(col.A)/255.0)
}

func (a *AppUI) buildStudioNodeTheme() (*corestudio.NodeWorkflowTheme, string) {
	if a.ThemeMgr == nil {
		return nil, ""
	}
	tc := a.ThemeMgr.Colors()
	key := a.ThemeMgr.Appearance + ":" + a.ThemeMgr.PaletteID
	return &corestudio.NodeWorkflowTheme{
		IsDark:              a.ThemeMgr.IsDark(),
		BgApp:               colorToCSS(tc.Canvas),
		BgSidebar:           colorToCSS(tc.Chrome),
		BgPanel:             colorToCSS(tc.Surface),
		BgCard:              colorToCSS(tc.SurfaceRaised),
		BgInput:             colorToCSS(tc.Input),
		HudBg:               colorToCSS(tc.Surface.Alpha(0.92)),
		Border:              colorToCSS(tc.Border),
		BorderStrong:        colorToCSS(tc.ToolbarBorder),
		GridDot:             colorToCSS(tc.Text.Alpha(0.10)),
		TextPrimary:         colorToCSS(tc.Text),
		TextSecondary:       colorToCSS(tc.SecondaryLabel),
		TextMuted:           colorToCSS(tc.TextMuted),
		Accent:              colorToCSS(tc.Accent),
		AccentFg:            colorToCSS(tc.AccentForeground),
		AccentSoft:          colorToCSS(tc.Accent.Alpha(0.15)),
		AccentGlow:          colorToCSS(tc.Accent.Alpha(0.35)),
		ToolbarControl:      colorToCSS(tc.ToolbarControl),
		ToolbarControlHover: colorToCSS(tc.ToolbarControlHover),
		Success:             colorToCSS(tc.Success),
		Warning:             colorToCSS(tc.Warning),
		Error:               colorToCSS(tc.Error),
		Info:                colorToCSS(tc.Info),
		Secondary:           colorToCSS(tc.Secondary),
	}, key
}

// ensureStudioNodeWebView initializes and attaches the interactive Visual Node Graph WebView.
func (a *AppUI) ensureStudioNodeWebView(project *corestudio.StudioProject) {
	if a.Window == nil || !mygo.WebViewAvailable() {
		return
	}
	pipeline := "image"
	if project != nil && project.Type == corestudio.ArtifactTypeVideo {
		pipeline = "video"
	}
	nodeTheme, themeKey := a.buildStudioNodeTheme()
	if a.StudioNodeWebView != nil && !a.StudioNodeWebView.IsDestroyed() {
		if !strings.HasSuffix(a.StudioNodeWebViewURL, "#"+pipeline) {
			baseURL := a.StudioNodeWebViewURL
			if idx := strings.Index(baseURL, "#"); idx >= 0 {
				baseURL = baseURL[:idx]
			}
			a.StudioNodeWebView.Page().Eval(fmt.Sprintf("if (typeof setPipeline === 'function') { setPipeline('%s'); }", pipeline))
			a.StudioNodeWebViewURL = baseURL + "#" + pipeline
		}
		if themeKey != "" && a.studioNodeThemeKey != themeKey && nodeTheme != nil {
			if b, err := json.Marshal(nodeTheme); err == nil {
				a.StudioNodeWebView.Page().Eval(fmt.Sprintf("if (typeof applyTheme === 'function') { applyTheme(%s); }", string(b)))
				a.studioNodeThemeKey = themeKey
			}
		}
		return
	}

	htmlPath := a.ensureStudioNodeAssetFile(project, pipeline)
	if htmlPath == "" {
		return
	}
	url := "file://" + htmlPath + "#" + pipeline

	bgHex := "#0c0d11"
	if nodeTheme != nil && nodeTheme.BgApp != "" {
		bgHex = nodeTheme.BgApp
	}
	wv, err := a.Window.NewWebView(mygo.WebViewOptions{
		URL:             url,
		BackgroundColor: bgHex,
	})
	if err == nil {
		a.StudioNodeWebView = wv
		a.StudioNodeWebViewURL = url
		a.studioNodeThemeKey = themeKey

		// Listen for IPC messages from webview via title changes
		wv.Page().OnPageTitleUpdated(func(e *mygo.TitleEvent) {
			if strings.HasPrefix(e.Title, "forgeade:") {
				e.PreventDefault()
				parts := strings.SplitN(e.Title, ":", 3)
				if len(parts) >= 2 {
					action := parts[1]
					payloadRaw := ""
					if len(parts) >= 3 {
						payloadRaw, _ = urlpkg.QueryUnescape(parts[2])
					}
					a.handleNodeWebViewMessage(action, payloadRaw)
				}
			}
		})
	}
}

func (a *AppUI) handleNodeWebViewMessage(action, payloadRaw string) {
	var filePath, fileName string
	if payloadRaw != "" {
		var payload struct {
			PositivePrompt string  `json:"positivePrompt"`
			NegativePrompt string  `json:"negativePrompt"`
			Model          string  `json:"model"`
			AspectRatio    string  `json:"aspectRatio"`
			Steps          int     `json:"steps"`
			CFGScale       float32 `json:"cfgScale"`
			Seed           int64   `json:"seed"`
			FilePath       string  `json:"filePath"`
			FileName       string  `json:"fileName"`
		}
		if err := json.Unmarshal([]byte(payloadRaw), &payload); err == nil {
			if payload.PositivePrompt != "" {
				a.StudioPositivePrompt = payload.PositivePrompt
			}
			if payload.NegativePrompt != "" {
				a.StudioNegativePrompt = payload.NegativePrompt
			}
			if payload.Model != "" {
				a.StudioModel = payload.Model
			}
			if payload.AspectRatio != "" {
				a.StudioAspectRatio = payload.AspectRatio
			}
			if payload.Steps > 0 {
				a.StudioSteps = payload.Steps
			}
			if payload.CFGScale > 0 {
				a.StudioCFGScale = payload.CFGScale
			}
			if payload.Seed != 0 {
				a.StudioSeed = payload.Seed
			}
			filePath = payload.FilePath
			fileName = payload.FileName
		}
	}
	switch action {
	case "queue_prompt", "queue_generation":
		a.studioQueuePrompt()
	case "open_settings":
		a.openSettings(settings.SettingsTabStudio)
	case "export_editor":
		a.studioExportActive()
	case "open_finder", "open_media_folder":
		target := filePath
		if target == "" {
			target = a.StudioGeneratedImagePath
		}
		if target == "" {
			target = a.studioOutputsDir()
			_ = os.MkdirAll(target, 0755)
		}
		mygo.Shell.ShowItemInFolder(target)
	case "download_media":
		src := filePath
		if src == "" {
			src = a.StudioGeneratedImagePath
		}
		if src == "" {
			return
		}
		go a.studioDownloadMediaFile(src, fileName)
	}
}

func (a *AppUI) studioOutputsDir() string {
	if a.Workspace != nil && a.Workspace.RootDir != "" {
		return filepath.Join(a.Workspace.RootDir, ".forge", "studio", "outputs")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge-ade", "studio_outputs")
}

func (a *AppUI) studioDownloadMediaFile(srcPath, suggestedName string) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return
	}
	if suggestedName == "" {
		suggestedName = filepath.Base(srcPath)
	}
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "Downloads", suggestedName)
	destPath, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.Window,
		Title:       "Download Studio Media",
		DefaultPath: defaultPath,
		ButtonLabel: "Save",
	})
	if err != nil || destPath == "" {
		return
	}
	if err := os.WriteFile(destPath, data, 0644); err == nil {
		mygo.Shell.ShowItemInFolder(destPath)
	}
}

func studioSafeStem(project *corestudio.StudioProject, assetPath string) string {
	name := ""
	if assetPath != "" {
		base := filepath.Base(assetPath)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if name == "" && project != nil {
		name = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(project.Name), " ", "-"))
	}
	if name == "" {
		name = "studio-canvas"
	}
	return name
}

func (a *AppUI) studioDownloadCanvasPDF(assetPath string, project *corestudio.StudioProject) {
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "Downloads", studioSafeStem(project, assetPath)+".pdf")
	destPath, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.Window,
		Title:       "Export Canvas as PDF",
		DefaultPath: defaultPath,
		ButtonLabel: "Export PDF",
	})
	if err != nil || destPath == "" {
		return
	}
	if !strings.HasSuffix(strings.ToLower(destPath), ".pdf") {
		destPath += ".pdf"
	}
	if err := corestudio.ExportHTMLToPDF(context.Background(), assetPath, destPath); err != nil {
		// Graceful fallback: invoke native WebView print dialog (Save as PDF)
		a.runOnUI(func() {
			a.studioWebViewRun("window.print()")
			a.queueToast(ui.Toast{
				Title:       "PDF Export",
				Description: "Opened print dialog (Save as PDF) — " + err.Error(),
				Type:        "info",
			})
			a.invalidateUI()
		})
		return
	}
	mygo.Shell.ShowItemInFolder(destPath)
	a.runOnUI(func() {
		a.queueToast(ui.Toast{
			Title:       "Exported PDF",
			Description: filepath.Base(destPath),
			Type:        "success",
		})
		a.invalidateUI()
	})
}

func (a *AppUI) studioDownloadCanvasPPTX(project *corestudio.StudioProject, slides []corestudio.SlideItem, assetPath string) {
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "Downloads", studioSafeStem(project, assetPath)+".pptx")
	destPath, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.Window,
		Title:       "Export Slides as PowerPoint (.pptx)",
		DefaultPath: defaultPath,
		ButtonLabel: "Export PPTX",
	})
	if err != nil || destPath == "" {
		return
	}
	if !strings.HasSuffix(strings.ToLower(destPath), ".pptx") {
		destPath += ".pptx"
	}
	title := "Presentation"
	if project != nil {
		title = project.Name
	}
	if err := corestudio.ExportSlidesToPPTX(title, slides, assetPath, destPath); err != nil {
		a.runOnUI(func() {
			a.queueToast(ui.Toast{
				Title:       "PPTX Export Failed",
				Description: err.Error(),
				Type:        "error",
			})
			a.invalidateUI()
		})
		return
	}
	mygo.Shell.ShowItemInFolder(destPath)
	a.runOnUI(func() {
		a.queueToast(ui.Toast{
			Title:       "Exported PowerPoint",
			Description: filepath.Base(destPath),
			Type:        "success",
		})
		a.invalidateUI()
	})
}

func (a *AppUI) studioDownloadCanvasMP4(project *corestudio.StudioProject, data corestudio.ArtifactData, assetPath string) {
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "Downloads", studioSafeStem(project, assetPath)+".mp4")
	destPath, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.Window,
		Title:       "Export Motion Video (.mp4)",
		DefaultPath: defaultPath,
		ButtonLabel: "Export MP4",
	})
	if err != nil || destPath == "" {
		return
	}
	if !strings.HasSuffix(strings.ToLower(destPath), ".mp4") {
		destPath += ".mp4"
	}
	a.runOnUI(func() {
		a.queueToast(ui.Toast{
			Title:       "Rendering MP4...",
			Description: "Encoding motion stage to " + filepath.Base(destPath),
			Type:        "info",
		})
		a.invalidateUI()
	})
	if err := corestudio.ExportMotionToMP4(context.Background(), assetPath, destPath, data.DurationSec, data.FPS); err != nil {
		a.runOnUI(func() {
			a.queueToast(ui.Toast{
				Title:       "MP4 Export Failed",
				Description: err.Error(),
				Type:        "error",
			})
			a.invalidateUI()
		})
		return
	}
	mygo.Shell.ShowItemInFolder(destPath)
	a.runOnUI(func() {
		a.queueToast(ui.Toast{
			Title:       "Exported MP4 Video",
			Description: filepath.Base(destPath),
			Type:        "success",
		})
		a.invalidateUI()
	})
}

func (a *AppUI) studioOpenLocalHTMLFile() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: a.Window,
		Title:  "Preview HTML / Web Content in Studio Viewer",
		Filters: []mygo.FileFilter{
			{Name: "Web / Studio Files", Extensions: []string{"html", "htm", "md", "pdf", "pptx"}},
		},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	p := paths[0]
	a.runOnUI(func() {
		a.openStudioPreviewFile(p)
	})
}

// openStudioPreviewFile opens any local file in the Studio viewer for viewing
// only, without importing it into ~/.forge artifacts or project history.
func (a *AppUI) openStudioPreviewFile(path string) {
	if path == "" {
		return
	}
	a.StudioPreviewPath = path
	a.studioWebviewReload = true
	if a.Mode != ModeStudio {
		a.setMode(ModeStudio)
	}
	a.studioEnsureCanvasProject()
	a.invalidateUI()
}

// closeStudioPreview exits viewer-only preview mode and returns to the active artifact.
func (a *AppUI) closeStudioPreview() {
	a.StudioPreviewPath = ""
	a.studioWebviewReload = true
	a.closeStudioWebView()
	a.invalidateUI()
}

func (a *AppUI) scanStudioMediaLibrary() []corestudio.MediaAsset {
	if !a.studioMediaScanAt.IsZero() && time.Since(a.studioMediaScanAt) < 2*time.Second {
		return a.studioMediaCache
	}
	a.studioMediaScanAt = time.Now()
	type foundFile struct {
		asset   corestudio.MediaAsset
		modTime int64
	}
	var dirs []struct {
		dir      string
		forceRef bool
	}
	if a.Workspace != nil && a.Workspace.RootDir != "" {
		dirs = append(dirs,
			struct {
				dir      string
				forceRef bool
			}{filepath.Join(a.Workspace.RootDir, ".forge", "studio", "outputs"), false},
			struct {
				dir      string
				forceRef bool
			}{filepath.Join(a.Workspace.RootDir, ".forge", "studio", "references"), true},
		)
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		dirs = append(dirs, struct {
			dir      string
			forceRef bool
		}{filepath.Join(home, ".forge-ade", "studio_outputs"), false})
	}

	seenPath := map[string]bool{}
	pngStems := map[string]bool{}
	var collected []foundFile

	for _, d := range dirs {
		entries, err := os.ReadDir(d.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".png" {
				stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				pngStems[filepath.Join(d.dir, stem)] = true
			}
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			ext := strings.ToLower(filepath.Ext(name))
			var kind string
			switch ext {
			case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg":
				if ext == ".svg" && pngStems[filepath.Join(d.dir, strings.TrimSuffix(name, filepath.Ext(name)))] {
					continue
				}
				kind = "image"
			case ".mp4", ".mov", ".webm", ".m4v":
				kind = "video"
			default:
				continue
			}
			if d.forceRef {
				kind = "reference"
			}
			fullPath := filepath.Join(d.dir, name)
			if seenPath[fullPath] {
				continue
			}
			seenPath[fullPath] = true
			var mod int64
			if info, err := e.Info(); err == nil {
				mod = info.ModTime().UnixNano()
			}
			collected = append(collected, foundFile{
				asset: corestudio.MediaAsset{
					ID:       fmt.Sprintf("disk-%d-%s", len(collected), name),
					Type:     kind,
					Title:    name,
					URL:      "file://" + fullPath,
					FilePath: fullPath,
				},
				modTime: mod,
			})
		}
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].modTime > collected[j].modTime
	})

	out := make([]corestudio.MediaAsset, 0, len(collected))
	for _, item := range collected {
		out = append(out, item.asset)
	}
	a.studioMediaCache = out
	return out
}

func (a *AppUI) ensureStudioNodeAssetFile(project *corestudio.StudioProject, pipeline string) string {
	outDir := ""
	if a.Workspace != nil && a.Workspace.RootDir != "" {
		outDir = filepath.Join(a.Workspace.RootDir, ".forge", "studio")
	} else {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, ".forge-ade", "studio_temp")
	}
	_ = os.MkdirAll(outDir, 0755)
	filePath := filepath.Join(outDir, "nodegraph.html")

	nodeTheme, _ := a.buildStudioNodeTheme()
	state := corestudio.NodeWorkflowInitialState{
		PipelineType:   pipeline,
		PositivePrompt: a.StudioPositivePrompt,
		NegativePrompt: a.StudioNegativePrompt,
		Model:          a.StudioModel,
		Steps:          a.StudioSteps,
		CFGScale:       a.StudioCFGScale,
		AspectRatio:    a.StudioAspectRatio,
		ResultURL:      "",
		MediaLibrary:   a.scanStudioMediaLibrary(),
		Theme:          nodeTheme,
	}
	if a.StudioGeneratedImagePath != "" {
		state.ResultURL = "file://" + a.StudioGeneratedImagePath
	}
	html := corestudio.GenerateNodeWorkflowHTML(pipeline, state)
	_ = os.WriteFile(filePath, []byte(html), 0644)
	return filePath
}

func (a *AppUI) syncNodeWorkflowProgress(isGen bool, status string, progress float32, resultPath string) {
	if a.StudioNodeWebView == nil || a.StudioNodeWebView.IsDestroyed() {
		return
	}
	resURL := ""
	if resultPath != "" {
		resURL = "file://" + resultPath
	}
	js := fmt.Sprintf("if (typeof setGenerationProgress === 'function') { setGenerationProgress(%t, %q, %f, %q, %q); }",
		isGen, status, progress, resURL, resultPath)
	a.StudioNodeWebView.Page().Eval(js)
}

func (a *AppUI) closeStudioNodeWebView() {
	if a.StudioNodeWebView != nil && !a.StudioNodeWebView.IsDestroyed() {
		a.StudioNodeWebView.Destroy()
	}
	a.StudioNodeWebView = nil
	a.StudioNodeWebViewURL = ""
	a.studioNodeThemeKey = ""
}

// renderStudioTimelineDock renders the contextual timeline dock for Video & Motion projects.
func (a *AppUI) renderStudioTimelineDock(c *ui.Context, tc theme.ThemeColors, project *corestudio.StudioProject, data corestudio.ArtifactData) {
	// Duration comes from the composition's own TTS timings (fallback 12s) so
	// the timecode and word highlight reflect the real artifact data.
	duration := 12.0
	for _, w := range data.TTSWords {
		if w.EndTime > duration {
			duration = w.EndTime
		}
	}
	if a.StudioTimelinePlaying {
		if a.studioTimelineAnchor.IsZero() {
			a.studioTimelineAnchor = time.Now().Add(-time.Duration(a.StudioTimelineTime * float64(time.Second)))
		}
		a.StudioTimelineTime = time.Since(a.studioTimelineAnchor).Seconds()
		if a.StudioTimelineTime >= duration {
			a.StudioTimelineTime = duration
			a.StudioTimelinePlaying = false
		}
	}

	ui.Column(c.Key("studio-timeline-dock")).Height(105).Background(tc.Surface).
		BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Padding(8, 16).Gap(6).Children(func() {

		// Timeline Header & Controls
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			playIcon := icons.TablerPlayerPlay
			if a.StudioTimelinePlaying {
				playIcon = icons.TablerPlayerPause
			}
			playBtn := ui.Row(c.Key("timeline-play-btn")).Size(24, 24).Radius(5).Background(tc.Accent).
				AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
			playBtn.Children(func() {
				ui.Icon(c, playIcon).Width(12).Height(12).TextColor(tc.AccentForeground)
			})
			if playBtn.Clicked() {
				if a.StudioTimelinePlaying {
					// Pause: freeze at the current position.
					a.StudioTimelineTime = time.Since(a.studioTimelineAnchor).Seconds()
				} else {
					if a.StudioTimelineTime >= duration {
						a.StudioTimelineTime = 0 // replay from the top
					}
					a.studioTimelineAnchor = time.Now().Add(-time.Duration(a.StudioTimelineTime * float64(time.Second)))
				}
				a.StudioTimelinePlaying = !a.StudioTimelinePlaying
				if a.StudioTimelinePlaying {
					a.studioWebViewRun("if (typeof setPlaying === 'function') { setPlaying(true); } else { document.body.classList.remove('paused'); } 'ok'")
				} else {
					a.studioWebViewRun("if (typeof setPlaying === 'function') { setPlaying(false); } else { document.body.classList.add('paused'); } 'ok'")
				}
			}

			ui.Text(c, fmt.Sprintf("%s / %s", studioTimecode(a.StudioTimelineTime), studioTimecode(duration))).FontSize(11).FontWeight(600).TextColor(tc.Text)

			ui.Box(c).Grow(1)
			voiceName := "TTS: Default Voice"
			if data.VoiceID != "" {
				voiceName = "TTS: " + data.VoiceID
			}
			ui.Row(c).Padding(2, 6).Radius(4).Background(tc.ToolbarControl).Children(func() {
				ui.Text(c, voiceName).FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
			})
		})

		// Audio Waveform & Word Timing Track — horizontally scrollable so long
		// scripts scroll instead of clipping.
		ui.Row(c.Key("timeline-track")).Height(46).Radius(6).Background(tc.Canvas).
			BorderWidth(1).BorderColor(tc.Border).Padding(4, 10).AlignItems(ui.Center).Children(func() {

			ui.ScrollHorizontal(c.Key("timeline-track-scroll")).Grow(1).MinWidth(0).Scrollbars(false).Children(func() {
				ui.Row(c).Height(36).AlignItems(ui.Center).Gap(6).Children(func() {
					if len(data.TTSWords) > 0 {
						for i, wordItem := range data.TTSWords {
							wordIdx := i
							w := wordItem
							// Word highlight follows the real playback position.
							isCurrentWord := a.StudioTimelinePlaying && w.StartTime <= a.StudioTimelineTime && a.StudioTimelineTime < w.EndTime

							chip := ui.Row(c.Key(fmt.Sprintf("tts-word-%d", wordIdx))).Height(24).Padding(0, 6).Radius(4).
								AlignItems(ui.Center).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
							if isCurrentWord {
								chip.Background(tc.Accent.Alpha(0.2)).BorderColor(tc.Accent)
							}
							chip.Children(func() {
								textColor := tc.TextMuted
								if isCurrentWord {
									textColor = tc.Accent
								}
								ui.Text(c, w.Word).FontSize(10).FontWeight(500).TextColor(textColor)
							})
						}
					} else {
						ui.Text(c, "Audio Waveform & Word Sync Stream").FontSize(11).TextColor(tc.TextMuted)
					}
				})
			})
		})
	})
}

// studioTimecode formats seconds as mm:ss.d for the timeline dock.
func studioTimecode(t float64) string {
	if t < 0 {
		t = 0
	}
	return fmt.Sprintf("%02d:%04.1f", int(t)/60, math.Mod(t, 60))
}

// ensureStudioAssetFile returns the HTML file the viewer should open for the
// project: an agent-authored page (EntryHTML) when present, otherwise the
// generated viewer template — regenerated whenever the active version or the
// generator changes, force-reloading the open WebView underneath.
func (a *AppUI) ensureStudioAssetFile(project *corestudio.StudioProject) string {
	if project == nil {
		return ""
	}
	activeVer := project.ActiveVersion()
	ver := 0
	var data corestudio.ArtifactData
	if activeVer != nil {
		data = activeVer.Data
		ver = activeVer.Version
	}

	// Pages authored by the agent are opened as-is (no generated template).
	if data.EntryHTML != "" {
		if _, err := os.Stat(data.EntryHTML); err == nil {
			return data.EntryHTML
		}
	}

	outDir := ""
	if a.Workspace != nil && a.Workspace.RootDir != "" {
		outDir = filepath.Join(a.Workspace.RootDir, ".forge", "studio")
	} else {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, ".forge-ade", "studio_temp")
	}
	_ = os.MkdirAll(outDir, 0755)

	var name, html string
	switch project.Type {
	case corestudio.ArtifactTypeSlides:
		name, html = "slides.html", corestudio.GenerateSlidesHTML(project.Name, data)
	case corestudio.ArtifactTypeMotion:
		name, html = "motion.html", corestudio.GenerateMotionHTML(project.Name, data)
	case corestudio.ArtifactTypeCodeJS:
		name, html = "scene3d.html", corestudio.Generate3DSceneHTML(project.Name, data.Code)
	default:
		return ""
	}
	p := filepath.Join(outDir, name)

	// The file embeds `forgeade-studio-asset-ver:N`; skip the rewrite while
	// it was generated from the active version by the current generator.
	if a.studioAssetVer[project.ID] == ver {
		return p
	}
	if prev, err := os.ReadFile(p); err == nil &&
		strings.Contains(string(prev), corestudio.HTMLVersionMarker) &&
		studioEmbeddedAssetVer(string(prev)) == ver {
		a.studioAssetVer[project.ID] = ver
		return p
	}

	_ = os.WriteFile(p, []byte(html+fmt.Sprintf("\n<!-- forgeade-studio-asset-ver:%d -->\n", ver)), 0644)
	a.studioAssetVer[project.ID] = ver
	// Content changed under an open WebView: force a reload next frame.
	if a.StudioWebViewURL == "file://"+p {
		a.closeStudioWebView()
	}
	return p
}

var studioAssetVerRe = regexp.MustCompile(`forgeade-studio-asset-ver:(\d+)`)

// studioEmbeddedAssetVer reads the version a studio asset file was generated
// from (-1 when the file predates version tracking).
func studioEmbeddedAssetVer(s string) int {
	m := studioAssetVerRe.FindStringSubmatch(s)
	if m == nil {
		return -1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	return n
}

// studioForgeRoot returns the global ~/.forge directory where Studio artifacts live.
func studioForgeRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".forge")
}

// isUnderForgeDir reports whether path resides inside ~/.forge.
func isUnderForgeDir(path string) bool {
	root := studioForgeRoot()
	if root == "" || path == "" {
		return false
	}
	absRoot, err1 := filepath.Abs(root)
	absPath, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// studioImportAgentArtifacts imports the files the agent wrote during its
// last turn into the active studio project, restricted strictly to ~/.forge.
func (a *AppUI) studioImportAgentArtifacts(sessionID string) {
	if a.Studio == nil || a.Agent == nil {
		return
	}
	proj := a.Studio.ActiveProject()
	if proj == nil {
		return
	}

	// Candidates: the turn's write/edit tool calls first, then the change list.
	candidates := a.Agent.LastTurnWrittenFiles(sessionID)
	for _, ch := range a.Agent.LastTurnChangedFiles(sessionID) {
		candidates = append(candidates, ch.Path)
	}

	for _, path := range candidates {
		for _, full := range a.studioResolveImportPath(sessionID, path) {
			if !isUnderForgeDir(full) {
				continue
			}
			flavor, targetType := detectPathFlavor(full)
			if targetType != "" && targetType != proj.Type {
				a.studioEnsureProjectForType(targetType)
				proj = a.Studio.ActiveProject()
			}
			imported, err := a.Studio.ImportAgentArtifact(full)
			if err != nil {
				continue
			}
			if imported {
				a.StudioPreviewPath = ""
				a.studioArtifactScanAt = time.Time{}
				a.studioArtifactCache = nil
				a.studioWebviewReload = true
				a.queueToast(ui.Toast{
					Title:       "Studio Import (" + strings.ToUpper(flavor) + ")",
					Description: filepath.Base(full) + " → " + proj.Name,
					Type:        "success",
				})
				return
			}
		}
	}
}

// studioResolveImportPath expands ~ and resolves a possibly-relative turn
// file against the agent session's workspace first, then the app workspace,
// then the home directory (a no-workspace session's files usually land
// under the user's home).
func (a *AppUI) studioResolveImportPath(sessionID, path string) []string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if filepath.IsAbs(path) {
		return []string{path}
	}
	var out []string
	if ws := a.Agent.SessionWorkspacePath(sessionID); ws != "" {
		out = append(out, filepath.Join(ws, path))
	}
	if a.Workspace != nil && a.Workspace.RootDir != "" {
		out = append(out, filepath.Join(a.Workspace.RootDir, path))
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, path))
	}
	return out
}

// studioArtifactFile is one browsable file the agent produced.
type studioArtifactFile struct {
	Path    string
	ModTime time.Time
}

// studioArtifactDirs lists the roots the explorer scans, strictly scoped to ~/.forge.
func (a *AppUI) studioArtifactDirs(_ string) []string {
	root := studioForgeRoot()
	if root == "" {
		return nil
	}
	candidates := []string{
		filepath.Join(root, "studio"),
		filepath.Join(root, "slides"),
		filepath.Join(root, "motion"),
		filepath.Join(root, "3d"),
		filepath.Join(root, "scenes"),
		root,
	}
	seen := map[string]bool{}
	var roots []string
	for _, p := range candidates {
		if seen[p] {
			continue
		}
		seen[p] = true
		if _, err := os.Stat(p); err == nil {
			roots = append(roots, p)
		}
	}
	return roots
}

// studioScanArtifactFiles browses ~/.forge for creative artifacts (html/md/js/pdf/pptx),
// newest first. Scans are cached for 2s so the per-frame render never walks disk every frame.
func (a *AppUI) studioScanArtifactFiles(sessionID string) []studioArtifactFile {
	if !a.studioArtifactScanAt.IsZero() && time.Since(a.studioArtifactScanAt) < 2*time.Second {
		return a.studioArtifactCache
	}
	a.studioArtifactScanAt = time.Now()

	skipDir := map[string]bool{
		".git": true, "node_modules": true, "skills": true, "plugins": true,
		"memory": true, "sessions": true, "attachments": true, "logs": true,
		"cache": true, "outputs": true, "build": true, ".zcode": true, "studio_temp": true,
	}
	var files []studioArtifactFile
	seenFile := map[string]bool{}
	var walk func(root string, depth int)
	walk = func(root string, depth int) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			p := filepath.Join(root, e.Name())
			if e.IsDir() {
				if depth < 3 && !skipDir[e.Name()] {
					walk(p, depth+1)
				}
				continue
			}
			if seenFile[p] {
				continue
			}
			switch strings.ToLower(filepath.Ext(e.Name())) {
			case ".html", ".htm", ".md", ".js", ".jsx", ".tsx", ".pdf", ".pptx":
			default:
				continue
			}
			seenFile[p] = true
			if info, err := e.Info(); err == nil {
				files = append(files, studioArtifactFile{Path: p, ModTime: info.ModTime()})
			}
		}
	}
	for _, r := range a.studioArtifactDirs(sessionID) {
		walk(r, 0)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	if len(files) > 40 {
		files = files[:40]
	}
	a.studioArtifactCache = files
	return files
}

// studioImportArtifactFile imports an artifact from ~/.forge into the active project,
// or opens external files in viewer-only preview mode without adding them to artifacts.
func (a *AppUI) studioImportArtifactFile(path string) {
	if !isUnderForgeDir(path) {
		a.openStudioPreviewFile(path)
		return
	}
	a.StudioPreviewPath = ""
	flavor, targetType := detectPathFlavor(path)
	if targetType != "" && a.Studio != nil {
		if cur := a.Studio.ActiveProject(); cur == nil || cur.Type != targetType {
			a.studioEnsureProjectForType(targetType)
		}
	}
	imported, err := a.Studio.ImportAgentArtifact(path)
	if err != nil || !imported {
		msg := "not importable into the active project"
		if err != nil {
			msg = err.Error()
		}
		a.queueToast(ui.Toast{Title: "Import Failed", Description: msg, Type: "error"})
		return
	}
	a.queueToast(ui.Toast{
		Title:       "Studio Import (" + strings.ToUpper(flavor) + ")",
		Description: filepath.Base(path) + " → unified viewer",
		Type:        "success",
	})
	a.invalidateUI()
}

// renderStudioArtifactsPane draws the browsable list of ~/.forge artifacts
// in the Right Sidebar; clicking a row loads it into the active project's viewer.
func (a *AppUI) renderStudioArtifactsPane(c *ui.Context, tc theme.ThemeColors) {
	sessionID := ""
	if a.Agent != nil {
		sessionID = a.Agent.ActiveSessionID
	}
	files := a.studioScanArtifactFiles(sessionID)
	activeEntry := ""
	if a.Studio != nil && a.StudioPreviewPath == "" {
		if proj := a.Studio.ActiveProject(); proj != nil {
			if ver := proj.ActiveVersion(); ver != nil {
				activeEntry = ver.Data.EntryHTML
			}
		}
	}

	ui.Column(c.Key("studio-artifacts-pane")).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
		// Header Toolbar
		ui.Row(c.Key("artifacts-toolbar")).Height(36).Padding(0, 12).Gap(6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Icon(c, icons.TablerFolderOpen).Width(14).Height(14).TextColor(tc.Accent)
			ui.Text(c, "Artifacts").FontSize(12).FontWeight(600).TextColor(tc.Text)
			ui.Row(c).Padding(1, 5).Radius(3).Background(tc.Accent.Alpha(0.12)).Children(func() {
				ui.Text(c, fmt.Sprintf("%d", len(files))).FontSize(10).FontWeight(600).TextColor(tc.Accent)
			})
			ui.Row(c).Padding(1, 5).Radius(3).Background(tc.ToolbarControl).Children(func() {
				ui.Text(c, "~/.forge").FontSize(9).FontWeight(500).TextColor(tc.TextMuted)
			})
			ui.Box(c).Grow(1)

			// Rescan / Refresh
			refreshBtn := IconButton(c, tc, icons.TablerRefresh, 12)
			refreshBtn.Tooltip("Rescan ~/.forge artifacts")
			if refreshBtn.Clicked() {
				a.studioArtifactScanAt = time.Time{}
				a.studioArtifactCache = nil
			}
		})

		// Filter Query Input
		ui.Row(c).Padding(6, 10).Children(func() {
			ui.TextInput(c, &a.StudioArtifactFilterQuery).Placeholder("Filter ~/.forge artifacts...").Grow(1)
		})

		// Filter files by query if set
		filtered := files
		if q := strings.TrimSpace(strings.ToLower(a.StudioArtifactFilterQuery)); q != "" {
			var match []studioArtifactFile
			for _, f := range files {
				if strings.Contains(strings.ToLower(filepath.Base(f.Path)), q) {
					match = append(match, f)
				}
			}
			filtered = match
		}

		if len(filtered) == 0 {
			ui.Column(c).Grow(1).Padding(24, 16).AlignItems(ui.Center).Justify(ui.Center).Gap(10).Children(func() {
				ui.Icon(c, icons.TablerSparkles).Width(24).Height(24).TextColor(tc.IconMuted)
				ui.Text(c, "No artifacts in ~/.forge").FontSize(12).FontWeight(600).TextColor(tc.Text)
				ui.Text(c, "Ask Studio Agent to generate slides, motion, or 3D scenes — outputs are saved to ~/.forge.").
					FontSize(11).TextColor(tc.TextMuted).Center()
			})
			return
		}

		ui.Scroll(c.Key("artifacts-list")).Grow(1).Children(func() {
			ui.Column(c).Padding(4, 8).Gap(2).Children(func() {
				for _, f := range filtered {
					f := f
					name := filepath.Base(f.Path)
					isActive := activeEntry != "" && activeEntry == f.Path
					row := ui.Row(c.Key("artifact-"+name+"-"+fmt.Sprint(f.ModTime.Unix()))).Height(34).Padding(0, 8).Radius(6).
						Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					if isActive {
						row.Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
					}
					row.Children(func() {
						flavor, _ := detectPathFlavor(f.Path)
						switch flavor {
						case "motion":
							ui.Icon(c, icons.TablerMovie).Width(14).Height(14).TextColor(tc.Accent)
						case "3d":
							ui.Icon(c, icons.TablerRotate).Width(14).Height(14).TextColor(tc.Success)
						case "pdf":
							ui.Icon(c, icons.TablerFileText).Width(14).Height(14).TextColor(tc.Error)
						case "pptx":
							ui.Icon(c, icons.TablerPresentation).Width(14).Height(14).TextColor(tc.Warning)
						case "slides":
							ui.Icon(c, icons.TablerPresentation).Width(14).Height(14).TextColor(tc.Accent)
						default:
							ui.Icon(c, icons.TablerFile).Width(14).Height(14).TextColor(tc.IconMuted)
						}
						ui.Column(c).Grow(1).MinWidth(0).Children(func() {
							ui.Text(c, name).FontSize(11).FontWeight(600).TextColor(tc.Text).SingleLine()
							ui.Text(c, flavor).FontSize(9).TextColor(tc.TextMuted).SingleLine()
						})
						ui.Text(c, relativeTime(f.ModTime)).FontSize(9).TextColor(tc.TextMuted)
					})
					if !isActive && row.Hovered() {
						row.Background(tc.ToolbarControlHover)
					}
					if row.Clicked() {
						a.studioImportArtifactFile(f.Path)
					}
				}
			})
		})
	})
}

// renderStudioArtifactExplorer draws the browsable list of files the agent produced.
func (a *AppUI) renderStudioArtifactExplorer(c *ui.Context, tc theme.ThemeColors) {
	a.renderStudioArtifactsPane(c, tc)
}

// openStudioWebView opens or navigates the dedicated studio webview. bg is
// the page background used until the page paints (avoids a white flash).
func (a *AppUI) openStudioWebView(targetPath, bg string) {
	if targetPath == "" || a.Window == nil || !mygo.WebViewAvailable() {
		return
	}
	url := "file://" + targetPath
	if a.StudioWebView != nil && !a.StudioWebView.IsDestroyed() {
		if a.StudioWebViewURL == url && !a.studioWebviewReload {
			return
		}
		a.StudioWebView.Destroy()
		a.StudioWebView = nil
	}
	a.studioWebviewReload = false
	wv, err := a.Window.NewWebView(mygo.WebViewOptions{URL: url, BackgroundColor: bg})
	if err == nil {
		a.StudioWebView = wv
		a.StudioWebViewURL = url
		a.studioWebviewSynced = false
	}
}

// closeStudioWebView cleans up the dedicated studio webview.
func (a *AppUI) closeStudioWebView() {
	if a.StudioWebView != nil && !a.StudioWebView.IsDestroyed() {
		a.StudioWebView.Destroy()
	}
	a.StudioWebView = nil
	a.StudioWebViewURL = ""
	a.studioWebviewSynced = false
}

// studioWebViewRun evaluates js in the studio WebView, logging failures
// instead of dropping them on the floor.
func (a *AppUI) studioWebViewRun(js string) {
	wv := a.StudioWebView
	if wv == nil || wv.IsDestroyed() {
		return
	}
	if _, err := wv.Page().Eval(js); err != nil {
		log.Printf("[studio] webview eval failed: %v", err)
	}
}

// studioWebViewSyncJS returns the one-shot script that pushes the current
// native viewer state (active slide / playback state) into the page. It
// throws until the page's control script is live, which turns the eval into
// an error the sync loop can retry on.
func (a *AppUI) studioWebViewSyncJS(project *corestudio.StudioProject) string {
	if project == nil {
		return "'ok'"
	}
	// Agent-authored pages carry no od:* control script; just wait for load.
	if ver := project.ActiveVersion(); ver != nil && ver.Data.EntryHTML != "" {
		return "if (document.readyState !== 'complete') { throw new Error('page not ready'); } 'ok'"
	}
	switch project.Type {
	case corestudio.ArtifactTypeSlides:
		n := 0
		if ver := project.ActiveVersion(); ver != nil {
			n = len(ver.Data.Slides)
		}
		if n == 0 {
			n = len(corestudio.DemoSlides())
		}
		idx := a.StudioSelectedSlideIdx
		if idx < 0 || idx >= n {
			idx = 0
		}
		return fmt.Sprintf("if (typeof showSlide !== 'function') { throw new Error('slides not ready'); } showSlide(%d); 'ok'", idx)
	case corestudio.ArtifactTypeMotion:
		return fmt.Sprintf("if (!document.body) { throw new Error('motion not ready'); } document.body.classList.toggle('paused', %v); 'ok'", !a.StudioTimelinePlaying)
	default:
		return "if (document.readyState !== 'complete') { throw new Error('scene not ready'); } 'ok'"
	}
}

// studioSyncWebView pushes the current viewer state into the studio WebView
// until the page confirms it is live. Evals fired before a page finishes
// loading never reach it (WebKit drops them), so every frame until the first
// success retries — this keeps the page in sync across WebView recreation
// (switching domains reloads the page and resets its state).
func (a *AppUI) studioSyncWebView(project *corestudio.StudioProject) {
	if a.studioWebviewSynced {
		return
	}
	wv := a.StudioWebView
	if wv == nil || wv.IsDestroyed() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if _, err := wv.Page().EvalContext(ctx, a.studioWebViewSyncJS(project)); err == nil {
		a.studioWebviewSynced = true
	}
}

// initStudioManager initializes the native studio manager.
func (a *AppUI) initStudioManager(workspaceDir string) {
	a.Studio = nativestudio.NewStudioManager(workspaceDir)
	a.StudioWorkspacePath = workspaceDir
	a.StudioViewMode = "canvas"
	a.StudioActiveTab = "assets"
	a.StudioLeftChatWidth = 420
	a.Studio.SetOnChanged(func() {
		if a.Window != nil {
			a.Window.Invalidate()
		}
	})
}

// studioExportActive exports the active project and hands off to Editor Mode (⌘⇧E).
func (a *AppUI) studioExportActive() {
	if a.Studio == nil {
		return
	}
	res, err := a.Studio.ExportActive()
	if err != nil {
		return
	}
	if res != nil && res.PrimaryFilePath != "" {
		a.setMode(ModeEditor)
		a.ShowLeftSidebar = true
		a.EditorActivity = ActivityExplorer
		relPath := res.PrimaryFilePath
		if a.Workspace != nil && a.Workspace.RootDir != "" {
			if r, err := filepath.Rel(a.Workspace.RootDir, res.PrimaryFilePath); err == nil {
				relPath = r
			}
		}
		_, _ = a.Editor.OpenFile(res.PrimaryFilePath, relPath)
	}
}

// studioTogglePinning toggles element selection mode (V shortcut).
func (a *AppUI) studioTogglePinning() {
	if a.Studio != nil {
		a.Studio.TogglePinning()
	}
}

// studioSelectPin selects an element pin.
func (a *AppUI) studioSelectPin(pinID string) {
	if a.Studio != nil {
		a.Studio.SelectPin(pinID)
		if pin := a.Studio.SelectedPin(); pin != nil {
			a.StudioPromptInput = fmt.Sprintf("Modify %s: ", pin.Selector)
		}
	}
}

// studioRunDirector runs the director agent pipeline.
func (a *AppUI) studioRunDirector(prompt string) {
	if a.Studio != nil {
		_ = a.Studio.RunDirector(context.Background(), prompt)
	}
}

// studioPromptRequestsNewFile detects explicit user intent to create a separate
// new artifact file rather than modifying an existing one.
func studioPromptRequestsNewFile(prompt string) bool {
	p := strings.ToLower(prompt)
	for _, kw := range []string{
		"new file", "file baru", "buat baru", "bikin baru", "buatkan baru",
		"create a new", "create new", "create another", "make a new", "make new",
		"separate file", "terpisah", "project baru", "dari awal", "from scratch",
		"yang baru", "lagi yang lain", "versi baru",
	} {
		if strings.Contains(p, kw) {
			return true
		}
	}
	return false
}

// studioTargetDir resolves the domain-specific directory under ~/.forge/studio
// where Studio artifacts are written.
func (a *AppUI) studioTargetDir(_ string, targetType corestudio.ArtifactType) string {
	sub := "slides"
	switch targetType {
	case corestudio.ArtifactTypeMotion:
		sub = "motion"
	case corestudio.ArtifactTypeCodeJS:
		sub = "3d"
	case corestudio.ArtifactTypeSlides:
		sub = "slides"
	}
	if root := studioForgeRoot(); root != "" {
		return filepath.Join(root, "studio", sub)
	}
	return sub
}

// studioSlugifyPrompt converts a user prompt into a concise lowercase kebab-case
// filename slug suitable for slides/motion/3D artifacts.
func studioSlugifyPrompt(prompt string) string {
	skip := map[string]bool{
		"buat": true, "buatkan": true, "bikin": true, "tolong": true, "yang": true,
		"untuk": true, "dengan": true, "tentang": true, "dari": true, "baru": true,
		"file": true, "create": true, "make": true, "generate": true, "build": true,
		"please": true, "with": true, "about": true, "from": true, "new": true,
		"the": true, "and": true, "for": true, "into": true, "another": true,
		"3d": true, "slide": true, "slides": true, "motion": true, "deck": true,
	}
	var cur strings.Builder
	var words []string
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		w := cur.String()
		cur.Reset()
		if !skip[w] && len(w) > 1 {
			words = append(words, w)
		}
	}
	for _, r := range strings.ToLower(prompt) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
		} else {
			flush()
			if len(words) >= 4 {
				break
			}
		}
	}
	flush()
	if len(words) > 4 {
		words = words[:4]
	}
	if len(words) == 0 {
		return "custom"
	}
	slug := strings.Join(words, "-")
	if len(slug) > 42 {
		slug = strings.TrimRight(slug[:42], "-")
	}
	if slug == "" {
		return "custom"
	}
	return slug
}

// studioSuggestUniqueArtifactPath returns a non-colliding file path in dir for
// targetType using a domain prefix (slides-, motion-, scene3d-) + prompt slug.
func studioSuggestUniqueArtifactPath(dir string, targetType corestudio.ArtifactType, prompt string) string {
	prefix := "slides"
	switch targetType {
	case corestudio.ArtifactTypeMotion:
		prefix = "motion"
	case corestudio.ArtifactTypeCodeJS:
		prefix = "scene3d"
	}
	slug := studioSlugifyPrompt(prompt)
	if strings.HasPrefix(slug, prefix+"-") {
		slug = strings.TrimPrefix(slug, prefix+"-")
	}
	if slug == "" {
		slug = "custom"
	}
	stem := prefix + "-" + slug
	candidate := filepath.Join(dir, stem+".html")
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 2; i <= 99; i++ {
		next := filepath.Join(dir, fmt.Sprintf("%s-%d.html", stem, i))
		if _, err := os.Stat(next); os.IsNotExist(err) {
			return next
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d.html", stem, time.Now().Unix()))
}

// studioExistingArtifactNames lists existing artifact filenames in dir so the
// prompt can warn the agent never to overwrite them.
func studioExistingArtifactNames(dir string, limit int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".html", ".htm", ".md", ".js", ".jsx", ".tsx":
			names = append(names, e.Name())
		}
		if limit > 0 && len(names) >= limit {
			break
		}
	}
	return names
}

// studioSessionActiveArtifact returns the most recent artifact file written by
// sessionID that belongs to targetType, or "" if the session has not authored
// an artifact for that domain yet.
func (a *AppUI) studioSessionActiveArtifact(sessionID string, targetType corestudio.ArtifactType) string {
	if a.Agent == nil || sessionID == "" {
		return ""
	}
	for _, raw := range a.Agent.SessionWrittenFiles(sessionID) {
		for _, full := range a.studioResolveImportPath(sessionID, raw) {
			if _, err := os.Stat(full); err != nil {
				continue
			}
			ext := strings.ToLower(filepath.Ext(full))
			switch ext {
			case ".html", ".htm", ".md", ".js":
			default:
				continue
			}
			_, typ := detectPathFlavor(full)
			if typ == targetType {
				return full
			}
		}
	}
	return ""
}

// decorateStudioPrompt injects Open Design system guidelines and studio capabilities
// into the agent turn prompt when executing in Studio Mode.
func (a *AppUI) decorateStudioPrompt(sessionID, prompt string) string {
	if strings.Contains(prompt, "[FORGE STUDIO ENVIRONMENT]") {
		return prompt
	}

	var targetType corestudio.ArtifactType
	intent := detectStudioIntent(prompt)
	switch intent {
	case IntentMotion:
		targetType = corestudio.ArtifactTypeMotion
	case Intent3D:
		targetType = corestudio.ArtifactTypeCodeJS
	case IntentSlides:
		targetType = corestudio.ArtifactTypeSlides
	case IntentImage:
		targetType = corestudio.ArtifactTypeImage
	case IntentVideo:
		targetType = corestudio.ArtifactTypeVideo
	}
	if targetType == "" && a.Studio != nil {
		if p := a.Studio.ActiveProject(); p != nil {
			targetType = p.Type
		}
	}
	if targetType == "" {
		targetType = corestudio.ArtifactTypeSlides
	}

	isCanvasDomain := targetType == corestudio.ArtifactTypeSlides ||
		targetType == corestudio.ArtifactTypeMotion ||
		targetType == corestudio.ArtifactTypeCodeJS

	targetDir := a.studioTargetDir(sessionID, targetType)
	existing := studioExistingArtifactNames(targetDir, 12)
	sessionFile := a.studioSessionActiveArtifact(sessionID, targetType)
	createNew := isCanvasDomain && (sessionFile == "" || studioPromptRequestsNewFile(prompt))
	suggestedPath := ""
	if createNew {
		suggestedPath = studioSuggestUniqueArtifactPath(targetDir, targetType, prompt)
	}

	var b strings.Builder
	b.WriteString("[FORGE STUDIO ENVIRONMENT — OPEN DESIGN SKILLS ACTIVATED]\n")
	if intent != IntentGeneral {
		b.WriteString(fmt.Sprintf("[STUDIO INTENT ROUTER: DETECTED TARGET = %s]\n", strings.ToUpper(string(intent))))
	}
	b.WriteString("You are executing in Forge ADE Studio Mode — an autonomous design and creative engineering environment.\n")
	b.WriteString("Official Reference: Open Design (https://open-design.ai/skills/), Claude Design, Visual Node Workflows, and Creative Agent Workflows.\n\n")
	b.WriteString("STUDIO CORE CAPABILITIES & ENGINE STACKS:\n")
	b.WriteString("1. Generate Image: Flux / Imagen / Visual Node API | Source: Prompt + Seed + ControlNet JSON | Formats: PNG, WebP, SVG\n")
	b.WriteString("2. Generate Video: Veo / Kling / Runway / Wan API | Source: Storyboard + Keyframe Spec | Formats: MP4, WebM\n")
	b.WriteString("3. Make Slides (PPT): HTML/CSS + PptxGenJS or Slidev | Source: Structured Slide JSON / Markdown | Formats: .pptx, PDF, Web (create a uniquely named file under slides/, e.g. slides-<topic>.html rather than overwriting an existing slides.html)\n")
	b.WriteString("4. Make Motion Video + TTS: Remotion (React) + ElevenLabs | Source: React TSX + Audio Timestamp JSON, or deterministic 60fps HTML5 canvas seekable per-frame | Formats: MP4 via headless render (create a uniquely named file under motion/, e.g. motion-<topic>.html rather than overwriting an existing motion.html)\n")
	b.WriteString("5. 2D & 3D in JS: Three.js / R3F, PixiJS, P5.js | Source: Live JS/TSX Sandbox with OrbitControls | Formats: Interactive Web, .glb, MP4 (create a uniquely named file under 3d/, e.g. scene3d-<topic>.html rather than overwriting an existing scene3d.html)\n\n")

	if isCanvasDomain {
		if createNew {
			b.WriteString("FILE CREATION RULE (CRITICAL — DO NOT OVERWRITE EXISTING FILES):\n")
			b.WriteString("- You MUST create a BRAND NEW file for this request. NEVER overwrite or rewrite existing files.\n")
			b.WriteString(fmt.Sprintf("- Recommended new file path: %s\n", suggestedPath))
			b.WriteString(fmt.Sprintf("- If you pick a different descriptive filename in %s, ensure it does NOT already exist and keeps the domain prefix (slides- / motion- / scene3d-).\n", targetDir))
			if len(existing) > 0 {
				b.WriteString(fmt.Sprintf("- Existing files to preserve untouched: %s\n", strings.Join(existing, ", ")))
			}
			b.WriteString("\n")
		} else {
			b.WriteString("FILE TARGET FOR THIS SESSION FOLLOW-UP:\n")
			b.WriteString(fmt.Sprintf("- Update this session's active artifact file: %s\n", sessionFile))
			b.WriteString("- Do NOT overwrite other unrelated files in the directory. If the prompt asks for a brand-new separate creation, write to a new uniquely named file instead.\n\n")
		}
	}

	if a.Studio != nil {
		if p := a.Studio.ActiveProject(); p != nil {
			b.WriteString(fmt.Sprintf("ACTIVE STUDIO TARGET: %s (Type: %s)\n", p.Name, p.Type))
			switch p.Type {
			case corestudio.ArtifactTypeSlides:
				if suggestedPath != "" {
					b.WriteString(fmt.Sprintf("PRIMARY DIRECTIVE: Create a NEW slide deck at %s (16:9 responsive presentation stage, showSlide() pagination, keyboard nav).\n", suggestedPath))
				} else {
					b.WriteString("PRIMARY DIRECTIVE: Update the current slide deck (16:9 responsive presentation stage, showSlide() pagination, keyboard nav).\n")
				}
			case corestudio.ArtifactTypeMotion:
				if suggestedPath != "" {
					b.WriteString(fmt.Sprintf("PRIMARY DIRECTIVE: Create a NEW motion composition at %s (1920x1080 canvas/HTML5 stage, seekable per-frame timeline, TTS audio timestamp sync).\n", suggestedPath))
				} else {
					b.WriteString("PRIMARY DIRECTIVE: Update the current motion composition (1920x1080 canvas/HTML5 stage, seekable per-frame timeline, TTS audio timestamp sync).\n")
				}
			case corestudio.ArtifactTypeCodeJS:
				if suggestedPath != "" {
					b.WriteString(fmt.Sprintf("PRIMARY DIRECTIVE: Create a NEW interactive 3D/2D scene at %s using Three.js with OrbitControls, responsive viewport resize, and clean lighting.\n", suggestedPath))
				} else {
					b.WriteString("PRIMARY DIRECTIVE: Update the current interactive 3D/2D scene using Three.js with OrbitControls, responsive viewport resize, and clean lighting.\n")
				}
			case corestudio.ArtifactTypeImage:
				b.WriteString("PRIMARY DIRECTIVE: Configure image generation prompt, seed, sampler, and visual node workflow graph.\n")
			case corestudio.ArtifactTypeVideo:
				b.WriteString("PRIMARY DIRECTIVE: Generate video storyboard, motion keyframe schedule, and clip timing.\n")
			}
		}
		if a.Studio.Skills() != nil {
			active := a.Studio.Skills().ActiveSkills()
			if len(active) > 0 {
				b.WriteString("\nACTIVE DESIGN GUIDELINES:\n")
				for _, s := range active {
					b.WriteString(fmt.Sprintf("• %s (%s): %s\n", s.Name, s.Category, strings.Join(s.Rules, "; ")))
				}
			}
		}
	}
	b.WriteString("\nCRITICAL DIRECTIVE: Do NOT report that skill /open-design is missing. You HAVE full Open Design skills built-in. Directly construct, write, and execute the requested design artifact without hesitation.\n\n")
	b.WriteString("USER PROMPT:\n")
	b.WriteString(prompt)
	return b.String()
}
