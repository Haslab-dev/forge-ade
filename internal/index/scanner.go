package index

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hasdev/forge-ade/internal/gitignore"
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
	return s.ScanContext(context.Background())
}

// ScanContext returns parseable source file paths under Root with context cancellation support.
func (s *Scanner) ScanContext(ctx context.Context) ([]string, error) {
	gi := gitignore.Load(s.Root)

	ignoreMap := make(map[string]bool, len(defaultIgnoreDirs)+len(s.Ignore))
	for k := range defaultIgnoreDirs {
		ignoreMap[strings.ToLower(k)] = true
	}
	for k := range s.Ignore {
		ignoreMap[strings.ToLower(k)] = true
	}

	var files []string
	err := filepath.WalkDir(s.Root, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if path != s.Root {
				name := d.Name()
				lower := strings.ToLower(name)
				if ignore.Name(lower) || ignoreMap[lower] {
					return filepath.SkipDir
				}
				if gi != nil {
					rel, relErr := filepath.Rel(s.Root, path)
					if relErr == nil && rel != "." && rel != "" {
						parts := strings.Split(filepath.ToSlash(rel), "/")
						if gi.Match(parts, true) {
							return filepath.SkipDir
						}
					}
				}
			}
			return nil
		}
		if gi != nil {
			rel, relErr := filepath.Rel(s.Root, path)
			if relErr == nil && rel != "." && rel != "" {
				parts := strings.Split(filepath.ToSlash(rel), "/")
				if gi.Match(parts, false) {
					return nil
				}
			}
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
