package index

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
			}
			prevPunct = ""
		case t.kind == tPunct:
			prevPunct = t.val
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
		// skip name and optional type annotation or destructuring
		k := j + 1
		for k < len(toks) && toks[k].val != "=" && toks[k].val != "," && toks[k].val != ";" && toks[k].val != "{" && toks[k].val != "[" && toks[k].val != ")" && toks[k].val != "}" && toks[k].kind != tEOF {
			k++
		}
		if k < len(toks) && toks[k].val == "=" {
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
	// skip to end of statement (; or next declaration start)
	for j < len(toks) && toks[j].val != ";" && toks[j].val != "}" && toks[j].val != "{" && toks[j].kind != tEOF {
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
			path = t.val
			j++
			break
		}
		if t.kind == tIdent {
			if t.val == "from" {
				// next token is the string
				if j+1 < len(toks) && toks[j+1].kind == tStr {
					path = toks[j+1].val
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
	// end of statement
	for j < len(toks) && toks[j].val != ";" && toks[j].val != "}" && toks[j].kind != tEOF {
		j++
	}
	imp := Import{FileID: file.ID, Path: path, Names: names, Line: toks[i].line, Column: toks[i].col}
	res.Imports = append(res.Imports, imp)
	return j + 1
}

// parseRequire handles `require("module")` calls.
func (p *jsParser) parseRequire(toks []token, i int, file *File) (int, Import, bool) {
	if i+2 < len(toks) && toks[i+1].val == "(" && toks[i+2].kind == tStr {
		imp := Import{FileID: file.ID, Path: toks[i+2].val, Line: toks[i].line, Column: toks[i].col}
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
