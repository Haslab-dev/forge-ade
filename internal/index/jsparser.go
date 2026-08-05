package index

import "strings"

// jsParser is a lightweight parser for JavaScript/TypeScript/JSX/TSX.
// It extracts top-level declarations, class methods, imports and exports.
// It performs no type checking and does not build a full AST.
type jsParser struct {
	lang Language
}

func (p *jsParser) Language() Language { return p.lang }

// token kinds.
type tokKind int

const (
	tIdent tokKind = iota
	tStr
	tPunct
	tEOF
)

type token struct {
	kind tokKind
	val  string // identifier/string text, or single punct char
	line int    // 1-based
	col  int    // 1-based
}

// lex tokenizes JS/TS source, stripping comments and string/template
// literals from the token stream. Line/column are 1-based.
func lex(src []byte) []token {
	var toks []token
	line, col := 1, 1
	i := 0
	n := len(src)

	// add appends a punct token and reports whether the char was consumed.
	add := func(val string) {
		toks = append(toks, token{kind: tPunct, val: val, line: line, col: col})
		col += len(val)
		i += len(val)
	}

	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			line++
			col = 1
			i++
		case c == ' ' || c == '\t' || c == '\r':
			col++
			i++
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			col += 2
			for i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
					col = 1
				} else {
					col++
				}
				i++
			}
			if i < n {
				i += 2
				col += 2
			}
		case c == '"' || c == '\'':
			start := i
			skipQuote(src, &i, &line, &col)
			toks = append(toks, token{kind: tStr, val: string(src[start:i]), line: line, col: col - (i - start)})
		case c == '`':
			start := i
			skipTemplate(src, &i, &line, &col)
			toks = append(toks, token{kind: tStr, val: string(src[start:i]), line: line, col: col - (i - start)})
		case c == '/' && isRegexStart(toks):
			start := i
			skipRegex(src, &i, &line, &col)
			toks = append(toks, token{kind: tStr, val: string(src[start:i]), line: line, col: col - (i - start)})
		case isIdentStart(c):
			start := i
			scol := col
			for i < n && isIdentPart(src[i]) {
				i++
				col++
			}
			toks = append(toks, token{kind: tIdent, val: string(src[start:i]), line: line, col: scol})
		case c >= '0' && c <= '9':
			start := i
			scol := col
			for i < n && (isIdentPart(src[i]) || src[i] == '.') {
				i++
				col++
			}
			toks = append(toks, token{kind: tIdent, val: string(src[start:i]), line: line, col: scol})
		default:
			// multi-char operators
			switch {
			case c == '=' && i+1 < n && src[i+1] == '>':
				add("=>")
				continue
			case c == '=' && i+1 < n && src[i+1] == '=':
				add("==")
				continue
			case c == '!' && i+1 < n && src[i+1] == '=':
				add("!=")
				continue
			case c == '<' && i+1 < n && src[i+1] == '=':
				add("<=")
				continue
			case c == '>' && i+1 < n && src[i+1] == '=':
				add(">=")
				continue
			case c == '&' && i+1 < n && src[i+1] == '&':
				add("&&")
				continue
			case c == '|' && i+1 < n && src[i+1] == '|':
				add("||")
				continue
			case c == '+' && i+1 < n && src[i+1] == '+':
				add("++")
				continue
			case c == '-' && i+1 < n && src[i+1] == '-':
				add("--")
				continue
			case c == '?' && i+1 < n && src[i+1] == '.':
				add("?.")
				continue
			}
			add(string(c))
		}
	}
	toks = append(toks, token{kind: tEOF, line: line, col: col})
	return toks
}

func skipQuote(src []byte, i *int, line, col *int) {
	c := src[*i]
	*i++
	*col++
	for *i < len(src) {
		switch src[*i] {
		case '\\':
			*i += 2
			*col += 2
		case c:
			*i++
			*col++
			return
		case '\n':
			*line++
			*col = 1
			*i++
		default:
			*i++
			*col++
		}
	}
}

// skipTemplate consumes a template literal, lexing ${...} interpolations
// inline so brace balancing in the surrounding code stays correct.
func skipTemplate(src []byte, i *int, line, col *int) {
	*i++ // `
	*col++
	for *i < len(src) {
		switch src[*i] {
		case '\\':
			*i += 2
			*col += 2
		case '`':
			*i++
			*col++
			return
		case '$':
			if *i+1 < len(src) && src[*i+1] == '{' {
				*i += 2
				*col += 2
				depth := 1
				for *i < len(src) && depth > 0 {
					switch src[*i] {
					case '{':
						depth++
						*i++
						*col++
					case '}':
						depth--
						*i++
						*col++
					case '"', '\'', '`':
						if src[*i] == '`' {
							skipTemplate(src, i, line, col)
						} else {
							skipQuote(src, i, line, col)
						}
					case '\n':
						*line++
						*col = 1
						*i++
					default:
						*i++
						*col++
					}
				}
			} else {
				*i++
				*col++
			}
		case '\n':
			*line++
			*col = 1
			*i++
		default:
			*i++
			*col++
		}
	}
}

// skipRegex consumes a regex literal. isRegexStart decides when a '/' begins
// one based on the previous significant token.
func skipRegex(src []byte, i *int, line, col *int) {
	*i++ // /
	*col++
	inClass := false
	for *i < len(src) {
		switch src[*i] {
		case '\\':
			*i += 2
			*col += 2
		case '[':
			inClass = true
			*i++
			*col++
		case ']':
			inClass = false
			*i++
			*col++
		case '/':
			if !inClass {
				*i++
				*col++
				// flags
				for *i < len(src) && isIdentPart(src[*i]) {
					*i++
					*col++
				}
				return
			}
			*i++
			*col++
		case '\n':
			*line++
			*col = 1
			*i++
		default:
			*i++
			*col++
		}
	}
}

// isRegexStart reports whether '/' at the current position starts a regex
// rather than being division.
func isRegexStart(toks []token) bool {
	for i := len(toks) - 1; i >= 0; i-- {
		t := toks[i]
		if t.kind == tPunct {
			switch t.val {
			case ")", "]", "}", "*", "/", "+", "-", "++", "--", "=", "==", "===", "=>":
				return false
			}
			return true
		}
		if t.kind == tIdent {
			switch t.val {
			case "return", "typeof", "instanceof", "in", "of", "new", "delete", "void", "throw", "case", "do", "else", "yield", "await":
				return true
			}
			return false
		}
		return false
	}
	return true // start of file
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// Parse extracts symbols, imports and exports (RFC §5.3).
func (p *jsParser) Parse(src []byte, file *File) (*ParseResult, error) {
	toks := lex(src)
	res := &ParseResult{}
	i := 0
	for i < len(toks) {
		t := toks[i]
		exported := false
		if t.kind == tIdent && t.val == "export" {
			exported = true
			i++
			if i >= len(toks) {
				break
			}
			t = toks[i]
			if t.kind == tIdent && t.val == "default" {
				i++
				if i >= len(toks) {
					break
				}
				t = toks[i]
				// export default <ref> — record the exported reference
				if t.kind == tIdent && t.val != "function" && t.val != "class" {
					res.Exports = append(res.Exports, Export{FileID: file.ID, Name: t.val, Line: t.line, Column: t.col})
				}
			} else if t.kind == tPunct && t.val == "{" {
				// export { a, b as c }
				i++
				for i < len(toks) && toks[i].val != "}" {
					if toks[i].kind == tIdent {
						if toks[i].val == "as" {
							i++ // skip alias target
						} else {
							res.Exports = append(res.Exports, Export{FileID: file.ID, Name: toks[i].val, Line: toks[i].line, Column: toks[i].col})
						}
					}
					i++
				}
				continue
			} else if t.kind == tPunct && t.val == "*" {
				// export * from "..."
				continue
			}
		}

		switch {
		case t.kind == tIdent && t.val == "function":
			i = p.parseFunction(toks, i, file, exported, "", res)
		case t.kind == tIdent && t.val == "class":
			i = p.parseClass(toks, i, file, exported, res)
		case t.kind == tIdent && t.val == "interface":
			i = p.parseDecl(toks, i, file, Interface, exported, res)
		case t.kind == tIdent && t.val == "enum":
			i = p.parseDecl(toks, i, file, Enum, exported, res)
		case t.kind == tIdent && t.val == "type":
			i = p.parseTypeAlias(toks, i, file, exported, res)
		case t.kind == tIdent && (t.val == "const" || t.val == "let" || t.val == "var"):
			i = p.parseVar(toks, i, file, exported, res)
		case t.kind == tIdent && t.val == "import":
			i = p.parseImport(toks, i, file, res)
		case t.kind == tIdent && t.val == "require":
			if end, imp, ok := p.parseRequire(toks, i, file); ok {
				res.Imports = append(res.Imports, imp)
				i = end
				continue
			}
			i++
		default:
			i++
		}
	}
	return res, nil
}

// parseFunction handles `[async] function name(...) { ... }` (also generators).
// toks[i] must be the `function` token. Returns the index after the body.
func (p *jsParser) parseFunction(toks []token, i int, file *File, exported bool, scope string, res *ParseResult) int {
	nameTok := token{}
	// skip `function` and optional `*`
	j := i + 1
	for j < len(toks) && toks[j].kind == tPunct && toks[j].val == "*" {
		j++
	}
	if j < len(toks) && toks[j].kind == tIdent && toks[j].val != "async" {
		nameTok = toks[j]
	} else {
		// anonymous (e.g. export default function () {}) — skip to body end
		end := skipToBraceEnd(toks, j)
		if end > j {
			return end
		}
		return j
	}
	// find body
	end := skipToBraceEnd(toks, j+1)
	sym := Symbol{
		Name:      nameTok.val,
		Kind:      Function,
		FileID:    file.ID,
		Line:      nameTok.line,
		Column:    nameTok.col,
		EndLine:   nameTok.line,
		EndColumn: nameTok.col + len(nameTok.val),
		Scope:     scope,
		Exported:  exported,
	}
	if end > j {
		sym.EndLine = toks[end-1].line
		sym.EndColumn = toks[end-1].col + 1
	}
	res.Symbols = append(res.Symbols, sym)
	// record return shape (FP member completion): `return {...}` / `return new Foo()`
	if fr := funcBodyReturn(toks, j+1); fr != nil {
		fr.Name = nameTok.val
		res.FuncReturn = append(res.FuncReturn, *fr)
	}
	if end > j {
		return end
	}
	return j + 1
}

// parseClass handles `class Name [extends X] { ... }`, adding the class symbol
// and its methods (RFC §7 Method kind).
func (p *jsParser) parseClass(toks []token, i int, file *File, exported bool, res *ParseResult) int {
	// optional `default` already handled by caller
	j := i + 1
	if j < len(toks) && toks[j].kind == tIdent {
		name := toks[j]
		// skip name, generics and extends clause until `{`
		k := j + 1
		for k < len(toks) && toks[k].val != "{" && toks[k].val != ";" && toks[k].kind != tEOF {
			k++
		}
		if k < len(toks) && toks[k].val == "{" {
			open := k
			close := matchBrace(toks, open)
			res.Symbols = append(res.Symbols, Symbol{
				Name:      name.val,
				Kind:      Class,
				FileID:    file.ID,
				Line:      name.line,
				Column:    name.col,
				EndLine:   toks[close-1].line,
				EndColumn: toks[close-1].col + 1,
				Exported:  exported,
			})
			p.parseMethods(toks, open+1, close, name.val, file, res)
			return close
		}
	}
	return j + 1
}

// parseMethods scans a class body (tokens open+1 .. close-1) for methods.
func (p *jsParser) parseMethods(toks []token, from, to int, className string, file *File, res *ParseResult) {
	depth := 0
	prevPunct := "{" // treat first token as after a punct
	for j := from; j < to; j++ {
		t := toks[j]
		switch {
		case t.kind == tPunct && t.val == "{":
			depth++
			prevPunct = "{"
		case t.kind == tPunct && t.val == "}":
			depth--
			prevPunct = "}"
		case depth == 0 && t.kind == tIdent:
			// method if followed by ( — but not after `.` or `(` or `:` (no parens: property)
			if j+1 < to && toks[j+1].kind == tPunct && toks[j+1].val == "(" &&
				prevPunct != "." && prevPunct != ":" && prevPunct != "(" && prevPunct != "?" {
				name := t.val
				kind := Method
				if name == "constructor" {
					kind = Function
				}
				res.Symbols = append(res.Symbols, Symbol{
					Name:      name,
					Kind:      kind,
					FileID:    file.ID,
					Line:      t.line,
					Column:    t.col,
					EndLine:   t.line,
					EndColumn: t.col + len(name),
					Scope:     className,
					Exported:  false,
				})
			} else if isFieldPos(prevPunct) && !isModifier(t.val) {
				// class field/property: `size: number`, `items = []`, `onClick = () => {}`
				res.Symbols = append(res.Symbols, Symbol{
					Name:      t.val,
					Kind:      Variable,
					FileID:    file.ID,
					Line:      t.line,
					Column:    t.col,
					EndLine:   t.line,
					EndColumn: t.col + len(t.val),
					Scope:     className,
				})
			}
			prevPunct = ""
		case t.kind == tPunct:
			prevPunct = t.val
		default:
			prevPunct = "" // strings, numbers, keywords: not a field boundary
		}
	}
}

// parseDecl handles `interface Name {...}` and `enum Name {...}`.
func (p *jsParser) parseDecl(toks []token, i int, file *File, kind SymbolKind, exported bool, res *ParseResult) int {
	j := i + 1
	if j < len(toks) && toks[j].kind == tIdent {
		name := toks[j]
		k := j + 1
		for k < len(toks) && toks[k].val != "{" && toks[k].kind != tEOF {
			k++
		}
		end := k
		if k < len(toks) && toks[k].val == "{" {
			close := matchBrace(toks, k)
			end = close
			if close > k+1 {
				// end position at closing brace
			}
		}
		sym := Symbol{
			Name:      name.val,
			Kind:      kind,
			FileID:    file.ID,
			Line:      name.line,
			Column:    name.col,
			EndLine:   name.line,
			EndColumn: name.col + len(name.val),
			Exported:  exported,
		}
		if end > k && k < len(toks) && toks[k].val == "{" {
			sym.EndLine = toks[end-1].line
			sym.EndColumn = toks[end-1].col + 1
		}
		res.Symbols = append(res.Symbols, sym)
		// interface members (fields + method signatures) → member completion
		if kind == Interface && k < len(toks) && toks[k].val == "{" && end > k+1 {
			p.parseMethods(toks, k+1, end-1, name.val, file, res)
		}
		// enum members `{ Red, Green, Blue }` → `Color.Red` member completion
		if kind == Enum && k < len(toks) && toks[k].val == "{" && end > k+1 {
			prev := "{"
			for m := k + 1; m < end-1; m++ {
				t := toks[m]
				if t.kind == tPunct {
					if t.val != "." {
						prev = t.val
					}
					continue
				}
				if t.kind == tIdent && (prev == "{" || prev == "," || prev == "=") {
					res.Symbols = append(res.Symbols, Symbol{
						Name:      t.val,
						Kind:      Constant,
						FileID:    file.ID,
						Line:      t.line,
						Column:    t.col,
						EndLine:   t.line,
						EndColumn: t.col + len(t.val),
						Scope:     name.val,
					})
					prev = ""
					continue
				}
				if t.kind == tIdent {
					prev = ""
				}
			}
		}
		return end
	}
	return i + 1
}

// parseTypeAlias handles `type Name<T> = ...`.
func (p *jsParser) parseTypeAlias(toks []token, i int, file *File, exported bool, res *ParseResult) int {
	j := i + 1
	if j < len(toks) && toks[j].kind == tIdent {
		name := toks[j]
		res.Symbols = append(res.Symbols, Symbol{
			Name:      name.val,
			Kind:      TypeAlias,
			FileID:    file.ID,
			Line:      name.line,
			Column:    name.col,
			EndLine:   name.line,
			EndColumn: name.col + len(name.val),
			Exported:  exported,
		})
		// `type Point = { x, y }` → members from the object body
		k := j + 1
		depth := 0
		for k < len(toks) && toks[k].kind != tEOF {
			switch toks[k].val {
			case "<", "(", "[", "{":
				depth++
			case ">", ")", "]", "}":
				if depth > 0 {
					depth--
				}
			default:
				if depth == 0 && toks[k].val == "=" {
					goto aliasEq
				}
			}
			k++
		}
	aliasEq:
		if k+1 < len(toks) && toks[k].val == "=" && toks[k+1].val == "{" {
			close := matchBrace(toks, k+1)
			for _, key := range collectKeys(toks, k+2, close-1) {
				res.Symbols = append(res.Symbols, Symbol{
					Name:     key,
					Kind:     Variable,
					FileID:   file.ID,
					Line:     name.line,
					Column:   name.col,
					Scope:    name.val,
					Exported: exported,
				})
			}
		}
		return j + 1
	}
	return i + 1
}

// parseVar handles `const/let/var name = ...` (multi-declarators too).
// Arrow-function constants are classified as Function.
func (p *jsParser) parseVar(toks []token, i int, file *File, exported bool, res *ParseResult) int {
	kw := toks[i].val
	j := i + 1
	for j < len(toks) && toks[j].kind == tIdent {
		name := toks[j]
		// skip name and optional type annotation or destructuring; stop at `=`/
		// top-level `,`/`;`, but skip balanced generics/parens/brackets inside
		k := j + 1
		depth := 0
		for k < len(toks) && toks[k].kind != tEOF {
			v := toks[k].val
			switch v {
			case "<", "(", "[", "{":
				depth++
			case ">", ")", "]", "}":
				if depth > 0 {
					depth--
				}
			default:
				if depth == 0 && (v == "=" || v == "," || v == ";") {
					goto scanDone
				}
			}
			k++
		}
	scanDone:
		if k < len(toks) && toks[k].val == "=" {
			// member-completion binding: type annotation wins (interface contract),
			// then `new Foo()` / `{ a, b }` / `foo()` call
			ne := parseBinding(toks, k+1, name.val, name.line)
			if typeName := typeAnnotation(toks, j+1); typeName != "" {
				ne = &NewExpr{Name: name.val, Class: typeName, Line: name.line}
			}
			if ne != nil {
				res.NewExprs = append(res.NewExprs, *ne)
			}
			// arrow-function return shape: `const f = () => ({...})`
			if isArrowFunction(toks, k+1) {
				if fr := arrowReturn(toks, k+1); fr != nil {
					fr.Name = name.val
					res.FuncReturn = append(res.FuncReturn, *fr)
				}
			}
			// const x = require("mod") — record the import too
			if k+3 < len(toks) && toks[k+1].val == "require" && toks[k+2].val == "(" && toks[k+3].kind == tStr {
				if end, imp, ok := p.parseRequire(toks, k+1, file); ok {
					res.Imports = append(res.Imports, imp)
					k = end
				}
			}
			kind := Variable
			if kw == "const" {
				kind = Constant
			}
			if isArrowFunction(toks, k+1) {
				kind = Function
			}
			res.Symbols = append(res.Symbols, Symbol{
				Name:      name.val,
				Kind:      kind,
				FileID:    file.ID,
				Line:      name.line,
				Column:    name.col,
				EndLine:   name.line,
				EndColumn: name.col + len(name.val),
				Exported:  exported,
			})
			j = k + 1
		} else {
			// declaration without initializer
			res.Symbols = append(res.Symbols, Symbol{
				Name:      name.val,
				Kind:      Variable,
				FileID:    file.ID,
				Line:      name.line,
				Column:    name.col,
				EndLine:   name.line,
				EndColumn: name.col + len(name.val),
				Exported:  exported,
			})
			j = k
		}
		// move to next declarator or end of statement
		if j < len(toks) && toks[j].val == "," {
			j++
			continue
		}
		break
	}
	// skip to end of statement — next declaration keyword, `;`, or block end.
	// (No-semicolon code: stop at the next `const`/`function`/..., NOT at any `{`,
	// which would swallow the next object literal and its binding.)
	for j < len(toks) && toks[j].kind != tEOF {
		switch toks[j].val {
		case ";", "}":
			return j + 1
		}
		if toks[j].kind == tIdent {
			switch toks[j].val {
			case "const", "let", "var", "function", "class", "export", "import", "interface", "type", "enum", "async":
				return j
			}
		}
		j++
	}
	return j + 1
}

// isArrowFunction reports whether tokens at j start an arrow function
// initializer: ( ... ) =>, async (...) =>, or identifier =>.
func isArrowFunction(toks []token, j int) bool {
	if j >= len(toks) {
		return false
	}
	if toks[j].kind == tIdent && toks[j].val == "async" {
		return true
	}
	if toks[j].kind == tPunct && toks[j].val == "(" {
		// find matching ) then check =>
		depth := 0
		for k := j; k < len(toks); k++ {
			switch toks[k].val {
			case "(":
				depth++
			case ")":
				depth--
				if depth == 0 {
					return k+1 < len(toks) && toks[k+1].val == "=>"
				}
			}
		}
		return false
	}
	if toks[j].kind == tIdent {
		return j+1 < len(toks) && toks[j+1].val == "=>"
	}
	return false
}

// parseImport handles all ES module import forms plus side-effect imports.
func (p *jsParser) parseImport(toks []token, i int, file *File, res *ParseResult) int {
	j := i + 1
	names := []string{}
	path := ""
	// scan until `from` or a string literal
	last := ""
	for j < len(toks) {
		t := toks[j]
		if t.kind == tStr {
			path = strings.Trim(t.val, `"'`)
			j++
			break
		}
		if t.kind == tIdent {
			if t.val == "from" {
				// next token is the string
				if j+1 < len(toks) && toks[j+1].kind == tStr {
					path = strings.Trim(toks[j+1].val, `"'`)
					j += 2
				}
				break
			}
			if t.val == "as" {
				if last != "*" {
					j++ // skip local alias target
				}
			} else if t.val != "import" && t.val != "default" {
				names = append(names, t.val)
			}
		}
		if t.kind == tEOF {
			break
		}
		last = t.val
		j++
	}
	// end of statement — only needed when no path was found (malformed import).
	// Don't scan forward otherwise: with no semicolons, that would swallow
	// every following statement up to the next `{`.
	if path == "" {
		for j < len(toks) && toks[j].val != ";" && toks[j].val != "}" && toks[j].kind != tEOF {
			j++
		}
	}
	imp := Import{FileID: file.ID, Path: path, Names: names, Line: toks[i].line, Column: toks[i].col}
	res.Imports = append(res.Imports, imp)
	return j + 1
}

// parseRequire handles `require("module")` calls.
func (p *jsParser) parseRequire(toks []token, i int, file *File) (int, Import, bool) {
	if i+2 < len(toks) && toks[i+1].val == "(" && toks[i+2].kind == tStr {
		imp := Import{FileID: file.ID, Path: strings.Trim(toks[i+2].val, `"'`), Line: toks[i].line, Column: toks[i].col}
		j := i + 3
		if j < len(toks) && toks[j].val == ")" {
			j++
		}
		return j, imp, true
	}
	return i, Import{}, false
}

// matchBrace returns the index of the token after the matching `}` for the
// `{` at index open. Falls back to len(toks)-1 on unbalanced input.
func matchBrace(toks []token, open int) int {
	depth := 0
	for j := open; j < len(toks); j++ {
		switch toks[j].val {
		case "{":
			depth++
		case "}":
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return len(toks) - 1
}

// skipToBraceEnd returns the index after the `{...}` block starting after j,
// or j if no block follows.
func skipToBraceEnd(toks []token, j int) int {
	for k := j; k < len(toks); k++ {
		if toks[k].val == "{" {
			return matchBrace(toks, k)
		}
		if toks[k].val == ";" || toks[k].kind == tEOF {
			return k
		}
	}
	return j
}

// isFieldPos reports whether an identifier at this position can start a class
// field declaration (previous significant punctuation allows a name).
func isFieldPos(prevPunct string) bool {
	switch prevPunct {
	case "{", ";", "}", "", "]", ")", ">", "`":
		return true
	}
	return false
}

// isModifier lists class member modifier keywords that are not field names.
func isModifier(s string) bool {
	switch s {
	case "public", "private", "protected", "static", "readonly", "declare",
		"abstract", "override", "async", "get", "set", "accessor", "new", "in":
		return true
	}
	return false
}

// parseBinding inspects a var initializer and returns what it refers to:
// `new Foo()` → Class, `{ a, b }` → Keys, `foo()` → Fn. Nil if none.
func parseBinding(toks []token, r int, name string, line int) *NewExpr {
	if r >= len(toks) {
		return nil
	}
	switch {
	case toks[r].val == "new" && r+1 < len(toks) && toks[r+1].kind == tIdent:
		return &NewExpr{Name: name, Class: toks[r+1].val, Line: line}
	case toks[r].val == "{":
		close := matchBrace(toks, r)
		return &NewExpr{Name: name, Keys: collectKeys(toks, r+1, close-1), Sub: collectSubPaths(toks, r+1, close-1), Line: line}
	case toks[r].val == "[" && r+1 < len(toks) && toks[r+1].val == "{":
		// array of objects `= [{ a, b }, ...]` → element-0 members via `x[0].`
		elemClose := matchBrace(toks, r+1)
		sub := map[string][]string{"[0]": collectKeys(toks, r+2, elemClose-1)}
		for k, v := range collectSubPaths(toks, r+2, elemClose-1) {
			sub["[0]."+k] = v
		}
		return &NewExpr{Name: name, Sub: sub, Line: line}
	case toks[r].kind == tIdent:
		// `foo(...)` call or `x = otherVar` alias
		if r+1 < len(toks) && toks[r+1].val == "(" {
			if isArrowFunction(toks, r+1) {
				return nil // arrow params `(x)=>...`
			}
			return &NewExpr{Name: name, Fn: toks[r].val, Line: line}
		}
		return &NewExpr{Name: name, Alias: toks[r].val, Line: line}
	}
	return nil
}

// typeAnnotation returns the type name after `name:` in `const name: Foo = ...`.
// Primitive/utility types are ignored (they carry no indexable members).
func typeAnnotation(toks []token, afterName int) string {
	if afterName+1 < len(toks) && toks[afterName].val == ":" && toks[afterName+1].kind == tIdent {
		t := toks[afterName+1].val
		switch t {
		case "string", "number", "boolean", "any", "unknown", "void", "never",
			"null", "undefined", "object", "bigint", "symbol", "Record", "Array",
			"Promise", "Map", "Set", "Date", "Error", "Function":
			return ""
		}
		return t
	}
	return ""
}

// funcBodyReturn scans a named function's body for its return shape.
// paramOpen points at the `(` after the function name.
func funcBodyReturn(toks []token, paramOpen int) *FuncReturn {
	if paramOpen >= len(toks) || toks[paramOpen].val != "(" {
		return nil
	}
	close := matchParen(toks, paramOpen)
	body := close + 1
	if body >= len(toks) || toks[body].val != "{" {
		return nil
	}
	end := matchBrace(toks, body)
	return scanReturns(toks, body+1, end-1)
}

// arrowReturn extracts the return shape of `(x) => {...}`, `x => {...}`,
// `async (x) => ...`, and the expression form `(x) => ({...})`.
func arrowReturn(toks []token, start int) *FuncReturn {
	pos := start
	if pos < len(toks) && toks[pos].val == "async" {
		pos++
	}
	if pos >= len(toks) {
		return nil
	}
	if toks[pos].val == "(" {
		pos = matchParen(toks, pos) + 1
	} else if toks[pos].kind == tIdent {
		pos++
	}
	if pos >= len(toks) || toks[pos].val != "=>" {
		return nil
	}
	body := pos + 1
	if body >= len(toks) {
		return nil
	}
	switch {
	case toks[body].val == "{":
		end := matchBrace(toks, body)
		return scanReturns(toks, body+1, end-1)
	case toks[body].val == "(":
		open := body
		close := matchParen(toks, open)
		if open+1 < close && toks[open+1].val == "{" {
			inner := matchBrace(toks, open+1)
			return &FuncReturn{Keys: collectKeys(toks, open+2, inner-1)}
		}
	}
	return nil
}

// scanReturns finds the first top-level `return {...}` or `return new Foo()`.
func scanReturns(toks []token, from, to int) *FuncReturn {
	depth := 0
	for j := from; j < to; j++ {
		t := toks[j]
		if t.kind == tPunct {
			switch t.val {
			case "{", "[", "(":
				depth++
			case "}", "]", ")":
				depth--
			}
			continue
		}
		if t.kind != tIdent || t.val != "return" || depth != 0 {
			continue
		}
		n := j + 1
		if n < to && toks[n].val == "{" {
			c := matchBrace(toks, n)
			return &FuncReturn{Keys: collectKeys(toks, n+1, c-1)}
		}
		if n+1 < to && toks[n].val == "new" && toks[n+1].kind == tIdent {
			return &FuncReturn{Class: toks[n+1].val}
		}
		return nil
	}
	return nil
}

// collectKeys returns top-level keys of an object literal body `from..to`
// (tokens between the braces). `{ a: 1, b, foo() {} }` → [a b foo].
// collectSubPaths: dari body object literal [from,to), map dotted-path → keys.
// `{ a: { b: { c } }, list: [{ x }] }` →
//
//	{"a": ["b"], "a.b": ["c"], "list[0]": ["x"]}
func collectSubPaths(toks []token, from, to int) map[string][]string {
	sub := map[string][]string{}
	var walk func(oLo, oHi int, prefix string)
	walk = func(oLo, oHi int, prefix string) {
		depth := 0
		var cur string
		for i := oLo; i <= oHi; i++ {
			t := toks[i]
			if t.kind != tIdent && t.kind != tPunct {
				continue
			}
			if t.kind == tIdent {
				if depth == 0 && cur == "" {
					cur = t.val
				}
				continue
			}
			switch t.val {
			case "{":
				if depth == 0 && cur != "" && i > oLo && toks[i-1].val == ":" {
					end := matchBrace(toks, i)
					path := prefix + cur
					sub[path] = collectKeys(toks, i+1, end-1)
					walk(i+1, end-1, path+".")
					i = end
					cur = ""
					continue
				}
				depth++
			case "[":
				if depth == 0 && cur != "" && i > oLo && toks[i-1].val == ":" && i+1 < oHi && toks[i+1].val == "{" {
					end := matchBrace(toks, i)
					path := prefix + cur + "[0]"
					sub[path] = collectKeys(toks, i+2, end-1)
					walk(i+2, end-1, path+".")
					i = end
					cur = ""
					continue
				}
				depth++
			case "}", "]", ")":
				depth--
				if depth < 0 {
					depth = 0
				}
			case ",":
				if depth == 0 {
					cur = ""
				}
			}
		}
	}
	walk(from, to, "")
	return sub
}

func collectKeys(toks []token, from, to int) []string {
	var keys []string
	depth := 0
	prev := "{"
	for j := from; j < to; j++ {
		t := toks[j]
		if t.kind == tPunct {
			switch t.val {
			case "{", "[", "(":
				depth++
			case "}", "]", ")":
				depth--
			}
			if depth < 0 {
				depth = 0
			}
			prev = t.val
			continue
		}
		if t.kind == tIdent && depth == 0 && (prev == "{" || prev == ",") {
			keys = append(keys, t.val)
		}
		prev = ""
	}
	return keys
}

// matchParen returns the index of the `)` matching the `(` at open.
func matchParen(toks []token, open int) int {
	depth := 0
	for j := open; j < len(toks); j++ {
		if toks[j].kind != tPunct {
			continue
		}
		switch toks[j].val {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(toks) - 1
}
