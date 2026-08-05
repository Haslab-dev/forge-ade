package index

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// defaultIgnoreDirs are always skipped during scans (RFC §5.1).
var defaultIgnoreDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"Pods":         true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	"vendor":       true,
	"target":       true,
	".gradle":      true,
	".next":        true,
	".idea":        true,
	".vscode":      true,
	".workspace":   true,
	".cortex":      true,
}

// Scanner walks a workspace and yields source files (RFC §5.1).
// It only produces a list of files; parsing happens elsewhere.
type Scanner struct {
	// Root is the workspace directory to scan.
	Root string
	// Ignore adds extra directories (by base name) on top of defaults.
	Ignore map[string]bool
}

// Scan returns parseable source file paths under Root, sorted.
// Files whose language has no registered parser are skipped.
func (s *Scanner) Scan() ([]string, error) {
	ignore := make(map[string]bool, len(defaultIgnoreDirs)+len(s.Ignore))
	for k := range defaultIgnoreDirs {
		ignore[k] = true
	}
	for k := range s.Ignore {
		ignore[k] = true
	}
	var files []string
	err := filepath.WalkDir(s.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			// Walk into node_modules (to index .d.ts types) but skip other
			// ignore-listed dirs. Nested node_modules/.git etc still ignored.
			if path != s.Root && ignore[d.Name()] && d.Name() != "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(path, "/node_modules/") && !strings.HasSuffix(path, ".d.ts") {
			return nil
		}
		lang := DetectLanguage(path)
		if lang == "" || ForLang(lang) == nil {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
