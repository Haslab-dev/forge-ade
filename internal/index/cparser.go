package index

import (
	"strings"
)

// cParser is a lightweight line-based symbol extractor for C-family languages
// (Go, Kotlin, Swift, Java, Dart). No type checking — enough for the index.
// Line-oriented: strips comments/strings, tracks brace depth, and recognizes
// declarations by leading keywords plus `name(` / `name =` patterns.
type cParser struct {
	lang Language
}

func (p *cParser) Language() Language { return p.lang }

// typeKws maps a leading keyword to a SymbolKind for the language.
func (p *cParser) typeKws() map[string]SymbolKind {
	switch p.lang {
	case LangGo:
		// `type X struct/interface` handled specially in typeDecl
		return map[string]SymbolKind{}
	case LangKotlin:
		return map[string]SymbolKind{"class": Class, "interface": Interface, "enum": Enum, "object": Class}
	case LangJava:
		return map[string]SymbolKind{"class": Class, "interface": Interface, "enum": Enum, "record": Class}
	case LangSwift:
		return map[string]SymbolKind{
			"class": Class, "struct": Struct, "enum": Enum, "protocol": Interface,
			"extension": Class, "actor": Class,
		}
	case LangDart:
		return map[string]SymbolKind{"class": Class, "enum": Enum, "mixin": Class, "typedef": TypeAlias}
	}
	return map[string]SymbolKind{}
}

func (p *cParser) Parse(src []byte, file *File) (*ParseResult, error) {
	res := &ParseResult{}
	addSym := func(kind SymbolKind, name string, col int, scope string) {
		if name == "" {
			return
		}
		// strip trailing chars / cut params: `foo()`, `Foo(val x`, `main(){}`
		name = strings.TrimSpace(name)
		if j := strings.IndexAny(name, "({"); j >= 0 {
			name = name[:j]
		}
		name = strings.TrimRight(name, "{}();:,")
		if name == "" || !isIdent(name) {
			return
		}
		res.Symbols = append(res.Symbols, Symbol{
			Name: name, Kind: kind, FileID: file.ID,
			Line: 0, Column: col, EndLine: 0, EndColumn: col + len(name), Scope: scope,
		})
	}
	lines := strings.Split(string(src), "\n")
	depth := 0
	curType := ""
	lineNo := 0
	for _, ln := range lines {
		lineNo++
		clean := stripCComments(ln)
		t := strings.TrimSpace(clean)
		if t == "" {
			depth = adjustDepth(depth, clean)
			if depth <= 0 {
				depth, curType = 0, ""
			}
			continue
		}
		words := strings.Fields(t)
		first := words[0]
		col := strings.Index(ln, first) + 1

		// package / import / library
		switch first {
		case "package":
			if len(words) > 1 {
				addSym(Package, words[1], col, "")
			}
			depth = adjustDepth(depth, clean)
			continue
		case "import":
			depth = adjustDepth(depth, clean)
			continue
		case "library", "part", "export", "use":
			depth = adjustDepth(depth, clean)
			continue
		}

		// Go: `type X struct {` / `type X interface {` / `type X = Y`
		if p.lang == LangGo && first == "type" {
			if len(words) >= 3 {
				kind := TypeAlias
				switch cutName(words[2]) {
				case "struct":
					kind = Struct
				case "interface":
					kind = Interface
				}
				addSym(kind, words[1], col, "")
				if kind == Struct || kind == Interface {
					curType = words[1]
				}
			}
			depth = adjustDepth(depth, clean)
			if depth == 0 {
				curType = ""
			}
			continue
		}

		// Go: `func`, `const`, `var`
		if p.lang == LangGo {
			if first == "func" {
				name := goFuncName(t)
				if depth > 0 && curType != "" {
					addSym(Method, name, col, curType)
				} else {
					addSym(Function, name, col, "")
				}
				depth = adjustDepth(depth, clean)
				continue
			}
			if first == "const" {
				addSym(Constant, firstIdentAfter(t, "const"), col, "")
				depth = adjustDepth(depth, clean)
				continue
			}
			if first == "var" {
				addSym(Variable, firstIdentAfter(t, "var"), col, "")
				depth = adjustDepth(depth, clean)
				continue
			}
			depth = adjustDepth(depth, clean)
			if depth == 0 {
				curType = ""
			}
			continue
		}

		// Kotlin: fun / val / var / const val / typealias
		if p.lang == LangKotlin {
			if first == "fun" {
				name := firstIdentAfter(t, "fun")
				if depth > 0 && curType != "" {
					addSym(Method, name, col, curType)
				} else {
					addSym(Function, name, col, "")
				}
				depth = adjustDepth(depth, clean)
				continue
			}
			if first == "typealias" {
				addSym(TypeAlias, firstIdentAfter(t, "typealias"), col, "")
				depth = adjustDepth(depth, clean)
				continue
			}
			if first == "const" {
				if idx := strings.Index(t, "val"); idx >= 0 {
					addSym(Constant, firstIdentAfter(t[idx:], "val"), col, "")
				}
				depth = adjustDepth(depth, clean)
				continue
			}
			if first == "val" || first == "var" {
				addSym(Variable, firstIdentAfter(t, first), col, curTypeFor(depth, curType))
				depth = adjustDepth(depth, clean)
				continue
			}
		}

		// Swift: func / let / var
		if p.lang == LangSwift {
			if first == "func" {
				name := firstIdentAfter(t, "func")
				if depth > 0 && curType != "" {
					addSym(Method, name, col, curType)
				} else {
					addSym(Function, name, col, "")
				}
				depth = adjustDepth(depth, clean)
				continue
			}
			if first == "let" || first == "var" {
				name := firstIdentAfter(t, first)
				kind := Variable
				if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
					kind = Constant
				}
				addSym(kind, name, col, curTypeFor(depth, curType))
				depth = adjustDepth(depth, clean)
				continue
			}
		}

		// generic type declarations: `class X {`, `enum X {`, `struct X {` ...
		// run BEFORE method/field detection so same-line bodies get a scope.
		if kind, name, ok := p.typeDecl(t); ok {
			addSym(kind, name, col, "")
			if kind == Class || kind == Struct || kind == Interface || kind == Enum {
				curType = name
			}
			depth = adjustDepth(depth, clean)
			if depth == 0 {
				curType = ""
			}
			continue
		}

		// Java / Dart: method or field detection inside type bodies, plus
		// `static final` constants (Java) and top-level vars (Dart).
		if p.lang == LangJava || p.lang == LangDart {
			if p.lang == LangJava && strings.Contains(t, "static final") && !strings.Contains(t, "(") {
				name := nameBeforeEqOrSemi(t)
				addSym(Constant, name, col, curTypeFor(depth, curType))
				depth = adjustDepth(depth, clean)
				continue
			}
			if p.lang == LangDart && (first == "var" || first == "final" || first == "const") {
				kind := Variable
				if first == "const" {
					kind = Constant
				}
				addSym(kind, firstIdentAfter(t, first), col, curTypeFor(depth, curType))
				depth = adjustDepth(depth, clean)
				continue
			}
			if i := strings.IndexByte(t, '('); i > 0 {
				// constructor or method: `Type name(` — name is the ident right
				// before `(`. Constructors (name == enclosing type) still useful.
				name := identBefore(t, i)
				if depth > 0 && curType != "" {
					addSym(Method, name, col, curType)
				} else {
					addSym(Function, name, col, "")
				}
				depth = adjustDepth(depth, clean)
				continue
			}
			// field: `Type name;` or `Type name = ...` inside a class body
			if depth > 0 && curType != "" {
				if name := nameBeforeEqOrSemi(t); name != "" && isIdent(name) {
					addSym(Variable, name, col, curType)
				}
			}
		}

		depth = adjustDepth(depth, clean)
		if depth == 0 {
			curType = ""
		}
	}
	return res, nil
}

// typeDecl matches `[modifiers] <kw> <Name>` for the language's type keywords.
func (p *cParser) typeDecl(t string) (SymbolKind, string, bool) {
	words := strings.Fields(t)
	for i, w := range words {
		// two-word forms: `enum class X`, `data class X`, `abstract class X`
		if i+2 < len(words) && words[i+1] == "class" &&
			(w == "enum" || w == "data" || w == "sealed" || w == "abstract" || w == "annotation") {
			kind := Class
			if w == "enum" {
				kind = Enum
			}
			return kind, cutName(words[i+2]), true
		}
		if kind, ok := p.typeKws()[w]; ok {
			if i+1 < len(words) {
				return kind, cutName(words[i+1]), true
			}
			return kind, "", true
		}
		// stop after modifiers; type keyword appears early in the line
		if i > 3 {
			break
		}
	}
	return 0, "", false
}

func curTypeFor(depth int, curType string) string {
	if depth > 0 {
		return curType
	}
	return ""
}

// goFuncName extracts the function/method name from a Go `func` line,
// skipping an optional receiver group: `func (r *T) Name(`, `func Name(`.
func goFuncName(t string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(t, "func"))
	if strings.HasPrefix(rest, "(") {
		d := 0
		for i, c := range rest {
			switch c {
			case '(':
				d++
			case ')':
				d--
				if d == 0 {
					rest = strings.TrimSpace(rest[i+1:])
					goto done
				}
			}
		}
	}
done:
	words := strings.Fields(rest)
	if len(words) == 0 {
		return ""
	}
	return cutName(words[0])
}

// cutName returns the identifier part of a declaration token:
// `Foo(val x` → Foo, `main(){}` → main, `User.greet()` → greet.
func cutName(tok string) string {
	if j := strings.IndexAny(tok, "({"); j >= 0 {
		tok = tok[:j]
	}
	tok = strings.TrimRight(tok, "{}();:,")
	if j := strings.LastIndexByte(tok, '.'); j >= 0 {
		tok = tok[j+1:]
	}
	tok = strings.TrimRight(tok, "{}();:,")
	return tok
}

// firstIdentAfter returns the first identifier after the given keyword.
func firstIdentAfter(t, kw string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(t, kw))
	words := strings.Fields(rest)
	if len(words) == 0 {
		return ""
	}
	return cutName(words[0])
}

// identBefore returns the identifier immediately before the byte at idx:
// `public void main(` → main, `Foo.bar(` → bar, `List<String> name(` → name.
func identBefore(t string, idx int) string {
	seg := strings.TrimSpace(t[:idx])
	words := strings.Fields(seg)
	if len(words) == 0 {
		return ""
	}
	tok := words[len(words)-1]
	tok = strings.TrimRight(tok, "[]{}(),;")
	if j := strings.IndexByte(tok, '<'); j >= 0 {
		tok = tok[:j]
	}
	if j := strings.LastIndexByte(tok, '.'); j >= 0 {
		tok = tok[j+1:]
	}
	tok = strings.TrimRight(tok, "[]{}(),;")
	if !isIdent(tok) {
		return ""
	}
	return tok
}

// nameBeforeEqOrSemi extracts `name` from `Type name = ...` / `Type name;`.
func nameBeforeEqOrSemi(t string) string {
	end := len(t)
	if i := strings.IndexByte(t, '='); i >= 0 {
		end = i
	} else if i := strings.IndexByte(t, ';'); i >= 0 {
		end = i
	}
	words := strings.Fields(t[:end])
	if len(words) == 0 {
		return ""
	}
	name := words[len(words)-1]
	// `Type<A,B> name` — name is after the generics
	if strings.HasSuffix(name, ">") {
		// try next-to-last
		return ""
	}
	return strings.TrimRight(name, "{(")
}

func isIdentByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '$'
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isIdentByte(s[i]) {
			return false
		}
	}
	return true
}

func adjustDepth(depth int, line string) int {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	if depth < 0 {
		return 0
	}
	return depth
}

// stripCComments removes // and /* */ comments plus string literals.
func stripCComments(ln string) string {
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
			inStr = c
			b.WriteByte(c)
			i++
		case c == '/' && i+1 < n && ln[i+1] == '/':
			return b.String()
		case c == '/' && i+1 < n && ln[i+1] == '*':
			for i+1 < n && !(ln[i] == '*' && ln[i+1] == '/') {
				i++
			}
			i += 2
		case c == '`':
			// Go raw string / Swift backtick ident
			b.WriteByte(c)
			i++
			for i < n && ln[i] != '`' {
				b.WriteByte(ln[i])
				i++
			}
			if i < n {
				b.WriteByte(ln[i])
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}
