package git

import (
	"testing"
	"time"
)

func TestParseGraphLine(t *testing.T) {
	cases := []struct {
		name       string
		line       string
		wantHash   string
		wantMsg    string
		wantDeco   string
		wantPrefix string
		wantOK     bool
	}{
		{
			name:       "plain line",
			line:       "* GITCOMMIT\x00abc1234\x00def5678\x00Alice\x00a@x.io\x001700000000\x00 (main)\x00fix bug",
			wantHash:   "abc1234",
			wantMsg:    "fix bug",
			wantDeco:   " (main)",
			wantPrefix: "* ",
			wantOK:     true,
		},
		{
			name:       "subject containing pipes is preserved",
			line:       "* GITCOMMIT\x00abc1234\x00\x00Bob\x00b@x.io\x001700000001\x00\x00feat(api): a|b|c endpoint",
			wantHash:   "abc1234",
			wantMsg:    "feat(api): a|b|c endpoint",
			wantPrefix: "* ",
			wantOK:     true,
		},
		{
			name:   "no marker",
			line:   "* some random graph line",
			wantOK: false,
		},
		{
			name:   "too few fields",
			line:   "* GITCOMMIT\x00abc1234\x00\x00Alice",
			wantOK: false,
		},
		{
			name:   "bad timestamp",
			line:   "* GITCOMMIT\x00abc1234\x00\x00Alice\x00a@x.io\x00notanumber\x00\x00msg",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseGraphLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v, want %v (got %+v)", ok, tc.wantOK, got)
			}
			if !tc.wantOK {
				return
			}
			if got.Hash != tc.wantHash || got.Message != tc.wantMsg || got.Decorations != tc.wantDeco || got.GraphPrefix != tc.wantPrefix {
				t.Fatalf("parsed %+v, want hash=%q msg=%q deco=%q prefix=%q",
					got, tc.wantHash, tc.wantMsg, tc.wantDeco, tc.wantPrefix)
			}
			if got.ShortHash != tc.wantHash[:7] {
				t.Errorf("short hash %q, want %q", got.ShortHash, tc.wantHash[:7])
			}
			if got.Timestamp.Unix() != 1700000000 && tc.name == "plain line" {
				t.Errorf("timestamp %v, want epoch 1700000000", got.Timestamp)
			}
		})
	}
}

// TestParseGraphLineTimestampIsUTCStable pins the epoch conversion.
func TestParseGraphLineTimestampIsUTCStable(t *testing.T) {
	node, ok := parseGraphLine("GITCOMMIT\x00abc1234\x00p1 p2\x00A\x00a@x\x001700000000\x00\x00m")
	if !ok {
		t.Fatal("expected ok")
	}
	if node.Timestamp != time.Unix(1700000000, 0) {
		t.Errorf("timestamp mismatch: %v", node.Timestamp)
	}
	if len(node.Parents) != 2 || node.Parents[0] != "p1" {
		t.Errorf("parents %v, want [p1 p2]", node.Parents)
	}
}
