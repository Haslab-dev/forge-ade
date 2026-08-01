package search

import (
	"path"
	"regexp"
	"strings"
)

// globRegexp wraps a compiled glob-as-regexp matcher.
type globRegexp struct {
	re *regexp.Regexp
}

// match reports whether relPath (slash-separated) matches the glob pattern.
// When the pattern contains no slash it is matched against the basename too,
// mirroring how common file search UIs behave.
func (g *globRegexp) match(relPath string) bool {
	if g == nil || g.re == nil {
		return true
	}
	if g.re.MatchString(relPath) {
		return true
	}
	base := relPath
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	return g.re.MatchString(base)
}

// compileGlob converts a glob pattern into a matcher. Supported syntax:
//
//	**   matches any number of path segments (including none)
//	*    matches any characters within a single path segment
//	?    matches a single character within a segment
//	[ab] character classes, [!ab] negation (via path.Match semantics)
//
// A pattern with no slash is matched against file basenames as well as paths.
func compileGlob(pattern string) (*globRegexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" || pattern == "**" {
		return nil, nil
	}

	// Split on "/" and convert each segment, then join. This keeps "**"
	// segment-aware so it only crosses directory boundaries.
	segments := strings.Split(pattern, "/")
	var sb strings.Builder
	sb.WriteString("^")
	for i, seg := range segments {
		if i > 0 {
			sb.WriteString("/")
		}
		if seg == "**" {
			sb.WriteString(".*")
			continue
		}
		for j := 0; j < len(seg); j++ {
			c := seg[j]
			switch c {
			case '*':
				sb.WriteString("[^/]*")
			case '?':
				sb.WriteString("[^/]")
			case '[':
				// copy through the closing bracket (path.Match character class)
				end := strings.IndexByte(seg[j+1:], ']')
				if end == -1 {
					sb.WriteString("\\[")
					continue
				}
				end += j + 1
				cls := seg[j : end+1]
				sb.WriteString(translateClass(cls))
				j = end
			default:
				sb.WriteString(regexp.QuoteMeta(string(c)))
			}
		}
	}
	sb.WriteString("$")

	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, err
	}
	return &globRegexp{re: re}, nil
}

// translateClass converts a "[...]" character class segment into regex.
func translateClass(cls string) string {
	if len(cls) < 2 {
		return regexp.QuoteMeta(cls)
	}
	inner := cls[1 : len(cls)-1]
	if strings.HasPrefix(inner, "!") {
		inner = "^" + inner[1:]
	}
	return "[" + inner + "]"
}

// globMatchesPath is a small helper used by tests and fallback search.
func globMatchesPath(pattern, relPath string) bool {
	g, err := compileGlob(pattern)
	if err != nil {
		return false
	}
	if g == nil {
		return true
	}
	return g.match(path.Clean(relPath))
}
