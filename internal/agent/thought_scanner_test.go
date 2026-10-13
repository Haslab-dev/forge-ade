package agent

import (
	"testing"
)

func TestThoughtStreamScanner(t *testing.T) {
	tests := []struct {
		name         string
		chunks       []string
		wantText     string
		wantThinking string
	}{
		{
			name:         "Single chunk with think tag",
			chunks:       []string{"<think>Reasoning carefully here</think>This is the final response."},
			wantText:     "This is the final response.",
			wantThinking: "Reasoning carefully here",
		},
		{
			name: "Split tag across chunks",
			chunks: []string{
				"<th",
				"ink>Part 1 of thought. ",
				"Part 2 of thought.</thi",
				"nk>Now the visible text.",
			},
			wantText:     "Now the visible text.",
			wantThinking: "Part 1 of thought. Part 2 of thought.",
		},
		{
			name: "Thought tag alternate names",
			chunks: []string{
				"<thought>Thinking with thought tag</thought>Text 1.",
				"<thinking>Thinking with thinking tag</thinking>Text 2.",
			},
			wantText:     "Text 1.Text 2.",
			wantThinking: "Thinking with thought tagThinking with thinking tag",
		},
		{
			name:         "Preserves HTML tags",
			chunks:       []string{"<div>Hello <b>world</b></div>"},
			wantText:     "<div>Hello <b>world</b></div>",
			wantThinking: "",
		},
		{
			name:         "No thought tags",
			chunks:       []string{"Just standard text response without any thinking tags."},
			wantText:     "Just standard text response without any thinking tags.",
			wantThinking: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newThoughtStreamScanner()
			var gotText, gotThinking string

			for _, c := range tc.chunks {
				events := s.feed(c)
				for _, ev := range events {
					if ev.kind == "text" {
						gotText += ev.text
					} else if ev.kind == "thinking" {
						gotThinking += ev.text
					}
				}
			}

			for _, ev := range s.flush() {
				if ev.kind == "text" {
					gotText += ev.text
				} else if ev.kind == "thinking" {
					gotThinking += ev.text
				}
			}

			if gotText != tc.wantText {
				t.Errorf("gotText = %q, want %q", gotText, tc.wantText)
			}
			if gotThinking != tc.wantThinking {
				t.Errorf("gotThinking = %q, want %q", gotThinking, tc.wantThinking)
			}
		})
	}
}
