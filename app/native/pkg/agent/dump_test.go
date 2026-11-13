package agent

import "testing"

func TestCleanHarnessDumps(t *testing.T) {
	dump := `{"\n  \"active_skills\": [{\n    \"description\": \"How to develop plugins\",\n    \"name\": \"cordis-plugin-dev\",\n    \"plugin_id\": \"dsh-creator-mode\"\n  }],\n  \"active_tools\": [{\"name\": \"find\"}],\n  \"nested\": {\"plugin_id\": \"x\"}\n}`
	text := "Real answer here.\n" + dump + "\nTrailing text."
	got := cleanHarnessDumps(text)
	if got != "Real answer here.\nTrailing text." {
		t.Fatalf("cleanHarnessDumps = %q", got)
	}
	if cleanHarnessDumps("normal text {not a dump}") != "normal text {not a dump}" {
		t.Fatal("removed non-dump braces")
	}
}

func TestIsHarnessDumpDelta(t *testing.T) {
	d := `{"\n "active_skills": [`
	if !isHarnessDumpDelta(d, "") {
		t.Fatal("dump delta not detected")
	}
	if isHarnessDumpDelta("Hello, how can I help?", "") {
		t.Fatal("normal reply flagged")
	}
	if isHarnessDumpDelta(d, "A real reply already started here so this JSON is quoted content worth keeping.") {
		t.Fatal("mid-reply JSON flagged")
	}
}
