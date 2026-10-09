package conffmt

import (
	"strings"
	"testing"
)

func TestFormatJSON(t *testing.T) {
	out, err := Format("settings.json", `{"b":1,"a":[1,2]}`)
	if err != nil {
		t.Fatal(err)
	}
	// Key order preserved (no re-sorting — config diffs stay clean).
	want := "{\n  \"b\": 1,\n  \"a\": [\n    1,\n    2\n  ]\n}\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
	// Invalid JSON must error, never mangle.
	if _, err := Format("settings.json", `{"b":`); err == nil {
		t.Error("expected error for invalid JSON")
	}
	// Integer precision preserved.
	out, err = Format("x.json", `{"n":1234567890123456789}`)
	if err != nil || !strings.Contains(out, "1234567890123456789") {
		t.Errorf("big int lost: %q err=%v", out, err)
	}
}

func TestFormatJSONL(t *testing.T) {
	in := "{\"b\":1,\"a\":2}\nnot json at all\n\n{\"c\":3}\n"
	out, err := Format("x.jsonl", in)
	if err != nil {
		t.Fatal(err)
	}
	// Key order preserved; the unparseable line and the blank line pass
	// through untouched.
	want := "{\"b\":1,\"a\":2}\nnot json at all\n\n{\"c\":3}\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestTidyTOML(t *testing.T) {
	in := "# comment kept\n\n\n[mcp_servers.context7]\ncommand   =  'npx'\nargs = [\"-y\",\"pkg\"]\n\n\n[model]\nmodel = \"gpt-5\"  # trailing comment\n"
	out := TidyTOML(in)
	want := "# comment kept\n\n[mcp_servers.context7]\ncommand = 'npx'\nargs = [\"-y\",\"pkg\"]\n\n[model]\nmodel = \"gpt-5\"  # trailing comment\n"
	if out != want {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
	// Idempotent.
	if again := TidyTOML(out); again != out {
		t.Errorf("not idempotent:\n%q", again)
	}
}

func TestTidyTOMLMultilineArrayAndString(t *testing.T) {
	in := "args = [\n  \"-y\",\n  \"pkg\",\n]\nprompt = \"\"\"\nline one   = kept\nline two\n\"\"\"\npath = 'C:\\dir'\n"
	out := TidyTOML(in)
	// Array items keep their inner spacing; multi-line string content untouched;
	// the assignment AFTER the multi-line string still normalizes.
	want := "args = [\n  \"-y\",\n  \"pkg\",\n]\nprompt = \"\"\"\nline one   = kept\nline two\n\"\"\"\npath = 'C:\\dir'\n"
	if out != want {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
	st := &tomlState{}
	scanTOMLLine("a = \"\"\"", st)
	if !st.multiline || st.strDelim != "\"\"\"" {
		t.Errorf("multiline string not detected: %+v", st)
	}
}

func TestFormatUnknownExt(t *testing.T) {
	if _, err := Format("x.yaml", "a: 1"); err == nil {
		t.Error("expected error for unsupported extension")
	}
}
