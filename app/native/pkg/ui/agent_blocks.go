package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	internalboard "github.com/hasdev/forge-ade/core/board"
)

// setupMarkdownOptions configures rich rendering for assistant markdown messages,
// adding specialized UI blocks for JSON, programming languages, and Forge-Design.
func (a *AppUI) setupMarkdownOptions(as *agent.AgentState, tc theme.ThemeColors) {
	if as == nil {
		return
	}
	as.MdOpts.Theme = a.ThemeMgr.MarkdownTheme()
	as.MdOpts.WrapCode = true

	// Custom CodeBlock renderer for standard programming languages
	as.MdOpts.CodeBlock = func(c *ui.Context, b *markdown.CodeBlockView) {
		a.renderMarkdownCodeBlock(c, tc, b)
	}

	// Specialized code block renderers for JSON & Forge-Design
	as.MdOpts.Code = map[string]func(c *ui.Context, b *markdown.CodeBlockView){
		"json": func(c *ui.Context, b *markdown.CodeBlockView) {
			a.renderMarkdownJSONBlock(c, tc, b)
		},
		"jsonc": func(c *ui.Context, b *markdown.CodeBlockView) {
			a.renderMarkdownJSONBlock(c, tc, b)
		},
		"forge-design": func(c *ui.Context, b *markdown.CodeBlockView) {
			a.renderMarkdownForgeDesignBlock(c, tc, b)
		},
		"opendesign": func(c *ui.Context, b *markdown.CodeBlockView) {
			a.renderMarkdownForgeDesignBlock(c, tc, b)
		},
		"open-design": func(c *ui.Context, b *markdown.CodeBlockView) {
			a.renderMarkdownForgeDesignBlock(c, tc, b)
		},
	}
}

// renderMarkdownCodeBlock renders syntax-highlighted code with language badge,
// file header, line count, copy action, and "Run in Shell" for scripts.
func (a *AppUI) renderMarkdownCodeBlock(c *ui.Context, tc theme.ThemeColors, b *markdown.CodeBlockView) {
	lang := strings.ToLower(strings.TrimSpace(b.Language))
	code := b.Code()
	isShell := lang == "bash" || lang == "sh" || lang == "zsh" || lang == "shell"
	lineCount := strings.Count(code, "\n") + 1

	box := ui.Column(c).Radius(8).Background(tc.Canvas).
		BorderWidth(1).BorderColor(tc.Border.Alpha(0.7)).Clip().AlignSelf(ui.Stretch)
	box.Children(func() {
		// Header bar
		header := ui.Row(c).Height(30).Padding(0, 10).AlignItems(ui.Center).
			Background(tc.SurfaceRaised.Alpha(0.6)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5))
		header.Children(func() {
			displayLang := strings.ToUpper(lang)
			if displayLang == "" {
				displayLang = "CODE"
			}
			ui.Box(c).Radius(4).Padding(1, 6).Background(tc.Text.Alpha(0.06)).
				BorderWidth(1).BorderColor(tc.Border.Alpha(0.4)).Children(func() {
				ui.Text(c, displayLang).FontSize(10).Font("monospace").FontWeight(700).TextColor(tc.TextMuted)
			})

			if b.Info != "" && b.Info != lang {
				fileTitle := strings.TrimSpace(strings.TrimPrefix(b.Info, b.Language))
				if fileTitle != "" {
					ui.Row(c).Gap(4).Padding(0, 0, 0, 8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.FileIconSVG(fileTitle)).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.Text(c, fileTitle).FontSize(11).FontWeight(500).TextColor(tc.Text)
					})
				}
			}

			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d lines", lineCount)).FontSize(10).Font("monospace").TextColor(tc.TextMuted)

			// Action: Run in Terminal (for shell scripts / commands)
			if isShell {
				runBtn := ui.Row(c.Key("cb-run-"+fmt.Sprint(b.Index))).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).Cursor(ui.CursorPointer)
				runBtn.Children(func() {
					ui.Icon(c, icons.TablerTerminal).Width(11).Height(11).TextColor(tc.Accent)
					ui.Text(c, "Run in Shell").FontSize(10).FontWeight(600).TextColor(tc.Accent)
				})
				if runBtn.Clicked() {
					if a.Terminal != nil {
						a.Terminal.SendInput(code + "\n")
						a.ShowBottomTerminal = true
						c.AddToast(ui.Toast{Title: "Running in terminal", Type: "success"})
					}
				}
			}

			// Action: Copy Code
			copyBtn := ui.Row(c.Key("cb-copy-"+fmt.Sprint(b.Index))).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Cursor(ui.CursorPointer)
			copyBtn.Children(func() {
				ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "Copy").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
			})
			if copyBtn.Clicked() {
				mygo.Clipboard.WriteText(code)
				c.AddToast(ui.Toast{Title: "Copied code", Type: "success"})
			}
		})

		// Syntax-highlighted code container
		b.Body(c)
	})
}

// renderMarkdownJSONBlock renders formatted JSON blocks with key count,
// copy action, and syntax highlighting.
func (a *AppUI) renderMarkdownJSONBlock(c *ui.Context, tc theme.ThemeColors, b *markdown.CodeBlockView) {
	code := b.Code()
	lineCount := strings.Count(code, "\n") + 1

	keyCount := 0
	var parsed interface{}
	isValid := json.Unmarshal([]byte(strings.TrimSpace(code)), &parsed) == nil
	if isValid {
		switch val := parsed.(type) {
		case map[string]interface{}:
			keyCount = len(val)
		case []interface{}:
			keyCount = len(val)
		}
	}

	box := ui.Column(c).Radius(8).Background(tc.Canvas).
		BorderWidth(1).BorderColor(ui.Hex("#eab308").Alpha(0.35)).Clip().AlignSelf(ui.Stretch)
	box.Children(func() {
		// Header bar with JSON badge
		header := ui.Row(c).Height(30).Padding(0, 10).AlignItems(ui.Center).
			Background(tc.SurfaceRaised.Alpha(0.6)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5))
		header.Children(func() {
			ui.Box(c).Radius(4).Padding(1, 6).Background(ui.Hex("#eab308").Alpha(0.15)).
				BorderWidth(1).BorderColor(ui.Hex("#eab308").Alpha(0.35)).Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerBraces).Width(12).Height(12).TextColor(ui.Hex("#eab308"))
					ui.Text(c, "JSON").FontSize(10).Font("monospace").FontWeight(700).TextColor(ui.Hex("#eab308"))
				})
			})

			metaText := fmt.Sprintf("%d lines", lineCount)
			if keyCount > 0 {
				metaText = fmt.Sprintf("%d keys · %d lines", keyCount, lineCount)
			}
			ui.Text(c, metaText).FontSize(10).Font("monospace").TextColor(tc.TextMuted).Padding(0, 0, 0, 8)

			ui.Spacer(c)

			// Inspect in Modal action
			modalBtn := ui.Row(c.Key("json-modal-btn-"+fmt.Sprint(b.Index))).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
				Background(ui.Hex("#eab308").Alpha(0.12)).BorderWidth(1).BorderColor(ui.Hex("#eab308").Alpha(0.3)).Cursor(ui.CursorPointer)
			modalBtn.Children(func() {
				ui.Icon(c, icons.TablerExternalLink).Width(10).Height(10).TextColor(ui.Hex("#eab308"))
				ui.Text(c, "Modal").FontSize(10).FontWeight(600).TextColor(ui.Hex("#eab308"))
			})
			if modalBtn.Clicked() {
				a.JSONModalTitle = "JSON Code Block"
				a.JSONModalContent = code
				a.ShowJSONModal = true
			}

			ui.Box(c).Width(4)

			// Copy JSON action
			copyBtn := ui.Row(c.Key("json-copy-"+fmt.Sprint(b.Index))).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Cursor(ui.CursorPointer)
			copyBtn.Children(func() {
				ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "Copy JSON").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
			})
			if copyBtn.Clicked() {
				mygo.Clipboard.WriteText(code)
				c.AddToast(ui.Toast{Title: "Copied JSON", Type: "success"})
			}
		})

		// Highlighted body
		b.Body(c)
	})
}

// renderMarkdownForgeDesignBlock renders the native Forge-Design Studio block
// (the native equivalent to Open Design), presenting slides, motion, and UI mockups.
func (a *AppUI) renderMarkdownForgeDesignBlock(c *ui.Context, tc theme.ThemeColors, b *markdown.CodeBlockView) {
	code := b.Code()
	targetPath := "deck.html"
	if a.Workspace != nil && a.Workspace.RootDir != "" {
		targetPath = filepath.Join(a.Workspace.RootDir, "deck.html")
	}

	isMotion := strings.Contains(strings.ToLower(code), "motion") || strings.Contains(strings.ToLower(b.Info), "motion")
	modeLabel := "PRESENTATION DECK"
	if isMotion {
		modeLabel = "MOTION GRAPHICS"
	}

	card := ui.Column(c).Radius(10).Background(tc.SurfaceRaised).
		BorderWidth(1).BorderColor(tc.Accent.Alpha(0.4)).Clip().AlignSelf(ui.Stretch)
	card.Children(func() {
		// Header
		ui.Row(c).Height(36).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			Background(tc.Accent.Alpha(0.12)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Accent.Alpha(0.2)).Children(func() {
			ui.Box(c).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Accent.Alpha(0.25)).Children(func() {
				ui.Icon(c, icons.TablerBrush).Width(13).Height(13).TextColor(tc.Accent)
			})
			ui.Text(c, "Forge Design Studio").FontSize(12).FontWeight(700).TextColor(tc.Text)
			ui.Box(c).Radius(4).Padding(1, 6).Background(tc.Accent.Alpha(0.2)).Children(func() {
				ui.Text(c, modeLabel).FontSize(9).FontWeight(700).TextColor(tc.Accent)
			})
			ui.Spacer(c)
			ui.Text(c, "16:9 • 1080p Full HD").FontSize(10).Font("monospace").TextColor(tc.TextMuted)
		})

		// Simulated visual canvas card
		ui.Column(c).Padding(14, 16).Gap(8).Background(tc.Canvas).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Native Design Studio Canvas").FontSize(13).FontWeight(700).TextColor(tc.Text)
				ui.Spacer(c)
				ui.Box(c).Radius(4).Padding(2, 6).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Children(func() {
					ui.Text(c, "Open Design Core").FontSize(10).FontWeight(600).TextColor(tc.Accent)
				})
			})
			ui.Text(c, "Code-driven presentation & motion artifact generated natively. Ready for instant presentation preview, slide navigation, and video export.").
				FontSize(12).TextColor(tc.TextMuted)

			// Action Controls
			ui.Row(c).Gap(8).Padding(4, 0, 0, 0).AlignItems(ui.Center).Children(func() {
				previewDeckBtn := ui.Row(c.Key("fdb-deck-"+fmt.Sprint(b.Index))).Height(26).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
					Background(tc.Accent).Cursor(ui.CursorPointer)
				previewDeckBtn.Children(func() {
					ui.Icon(c, icons.TablerPresentation).Width(13).Height(13).TextColor(ui.Hex("#ffffff"))
					ui.Text(c, "Open in Deck Viewer").FontSize(11).FontWeight(600).TextColor(ui.Hex("#ffffff"))
				})
				if previewDeckBtn.Clicked() {
					if a.Workspace != nil && a.Workspace.RootDir != "" && len(code) > 0 {
						_ = os.WriteFile(targetPath, []byte(code), 0644)
					}
					a.openDeckViewer(targetPath)
					a.RightDrawerTab = RightTabDeck
					a.ShowRightDrawer = true
				}

				previewBrowserBtn := ui.Row(c.Key("fdb-browser-"+fmt.Sprint(b.Index))).Height(26).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
				previewBrowserBtn.Children(func() {
					ui.Icon(c, icons.TablerBrowser).Width(13).Height(13).TextColor(tc.Text)
					ui.Text(c, "Browser Viewer").FontSize(11).FontWeight(500).TextColor(tc.Text)
				})
				if previewBrowserBtn.Clicked() {
					a.RightDrawerTab = RightTabBrowser
					a.ShowRightDrawer = true
				}

				copyBtn := ui.Row(c.Key("fdb-copy-"+fmt.Sprint(b.Index))).Height(26).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
				copyBtn.Children(func() {
					ui.Icon(c, icons.TablerCopy).Width(12).Height(12).TextColor(tc.TextMuted)
					ui.Text(c, "Copy Code").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
				})
				if copyBtn.Clicked() {
					mygo.Clipboard.WriteText(code)
					c.AddToast(ui.Toast{Title: "Copied Design Code", Type: "success"})
				}
			})
		})
	})
}

// boardTaskEnvelope represents an agent JSON payload containing board task data.
type boardTaskEnvelope struct {
	Message string              `json:"message"`
	Success bool                `json:"success"`
	Task    *internalboard.Task `json:"task"`
}

func parseBoardTaskJSON(raw string) (*internalboard.Task, string) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		return nil, ""
	}
	var env boardTaskEnvelope
	if err := json.Unmarshal([]byte(trimmed), &env); err == nil && env.Task != nil {
		if env.Task.Key != "" || env.Task.Title != "" {
			return env.Task, env.Message
		}
	}
	var direct internalboard.Task
	if err := json.Unmarshal([]byte(trimmed), &direct); err == nil {
		if direct.Key != "" || (direct.Title != "" && direct.Status != "") {
			return &direct, ""
		}
	}
	return nil, ""
}

func boardStatusPill(status string, tc theme.ThemeColors) (string, ui.Color, ui.Color) {
	st := strings.ToLower(strings.TrimSpace(status))
	switch st {
	case "done", "completed":
		return "Done", tc.Success, tc.Success.Alpha(0.16)
	case "in_progress", "in-progress", "working":
		return "In Progress", tc.Warning, tc.Warning.Alpha(0.16)
	case "in_review", "review":
		return "In Review", ui.Hex("#a855f7"), ui.Hex("#a855f7").Alpha(0.16)
	case "in_testing", "testing":
		return "In Testing", ui.Hex("#3b82f6"), ui.Hex("#3b82f6").Alpha(0.16)
	case "todo", "to_do":
		return "To Do", tc.TextMuted, tc.Text.Alpha(0.08)
	default:
		return "Backlog", tc.TextMuted, tc.Text.Alpha(0.08)
	}
}

func boardPriorityPill(priority string, tc theme.ThemeColors) (string, ui.Color, ui.Color) {
	pr := strings.ToLower(strings.TrimSpace(priority))
	switch pr {
	case "urgent":
		return "Urgent", tc.Error, tc.Error.Alpha(0.16)
	case "high":
		return "High", ui.Hex("#f97316"), ui.Hex("#f97316").Alpha(0.16)
	case "medium":
		return "Medium", ui.Hex("#eab308"), ui.Hex("#eab308").Alpha(0.16)
	default:
		return "Low", tc.TextMuted, tc.Text.Alpha(0.08)
	}
}

// renderBoardTaskCard renders an interactive Kanban Board Task card instead of
// dumping raw JSON into the chat transcript.
func (a *AppUI) renderBoardTaskCard(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage, task *internalboard.Task, statusMsg string, rawJSON string) {
	statusLabel, statusColor, statusBg := boardStatusPill(task.Status, tc)
	prioLabel, prioColor, prioBg := boardPriorityPill(task.Priority, tc)

	card := ui.Column(c.Key("board-task-" + task.Key + fmt.Sprint(m.ID))).Radius(10).Background(tc.SurfaceRaised).
		BorderWidth(1).BorderColor(tc.Border).Clip().AlignSelf(ui.Stretch)
	card.Children(func() {
		// Header row
		hdr := ui.Row(c).Height(34).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6))
		hdr.Children(func() {
			ui.Box(c).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Accent.Alpha(0.15)).Children(func() {
				ui.Icon(c, icons.TablerLayoutKanban).Width(12).Height(12).TextColor(tc.Accent)
			})
			ui.Text(c, "BOARD TASK").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)

			// Key chip
			if task.Key != "" {
				ui.Row(c).Padding(1, 6).Radius(4).Background(tc.Accent.Alpha(0.12)).
					BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).Children(func() {
					ui.Text(c, task.Key).FontSize(11).FontWeight(700).TextColor(tc.Accent)
				})
			}

			// Status badge
			ui.Row(c).Padding(1, 6).Radius(4).Background(statusBg).Children(func() {
				ui.Text(c, statusLabel).FontSize(10).FontWeight(600).TextColor(statusColor)
			})

			// Priority badge
			if task.Priority != "" {
				ui.Row(c).Padding(1, 6).Radius(4).Background(prioBg).Children(func() {
					ui.Text(c, prioLabel).FontSize(10).FontWeight(600).TextColor(prioColor)
				})
			}

			ui.Spacer(c)

			// Action: Open in Board (⌘B)
			boardBtn := ui.Row(c.Key("b-open-"+task.Key+fmt.Sprint(m.ID))).Height(22).Radius(4).Padding(0, 8).Gap(4).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			boardBtn.Children(func() {
				ui.Icon(c, icons.TablerLayoutKanban).Width(11).Height(11).TextColor(tc.Accent)
				ui.Text(c, "Open Board (⌘B)").FontSize(10).FontWeight(500).TextColor(tc.Text)
			})
			if boardBtn.Hovered() {
				boardBtn.Background(tc.Text.Alpha(0.06))
			}
			if boardBtn.Clicked() {
				a.openBoardSurface()
			}

			// Action: View JSON in Modal
			jsonBtn := ui.Row(c.Key("b-json-"+task.Key+fmt.Sprint(m.ID))).Height(22).Radius(4).Padding(0, 8).Gap(4).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			jsonBtn.Children(func() {
				ui.Icon(c, icons.TablerExternalLink).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "JSON").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
			})
			if jsonBtn.Hovered() {
				jsonBtn.Background(tc.Text.Alpha(0.06))
			}
			if jsonBtn.Clicked() {
				a.JSONModalTitle = fmt.Sprintf("Board Task Payload (%s)", task.Key)
				a.JSONModalContent = rawJSON
				a.ShowJSONModal = true
			}
		})

		// Card Body
		ui.Column(c).Padding(12, 14).Gap(6).Background(tc.Canvas).Children(func() {
			title := task.Title
			if title == "" {
				title = "Kanban Board Task"
			}
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(tc.Text).Selectable()

			if statusMsg != "" && statusMsg != title {
				ui.Text(c, statusMsg).FontSize(11).TextColor(tc.TextMuted)
			}
			if task.Description != "" {
				ui.Text(c, task.Description).FontSize(11).LineHeight(16).TextColor(tc.TextMuted).Selectable()
			}
		})

		// Card Footer
		ui.Row(c).Height(36).Padding(0, 12).AlignItems(ui.Center).
			Background(tc.SurfaceRaised.Alpha(0.4)).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.4)).Children(func() {
			if task.Assignee != "" {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerRobot).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.Text(c, task.Assignee).FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
				})
			}
			if task.WorkspacePath != "" {
				ui.Box(c).Width(8)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerFolder).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, filepath.Base(task.WorkspacePath)).FontSize(10).TextColor(tc.TextMuted)
				})
			}

			ui.Spacer(c)

			// Quick state transition button: Move to Done
			if task.Status != "done" && task.Status != "completed" {
				moveDoneBtn := ui.Row(c.Key("b-done-"+task.Key+fmt.Sprint(m.ID))).Height(24).Radius(5).Padding(0, 10).Gap(4).
					Background(tc.Success.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.3)).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				moveDoneBtn.Children(func() {
					ui.Icon(c, icons.TablerCheck).Width(11).Height(11).TextColor(tc.Success)
					ui.Text(c, "Move to Done").FontSize(10).FontWeight(600).TextColor(tc.Success)
				})
				if moveDoneBtn.Hovered() {
					moveDoneBtn.Background(tc.Success.Alpha(0.24))
				}
				if moveDoneBtn.Clicked() {
					taskID := task.ID
					if taskID == "" {
						taskID = task.Key
					}
					if a.Board != nil {
						_ = a.Board.MoveTaskStatus(taskID, "done")
					}
					task.Status = "done"
					c.AddToast(ui.Toast{Title: "Task moved to Done", Type: "success"})
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}
			} else {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerCircleCheck).Width(13).Height(13).TextColor(tc.Success)
					ui.Text(c, "Completed").FontSize(11).FontWeight(600).TextColor(tc.Success)
				})
			}
		})
	})
}

// agentSubagentPayload represents a subagent spawn or execution status payload.
type agentSubagentPayload struct {
	Status           string `json:"status"`
	Role             string `json:"role"`
	SelectedSubagent string `json:"selected_subagent"`
	SubagentID       string `json:"subagent_id"`
	Message          string `json:"message"`
	Task             string `json:"task"`
}

// renderAgentJSONCard draws a clean compact card for agent/subagent JSON responses
// with a "View in Modal ↗" action instead of expanding raw JSON into the chat stream.
func (a *AppUI) renderAgentJSONCard(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage, rawJSON string) {
	var pretty bytes.Buffer
	formatted := rawJSON
	if json.Indent(&pretty, []byte(rawJSON), "", "  ") == nil {
		formatted = pretty.String()
	}
	lineCount := strings.Count(formatted, "\n") + 1

	var subPayload agentSubagentPayload
	isSubagent := false
	if json.Unmarshal([]byte(rawJSON), &subPayload) == nil {
		if subPayload.Status == "spawned" || subPayload.SelectedSubagent != "" || subPayload.Role != "" {
			isSubagent = true
		}
	}

	card := ui.Column(c.Key("agent-json-" + fmt.Sprint(m.ID))).Radius(10).Background(tc.SurfaceRaised).
		BorderWidth(1).BorderColor(tc.Border).Clip().AlignSelf(ui.Stretch)
	card.Children(func() {
		// Header row
		ui.Row(c).Height(36).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
			if isSubagent {
				ui.Box(c).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent.Alpha(0.18)).Children(func() {
					ui.Icon(c, icons.TablerSparkles).Width(13).Height(13).TextColor(tc.Accent)
				})
				ui.Text(c, "SUB-AGENT STATUS").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.Accent)
				if subPayload.Role != "" {
					ui.Row(c).Padding(1, 6).Radius(4).Background(tc.Text.Alpha(0.06)).Children(func() {
						ui.Text(c, subPayload.Role).FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
					})
				}
				if subPayload.Status != "" {
					ui.Row(c).Padding(1, 6).Radius(4).Background(tc.Success.Alpha(0.15)).Children(func() {
						ui.Text(c, subPayload.Status).FontSize(10).FontWeight(600).TextColor(tc.Success)
					})
				}
			} else {
				ui.Box(c).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).
					Background(ui.Hex("#eab308").Alpha(0.15)).Children(func() {
					ui.Icon(c, icons.TablerBraces).Width(13).Height(13).TextColor(ui.Hex("#eab308"))
				})
				ui.Text(c, "AGENT DATA PAYLOAD").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(ui.Hex("#eab308"))
				ui.Text(c, fmt.Sprintf("%d lines", lineCount)).FontSize(10).Font("monospace").TextColor(tc.TextMuted)
			}

			ui.Spacer(c)

			// Action: View in Modal ↗
			modalBtn := ui.Row(c.Key("aj-modal-"+fmt.Sprint(m.ID))).Height(22).Radius(4).Padding(0, 8).Gap(4).
				Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			modalBtn.Children(func() {
				ui.Icon(c, icons.TablerExternalLink).Width(11).Height(11).TextColor(tc.Accent)
				ui.Text(c, "View in Modal ↗").FontSize(10).FontWeight(600).TextColor(tc.Accent)
			})
			if modalBtn.Hovered() {
				modalBtn.Background(tc.Accent.Alpha(0.22))
			}
			if modalBtn.Clicked() {
				title := "Agent JSON Payload"
				if isSubagent && subPayload.SelectedSubagent != "" {
					title = fmt.Sprintf("Sub-agent: %s", subPayload.SelectedSubagent)
				}
				a.JSONModalTitle = title
				a.JSONModalContent = formatted
				a.ShowJSONModal = true
			}

			// Action: Copy JSON
			copyBtn := ui.Row(c.Key("aj-copy-"+fmt.Sprint(m.ID))).Height(22).Radius(4).Padding(0, 8).Gap(4).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			copyBtn.Children(func() {
				ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "Copy").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
			})
			if copyBtn.Hovered() {
				copyBtn.Background(tc.Text.Alpha(0.06))
			}
			if copyBtn.Clicked() {
				mygo.Clipboard.WriteText(formatted)
				c.AddToast(ui.Toast{Title: "Copied JSON to clipboard", Type: "success"})
			}
		})

		// Card Body
		ui.Column(c).Padding(10, 14).Gap(4).Background(tc.Canvas).Children(func() {
			if isSubagent {
				if subPayload.SelectedSubagent != "" {
					ui.Text(c, subPayload.SelectedSubagent).FontSize(13).FontWeight(600).TextColor(tc.Text)
				}
				summaryTxt := subPayload.Message
				if subPayload.Task != "" {
					summaryTxt = subPayload.Task
				}
				if summaryTxt != "" {
					ui.Text(c, summaryTxt).FontSize(11).LineHeight(16).TextColor(tc.TextMuted).SingleLine()
				}
			} else {
				firstLine := strings.TrimSpace(strings.Split(formatted, "\n")[0])
				if len(firstLine) > 80 {
					firstLine = firstLine[:80] + "…"
				}
				ui.Text(c, firstLine).FontSize(11).Font("monospace").TextColor(tc.TextMuted).SingleLine()
			}
		})
	})
}

// renderJSONResponseCard routes assistant pure JSON data to a Kanban Board Task Card
// or an Agent JSON Card with "View in Modal ↗" action.
func (a *AppUI) renderJSONResponseCard(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage, rawJSON string) {
	if task, msg := parseBoardTaskJSON(rawJSON); task != nil {
		a.renderBoardTaskCard(c, tc, m, task, msg, rawJSON)
		return
	}
	a.renderAgentJSONCard(c, tc, m, rawJSON)
}

// renderJSONModal displays pure JSON payloads in a focused, scrollable dialog.
func (a *AppUI) renderJSONModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Modal(c, &a.ShowJSONModal, func() {
		ui.Column(c).Width(680).Height(500).Radius(12).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Children(func() {
			// Header
			ui.Row(c).Height(50).Padding(0, 16).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Size(30, 30).Radius(6).Background(ui.Hex("#eab308").Alpha(0.15)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerBraces).Width(16).Height(16).TextColor(ui.Hex("#eab308"))
				})
				ui.Box(c).Width(10)
				title := a.JSONModalTitle
				if title == "" {
					title = "JSON Payload Inspector"
				}
				ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(tc.Text).Grow(1)

				copyBtn := ui.Row(c.Key("json-modal-copy")).Height(26).Radius(5).Padding(0, 10).Gap(4).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				copyBtn.Children(func() {
					ui.Icon(c, icons.TablerCopy).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.Text(c, "Copy JSON").FontSize(11).TextColor(tc.Text)
				})
				if copyBtn.Clicked() {
					mygo.Clipboard.WriteText(a.JSONModalContent)
					c.AddToast(ui.Toast{Title: "Copied JSON to clipboard", Type: "success"})
				}

				ui.Box(c).Width(8)

				closeBtn := ui.Row(c.Key("json-modal-close")).Size(26, 26).Radius(5).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(14).Height(14).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.ShowJSONModal = false
				}
			})

			// Body: Monospace code view
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(16, 18).Background(tc.Canvas).Children(func() {
					ui.Text(c, a.JSONModalContent).Font("monospace").FontSize(12).LineHeight(18).TextColor(tc.Text).Selectable()
				})
			})

			// Footer
			ui.Row(c).Height(40).Padding(0, 16).AlignItems(ui.Center).Background(tc.Surface).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				lines := strings.Count(a.JSONModalContent, "\n") + 1
				bytesCount := len(a.JSONModalContent)
				ui.Text(c, fmt.Sprintf("%d lines · %d bytes", lines, bytesCount)).FontSize(11).TextColor(tc.TextMuted)
				ui.Spacer(c)
				doneBtn := ui.Row(c.Key("json-modal-done")).Height(26).Radius(5).Padding(0, 14).
					Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				doneBtn.Children(func() {
					ui.Text(c, "Close").FontSize(11).FontWeight(600).TextColor(tc.Canvas)
				})
				if doneBtn.Clicked() {
					a.ShowJSONModal = false
				}
			})
		})
	})
}

// renderPlanModal renders the full Plan, runtime checklist, and subagent progress in a centered modal dialog.
func (a *AppUI) renderPlanModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	if a.Agent == nil {
		return
	}
	tm := a.Agent.TaskManager
	var tasks []agent.TaskItem
	var completed, total int
	goal := ""
	if tm != nil {
		tasks = tm.GetTasks()
		completed, total = tm.Progress()
		goal = tm.Goal
	}
	if goal == "" {
		if activeSess := a.Agent.ActiveSession(); activeSess != nil && activeSess.Title != "" {
			goal = activeSess.Title
		} else {
			goal = "Active Development Plan"
		}
	}
	percent := 0
	if total > 0 {
		percent = (completed * 100) / total
	}

	ui.Modal(c, &a.ShowPlanModal, func() {
		ui.Column(c).Width(620).Height(520).Radius(12).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Children(func() {
			// Header
			ui.Row(c).Height(54).Padding(0, 18).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Size(32, 32).Radius(7).Background(tc.Accent.Alpha(0.14)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerChecklist).Width(17).Height(17).TextColor(tc.Accent)
				})
				ui.Box(c).Width(10)
				ui.Column(c).Grow(1).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Plans & Runtime Tasks").FontSize(13).FontWeight(600).TextColor(tc.Text)
						if total > 0 {
							ui.Row(c).Padding(1, 6).Radius(4).Background(tc.Accent.Alpha(0.15)).Children(func() {
								ui.Text(c, fmt.Sprintf("%d/%d (%d%%)", completed, total, percent)).FontSize(10).FontWeight(700).TextColor(tc.Accent)
							})
						}
					})
					ui.Text(c, "Inspect execution goal, subagent status, and checklist items").FontSize(11).TextColor(tc.TextMuted)
				})
				closeBtn := ui.Row(c.Key("plan-modal-close")).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(15).Height(15).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.ShowPlanModal = false
				}
			})

			// Body
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(16, 20).Gap(14).Children(func() {
					// Goal Card
					ui.Column(c).Padding(12, 14).Radius(8).Gap(4).Background(tc.Canvas).
						BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, icons.TablerSparkles).Width(13).Height(13).TextColor(tc.Accent)
							ui.Text(c, "GOAL").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
						})
						ui.Text(c, goal).FontSize(13).FontWeight(500).TextColor(tc.Text)
					})

					// Progress Bar
					if total > 0 {
						ui.Column(c).Gap(6).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "Checklist Progress").FontSize(11).FontWeight(600).TextColor(tc.Text)
								ui.Spacer(c)
								ui.Text(c, fmt.Sprintf("%d of %d completed", completed, total)).FontSize(11).TextColor(tc.TextMuted)
							})
							track := ui.Row(c).Height(6).Radius(3).Background(tc.Canvas).
								BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Clip()
							track.Children(func() {
								if percent > 0 {
									ui.Box(c).FillHeight().Width(float32(percent) * 5.8).Background(tc.Accent)
								}
							})
						})
					}

					// Tasks checklist
					ui.Column(c).Gap(6).Children(func() {
						ui.Text(c, "TASKS").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
						if len(tasks) == 0 {
							ui.Row(c).Padding(16, 12).Radius(8).Background(tc.Canvas).
								BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Justify(ui.Center).Children(func() {
								ui.Text(c, "No tasks tracked for this session").FontSize(12).TextColor(tc.TextMuted)
							})
						} else {
							for _, t := range tasks {
								taskItem := t
								isDone := taskItem.Status == agent.TaskCompleted
								isInProg := taskItem.Status == agent.TaskInProgress

								tRow := ui.Row(c.Key("plan-modal-task-"+taskItem.ID)).Padding(8, 10).Radius(6).Gap(8).
									AlignItems(ui.Center).Background(tc.Canvas).BorderWidth(1).
									BorderColor(tc.Border.Alpha(0.5)).Cursor(ui.CursorPointer)
								if isInProg {
									tRow.BorderColor(tc.Accent.Alpha(0.5)).Background(tc.Accent.Alpha(0.04))
								}
								tRow.Children(func() {
									if isDone {
										ui.Icon(c, icons.TablerCircleCheck).Width(15).Height(15).TextColor(tc.Success)
									} else if isInProg {
										ui.Spinner(c).Size(13, 13).Label("running")
									} else {
										ui.Icon(c, icons.TablerCircle).Width(15).Height(15).TextColor(tc.TextMuted)
									}
									ui.Column(c).Grow(1).Gap(2).Children(func() {
										txt := ui.Text(c, taskItem.Title).FontSize(12).LineHeight(16)
										if isDone {
											txt.TextColor(tc.TextMuted)
										} else if isInProg {
											txt.FontWeight(600).TextColor(tc.Accent)
										} else {
											txt.TextColor(tc.Text)
										}
										if taskItem.ActiveForm != "" && isInProg {
											ui.Text(c, taskItem.ActiveForm).FontSize(10).TextColor(tc.TextMuted)
										}
									})
								})
								if tRow.Clicked() && tm != nil {
									tm.ToggleTask(taskItem.ID)
								}
							}
						}
					})

					// Subagents if any
					subs := a.Agent.Subagents(a.Agent.ActiveSessionID)
					if len(subs) > 0 {
						ui.Column(c).Gap(6).Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "SPAWNED SUBAGENTS").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
								ui.Row(c).Padding(1, 5).Radius(4).Background(tc.Text.Alpha(0.06)).Children(func() {
									ui.Text(c, fmt.Sprintf("%d", len(subs))).FontSize(9).TextColor(tc.TextMuted)
								})
							})
							for _, sub := range subs {
								subItem := sub
								_, statusColor := subagentStatus(subItem.State, tc)
								sRow := ui.Row(c.Key("plan-modal-sub-"+subItem.ID)).Padding(8, 10).Radius(6).Gap(8).
									AlignItems(ui.Center).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
								sRow.Children(func() {
									ui.Box(c).Size(7, 7).Radius(4).Background(statusColor)
									ui.Column(c).Grow(1).Gap(2).Children(func() {
										ui.Text(c, subItem.Name).FontSize(12).FontWeight(500).TextColor(tc.Text)
										if subItem.RoleFilter != "" {
											ui.Text(c, string(subItem.RoleFilter)).FontSize(10).TextColor(tc.TextMuted)
										}
									})
								})
							}
						})
					}
				})
			})

			// Footer
			ui.Row(c).Height(50).Padding(0, 18).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				if len(tasks) > 0 {
					copyBtn := ui.Row(c.Key("plan-copy-checklist")).Height(28).Radius(5).Padding(0, 10).Gap(4).
						Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					copyBtn.Children(func() {
						ui.Icon(c, icons.TablerCopy).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.Text(c, "Copy Checklist").FontSize(11).TextColor(tc.Text)
					})
					if copyBtn.Clicked() {
						var sb strings.Builder
						for _, t := range tasks {
							box := "[ ]"
							if t.Status == agent.TaskCompleted {
								box = "[x]"
							}
							sb.WriteString(fmt.Sprintf("%s %s\n", box, t.Title))
						}
						mygo.Clipboard.WriteText(sb.String())
						c.AddToast(ui.Toast{Title: "Checklist copied to clipboard", Type: "success"})
					}

					ui.Box(c).Width(8)

					if tm != nil {
						clearBtn := ui.Row(c.Key("plan-clear-tasks")).Height(28).Radius(5).Padding(0, 10).
							AlignItems(ui.Center).Cursor(ui.CursorPointer)
						clearBtn.Children(func() {
							ui.Text(c, "Clear Tasks").FontSize(11).TextColor(tc.Error)
						})
						if clearBtn.Hovered() {
							clearBtn.Background(tc.Error.Alpha(0.1))
						}
						if clearBtn.Clicked() {
							tm.Clear()
							c.AddToast(ui.Toast{Title: "Tasks cleared", Type: "info"})
						}
					}
				}

				ui.Spacer(c)

				closeModalBtn := ui.Row(c.Key("plan-modal-done-btn")).Height(28).Radius(5).Padding(0, 16).
					Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				closeModalBtn.Children(func() {
					ui.Text(c, "Done").FontSize(11).FontWeight(600).TextColor(tc.Canvas)
				})
				if closeModalBtn.Clicked() {
					a.ShowPlanModal = false
				}
			})
		})
	})
}

// renderTurnDesignCard renders an interactive Forge Design Studio banner if
// a presentation deck or motion file was created or modified during the turn.
func (a *AppUI) renderTurnDesignCard(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage) {
	if m == nil {
		return
	}
	var targetFile string
	for _, ch := range m.Changes {
		if isDeckOrMotionFile(ch.Path) {
			targetFile = ch.Path
			break
		}
	}
	if targetFile == "" {
		for _, t := range m.Tools {
			args := parseToolArgs(t.Command, t.ToolName)
			if isDeckOrMotionFile(args.Path) {
				targetFile = args.Path
				break
			}
			nl := strings.ToLower(t.ToolName)
			if strings.Contains(nl, "deck") || strings.Contains(nl, "motion") || strings.Contains(nl, "opendesign") || strings.Contains(nl, "open-design") {
				if a.Workspace != nil && a.Workspace.RootDir != "" {
					targetFile = filepath.Join(a.Workspace.RootDir, "deck.html")
				}
				break
			}
		}
	}
	if targetFile == "" {
		return
	}

	fullPath := targetFile
	if !filepath.IsAbs(fullPath) && a.Workspace != nil && a.Workspace.RootDir != "" {
		fullPath = filepath.Join(a.Workspace.RootDir, fullPath)
	}

	isMotion := strings.Contains(strings.ToLower(targetFile), "motion") || strings.HasSuffix(strings.ToLower(targetFile), ".mp4")
	modeLabel := "PRESENTATION DECK"
	if isMotion {
		modeLabel = "MOTION GRAPHICS"
	}

	card := ui.Column(c.Key("turn-design-" + fmt.Sprint(m.ID))).Radius(10).Background(tc.SurfaceRaised).
		BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35)).Clip()
	card.Children(func() {
		ui.Row(c).Height(36).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			Background(tc.Accent.Alpha(0.12)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Accent.Alpha(0.2)).Children(func() {
			ui.Box(c).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Accent.Alpha(0.25)).Children(func() {
				ui.Icon(c, icons.TablerBrush).Width(13).Height(13).TextColor(tc.Accent)
			})
			ui.Text(c, "Forge Design Studio").FontSize(12).FontWeight(700).TextColor(tc.Text)
			ui.Box(c).Radius(4).Padding(1, 6).Background(tc.Accent.Alpha(0.2)).Children(func() {
				ui.Text(c, modeLabel).FontSize(9).FontWeight(700).TextColor(tc.Accent)
			})
			ui.Spacer(c)
			ui.Text(c, filepath.Base(targetFile)).FontSize(10).Font("monospace").TextColor(tc.TextMuted)
		})

		ui.Column(c).Padding(12, 14).Gap(8).Background(tc.Canvas).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Presentation ready to preview & present").FontSize(13).FontWeight(600).TextColor(tc.Text)
				ui.Spacer(c)
				ui.Text(c, "16:9 • 1080p Full HD").FontSize(10).Font("monospace").TextColor(tc.TextMuted)
			})
			ui.Text(c, "Generated slide deck with Open Design mathematical layout and theme styling.").
				FontSize(12).TextColor(tc.TextMuted)

			ui.Row(c).Gap(8).Padding(4, 0, 0, 0).AlignItems(ui.Center).Children(func() {
				deckBtn := ui.Row(c.Key("td-deck-"+fmt.Sprint(m.ID))).Height(26).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
					Background(tc.Accent).Cursor(ui.CursorPointer)
				deckBtn.Children(func() {
					ui.Icon(c, icons.TablerPresentation).Width(13).Height(13).TextColor(ui.Hex("#ffffff"))
					ui.Text(c, "Open in Deck Viewer").FontSize(11).FontWeight(600).TextColor(ui.Hex("#ffffff"))
				})
				if deckBtn.Clicked() {
					a.openDeckViewer(fullPath)
					a.RightDrawerTab = RightTabDeck
					a.ShowRightDrawer = true
				}

				browserBtn := ui.Row(c.Key("td-browser-"+fmt.Sprint(m.ID))).Height(26).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
				browserBtn.Children(func() {
					ui.Icon(c, icons.TablerBrowser).Width(13).Height(13).TextColor(tc.Text)
					ui.Text(c, "Browser Viewer").FontSize(11).FontWeight(500).TextColor(tc.Text)
				})
				if browserBtn.Clicked() {
					a.RightDrawerTab = RightTabBrowser
					a.ShowRightDrawer = true
				}

				editBtn := ui.Row(c.Key("td-edit-"+fmt.Sprint(m.ID))).Height(26).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
				editBtn.Children(func() {
					ui.Icon(c, icons.TablerCode).Width(13).Height(13).TextColor(tc.Text)
					ui.Text(c, "Open in Editor").FontSize(11).FontWeight(500).TextColor(tc.Text)
				})
				if editBtn.Clicked() {
					if a.Editor != nil {
						rel := targetFile
						if a.Workspace != nil && a.Workspace.RootDir != "" {
							if r, err := filepath.Rel(a.Workspace.RootDir, fullPath); err == nil {
								rel = r
							}
						}
						_, _ = a.Editor.OpenFile(fullPath, rel)
						a.Mode = ModeEditor
						a.IsGitClientOpen = false
					}
				}
			})
		})
	})
}

// renderToolSkillBlock renders the specialized Skills Load UI Card.
func (a *AppUI) renderToolSkillBlock(c *ui.Context, tc theme.ThemeColors, t *agent.ToolExecution, cleanOut string, failed bool) {
	skillName := extractSkillLabel(t.Command, t.ToolName)
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).
				Background(ui.Hex("#8b5cf6").Alpha(0.18)).Children(func() {
				ui.Icon(c, icons.TablerSparkles).Width(13).Height(13).TextColor(ui.Hex("#a78bfa"))
			})
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, skillName).FontSize(12).FontWeight(600).TextColor(tc.Text)
					ui.Box(c).Radius(3).Padding(1, 5).Background(ui.Hex("#8b5cf6").Alpha(0.15)).Children(func() {
						ui.Text(c, "SKILL LOADED").FontSize(9).FontWeight(700).TextColor(ui.Hex("#a78bfa"))
					})
				})
			})
			openSkillBtn := ui.Row(c.Key("tskill-"+t.ID)).Height(22).Radius(4).Padding(0, 7).Gap(4).AlignItems(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
			openSkillBtn.Children(func() {
				ui.Icon(c, icons.TablerSettings).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "Skills & Tools").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
			})
			if openSkillBtn.Clicked() {
				a.openSettings(settings.SettingsTabSkills)
			}
		})
		if t.Command != "" && t.Command != "{}" {
			ui.Text(c, "Parameters: "+t.Command).FontSize(11).Font("monospace").TextColor(tc.TextMuted).Selectable()
		}
		if cleanOut != "" {
			renderToolOutputBox(c, tc, cleanOut, failed)
		}
	})
}

// renderToolMCPLoadBlock renders the specialized MCP Server Load UI Card.
func (a *AppUI) renderToolMCPLoadBlock(c *ui.Context, tc theme.ThemeColors, t *agent.ToolExecution, cleanOut string, failed bool) {
	serverName := extractMCPServerLabel(t.Command, t.ToolName)
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).
				Background(ui.Hex("#06b6d4").Alpha(0.18)).Children(func() {
				ui.Icon(c, icons.TablerPlug).Width(13).Height(13).TextColor(ui.Hex("#22d3ee"))
			})
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, serverName).FontSize(12).FontWeight(600).TextColor(tc.Text)
					ui.Box(c).Radius(3).Padding(1, 5).Background(ui.Hex("#06b6d4").Alpha(0.15)).Children(func() {
						ui.Text(c, "MCP SERVER").FontSize(9).FontWeight(700).TextColor(ui.Hex("#22d3ee"))
					})
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success)
				})
			})
			mcpBtn := ui.Row(c.Key("tmcp-load-"+t.ID)).Height(22).Radius(4).Padding(0, 7).Gap(4).AlignItems(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
			mcpBtn.Children(func() {
				ui.Icon(c, icons.TablerPlug).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "MCP Settings").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
			})
			if mcpBtn.Clicked() {
				a.openSettings(settings.SettingsTabMCP)
			}
		})
		if cleanOut != "" {
			renderToolOutputBox(c, tc, cleanOut, failed)
		}
	})
}

// renderToolMCPCallBlock renders the specialized MCP Tool Calling UI Card.
func (a *AppUI) renderToolMCPCallBlock(c *ui.Context, tc theme.ThemeColors, t *agent.ToolExecution, cleanOut string, failed bool) {
	server, tool := extractMCPToolParts(t.ToolName)
	statusText := "Completed"
	statusColor := tc.Success
	if t.Status == "running" {
		statusText = "Running..."
		statusColor = tc.Accent
	} else if failed {
		statusText = "Failed"
		statusColor = tc.Error
	}

	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).
				Background(ui.Hex("#0284c7").Alpha(0.18)).Children(func() {
				ui.Icon(c, icons.TablerCpu).Width(13).Height(13).TextColor(ui.Hex("#38bdf8"))
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Grow(1).Children(func() {
				if server != "" {
					ui.Box(c).Radius(3).Padding(1, 5).Background(ui.Hex("#0284c7").Alpha(0.15)).Children(func() {
						ui.Text(c, strings.ToUpper(server)).FontSize(9).FontWeight(700).TextColor(ui.Hex("#38bdf8"))
					})
				}
				ui.Text(c, tool).FontSize(12).Font("monospace").FontWeight(600).TextColor(tc.Text)
			})
			ui.Text(c, statusText).FontSize(11).FontWeight(600).TextColor(statusColor)
		})

		// Arguments block if provided
		if t.Command != "" && t.Command != "{}" {
			formattedCmd := t.Command
			var pretty bytes.Buffer
			if json.Indent(&pretty, []byte(t.Command), "", "  ") == nil {
				formattedCmd = pretty.String()
			}
			ui.Column(c).Radius(6).Padding(6, 8).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4)).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "PARAMETERS").FontSize(9).FontWeight(700).TextColor(tc.TextMuted)
					ui.Spacer(c)
					copyArgs := ui.Row(c.Key("mcp-copy-args-"+t.ID)).Height(18).Radius(3).Padding(0, 5).Gap(3).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					copyArgs.Children(func() {
						ui.Icon(c, icons.TablerCopy).Width(10).Height(10).TextColor(tc.IconMuted)
						ui.Text(c, "Copy").FontSize(9).TextColor(tc.TextMuted)
					})
					if copyArgs.Clicked() {
						mygo.Clipboard.WriteText(formattedCmd)
						c.AddToast(ui.Toast{Title: "Copied Parameters", Type: "success"})
					}
				})
				ui.Text(c, formattedCmd).FontSize(11).Font("monospace").TextColor(tc.Text).Selectable()
			})
		}

		if cleanOut != "" {
			renderToolOutputBox(c, tc, cleanOut, failed)
		}
	})
}

// renderToolDesignBlock renders the specialized Forge-Design Tool Execution Card.
func (a *AppUI) renderToolDesignBlock(c *ui.Context, tc theme.ThemeColors, t *agent.ToolExecution, cleanOut string, failed bool) {
	args := parseToolArgs(t.Command, t.ToolName)
	targetPath := args.Path
	if targetPath == "" {
		if a.Workspace != nil && a.Workspace.RootDir != "" {
			targetPath = filepath.Join(a.Workspace.RootDir, "deck.html")
		} else {
			targetPath = "deck.html"
		}
	}
	isMotion := strings.Contains(strings.ToLower(targetPath), "motion") || strings.HasSuffix(strings.ToLower(targetPath), ".mp4")
	modeTag := "PRESENTATION DECK"
	if isMotion {
		modeTag = "MOTION GRAPHICS"
	}

	ui.Column(c).Radius(8).Padding(8, 10).Gap(8).
		Background(tc.SurfaceRaised.Alpha(0.5)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.Accent.Alpha(0.2)).Children(func() {
				ui.Icon(c, icons.TablerPalette).Width(14).Height(14).TextColor(tc.Accent)
			})
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Forge Design Studio").FontSize(12).FontWeight(700).TextColor(tc.Text)
					ui.Box(c).Radius(3).Padding(1, 5).Background(tc.Accent.Alpha(0.15)).Children(func() {
						ui.Text(c, modeTag).FontSize(9).FontWeight(700).TextColor(tc.Accent)
					})
				})
				ui.Text(c, filepath.Base(targetPath)).FontSize(11).Font("monospace").TextColor(tc.TextMuted)
			})
		})

		// Mini preview wireframe/card
		ui.Column(c).Radius(6).Padding(10, 12).Gap(6).Background(tc.Canvas).
			BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "16:9 1080p • Mathematical Typography • Open Design Core").FontSize(10).FontWeight(600).TextColor(tc.Accent)
				ui.Spacer(c)
				ui.Text(c, "GSAP 3.12").FontSize(9).Font("monospace").TextColor(tc.TextMuted)
			})
			ui.Text(c, "Ready for real-time presentation, slide navigation & broadcast export.").FontSize(11).TextColor(tc.TextMuted)
		})

		// Action buttons
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			deckBtn := ui.Row(c.Key("tdb-deck-"+t.ID)).Height(24).Radius(5).Padding(0, 8).Gap(5).AlignItems(ui.Center).
				Background(tc.Accent).Cursor(ui.CursorPointer)
			deckBtn.Children(func() {
				ui.Icon(c, icons.TablerPresentation).Width(12).Height(12).TextColor(ui.Hex("#ffffff"))
				ui.Text(c, "Open in Deck Viewer").FontSize(11).FontWeight(600).TextColor(ui.Hex("#ffffff"))
			})
			if deckBtn.Clicked() {
				a.openDeckViewer(targetPath)
				a.RightDrawerTab = RightTabDeck
				a.ShowRightDrawer = true
			}

			browserBtn := ui.Row(c.Key("tdb-browser-"+t.ID)).Height(24).Radius(5).Padding(0, 8).Gap(5).AlignItems(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
			browserBtn.Children(func() {
				ui.Icon(c, icons.TablerBrowser).Width(12).Height(12).TextColor(tc.Text)
				ui.Text(c, "Browser Preview").FontSize(11).FontWeight(500).TextColor(tc.Text)
			})
			if browserBtn.Clicked() {
				a.RightDrawerTab = RightTabBrowser
				a.ShowRightDrawer = true
			}

			editBtn := ui.Row(c.Key("tdb-edit-"+t.ID)).Height(24).Radius(5).Padding(0, 8).Gap(5).AlignItems(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
			editBtn.Children(func() {
				ui.Icon(c, icons.TablerCode).Width(12).Height(12).TextColor(tc.Text)
				ui.Text(c, "Open in Editor").FontSize(11).FontWeight(500).TextColor(tc.Text)
			})
			if editBtn.Clicked() {
				if a.Editor != nil {
					rel := targetPath
					if a.Workspace != nil && a.Workspace.RootDir != "" {
						if r, err := filepath.Rel(a.Workspace.RootDir, targetPath); err == nil {
							rel = r
						}
					}
					_, _ = a.Editor.OpenFile(targetPath, rel)
					a.Mode = ModeEditor
					a.IsGitClientOpen = false
				}
			}
		})

		if cleanOut != "" {
			renderToolOutputBox(c, tc, cleanOut, failed)
		}
	})
}

// Helper functions for metadata extraction
func extractSkillLabel(command string, toolName string) string {
	var raw map[string]interface{}
	if json.Unmarshal([]byte(command), &raw) == nil {
		for _, k := range []string{"name", "Name", "skill", "Skill", "skill_name", "SkillName"} {
			if v, ok := raw[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	clean := strings.TrimPrefix(toolName, "skill__")
	clean = strings.TrimPrefix(clean, "skill_")
	clean = strings.TrimPrefix(clean, "load_")
	if clean == "" || clean == "skill" {
		return "Skill"
	}
	return clean
}

func extractMCPServerLabel(command string, toolName string) string {
	var raw map[string]interface{}
	if json.Unmarshal([]byte(command), &raw) == nil {
		for _, k := range []string{"server", "Server", "name", "Name", "server_name"} {
			if v, ok := raw[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	clean := strings.TrimPrefix(toolName, "mcp_server_")
	clean = strings.TrimPrefix(clean, "mcp_")
	clean = strings.TrimPrefix(clean, "load_")
	if clean == "" || clean == "mcp" {
		return "MCP Server"
	}
	return clean
}

func extractMCPToolParts(toolName string) (server, tool string) {
	if strings.HasPrefix(toolName, "mcp__") {
		trimmed := strings.TrimPrefix(toolName, "mcp__")
		parts := strings.SplitN(trimmed, "__", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "", trimmed
	}
	if strings.HasPrefix(toolName, "mcp_") {
		trimmed := strings.TrimPrefix(toolName, "mcp_")
		parts := strings.SplitN(trimmed, "_", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "mcp", trimmed
	}
	return "mcp", toolName
}
