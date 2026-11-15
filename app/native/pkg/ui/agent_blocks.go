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

// renderJSONResponseCard draws a dedicated JSON response card when an assistant
// message returns pure JSON data.
func (a *AppUI) renderJSONResponseCard(c *ui.Context, tc theme.ThemeColors, m *agent.ChatMessage, rawJSON string) {
	var pretty bytes.Buffer
	formatted := rawJSON
	if json.Indent(&pretty, []byte(rawJSON), "", "  ") == nil {
		formatted = pretty.String()
	}
	lineCount := strings.Count(formatted, "\n") + 1

	card := ui.Column(c.Key("json-resp-" + fmt.Sprint(m.ID))).Radius(8).Background(tc.Canvas).
		BorderWidth(1).BorderColor(ui.Hex("#eab308").Alpha(0.35)).Clip()
	card.Children(func() {
		header := ui.Row(c).Height(30).Padding(0, 10).AlignItems(ui.Center).
			Background(tc.SurfaceRaised.Alpha(0.6)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5))
		header.Children(func() {
			ui.Box(c).Radius(4).Padding(1, 6).Background(ui.Hex("#eab308").Alpha(0.15)).
				BorderWidth(1).BorderColor(ui.Hex("#eab308").Alpha(0.35)).Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerBraces).Width(12).Height(12).TextColor(ui.Hex("#eab308"))
					ui.Text(c, "JSON RESPONSE").FontSize(10).Font("monospace").FontWeight(700).TextColor(ui.Hex("#eab308"))
				})
			})
			ui.Text(c, fmt.Sprintf("%d lines", lineCount)).FontSize(10).Font("monospace").TextColor(tc.TextMuted).Padding(0, 0, 0, 8)
			ui.Spacer(c)
			copyBtn := ui.Row(c.Key("json-resp-copy-"+fmt.Sprint(m.ID))).Height(20).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).Cursor(ui.CursorPointer)
			copyBtn.Children(func() {
				ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "Copy JSON").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
			})
			if copyBtn.Clicked() {
				mygo.Clipboard.WriteText(formatted)
				c.AddToast(ui.Toast{Title: "Copied JSON Response", Type: "success"})
			}
		})
		ui.Column(c).Padding(10, 12).Children(func() {
			ui.Text(c, formatted).FontSize(12).Font("monospace").TextColor(tc.Text).Selectable()
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
