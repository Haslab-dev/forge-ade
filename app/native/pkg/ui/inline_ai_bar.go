package ui

// inline_ai_bar.go ports the webview's InlineAiBar: ⌘I opens a floating
// bar over the editor with quick chips, a generating state, and the
// proposed-changes Accept/Reject/Retry resolution. Also hosts Format
// Document (core/editor tooling) and the image / markdown preview panes.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	coreeditor "github.com/hasdev/forge-ade/core/editor"
)

// llmMessagesFrom builds the chat messages for the inline AI call.
func llmMessagesFrom(system, user string) []llm.LLMMessage {
	return []llm.LLMMessage{
		{Role: llm.RoleSystem, Content: system},
		{Role: llm.RoleUser, Content: user},
	}
}

// newEditorTooling builds the core editor tooling (syntax/format).
func newEditorTooling(root string) *coreeditor.Tooling {
	return coreeditor.NewTooling(root)
}

// openInlineAI arms the bar over the active document's selection.
func (a *AppUI) openInlineAI() {
	if a.Editor == nil || a.InlineAI == nil {
		return
	}
	doc := a.Editor.ActiveDoc()
	if doc == nil {
		return
	}
	sel := doc.Editor.Selection()
	start, end := sel.Anchor, sel.Head
	if start > end {
		start, end = end, start
	}
	a.InlineAI.OpenForSelection(
		doc.FullPath, doc.RelativePath,
		start, end,
		func() string { return doc.Editor.Buffer.String() },
		func(s, e int, text string) {
			buf := doc.Editor.Buffer
			if s > buf.Len() {
				s = buf.Len()
			}
			if e > buf.Len() {
				e = buf.Len()
			}
			buf.Edit(mygoeditor.Edit{Start: s, End: e, Text: text})
		},
	)
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// renderInlineAiBar draws the floating bar; call inside the editor pane
// container so it overlays the buffer.
func (a *AppUI) renderInlineAiBar(c *ui.Context, tc theme.ThemeColors) {
	ia := a.InlineAI
	if ia == nil || !ia.Open {
		return
	}

	bar := ui.Column(c).Absolute().Top(38).Left(40).Width(480).Radius(14).
		Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Focus.Alpha(0.5)).
		Shadow(0, 16, 40, -16, ui.RGBA(0, 0, 0, 0.5)).Padding(10).Gap(8).Clip()
	bar.Children(func() {
		// Header: icon + title + close
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.TablerSparkles).Width(14).Height(14).TextColor(tc.Accent)
			title := "Edit with AI"
			if ia.HasProposal {
				title = "Proposed changes"
			}
			ui.Text(c, title).FontSize(12).FontWeight(600).Grow(1)
			ui.Text(c, ia.RelPath).FontSize(10).TextColor(tc.TextMuted)
			if IconButton(c, tc, icons.TablerX, 12).Clicked() {
				ia.Close()
			}
		})

		// Generating state: spinner + elapsed + stop.
		if ia.Generating {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Size(13, 13).Label("Generating")
				ui.Text(c, fmt.Sprintf("%.0fs", time.Since(ia.StartedAt).Seconds())).FontSize(11).TextColor(tc.TextMuted)
				ui.Box(c).Grow(1)
				stop := composerControlChip(c, tc, icons.TablerSquare, "Stop", "", true)
				if stop.Clicked() {
					if a.InlineAICancel != nil {
						close(a.InlineAICancel)
					}
				}
			})
		}

		// Error line.
		if ia.Error != "" {
			ui.Text(c, ia.Error).FontSize(11).TextColor(tc.Error)
		}

		// Prompt input (hidden once a proposal shows).
		if !ia.HasProposal {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &ia.Prompt).Placeholder("Describe the edit… (⏎ to generate)").Grow(1)
				if in.Submitted() && !ia.Generating && strings.TrimSpace(ia.Prompt) != "" {
					a.startInlineAIGenerate()
				}
			})
			// Quick chips, InlineAiBar quick actions parity.
			ui.Row(c).Gap(4).Wrap().Children(func() {
				for _, chip := range []string{"Add error handling", "Add docstring", "Refactor for clarity", "Write tests"} {
					chip := chip
					b := ui.Row(c.Key("iaq-"+chip)).Height(22).Radius(999).Padding(0, 9).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					b.Children(func() {
						ui.Text(c, chip).FontSize(10).TextColor(tc.TextMuted)
					})
					if b.Clicked() {
						ia.Prompt = chip
						a.startInlineAIGenerate()
					}
				}
			})
		}

		// Proposal preview + resolution buttons.
		if ia.HasProposal {
			preview := ia.Proposed
			lines := strings.Split(preview, "\n")
			if len(lines) > 10 {
				lines = lines[:10]
				preview = strings.Join(lines, "\n") + "\n…"
			}
			box := ui.Column(c).Radius(8).Background(tc.CodeBackground).Padding(8).Clip()
			box.Children(func() {
				ui.Text(c, preview).FontSize(11).Font("Geist Mono").Selectable()
			})
			ui.Row(c).Gap(6).Justify(ui.End).AlignItems(ui.Center).Children(func() {
				retry := composerControlChip(c, tc, icons.TablerRefresh, "Retry", "", false)
				if retry.Clicked() && !ia.Generating {
					a.startInlineAIGenerate()
				}
				reject := composerControlChip(c, tc, icons.TablerX, "Reject", "", true)
				if reject.Clicked() {
					ia.Reject()
				}
				accept := ui.Row(c).Height(28).Radius(8).Padding(0, 14).Gap(5).AlignItems(ui.Center).Background(tc.Success)
				accept.Children(func() {
					ui.Icon(c, icons.TablerCheck).Width(13).Height(13).TextColor(tc.SuccessForeground)
					ui.Text(c, "Accept").FontSize(12).FontWeight(600).TextColor(tc.SuccessForeground)
				})
				if accept.Clicked() {
					ia.Accept()
				}
			})
		}
	})
}

// startInlineAIGenerate launches the model call for the current prompt.
func (a *AppUI) startInlineAIGenerate() {
	if a.Engine == nil || a.Engine.LLMClient == nil || a.InlineAI == nil {
		return
	}
	// Close any previous cancel channel and open a fresh one.
	if a.InlineAICancel != nil {
		close(a.InlineAICancel)
	}
	cancelCh := make(chan struct{})
	a.InlineAICancel = cancelCh

	llmClient := a.Engine.LLMClient
	go func() {
		a.InlineAI.Generate(func(ctx context.Context, system, user string, onDelta func(string)) error {
			_, err := llmClient.ChatWithStream(ctx, llmMessagesFrom(system, user), nil, func(deltaContent, deltaReasoning string) {
				if deltaContent != "" {
					onDelta(deltaContent)
				}
			})
			return err
		}, cancelCh)
		if a.Window != nil {
			a.Window.Invalidate()
		}
	}()
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// formatActiveDocument formats the open file via the core editor tooling
// (prettier-backed), matching the webview's Format Document action.
func (a *AppUI) formatActiveDocument(c *ui.Context) {
	doc := a.Editor.ActiveDoc()
	if doc == nil {
		return
	}
	tooling := newEditorTooling(a.Workspace.RootDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	text := doc.Editor.Buffer.String()
	formatted, err := tooling.FormatCode(ctx, doc.FullPath, text)
	if err != nil || formatted == "" || formatted == text {
		if err != nil {
			c.AddToast(ui.Toast{Title: "Format failed", Description: err.Error(), Type: "error"})
		}
		return
	}
	buf := doc.Editor.Buffer
	buf.Edit(mygoeditor.Edit{Start: 0, End: buf.Len(), Text: formatted})
	c.AddToast(ui.Toast{Title: "Formatted", Description: doc.RelativePath, Type: "success"})
}

// openSidePreviewFile renders image/markdown previews in the drawer Files
// pane (DeckMotionViewer auto-detect sends decks to the browser tab).
func (a *AppUI) openSidePreviewFile(c *ui.Context, relPath string) {
	ext := strings.ToLower(strings.TrimPrefix(relPath, "."))
	_ = ext
	switch {
	case strings.HasSuffix(relPath, ".png"), strings.HasSuffix(relPath, ".jpg"),
		strings.HasSuffix(relPath, ".jpeg"), strings.HasSuffix(relPath, ".gif"),
		strings.HasSuffix(relPath, ".webp"), strings.HasSuffix(relPath, ".svg"):
		a.SideFilePreviewImage = relPath
		a.SideFilePreviewMd = ""
		a.openSideFile(relPath)
	case strings.HasSuffix(relPath, ".md"), strings.HasSuffix(relPath, ".mdx"):
		a.SideFilePreviewImage = ""
		a.SideFilePreviewMd = relPath
		a.openSideFile(relPath)
	case strings.Contains(strings.ToLower(relPath), "deck"), strings.Contains(strings.ToLower(relPath), "motion"):
		// Deck/motion HTML files open in the browser viewer tab.
		full := a.Workspace.ResolveFullPath(relPath)
		a.openBrowserTab("file://" + full)
	default:
		a.openSideFile(relPath)
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// renderSideFilePreviewBody renders the special preview modes of the side
// file pane: images via ui.Image and markdown via the themed renderer.
func (a *AppUI) renderSideFilePreviewBody(c *ui.Context, tc theme.ThemeColors, fallback func()) {
	if a.SideFilePreviewImage != "" {
		full := a.Workspace.ResolveFullPath(a.SideFilePreviewImage)
		if data, err := os.ReadFile(full); err == nil {
			if src, derr := ui.DecodeImage(data); derr == nil {
				ui.Box(c).Grow(1).Center().Children(func() {
					ui.Image(c, src).MaxWidth(420)
				})
				return
			}
		}
		fallback()
		return
	}
	if a.SideFilePreviewMd != "" {
		full := a.Workspace.ResolveFullPath(a.SideFilePreviewMd)
		if data, err := os.ReadFile(full); err == nil {
			scroll := ui.Scroll(c)
			scroll.Children(func() {
				ui.Column(c).Padding(14).Children(func() {
					markdown.View(c, string(data), &a.Agent.MdOpts)
				})
			}).Grow(1)
			return
		}
		fallback()
		return
	}
	fallback()
}
