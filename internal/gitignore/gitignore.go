package gitignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	gogitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// Matcher checks whether paths are gitignored using go-git's gitignore parser.
type Matcher struct {
	inner gogitignore.Matcher
	Root  string // absolute path of the git root this Matcher was built from
}

// Load reads .gitignore patterns from a directory, walking up to find the git root.
func Load(dir string) *Matcher {
	abs, _ := filepath.Abs(dir)
	root := findRoot(abs)
	if root == "" {
		return nil
	}

	var allPatterns []gogitignore.Pattern
	collectPatterns(root, abs, &allPatterns)

	if len(allPatterns) == 0 {
		return nil
	}
	return &Matcher{inner: gogitignore.NewMatcher(allPatterns), Root: root}
}

func findRoot(dir string) string {
	current := dir
	for {
		if info, err := os.Stat(filepath.Join(current, ".git")); err == nil && info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func collectPatterns(root, dir string, patterns *[]gogitignore.Pattern) {
	// Walk from root to dir, collecting .gitignore files along the way
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 1 && parts[0] == "." {
		parts = nil
	}

	// Always check root
	loadFrom(filepath.Join(root, ".gitignore"), nil, patterns)

	// Walk down to dir
	for i := 0; i < len(parts); i++ {
		domain := parts[:i]
		gitignorePath := filepath.Join(root, filepath.Join(parts[:i+1]...), ".gitignore")
		loadFrom(gitignorePath, domain, patterns)
	}
}

func loadFrom(path string, domain []string, patterns *[]gogitignore.Pattern) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		*patterns = append(*patterns, gogitignore.ParsePattern(line, domain))
	}
}

// MatchDir returns true if a directory name matches any gitignore pattern.
func (m *Matcher) MatchDir(name string) bool {
	if m == nil {
		return false
	}
	return m.inner.Match([]string{name}, true)
}

// Match returns true if the path matches any gitignore pattern.
// path components must be relative to the git root.
func (m *Matcher) Match(path []string, isDir bool) bool {
	if m == nil {
		return false
	}
	return m.inner.Match(path, isDir)
}

// MatchAbs returns true if the absolute file path is gitignored.
// It computes the path relative to the Matcher's git root automatically.
func (m *Matcher) MatchAbs(absPath string, isDir bool) bool {
	if m == nil {
		return false
	}
	rel, err := filepath.Rel(m.Root, absPath)
	if err != nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return m.inner.Match(parts, isDir)
}
