package explorer

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/hasdev/forge-ade/internal/events"
)

// genTree materializes a deterministic synthetic workspace with n source
// files (branching factor 8, ~4 levels) plus ignored junk, mirroring
// scripts/benchws. Skipped when the dir already exists so repeated benchmark
// runs don't regenerate tens of thousands of files.
func genTree(b *testing.B, n int) string {
	b.Helper()
	root := filepath.Join(b.TempDir(), fmt.Sprintf("bench-%d", n))
	if _, err := os.Stat(root); err == nil {
		return root
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		b.Fatal(err)
	}
	rng := rand.New(rand.NewSource(42))
	exts := []string{".ts", ".go", ".js", ".json", ".md"}

	dirs := []string{root}
	for len(dirs) < 1024 {
		var next []string
		for _, d := range dirs {
			for i := 0; i < 8 && len(dirs)+len(next) < 1024; i++ {
				p := filepath.Join(d, fmt.Sprintf("d%d", len(dirs)+len(next)+i))
				if err := os.MkdirAll(p, 0o755); err != nil {
					b.Fatal(err)
				}
				next = append(next, p)
			}
		}
		dirs = append(dirs, next...)
	}

	for i := 0; i < n; i++ {
		dir := dirs[rng.Intn(len(dirs))]
		name := filepath.Join(dir, fmt.Sprintf("f%06d%s", i, exts[rng.Intn(len(exts))]))
		if err := os.WriteFile(name, []byte(fmt.Sprintf("export function sym%d() { return %d; }\n", i, i)), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	// Ignored junk the tree builder must skip.
	junk := filepath.Join(root, "node_modules", "dep", "dist")
	if err := os.MkdirAll(junk, 0o755); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 500; i++ {
		if err := os.WriteFile(filepath.Join(junk, fmt.Sprintf("x%d.js", i)), []byte("x"), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

// BenchmarkGetTree10k / 25k measure the full-tree walk the frontend explorer
// performs when a workspace opens (GetFileTree(-1)). This is the backend half
// of "workspace open latency"; the numbers go into docs/PERFORMANCE.md.
func BenchmarkGetTree10k(b *testing.B) {
	benchmarkGetTree(b, 10000)
}

func BenchmarkGetTree25k(b *testing.B) {
	benchmarkGetTree(b, 25000)
}

func benchmarkGetTree(b *testing.B, n int) {
	root := genTree(b, n)
	ex := New(events.NewBus())
	ex.SetRoots([]string{root})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree, err := ex.GetTree(-1)
		if err != nil {
			b.Fatal(err)
		}
		if len(tree) == 0 {
			b.Fatal("empty tree")
		}
	}
}

// BenchmarkListDir measures a single directory listing (folder expand path).
func BenchmarkListDir(b *testing.B) {
	root := genTree(b, 10000)
	ex := New(events.NewBus())
	ex.SetRoots([]string{root})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ex.ListDirectory(root); err != nil {
			b.Fatal(err)
		}
	}
}
