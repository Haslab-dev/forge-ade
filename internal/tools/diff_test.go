package tools

import (
	"strings"
	"testing"
)

func TestUnifiedDiffAddRemoveUpdate(t *testing.T) {
	old := "line1\nline2\nline3\nline4\nline5\n"
	// remove line2, add a new line after line4, update line5
	new := "line1\nline3\nline4\ninserted\nline5-changed\n"
	d := unifiedDiff(old, new)
	if d == "" {
		t.Fatal("expected non-empty diff")
	}
	for _, want := range []string{"-line2", "+inserted", "-line5", "+line5-changed", "@@ -1,5 +1,5 @@"} {
		if !strings.Contains(d, want) {
			t.Errorf("diff missing %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, "line1") && !strings.Contains(d, " line1") {
		t.Errorf("unchanged line1 should appear as context, not change:\n%s", d)
	}
}

func TestUnifiedDiffNoChange(t *testing.T) {
	s := "a\nb\nc\n"
	if d := unifiedDiff(s, s); d != "" {
		t.Errorf("expected empty diff, got %q", d)
	}
}

func TestUnifiedDiffNewFile(t *testing.T) {
	d := unifiedDiff("", "a\nb\nc\n")
	for _, want := range []string{"+a", "+b", "+c"} {
		if !strings.Contains(d, want) {
			t.Errorf("new-file diff missing %q:\n%s", want, d)
		}
	}
}

func TestUnifiedDiffTooLarge(t *testing.T) {
	big := strings.Repeat("x\n", 4000)
	d := unifiedDiff(big, big+"y\n")
	if !strings.Contains(d, "too large") {
		t.Errorf("expected too-large fallback, got %d chars", len(d))
	}
}
