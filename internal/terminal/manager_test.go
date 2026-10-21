package terminal

import (
	"strings"
	"testing"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
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
			name: "color code then emoji split across three reads",
			chunks: [][]byte{
				[]byte("\x1b[38;2;1;2;3m"),
				{0xF0, 0x9F},
				{0x98, 0x80, '!'},
			},
			want: "\x1b[38;2;1;2;3m😀!",
		},
		{
			name: "carriage-return redraw preserved",
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
	defer m.Stop(sh.ID)
	time.Sleep(1 * time.Second)
	outInit, codeInit, errInit := m.Exec(sh.ID, "stty -echo 2>/dev/null; unsetopt zle 2>/dev/null; unsetopt prompt_cr prompt_sp 2>/dev/null; precmd() { :; }; chpwd() { :; }; PROMPT=\"\"; PS1=\"\"; RPS1=\"\"; RPROMPT=\"\"", 10*time.Second)
	t.Logf("INIT: err=%v code=%d out=%q", errInit, codeInit, outInit)
	out, code, err := m.Exec(sh.ID, "echo hello-agent", 10*time.Second)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	t.Logf("FIRST OUT: %q", out)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (out=%q)", code, out)
	}
	if !strings.Contains(out, "hello-agent") {
		t.Fatalf("expected output to contain hello-agent, got %q", out)
	}
	userCmd := `code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://localhost:59971/); echo "HTTP $code"; lsof -nP -iTCP:59971 -sTCP:LISTEN`
	outUser, codeUser, errUser := m.Exec(sh.ID, userCmd, 10*time.Second)
	if errUser != nil {
		t.Fatalf("userCmd Exec error: %v", errUser)
	}
	if !strings.Contains(outUser, "HTTP 000") {
		t.Fatalf("expected output to contain HTTP 000, got %q", outUser)
	}
	if codeUser != 1 {
		t.Fatalf("expected exit code 1 from lsof failing, got %d", codeUser)
	}
	// Second command: check persistence and cleanliness
	out2, code2, err2 := m.Exec(sh.ID, "echo second-agent", 10*time.Second)
	if err2 != nil || code2 != 0 || strings.TrimSpace(out2) != "second-agent" {
		t.Fatalf("second Exec failed: err=%v code=%d out=%q", err2, code2, out2)
	}
	_ = m.Stop(sh.ID)
}

func TestInteractiveBracketedPaste(t *testing.T) {
	m := NewManager(events.NewBus())
	sh, err := m.CreateShell("test", "")
	if err != nil {
		t.Skipf("no shell: %v", err)
	}
	defer m.Stop(sh.ID)
	time.Sleep(500 * time.Millisecond)
	// Write bracketed paste text with a newline
	_, err = m.Write(sh.ID, []byte("\x1b[200~echo multiline1\necho multiline2\x1b[201~"))
	if err != nil {
		t.Fatalf("Write bracketed paste error: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	out := sh.snapshotOutput()
	if !strings.Contains(out, "multiline1") {
		t.Fatalf("expected output to contain multiline1, got %q", out)
	}
}

func TestStopByFolder(t *testing.T) {
	m := NewManager(events.NewBus())

	s1 := &Session{ID: "sess-local", Folder: "/Users/dev/project"}
	s2 := &Session{ID: "sess-local-sub", Folder: "/Users/dev/project/sub"}
	s3 := &Session{ID: "sess-ssh", Folder: "ssh://conn-1/var/www"}
	s4 := &Session{ID: "sess-other", Folder: "/Users/dev/other"}
	s5 := &Session{ID: "sess-isolated", Folder: "/Users/dev/project", Isolated: true}

	m.mu.Lock()
	m.sessions[s1.ID] = s1
	m.sessions[s2.ID] = s2
	m.sessions[s3.ID] = s3
	m.sessions[s4.ID] = s4
	m.sessions[s5.ID] = s5
	m.mu.Unlock()

	// Stopping local project should NOT stop remote ssh or other or isolated session
	m.StopByFolder([]string{"/Users/dev/project"})

	m.mu.RLock()
	_, s1Exists := m.sessions[s1.ID]
	_, s2Exists := m.sessions[s2.ID]
	_, s3Exists := m.sessions[s3.ID]
	_, s4Exists := m.sessions[s4.ID]
	_, s5Exists := m.sessions[s5.ID]
	m.mu.RUnlock()

	if s1Exists || s2Exists {
		t.Errorf("expected local sessions to be stopped")
	}
	if !s3Exists {
		t.Errorf("expected ssh session to remain open when closing local workspace")
	}
	if !s4Exists {
		t.Errorf("expected other session to remain open")
	}
	if !s5Exists {
		t.Errorf("expected isolated session to remain open when closing local workspace")
	}

	// Now stop remote ssh folder
	m.StopByFolder([]string{"ssh://conn-1/var/www/"})

	m.mu.RLock()
	_, s3ExistsAfter := m.sessions[s3.ID]
	m.mu.RUnlock()

	if s3ExistsAfter {
		t.Errorf("expected ssh session to be stopped when closing remote workspace")
	}
}
