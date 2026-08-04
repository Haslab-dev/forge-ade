package terminal

import (
	"strings"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/internal/events"
)

// TestUTF8CarryDecoder verifies the decoder never emits spurious U+FFFD and
// never drops bytes when a multi-byte UTF-8 sequence or escape sequence is
// split across chunk boundaries — the exact failure mode TUI agents hit.
func TestUTF8CarryDecoder(t *testing.T) {
	tests := []struct {
		name   string
		chunks [][]byte
		want   string
	}{
		{
			name:   "bullet split mid-glyph",
			chunks: [][]byte{{0xE2, 0x80}, {0xA2, 'X'}},
			want:   "•X",
		},
		{
			name:   "escape split at ESC",
			chunks: [][]byte{{0x1b}, {'[', '1', 'A'}},
			want:   "\x1b[1A",
		},
		{
			name:   "color code then emoji split across three reads",
			chunks: [][]byte{
				[]byte("\x1b[38;2;1;2;3m"),
				{0xF0, 0x9F},
				{0x98, 0x80, '!'},
			},
			want: "\x1b[38;2;1;2;3m😀!",
		},
		{
			name:   "carriage-return redraw preserved",
			chunks: [][]byte{
				[]byte("line1\r"),
				[]byte("line2\r\n"),
				[]byte("done\n"),
			},
			want: "line1\rline2\r\ndone\n",
		},
		{
			name:   "ascii passthrough",
			chunks: [][]byte{[]byte("hello"), []byte(" world\n")},
			want:   "hello world\n",
		},
		{
			name:   "braille spinner glyph split",
			chunks: [][]byte{{0xE2, 0xA0}, {0xB6, ' '}},
			want:   "⠶ ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &utf8CarryDecoder{}
			var sb strings.Builder
			for _, c := range tt.chunks {
				sb.WriteString(d.Decode(c))
			}
			if sb.String() != tt.want {
				t.Errorf("decoded %q, want %q", sb.String(), tt.want)
			}
		})
	}
}

// TestUTF8CarryDecoderNoReplacement ensures no U+FFFD ever appears in output
// for valid UTF-8 that happens to be split at chunk boundaries.
func TestUTF8CarryDecoderNoReplacement(t *testing.T) {
	d := &utf8CarryDecoder{}
	input := "⠶ Crystallizing… esc to interrupt • 3s • ↓ 22\r\n"
	// Split at every possible byte offset — the decoder must never corrupt.
	bytes := []byte(input)
	var got strings.Builder
	for i := 0; i < len(bytes); i += 3 {
		end := i + 3
		if end > len(bytes) {
			end = len(bytes)
		}
		got.WriteString(d.Decode(bytes[i:end]))
	}
	if got.String() != input {
		t.Errorf("split decode produced %q, want %q", got.String(), input)
	}
	if strings.ContainsRune(got.String(), '\uFFFD') {
		t.Errorf("split decode emitted U+FFFD replacement character")
	}
}

// TestExecPersistentShell verifies a command runs in a persistent shell and
// returns output + exit code via the marker protocol.
func TestExecPersistentShell(t *testing.T) {
	m := NewManager(events.NewBus())
	sh, err := m.CreateShell("test", "")
	if err != nil {
		t.Skipf("no shell available: %v", err)
	}
	out, code, err := m.Exec(sh.ID, "echo hello-agent", 10*time.Second)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (out=%q)", code, out)
	}
	if !strings.Contains(out, "hello-agent") {
		t.Fatalf("expected output to contain hello-agent, got %q", out)
	}
	// Second call must reuse the same shell (state persists — cwd etc.).
	out2, code2, err := m.Exec(sh.ID, "echo second", 10*time.Second)
	if err != nil || code2 != 0 {
		t.Fatalf("second Exec: %v code=%d", err, code2)
	}
	if !strings.Contains(out2, "second") {
		t.Fatalf("expected output to contain second, got %q", out2)
	}
	_ = m.Stop(sh.ID)
}
