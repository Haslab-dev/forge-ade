package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Dependency index (review: Dependency Index).
//
// We never parse whole .d.ts files. A bare import like `import React from
// "react"` resolves to node_modules/react/package.json → types entry → the
// single .d.ts file, and only its `export` statements are recorded (names +
// kinds + line). `export * from "./hooks"` / `export { x } from "./y"`
// recurse through the export graph only — no folder scans.
//
// The result is one small Symbol per exported name, with Module set to the
// import specifier so the UI can offer go-to-definition and auto-import.

// DepSym is one exported name pulled from a dependency .d.ts.
type DepSym struct {
	Name string
	Kind SymbolKind
	Line int
}

// resolveModuleFile maps a bare import specifier to its types .d.ts path,
// or "" when nothing resolvable. node_modules is found by walking up from
// the importing file (so frontend/node_modules resolves for src/ files).
// Follows package.json `types`/`typings`, falls back to `index.d.ts`.
func resolveModuleFile(root, spec, from string) string {
	for dir := filepath.Dir(from); ; dir = filepath.Dir(dir) {
		p := filepath.Join(dir, "node_modules", spec)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			if f := packageTypes(p); f != "" {
				return f
			}
			// no types field → fall back to @types/<spec> (e.g. react)
			t := filepath.Join(dir, "node_modules", "@types", spec, "index.d.ts")
			if st, err := os.Stat(t); err == nil && !st.IsDir() {
				return t
			}
			return ""
		}
		// direct file form: `foo/bar.d.ts`
		if strings.HasSuffix(spec, ".d.ts") {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		if dir == root || filepath.Dir(dir) == dir {
			return ""
		}
	}
}

// packageTypes picks the .d.ts entry of a package dir from package.json.
func packageTypes(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err == nil {
		var m struct {
			Types   string `json:"types"`
			Typings string `json:"typings"`
			Main    string `json:"main"`
		}
		if json.Unmarshal(data, &m) == nil {
			for _, f := range []string{m.Types, m.Typings} {
				if f == "" {
					continue
				}
				p := filepath.Join(dir, f)
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
			if strings.HasSuffix(m.Main, ".d.ts") {
				p := filepath.Join(dir, m.Main)
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
	}
	p := filepath.Join(dir, "index.d.ts")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// parseDepExports extracts only export statements from a .d.ts file and
// follows `export * from` / `export { x } from` edges. visited guards cycles
// and repeated work. Line numbers are 1-based for go-to-definition.
func parseDepExports(file string, visited map[string]bool) []DepSym {
	if file == "" || visited[file] {
		return nil
	}
	visited[file] = true
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	// strip comments first so brace counting can't be thrown off by
	// braces inside doc comments / examples.
	data = stripDSText(data)
	var out []DepSym
	base := filepath.Dir(file)
	lines := strings.Split(string(data), "\n")
	for i := 0; i < len(lines); i++ {
		ln := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(ln, "export") && !strings.HasPrefix(ln, "declare") {
			continue
		}
		body := ln
		// multi-line export blocks: `export {\n a,\n b\n}` / `declare module`.
		// Newlines are kept so namespace bodies can be re-scanned per line.
		for braceCount(body) > 0 && i+1 < len(lines) {
			i++
			body += "\n" + strings.TrimSpace(lines[i])
		}
		body = stripLineComment(body)
		if !strings.HasPrefix(body, "export") && !strings.HasPrefix(body, "declare") {
			continue
		}
		body = strings.TrimPrefix(body, "export")
		body = strings.TrimPrefix(body, "declare")
		// `export * from "./x"` / `export * as ns from "./x"`
		if m := reDepStar.FindStringSubmatch(body); m != nil {
			for _, s := range parseDepExports(resolveDepRel(base, m[1]), visited) {
				out = append(out, s)
			}
			continue
		}
		// `export { a, b } from "./x"` → re-exported names, then follow file
		if m := reDepBrace.FindStringSubmatch(body); m != nil {
			for _, n := range splitDepNames(m[1]) {
				out = append(out, DepSym{Name: n, Kind: Variable, Line: i + 1})
			}
			if len(m) > 2 && m[2] != "" {
				for _, s := range parseDepExports(resolveDepRel(base, m[2]), visited) {
					out = append(out, s)
				}
			}
			continue
		}
		// `declare namespace React { function useState(...) ... }` — the
		// namespace is exported, so its members are too. Scan the inner
		// block for declarations (they carry no `export` prefix).
		if m := reDepNS.FindStringSubmatch(body); m != nil {
			name := strings.TrimSpace(m[1])
			if name != "" {
				out = append(out, DepSym{Name: name, Kind: Package, Line: i + 1})
			}
			if inner := nsInner(body); inner != "" {
				for _, s := range scanNSDecls(inner, i+1) {
					out = append(out, s)
				}
			}
			continue
		}
		// `export declare function useState(...)`, `export interface FC`, ...
		if m := reDepDecl.FindStringSubmatch(body); m != nil {
			name := strings.TrimSpace(m[2])
			if name != "" {
				out = append(out, DepSym{Name: name, Kind: depDeclKind(m[1]), Line: i + 1})
			}
			continue
		}
	}
	return out
}

var (
	reDepStar  = regexp.MustCompile(`^\s*\*\s+from\s+["']([^"']+)["']`)
	reDepBrace = regexp.MustCompile(`^\s*\{([^}]*)\}\s*(?:from\s+["']([^"']+)["'])?`)
	reDepDecl  = regexp.MustCompile(`^\s*(?:declare\s+|default\s+)*(?:abstract\s+)?(function|class|interface|type|enum|const|let|var|namespace|module)\s+([A-Za-z_$][\w$]*)`)
	reDepNS    = regexp.MustCompile(`^\s*(?:declare\s+)?namespace\s+([A-Za-z_$][\w$]*)`)
	reNSDecl   = regexp.MustCompile(`^\s*(?:export\s+|declare\s+|default\s+)*(?:abstract\s+)?(function|class|interface|type|enum|const|let|var)\s+([A-Za-z_$][\w$]*)`)
)

// nsInner extracts the text between the first { and its matching }.
func nsInner(body string) string {
	depth := 0
	start := -1
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '{':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				return body[start:i]
			}
		}
	}
	return ""
}

// scanNSDecls extracts declarations from an exported namespace body.
func scanNSDecls(inner string, baseLine int) []DepSym {
	var out []DepSym
	for i, ln := range strings.Split(inner, "\n") {
		ln = stripLineComment(strings.TrimSpace(ln))
		if ln == "" || strings.HasPrefix(ln, "//") {
			continue
		}
		// `export function useState(...)` or bare `function useState(...)`
		if m := reNSDecl.FindStringSubmatch(ln); m != nil {
			name := strings.TrimSpace(m[2])
			if name != "" {
				out = append(out, DepSym{Name: name, Kind: depDeclKind(m[1]), Line: baseLine + i})
			}
		}
	}
	return out
}

func depDeclKind(kw string) SymbolKind {
	switch kw {
	case "function":
		return Function
	case "class":
		return Class
	case "interface":
		return Interface
	case "type":
		return TypeAlias
	case "enum":
		return Enum
	case "const":
		return Constant
	case "let", "var":
		return Variable
	case "namespace", "module":
		return Package
	}
	return 0
}

// splitDepNames splits `a, b as c, default as d` → exported local names.
func splitDepNames(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// `name as alias` → the exported name is the alias
		words := strings.Fields(part)
		name := strings.TrimRight(words[0], "{}")
		for i := 0; i+1 < len(words); i++ {
			if words[i] == "as" {
				name = strings.TrimRight(words[i+1], "{}")
				break
			}
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// resolveDepRel resolves a relative `from "./hooks"` against the base dir of
// the current .d.ts, trying ./hooks.d.ts then ./hooks/index.d.ts.
func resolveDepRel(base, rel string) string {
	rel = strings.Trim(rel, `"'`)
	p := filepath.Join(base, rel)
	for _, cand := range []string{p, p + ".d.ts", filepath.Join(p, "index.d.ts"), p + ".ts"} {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
	}
	return ""
}

// stripDSText removes // and /* */ comments, preserving newlines so line
// numbers and per-line scanning stay intact.
func stripDSText(src []byte) []byte {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				if src[i] == '\n' {
					out = append(out, '\n')
				}
				i++
			}
			i++
		default:
			out = append(out, src[i])
		}
	}
	return out
}

// braceCount counts { minus } in a line.
func braceCount(s string) int {
	open, close := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			open++
		case '}':
			close++
		}
	}
	return open - close
}

// stripLineComment removes a trailing // comment (outside string literals).
func stripLineComment(s string) string {
	if i := strings.Index(s, "//"); i >= 0 {
		return s[:i]
	}
	return s
}
