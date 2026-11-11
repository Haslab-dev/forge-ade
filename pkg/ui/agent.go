package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/internal/app"
)

// ChatMessage represents a single interaction turn in the AI Agent conversation.
type ChatMessage struct {
	ID        int
	Role      string // "user", "assistant", "system"
	Text      string
	Done      bool
	Timestamp time.Time
}

// AgentState tracks the conversation, streaming responses, and prompt composer.
type AgentState struct {
	mu          sync.Mutex
	backend     *app.App
	Messages    []*ChatMessage
	InputText   string
	ListState   ui.ListState
	MdOpts      markdown.Options
	ActiveModel string
	NextID      int
	IsStreaming bool
	Invalidate  func()
}

// NewAgentState initializes the native agent chat state.
func NewAgentState(backend *app.App, invalidate func()) *AgentState {
	as := &AgentState{
		backend:     backend,
		ActiveModel: "gpt-4o / Claude 3.5 Sonnet",
		Invalidate:  invalidate,
		MdOpts: markdown.Options{
			Reveal: &markdown.Reveal{Unit: markdown.ByWord},
		},
	}

	// Welcome greeting
	as.Messages = append(as.Messages, &ChatMessage{
		ID:        1,
		Role:      "assistant",
		Text:      "### Welcome to ForgeADE Native\n\nI am your native autonomous coding assistant running with **GPU acceleration** and pure Go runtime.\n\nType `/help` to see available slash commands, or enter a prompt to begin pair programming.",
		Done:      true,
		Timestamp: time.Now(),
	})
	as.NextID = 2
	return as
}

// SubmitMessage handles user prompt submissions and slash commands.
func (as *AgentState) SubmitMessage() {
	input := strings.TrimSpace(as.InputText)
	if input == "" || as.IsStreaming {
		return
	}

	as.mu.Lock()
	as.InputText = ""
	userMsg := &ChatMessage{
		ID:        as.NextID,
		Role:      "user",
		Text:      input,
		Done:      true,
		Timestamp: time.Now(),
	}
	as.NextID++
	as.Messages = append(as.Messages, userMsg)
	as.mu.Unlock()

	// Handle Slash Commands
	if strings.HasPrefix(input, "/") {
		as.handleSlashCommand(input)
		return
	}

	// Trigger Assistant Stream
	as.triggerAgentTurn(input)
}

func (as *AgentState) handleSlashCommand(cmd string) {
	parts := strings.Fields(cmd)
	rootCmd := parts[0]

	as.mu.Lock()
	defer as.mu.Unlock()

	var reply string
	switch rootCmd {
	case "/clear":
		as.Messages = as.Messages[:0]
		reply = "Chat history cleared."
	case "/model":
		if as.backend != nil {
			cfg := as.backend.GetLLMConfig()
			reply = fmt.Sprintf("Active Provider: `%s`\nActive Model: `%s`", cfg.ProviderID, cfg.Model)
		} else {
			reply = fmt.Sprintf("Active Model: `%s`", as.ActiveModel)
		}
	case "/help":
		reply = "### Available Slash Commands:\n" +
			"- `/clear` — Clear the current conversation history\n" +
			"- `/model` — Inspect active LLM provider and model profile\n" +
			"- `/status` — View workspace indexing and Git status\n" +
			"- `/vibe` — Activate high-velocity autonomous execution\n" +
			"- `/help` — Display this command reference"
	case "/status":
		reply = "### Workspace Health\n" +
			"- **Engine**: ForgeADE Go Monolith\n" +
			"- **UI Runtime**: MyGo Native GPU (`CGO_ENABLED=0`)\n" +
			"- **Terminal**: Ghostty `libghostty-vt` embedded\n" +
			"- **Editor**: Zed-style virtualized buffer with live diff gutters"
	default:
		reply = fmt.Sprintf("Unknown command `%s`. Type `/help` for available commands.", rootCmd)
	}

	as.Messages = append(as.Messages, &ChatMessage{
		ID:        as.NextID,
		Role:      "assistant",
		Text:      reply,
		Done:      true,
		Timestamp: time.Now(),
	})
	as.NextID++
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

func (as *AgentState) triggerAgentTurn(prompt string) {
	as.mu.Lock()
	as.IsStreaming = true
	assistantMsg := &ChatMessage{
		ID:        as.NextID,
		Role:      "assistant",
		Text:      "",
		Done:      false,
		Timestamp: time.Now(),
	}
	as.NextID++
	as.Messages = append(as.Messages, assistantMsg)
	as.mu.Unlock()

	go func() {
		defer func() {
			as.mu.Lock()
			assistantMsg.Done = true
			as.IsStreaming = false
			as.mu.Unlock()
			if as.Invalidate != nil {
				as.Invalidate()
			}
		}()

		// Real-time token streaming demonstration
		chunks := []string{
			"Analyzing your request...\n\n",
			"Here is an overview of the implementation:\n\n",
			"```go\n",
			"// Native zero-copy UI bridge\n",
			"func StreamResponse(token string) {\n",
			"    buffer.Append(token)\n",
			"    window.Invalidate()\n",
			"}\n",
			"```\n\n",
			"The task has been decomposed and verified natively without browser webview latency.",
		}

		for _, chunk := range chunks {
			time.Sleep(120 * time.Millisecond)
			as.mu.Lock()
			assistantMsg.Text += chunk
			as.mu.Unlock()
			if as.Invalidate != nil {
				as.Invalidate()
			}
		}
	}()
}

// RenderChat renders the conversation view, message bubbles, and the prompt input.
func (as *AgentState) RenderChat(c *ui.Context) {
	t := c.Theme()

	as.ListState.Key = func(i int) any { return as.Messages[i].ID }
	as.ListState.FollowEnd = true
	as.ListState.FollowAnimation = &ui.Smooth

	ui.Column(c).Fill().Children(func() {
		// Header info
		ui.Row(c).Gap(12).Padding(8, 16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Agent Assistant").Bold()
			ui.Text(c, as.ActiveModel).FontSize(12).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if as.IsStreaming {
				ui.Text(c, "Streaming tokens…").FontSize(12).TextColor(t.Accent)
			}
		})

		// Message List
		ui.List(c, &as.ListState, len(as.Messages), func(i int) {
			m := as.Messages[i]
			if m.Role == "user" {
				ui.Row(c).Padding(4, 0).Children(func() {
					ui.Spacer(c)
					ui.Text(c, m.Text).Selectable().Padding(8, 14).Radius(12).Background(t.Surface).MaxWidth(560)
				})
			} else {
				ui.Column(c).Padding(4, 0).MaxWidth(800).Children(func() {
					if m.Done {
						markdown.View(c, m.Text, &as.MdOpts)
					} else {
						markdown.Stream(c, m.Text, &as.MdOpts)
					}
				})
			}
		}).Grow(1).Padding(16, 20).Gap(12).Justify(ui.End)

		// Input Composer Bar
		ui.Row(c).Gap(8).Padding(12, 16).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
			in := ui.TextInput(c, &as.InputText).Placeholder("Ask agent, enter slash command (/help)...").Grow(1).AutoFocus()
			if in.Submitted() && !as.IsStreaming {
				as.SubmitMessage()
			}

			if as.IsStreaming {
				if ui.Button(c, "Stop").Clicked() {
					as.mu.Lock()
					as.IsStreaming = false
					if len(as.Messages) > 0 {
						as.Messages[len(as.Messages)-1].Done = true
					}
					as.mu.Unlock()
				}
			} else {
				sendBtn := ui.PrimaryButton(c, "Send")
				if strings.TrimSpace(as.InputText) == "" {
					sendBtn.Disabled(true)
				}
				if sendBtn.Clicked() {
					as.SubmitMessage()
				}
			}
		})
	})
}
