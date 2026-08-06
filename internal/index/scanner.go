package index

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/hasdev/forge-ade/internal/ignore"
)

// defaultIgnoreDirs are always skipped during scans (RFC §5.1). The base set
// comes from the shared ignore package; index-only entries are added here.
var defaultIgnoreDirs = func() map[string]bool {
	m := make(map[string]bool, len(ignore.Dir)+4)
	for k := range ignore.Dir {
		m[k] = true
	}
	m[".workspace"] = true
	m[".cortex"] = true
	return m
}()

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
			// node_modules types are handled lazily by the dependency index
			// (export graph only, review: Dependency Index) — never scanned.
			if path != s.Root && ignore[d.Name()] {
				return filepath.SkipDir
			}
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
