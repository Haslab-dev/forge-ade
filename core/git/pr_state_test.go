package git

import "testing"

func TestGitlabStateFromText(t *testing.T) {
	cases := map[string]string{
		"title: Fix bug\nstate: opened\nauthor: me": "OPEN",
		"title: Fix bug\nState: Open\n":             "OPEN",
		"  state:   merged  \n":                     "MERGED",
		"state: closed":                             "CLOSED",
		"State: CLOSED\n":                           "CLOSED",
		"no state line here\njust body text":        "UNKNOWN",
		"state: something-else":                     "UNKNOWN",
		"states: opened":                            "UNKNOWN", // prefix must be exactly "state:"
		"restype: opened\nstate:\tmerged":           "MERGED",
	}

	for text, want := range cases {
		if got := gitlabStateFromText(text); got != want {
			t.Errorf("gitlabStateFromText(%q) = %q, want %q", text, got, want)
		}
	}
}
