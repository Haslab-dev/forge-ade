package index

import "testing"

func TestMatchModes(t *testing.T) {
	cases := []struct {
		name, query string
		mode        matchMode
		want        bool
	}{
		{"FileBrowser", "FileBrowser", matchExact, true},
		{"FileBrowser", "File", matchExact, false},
		{"FileBrowser", "File", matchPrefix, true},
		{"FileBrowser", "FB", matchCamel, true},
		{"fileBrowser", "FB", matchCamel, true},
		{"readFile", "RF", matchCamel, true},
		{"APIResponse", "APIR", matchCamel, true},
		{"parseFile", "PF", matchCamel, true},
		{"read_file", "RF", matchCamel, true},
		{"FileBrowser", "FileBrowsr", matchFuzzy, true},
		{"FileBrowser", "XFileBrowsr", matchFuzzy, false},
		{"getUserById", "gUID", matchFuzzy, true},
	}
	for _, c := range cases {
		if got := match(c.name, c.query, c.mode); got != c.want {
			t.Errorf("match(%q, %q, mode=%d) = %v, want %v", c.name, c.query, c.mode, got, c.want)
		}
	}
}

func TestCamelInitials(t *testing.T) {
	cases := map[string]string{
		"FileBrowser": "FB",
		"parseFile":   "PF",
		"APIResponse": "APIR",
		"read_file":   "RF",
		"simple":      "S",
		"URLParser":   "URLP",
	}
	for in, want := range cases {
		if got := camelInitials(in); got != want {
			t.Errorf("camelInitials(%q) = %q, want %q", in, got, want)
		}
	}
}
