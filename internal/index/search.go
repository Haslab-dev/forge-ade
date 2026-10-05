package index

import "strings"

// Search mode, ordered by precision (RFC §15).
type matchMode int

const (
	matchExact matchMode = iota
	matchPrefix
	matchCamel
	matchFuzzy
)

// match reports whether name matches query under mode. Prefix, camel and
// fuzzy matches are case-insensitive.
func match(name, query string, mode matchMode) bool {
	if query == "" {
		return true
	}
	lname, lquery := strings.ToLower(name), strings.ToLower(query)
	switch mode {
	case matchExact:
		return name == query
	case matchPrefix:
		return strings.HasPrefix(lname, lquery)
	case matchCamel:
		return fuzzy(strings.ToLower(camelInitials(name)), lquery)
	default:
		return fuzzy(lname, lquery)
	}
}

// fuzzy reports whether all runes of query appear in name, in order
// (subsequence match).
func fuzzy(name, query string) bool {
	if query == "" {
		return true
	}
	ni, qi := 0, 0
	for qi < len(query) && ni < len(name) {
		if name[ni] == query[qi] {
			qi++
		}
		ni++
	}
	return qi == len(query)
}

// camelInitials returns the camel-case humps of s, e.g. "FileBrowser" → "FB",
// "readFile" → "RF", "APIResponse" → "APIR", "read_file" → "RF".
func camelInitials(s string) string {
	var b strings.Builder
	start := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_' || c == '$':
			start = true
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c)
			start = false
		case c >= 'a' && c <= 'z':
			if start {
				b.WriteByte(c - 32)
			}
			start = false
		}
	}
	return b.String()
}
