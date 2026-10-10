// benchws generates a synthetic workspace for performance benchmarking:
//
//	go run ./scripts/benchws -out /tmp/bench-ws -files 50000
//
// Layout: nested directories (branching factor 8, ~4 levels deep), a mix of
// source extensions matching what the index parses, plus ignored junk
// (node_modules/, .git/, dist/) to exercise ignore rules. Content is
// deterministic (seeded PRNG) so repeated runs are comparable.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	out := flag.String("out", "/tmp/forge-bench-ws", "output directory")
	files := flag.Int("files", 10000, "number of indexed source files to generate")
	junk := flag.Int("junk", 2000, "number of ignored (node_modules) files")
	flag.Parse()

	rng := rand.New(rand.NewSource(42))
	extensions := []string{".ts", ".tsx", ".go", ".js", ".json", ".md", ".css"}

	if err := os.RemoveAll(*out); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err)
	}

	// Breadth-first dir tree: level 0 = root, then 8 dirs per level to ~4 levels.
	type job struct{ path string }
	dirs := []job{{path: *out}}
	level, breadth, depth := 1, 8, 0
	for len(dirs) < 4096 && depth < 4 {
		var next []job
		for _, d := range dirs {
			if depthOf(d.path, *out) >= level {
				continue
			}
			for i := 0; i < breadth; i++ {
				p := filepath.Join(d.path, fmt.Sprintf("mod%02d_%d", depth, i))
				if err := os.MkdirAll(p, 0o755); err != nil {
					fatal(err)
				}
				next = append(next, job{path: p})
			}
		}
		dirs = append(dirs, next...)
		level++
		depth++
	}

	for i := 0; i < *files; i++ {
		dir := dirs[rng.Intn(len(dirs))].path
		ext := extensions[rng.Intn(len(extensions))]
		name := fmt.Sprintf("file%06d%s", i, ext)
		content := syntheticSource(i, ext, rng)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			fatal(err)
		}
	}

	// Ignored junk: node_modules (deep), .git, dist.
	nodeModules := filepath.Join(*out, "node_modules", "lib-dep", "dist")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		fatal(err)
	}
	for i := 0; i < *junk; i++ {
		name := fmt.Sprintf("dep%05d.js", i)
		if err := os.WriteFile(filepath.Join(nodeModules, name), []byte("module.exports = {}\n"), 0o644); err != nil {
			fatal(err)
		}
	}
	for _, d := range []string{".git/objects/ab", "dist"} {
		if err := os.MkdirAll(filepath.Join(*out, d), 0o755); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*out, d, "blob.bin"), make([]byte, 4096), 0o644); err != nil {
			fatal(err)
		}
	}

	fmt.Printf("generated %d source files + %d junk files in %s\n", *files, *junk, *out)
}

func depthOf(path, root string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator))
}

func syntheticSource(i int, ext string, rng *rand.Rand) string {
	var b strings.Builder
	syms := 3 + rng.Intn(6)
	for s := 0; s < syms; s++ {
		n := fmt.Sprintf("%s%06d_%d", symbolPrefix(ext), i, s)
		switch ext {
		case ".go":
			fmt.Fprintf(&b, "func %s() int {\n\treturn %d\n}\n\n", n, s)
		case ".ts", ".tsx":
			fmt.Fprintf(&b, "export function %s(): number {\n  return %d;\n}\n\n", n, s)
		case ".js":
			fmt.Fprintf(&b, "export function %s() {\n  return %d;\n}\n\n", n, s)
		case ".css":
			fmt.Fprintf(&b, ".cls-%s { color: #%06x; }\n", n, rng.Intn(0xffffff))
		case ".json":
			if s == 0 {
				fmt.Fprintf(&b, "{\n")
			}
			fmt.Fprintf(&b, "  \"key_%s\": %d%s\n", n, s, map[bool]string{true: ",", false: ""}[s < syms-1])
		case ".md":
			fmt.Fprintf(&b, "# %s\n\nSection %d body text.\n\n", n, s)
		}
	}
	if ext == ".json" {
		b.WriteString("}\n")
	}
	return b.String()
}

func symbolPrefix(ext string) string {
	switch ext {
	case ".go":
		return "Handle"
	case ".ts", ".tsx":
		return "render"
	case ".js":
		return "util"
	case ".css":
		return "rule"
	case ".md":
		return "Doc"
	default:
		return "sym"
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "benchws:", err)
	os.Exit(1)
}
