package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Haslab-dev/RakitKode/index"
)

// indexTool produces a compact skeleton of a source file: imports, types,
// and function signatures with their exact line bounds [start-end].
// This saves ~70-90% of context tokens compared to reading full files.
func indexTool() ToolSpec {
	return ToolSpec{
		Name: "index",
		Description: "Return a compact overview of a source file: imports, type definitions, structs, " +
			"interfaces, and function signatures with exact line numbers [start-end]. " +
			"Always use this FIRST on unfamiliar source files before reading with start_line/end_line.",
		Cost: "cheap",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory path to index",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			path := resolveToolPath(ctx, argPath(args))
			if path == "" {
				return nil, fmt.Errorf("path is required")
			}

			fi, err := os.Stat(path)
			if err != nil {
				return nil, fmt.Errorf("failed to stat %s: %w", path, err)
			}

			if fi.IsDir() {
				return indexDirectory(path)
			}

			if fi.Size() > 5*1024*1024 {
				return nil, fmt.Errorf("file %s is too large (%d bytes) to index; use read with start_line/end_line instead", path, fi.Size())
			}

			data, err := cachedReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", path, err)
			}

			skeleton, totalLines, err := GenerateSkeleton(path, []byte(data))
			if err != nil {
				return nil, fmt.Errorf("failed to generate skeleton for %s: %w", path, err)
			}

			return toolResult(map[string]any{
				"path":        path,
				"total_lines": totalLines,
				"skeleton":    skeleton,
			}), nil
		},
	}
}

// GenerateSkeleton returns a formatted skeleton string and total line count for a file.
func GenerateSkeleton(path string, src []byte) (string, int, error) {
	lines := strings.Split(string(src), "\n")
	totalLines := len(lines)
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return indexGoFile(path, src, totalLines)
	case ".md", ".markdown":
		return indexMarkdownFile(path, lines, totalLines)
	default:
		// Try ForgeADE's built-in language parsers (JS/TS/Python/Java/Swift/etc.)
		lang := index.DetectLanguage(path)
		if lang != "" {
			parsed, err := index.Parse(src, &index.File{Path: path, Language: lang})
			if err == nil && parsed != nil && (len(parsed.Symbols) > 0 || len(parsed.Imports) > 0) {
				return formatParsedSkeleton(path, string(lang), parsed, lines, totalLines), totalLines, nil
			}
		}
		// Fallback line scanner for other languages (Rust, C++, etc.)
		return indexFallback(path, lines, totalLines)
	}
}

func indexGoFile(path string, src []byte, totalLines int) (string, int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		// If syntax error, fall back to line scanner
		lines := strings.Split(string(src), "\n")
		return indexFallback(path, lines, totalLines)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Index: %s (%d lines)\n", filepath.Base(path), totalLines))
	sb.WriteString(fmt.Sprintf("package %s\n\n", f.Name.Name))

	// Imports
	if len(f.Imports) > 0 {
		minLine, maxLine := 9999999, 0
		for _, imp := range f.Imports {
			pos := fset.Position(imp.Pos()).Line
			end := fset.Position(imp.End()).Line
			if pos < minLine {
				minLine = pos
			}
			if end > maxLine {
				maxLine = end
			}
		}
		sb.WriteString(fmt.Sprintf("imports [%d-%d]:\n", minLine, maxLine))
		for _, imp := range f.Imports {
			line := fset.Position(imp.Pos()).Line
			pathVal := imp.Path.Value
			if imp.Name != nil {
				sb.WriteString(fmt.Sprintf("  %s %s [%d]\n", imp.Name.Name, pathVal, line))
			} else {
				sb.WriteString(fmt.Sprintf("  %s [%d]\n", pathVal, line))
			}
		}
		sb.WriteString("\n")
	}

	// Decls: Types, Constants, Functions, Methods
	var types []string
	var funcs []string
	var consts []string

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			start := fset.Position(d.Pos()).Line
			end := fset.Position(d.End()).Line
			switch d.Tok {
			case token.TYPE:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						sStart := fset.Position(ts.Pos()).Line
						sEnd := fset.Position(ts.End()).Line
						if sStart == sEnd && sStart == start {
							sEnd = end
						}
						kind := "type"
						switch ts.Type.(type) {
						case *ast.StructType:
							kind = "struct"
						case *ast.InterfaceType:
							kind = "interface"
						}
						types = append(types, fmt.Sprintf("  type %s %s [%d-%d]", ts.Name.Name, kind, sStart, sEnd))
					}
				}
			case token.CONST:
				if len(d.Specs) > 0 {
					var names []string
					for _, spec := range d.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, n := range vs.Names {
								names = append(names, n.Name)
							}
						}
					}
					if len(names) > 0 {
						consts = append(consts, fmt.Sprintf("  const (%s) [%d-%d]", strings.Join(names, ", "), start, end))
					}
				}
			}
		case *ast.FuncDecl:
			start := fset.Position(d.Pos()).Line
			end := fset.Position(d.End()).Line
			sig := formatFuncSig(d)
			funcs = append(funcs, fmt.Sprintf("  %s [%d-%d]", sig, start, end))
		}
	}

	if len(types) > 0 {
		sb.WriteString("types:\n")
		for _, t := range types {
			sb.WriteString(t + "\n")
		}
		sb.WriteString("\n")
	}

	if len(consts) > 0 {
		sb.WriteString("constants:\n")
		for _, c := range consts {
			sb.WriteString(c + "\n")
		}
		sb.WriteString("\n")
	}

	if len(funcs) > 0 {
		sb.WriteString("functions & methods:\n")
		for _, fn := range funcs {
			sb.WriteString(fn + "\n")
		}
	}

	return strings.TrimSpace(sb.String()), totalLines, nil
}

func formatFuncSig(d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		sb.WriteString("(")
		recv := d.Recv.List[0]
		if len(recv.Names) > 0 {
			sb.WriteString(recv.Names[0].Name + " ")
		}
		sb.WriteString(typeString(recv.Type))
		sb.WriteString(") ")
	}
	sb.WriteString(d.Name.Name)
	sb.WriteString("(")
	if d.Type.Params != nil {
		var params []string
		for _, p := range d.Type.Params.List {
			pType := typeString(p.Type)
			if len(p.Names) == 0 {
				params = append(params, pType)
			} else {
				var names []string
				for _, n := range p.Names {
					names = append(names, n.Name)
				}
				params = append(params, fmt.Sprintf("%s %s", strings.Join(names, ", "), pType))
			}
		}
		sb.WriteString(strings.Join(params, ", "))
	}
	sb.WriteString(")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		sb.WriteString(" ")
		if len(d.Type.Results.List) == 1 && len(d.Type.Results.List[0].Names) == 0 {
			sb.WriteString(typeString(d.Type.Results.List[0].Type))
		} else {
			var results []string
			for _, r := range d.Type.Results.List {
				rType := typeString(r.Type)
				if len(r.Names) == 0 {
					results = append(results, rType)
				} else {
					for _, n := range r.Names {
						results = append(results, fmt.Sprintf("%s %s", n.Name, rType))
					}
				}
			}
			sb.WriteString("(" + strings.Join(results, ", ") + ")")
		}
	}
	return sb.String()
}

func typeString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", typeString(t.Key), typeString(t.Value))
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + typeString(t.Elt)
	case *ast.FuncType:
		return "func(...)"
	default:
		return "any"
	}
}

func formatParsedSkeleton(path string, lang string, res *index.ParseResult, lines []string, totalLines int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Index: %s (%s, %d lines)\n", filepath.Base(path), lang, totalLines))

	if len(res.Imports) > 0 {
		sb.WriteString("imports:\n")
		for _, imp := range res.Imports {
			if len(imp.Names) > 0 {
				sb.WriteString(fmt.Sprintf("  %s from %s [%d]\n", strings.Join(imp.Names, ", "), imp.Path, imp.Line))
			} else {
				sb.WriteString(fmt.Sprintf("  %s [%d]\n", imp.Path, imp.Line))
			}
		}
		sb.WriteString("\n")
	}

	var types []index.Symbol
	var funcs []index.Symbol
	var vars []index.Symbol

	for _, sym := range res.Symbols {
		switch sym.Kind {
		case index.Class, index.Interface, index.Struct, index.Enum, index.TypeAlias:
			types = append(types, sym)
		case index.Function, index.Method:
			funcs = append(funcs, sym)
		case index.Constant, index.Variable:
			vars = append(vars, sym)
		}
	}

	if len(types) > 0 {
		sb.WriteString("types & classes:\n")
		for _, sym := range types {
			rangeStr := formatRange(sym.Line, sym.EndLine)
			exportPrefix := ""
			if sym.Exported {
				exportPrefix = "export "
			}
			sb.WriteString(fmt.Sprintf("  %s%s %s %s\n", exportPrefix, sym.Kind.String(), sym.Name, rangeStr))
		}
		sb.WriteString("\n")
	}

	if len(funcs) > 0 {
		sb.WriteString("functions & methods:\n")
		for _, sym := range funcs {
			rangeStr := formatRange(sym.Line, sym.EndLine)
			scopePrefix := ""
			if sym.Scope != "" {
				scopePrefix = sym.Scope + "."
			}
			exportPrefix := ""
			if sym.Exported {
				exportPrefix = "export "
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s %s\n", exportPrefix, scopePrefix, sym.Name, rangeStr))
		}
		sb.WriteString("\n")
	}

	if len(vars) > 0 {
		sb.WriteString("constants & variables:\n")
		for _, sym := range vars {
			sb.WriteString(fmt.Sprintf("  %s %s [%d]\n", sym.Kind.String(), sym.Name, sym.Line))
		}
	}

	return strings.TrimSpace(sb.String())
}

func formatRange(start, end int) string {
	if start <= 0 {
		return ""
	}
	if end > start {
		return fmt.Sprintf("[%d-%d]", start, end)
	}
	return fmt.Sprintf("[%d]", start)
}

func indexMarkdownFile(path string, lines []string, totalLines int) (string, int, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Outline: %s (%d lines)\n\n", filepath.Base(path), totalLines))
	found := 0
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "#") {
			sb.WriteString(fmt.Sprintf("%s [%d]\n", trimmed, i+1))
			found++
		}
	}
	if found == 0 {
		sb.WriteString("(no markdown headings found)\n")
	}
	return strings.TrimSpace(sb.String()), totalLines, nil
}

func indexFallback(path string, lines []string, totalLines int) (string, int, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Skeleton: %s (%d lines)\n\n", filepath.Base(path), totalLines))

	var decls []string
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Match common declaration keywords across Rust, C++, Python, Shell
		if strings.HasPrefix(trimmed, "fn ") ||
			strings.HasPrefix(trimmed, "pub fn ") ||
			strings.HasPrefix(trimmed, "pub async fn ") ||
			strings.HasPrefix(trimmed, "struct ") ||
			strings.HasPrefix(trimmed, "pub struct ") ||
			strings.HasPrefix(trimmed, "enum ") ||
			strings.HasPrefix(trimmed, "pub enum ") ||
			strings.HasPrefix(trimmed, "trait ") ||
			strings.HasPrefix(trimmed, "pub trait ") ||
			strings.HasPrefix(trimmed, "impl ") ||
			strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "def ") ||
			strings.HasPrefix(trimmed, "async def ") {
			if idx := strings.Index(trimmed, "{"); idx > 0 {
				trimmed = strings.TrimSpace(trimmed[:idx])
			}
			decls = append(decls, fmt.Sprintf("  %s [%d]", trimmed, i+1))
		}
	}

	if len(decls) > 0 {
		sb.WriteString("declarations:\n")
		for _, d := range decls {
			sb.WriteString(d + "\n")
		}
	} else {
		// If no declarations found, show top 20 lines
		limit := 20
		if totalLines < limit {
			limit = totalLines
		}
		sb.WriteString(fmt.Sprintf("preview (lines 1-%d):\n", limit))
		for i := 0; i < limit; i++ {
			sb.WriteString(fmt.Sprintf("%4d: %s\n", i+1, lines[i]))
		}
	}

	return strings.TrimSpace(sb.String()), totalLines, nil
}

func indexDirectory(path string) (any, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", path, err)
	}

	var dirs []string
	var files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Directory: %s (%d entries)\n\n", filepath.Base(path), len(dirs)+len(files)))
	if len(dirs) > 0 {
		sb.WriteString("directories:\n")
		for _, d := range dirs {
			sb.WriteString("  " + d + "\n")
		}
		sb.WriteString("\n")
	}
	if len(files) > 0 {
		sb.WriteString("files:\n")
		for _, f := range files {
			sb.WriteString("  " + f + "\n")
		}
	}

	return toolResult(map[string]any{
		"path":     path,
		"type":     "directory",
		"skeleton": strings.TrimSpace(sb.String()),
		"count":    len(dirs) + len(files),
	}), nil
}
