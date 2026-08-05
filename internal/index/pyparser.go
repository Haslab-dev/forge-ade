package index

import (
	"strings"
)

// pyParser extracts symbols from Python source. Line-based, indentation-aware.
// Enough for the index: classes, functions/methods, module-level vars.
type pyParser struct{}

func (p *pyParser) Language() Language { return LangPython }

func (p *pyParser) Parse(src []byte, file *File) (*ParseResult, error) {
	res := &ParseResult{}
	addSym := func(kind SymbolKind, name string, line, col int, scope string) {
		if name == "" || !isIdent(name) {
			return
		}
		res.Symbols = append(res.Symbols, Symbol{
			Name: name, Kind: kind, FileID: file.ID,
			Line: line, Column: col, EndLine: line, EndColumn: col + len(name), Scope: scope,
		})
	}
	lines := strings.Split(string(src), "\n")
	clsIndent, clsName := -1, ""
	for i, ln := range lines {
		line := i + 1
		indent := 0
		for indent < len(ln) && ln[indent] == ' ' {
			indent++
		}
		clean := stripPy(ln)
		t := strings.TrimSpace(clean)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if indent < clsIndent {
			clsIndent, clsName = -1, ""
		}
		words := strings.Fields(t)
		col := strings.Index(ln, words[0]) + 1
		switch words[0] {
		case "import":
			continue
		case "from":
			continue
		case "class":
			name := strings.TrimRight(words[1], ":(")
			addSym(Class, name, line, col, "")
			clsIndent, clsName = indent, name
			continue
		case "def":
			fallthrough
		case "async":
			if words[0] == "async" && len(words) > 1 && words[1] == "def" {
				// fallthrough handled below
			}
			k := 0
			if words[0] == "async" {
				k = 1
			}
			if k+1 < len(words) {
				name := words[k+1]
				if j := strings.IndexByte(name, '('); j >= 0 {
					name = name[:j]
				}
				name = strings.TrimRight(name, ":")
				if indent > clsIndent && clsName != "" {
					addSym(Method, name, line, col, clsName)
				} else {
					addSym(Function, name, line, col, "")
				}
			}
			continue
		case "@":
			// decorator — remember the next def is a method if inside class
			continue
		}
		// module-level assignments: `NAME = ...` (constant) or `name = ...`
		if indent == 0 && (strings.Contains(t, "=") || strings.HasSuffix(t, ":")) {
			name := t
			if i := strings.IndexByte(name, '='); i >= 0 {
				name = strings.TrimSpace(name[:i])
			} else {
				name = strings.TrimRight(name, ":")
			}
			if strings.HasSuffix(name, ":") {
				name = strings.TrimRight(name, ":")
			}
			if isIdent(name) {
				kind := Variable
				if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
					kind = Constant
				}
				addSym(kind, name, line, col, "")
			}
		}
	}
	return res, nil
}

// stripPy removes comments and string literals from a Python line.
func stripPy(ln string) string {
	var b strings.Builder
	i, n := 0, len(ln)
	inStr := byte(0)
	for i < n {
		c := ln[i]
		if inStr != 0 {
			if c == '\\' {
				b.WriteByte(c)
				if i+1 < n {
					i++
					b.WriteByte(ln[i])
				}
				i++
				continue
			}
			if c == inStr {
				inStr = 0
			}
			b.WriteByte(c)
			i++
			continue
		}
		switch {
		case c == '"' || c == '\'':
			// triple-quoted: skip as string
			if i+2 < n && ln[i+1] == c && ln[i+2] == c {
				b.WriteString(strings.Repeat(string(c), 3))
				i += 3
				for i+2 < n && !(ln[i] == c && ln[i+1] == c && ln[i+2] == c) {
					i++
				}
				i += 3
				continue
			}
			inStr = c
			b.WriteByte(c)
			i++
		case c == '#':
			return b.String()
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}
