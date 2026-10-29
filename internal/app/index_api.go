package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Haslab-dev/RakitKode/index"
	"github.com/Haslab-dev/RakitKode/search"
)

// ---------------------------------------------------------------------------
// Workspace Index (FWI) — RFC-0001
// ---------------------------------------------------------------------------

// IndexStatus reports whether the workspace index is built and its size.
func (a *App) IndexStatus() map[string]interface{} {
	if a.indexStore == nil {
		return map[string]interface{}{"built": false}
	}
	syms := a.indexStore.Symbols()
	filesByLang, symsByLang := a.indexStore.LanguageStats()
	return map[string]interface{}{
		"built":            true,
		"symbols":          len(syms),
		"languages":        filesByLang,
		"symbols_language": symsByLang,
	}
}

// ReindexWorkspace triggers a complete rebuild of the workspace symbol index.
func (a *App) ReindexWorkspace() (map[string]interface{}, error) {
	if a.indexStore == nil {
		if ws := a.workspaceMgr.Current(); ws != nil {
			folders := ws.GetFolders()
			if len(folders) > 0 {
				a.indexStore = index.New(folders[0])
				_ = a.indexStore.Load()
				a.indexUnsub = a.indexStore.Listen(a.bus)
			}
		}
	}
	if a.indexStore == nil {
		return map[string]interface{}{"built": false}, fmt.Errorf("no workspace opened to index")
	}
	if err := a.indexStore.Build(); err != nil {
		return a.IndexStatus(), err
	}
	_ = a.indexStore.Save()
	a.emitEvent("index:status:changed", a.IndexStatus())
	return a.IndexStatus(), nil
}

func (a *App) ensureIndexStore() *index.Store {
	if a.indexStore != nil {
		return a.indexStore
	}
	var root string
	if a.workspaceMgr != nil {
		if ws := a.workspaceMgr.Current(); ws != nil {
			folders := ws.GetFolders()
			if len(folders) > 0 {
				root = folders[0]
			}
		}
	}
	if root == "" {
		if cwd, err := os.Getwd(); err == nil && cwd != "" {
			root = cwd
		}
	}
	if root != "" {
		a.indexStore = index.New(root)
		_ = a.indexStore.Load()
		if a.bus != nil {
			a.indexUnsub = a.indexStore.Listen(a.bus)
		}
		if a.searchMgr != nil {
			a.searchMgr.SetDirectories([]string{root})
		}
		return a.indexStore
	}
	return nil
}

// GetSymbols returns all indexed symbols.
func (a *App) GetSymbols() []index.Symbol {
	idx := a.ensureIndexStore()
	if idx == nil {
		return nil
	}
	return idx.Symbols()
}

// FindSymbol returns declarations matching name exactly (go-to-definition).
func (a *App) FindSymbol(name string) []index.Symbol {
	idx := a.ensureIndexStore()
	if idx == nil {
		return nil
	}
	return idx.Definition(name)
}

// FindDeclaration returns declaration matching name.
func (a *App) FindDeclaration(name string) []index.Symbol {
	idx := a.ensureIndexStore()
	if idx == nil {
		return nil
	}
	return idx.Definition(name)
}

// FindTypeDefinition returns type definitions matching name.
func (a *App) FindTypeDefinition(name string) []index.Symbol {
	idx := a.ensureIndexStore()
	if idx == nil {
		return nil
	}
	all := idx.Find(name)
	types := make([]index.Symbol, 0, len(all))
	for _, s := range all {
		switch s.Kind {
		case index.Class, index.Interface, index.Enum, index.Struct, index.TypeAlias:
			types = append(types, s)
		}
	}
	if len(types) > 0 {
		return types
	}
	return idx.Definition(name)
}

// FindImplementation returns implementations matching name.
func (a *App) FindImplementation(name string) []index.Symbol {
	idx := a.ensureIndexStore()
	if idx == nil {
		return nil
	}
	all := idx.Find(name)
	impls := make([]index.Symbol, 0, len(all))
	for _, s := range all {
		switch s.Kind {
		case index.Function, index.Method, index.Class, index.Struct:
			impls = append(impls, s)
		}
	}
	if len(impls) > 0 {
		return impls
	}
	return idx.Definition(name)
}

// SearchIndexSymbols finds symbols by exact/prefix/camel/fuzzy query (RFC §15).
func (a *App) SearchIndexSymbols(query string) []index.Symbol {
	idx := a.ensureIndexStore()
	if idx == nil {
		return nil
	}
	return idx.Search(query)
}

// GetCompletion returns completion candidates for a prefix (RFC §11),
// scoped to the language of `path` so suggestions never cross languages.
func (a *App) GetCompletion(prefix, path string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Completion(prefix, index.DetectLanguage(path), path)
}

// GetMembers returns member suggestions for `instance.` (RFC §7): class
// members, object-literal keys, or function return shapes — scoped to the
// language of `path`.
func (a *App) GetMembers(instance, path string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Members(instance, string(index.DetectLanguage(path)))
}

// GetOutline returns the symbols declared in a file, sorted by line (RFC §14).
func (a *App) GetOutline(file string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Outline(a.resolveWorkspacePath(file))
}

// GetImports returns the import statements of a file.
func (a *App) GetImports(file string) []index.Import {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Imports(a.resolveWorkspacePath(file))
}

// GetExports returns the export statements of a file.
func (a *App) GetExports(file string) []index.Export {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Exports(a.resolveWorkspacePath(file))
}

// SearchSymbols searches code symbols (functions, types, structs, classes, interfaces).
func (a *App) SearchSymbols(query string, limit int) ([]search.RankedResult, error) {
	return a.searchMgr.SearchSymbols(query, limit)
}

// SearchSymbolsWithOptions searches code symbols with options.
func (a *App) SearchSymbolsWithOptions(opts search.SearchOptions) ([]search.RankedResult, error) {
	return a.searchMgr.SearchSymbolsWithOptions(opts)
}

// DefinitionLocation is the resolved target location for a symbol or import.
type DefinitionLocation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Snippet string `json:"snippet,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

func resolveFilePath(currentDir, root, specifier string) string {
	specifier = strings.TrimSpace(specifier)
	specifier = strings.Trim(specifier, "'\"`")
	if specifier == "" {
		return ""
	}

	var candidates []string

	if strings.HasPrefix(specifier, "@/") || strings.HasPrefix(specifier, "~/") {
		rel := specifier[2:]
		candidates = append(candidates,
			filepath.Join(root, "src", rel),
			filepath.Join(root, "frontend/src", rel),
			filepath.Join(root, rel),
		)
	} else if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") {
		candidates = append(candidates, filepath.Join(currentDir, specifier))
	} else if filepath.IsAbs(specifier) {
		candidates = append(candidates, specifier)
	} else {
		// Go module import check
		if root != "" {
			if modBytes, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
				for _, line := range strings.Split(string(modBytes), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "module ") {
						modName := strings.TrimSpace(strings.TrimPrefix(line, "module"))
						if strings.HasPrefix(specifier, modName+"/") {
							rel := strings.TrimPrefix(specifier, modName+"/")
							candidates = append(candidates, filepath.Join(root, rel))
						}
					}
				}
			}
		}
		// Also try relative to currentDir and root
		candidates = append(candidates,
			filepath.Join(currentDir, specifier),
			filepath.Join(root, specifier),
			filepath.Join(root, "src", specifier),
			filepath.Join(root, "frontend/src", specifier),
		)
	}

	extensions := []string{
		"", ".ts", ".tsx", ".js", ".jsx", ".go", ".py", ".rs", ".json", ".css", ".html", ".vue", ".svelte", ".mjs", ".cjs",
	}
	indexExts := []string{
		"/index.ts", "/index.tsx", "/index.js", "/index.jsx", "/index.mjs",
	}

	for _, cand := range candidates {
		cand = filepath.Clean(cand)
		for _, ext := range extensions {
			target := cand + ext
			if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
				return target
			}
		}
		for _, idx := range indexExts {
			target := filepath.Clean(cand + idx)
			if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
				return target
			}
		}
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			entries, err := os.ReadDir(cand)
			if err == nil {
				var fallback string
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
						return filepath.Join(cand, name)
					}
					if strings.HasPrefix(name, "index.") {
						return filepath.Join(cand, name)
					}
					if fallback == "" && !strings.HasPrefix(name, ".") {
						fallback = filepath.Join(cand, name)
					}
				}
				if fallback != "" {
					return fallback
				}
			}
		}
	}
	return ""
}

func findDefinitionInFile(filePath, symbol string) (int, int, string) {
	if filePath == "" {
		return 1, 1, ""
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 1, 1, ""
	}

	lines := strings.Split(string(content), "\n")
	if symbol == "" || symbol == "default" {
		defRe := regexp.MustCompile(`\bexport\s+default\b`)
		for i, line := range lines {
			if loc := defRe.FindStringIndex(line); loc != nil {
				return i + 1, loc[0] + 1, strings.TrimSpace(line)
			}
		}
		return 1, 1, ""
	}

	escaped := regexp.QuoteMeta(symbol)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\bexport\s+(?:default\s+)?(?:const|let|var|function|class|interface|type|enum)\s+` + escaped + `\b`),
		regexp.MustCompile(`\bexport\s+default\s+(?:function|class)?\s*` + escaped + `\b`),
		regexp.MustCompile(`\b(?:const|let|var|function|class|interface|type|enum)\s+` + escaped + `\b`),
		regexp.MustCompile(`\bfunc\s+(?:\([^)]+\)\s+)?` + escaped + `\b`),
		regexp.MustCompile(`\btype\s+` + escaped + `\b`),
		regexp.MustCompile(`\b(?:var|const)\s+` + escaped + `\b`),
		regexp.MustCompile(`\b` + escaped + `\s*:=`),
		regexp.MustCompile(`\b` + escaped + `\s*=[^=>]`),
		regexp.MustCompile(`\b` + escaped + `\s*=$`),
		regexp.MustCompile(`\b(?:def|class)\s+` + escaped + `\b`),
		regexp.MustCompile(`\b(?:fn|struct|enum|trait|type)\s+` + escaped + `\b`),
		regexp.MustCompile(`\b` + escaped + `\b`),
	}

	for i, line := range lines {
		for _, re := range patterns {
			if loc := re.FindStringIndex(line); loc != nil {
				return i + 1, loc[0] + 1, strings.TrimSpace(line)
			}
		}
	}

	// Fallback to export default line if present
	defRe := regexp.MustCompile(`\bexport\s+default\b`)
	for i, line := range lines {
		if loc := defRe.FindStringIndex(line); loc != nil {
			return i + 1, loc[0] + 1, strings.TrimSpace(line)
		}
	}

	return 1, 1, ""
}

// ResolveDefinition resolves an import path or symbol to its defining file and line.
func (a *App) ResolveDefinition(currentFile, query string) *DefinitionLocation {
	query = strings.TrimSpace(query)
	query = strings.Trim(query, "'\"`")
	if query == "" {
		return nil
	}

	var root string
	if a.workspaceMgr != nil {
		if ws := a.workspaceMgr.Current(); ws != nil {
			folders := ws.GetFolders()
			if len(folders) > 0 {
				root = folders[0]
			}
		}
	}
	if root == "" {
		if currentFile != "" {
			dir := filepath.Dir(currentFile)
			for d := dir; d != "/" && d != "."; d = filepath.Dir(d) {
				if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
					root = d
					break
				}
				if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
					root = d
					break
				}
				if _, err := os.Stat(filepath.Join(d, "package.json")); err == nil {
					root = d
					break
				}
			}
		}
	}
	if root == "" {
		root, _ = os.Getwd()
	}

	currentDir := ""
	if currentFile != "" {
		currentDir = filepath.Dir(currentFile)
	} else {
		currentDir = root
	}

	// 1. Direct path check: if query looks like a path (contains /, starts with ., @, ~)
	if strings.Contains(query, "/") || strings.HasPrefix(query, ".") || strings.HasPrefix(query, "@") || strings.HasPrefix(query, "~") {
		if resolved := resolveFilePath(currentDir, root, query); resolved != "" {
			return &DefinitionLocation{
				File:   resolved,
				Line:   1,
				Column: 1,
				Kind:   "file",
			}
		}
	}

	// 2. Check import statements in currentFile (multi-line aware)
	if currentFile != "" {
		if contentBytes, err := os.ReadFile(currentFile); err == nil {
			contentStr := string(contentBytes)

			var matchedSpecifier string
			var targetSymbol string

			// 2a. JS/TS Multi-line & Single-line imports & re-exports:
			// import ... from '...'
			// export ... from '...'
			jsImportRe := regexp.MustCompile(`(?s)\b(?:import|export)\s+(?:type\s+)?([\s\S]*?)\s+from\s+['"]([^'"]+)['"]`)
			for _, matches := range jsImportRe.FindAllStringSubmatch(contentStr, -1) {
				clause := matches[1]
				spec := matches[2]

				// Check alias: orig as query (e.g. ListAutomations as WailsListAutomations)
				aliasRe := regexp.MustCompile(`\b([a-zA-Z0-9_$]+)\s+as\s+` + regexp.QuoteMeta(query) + `\b`)
				if m := aliasRe.FindStringSubmatch(clause); len(m) == 2 {
					matchedSpecifier = spec
					targetSymbol = m[1]
					break
				}

				// Check orig: query as alias
				origRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(query) + `\s+as\s+([a-zA-Z0-9_$]+)\b`)
				if origRe.MatchString(clause) {
					matchedSpecifier = spec
					targetSymbol = query
					break
				}

				// Check namespace: * as query
				nsRe := regexp.MustCompile(`\*\s+as\s+` + regexp.QuoteMeta(query) + `\b`)
				if nsRe.MatchString(clause) {
					matchedSpecifier = spec
					targetSymbol = ""
					break
				}

				// Check named or default identifier in clause
				if query != "type" && query != "as" && query != "from" && query != "import" && query != "export" {
					wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(query) + `\b`)
					if wordRe.MatchString(clause) {
						matchedSpecifier = spec
						targetSymbol = query
						break
					}
				}

				// Check if query is specifier or its basename
				base := filepath.Base(spec)
				nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
				if query == spec || query == base || query == nameWithoutExt {
					matchedSpecifier = spec
					targetSymbol = query
					break
				}
			}

			// 2b. JS/TS Side-effect imports: import '...'
			if matchedSpecifier == "" {
				sideImportRe := regexp.MustCompile(`\bimport\s+['"]([^'"]+)['"]`)
				for _, matches := range sideImportRe.FindAllStringSubmatch(contentStr, -1) {
					spec := matches[1]
					base := filepath.Base(spec)
					nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
					if query == spec || query == base || query == nameWithoutExt {
						matchedSpecifier = spec
						targetSymbol = ""
						break
					}
				}
			}

			// 2c. CommonJS require: const ... = require('...')
			if matchedSpecifier == "" {
				reqRe := regexp.MustCompile(`(?s)(?:const|let|var)\s+([\s\S]*?)\s*=\s*require\(['"]([^'"]+)['"]\)`)
				for _, matches := range reqRe.FindAllStringSubmatch(contentStr, -1) {
					clause := matches[1]
					spec := matches[2]

					aliasRe := regexp.MustCompile(`\b([a-zA-Z0-9_$]+)\s*:\s*` + regexp.QuoteMeta(query) + `\b`)
					if m := aliasRe.FindStringSubmatch(clause); len(m) == 2 {
						matchedSpecifier = spec
						targetSymbol = m[1]
						break
					}

					wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(query) + `\b`)
					if wordRe.MatchString(clause) {
						matchedSpecifier = spec
						targetSymbol = query
						break
					}

					base := filepath.Base(spec)
					nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
					if query == spec || query == base || query == nameWithoutExt {
						matchedSpecifier = spec
						targetSymbol = query
						break
					}
				}
			}

			// 2d. Python imports: from ... import ...
			if matchedSpecifier == "" {
				pyFromRe := regexp.MustCompile(`(?s)\bfrom\s+([a-zA-Z0-9_.]+)\s+import\s+([\s\S]*?)(?:\n[a-zA-Z0-9_#]|\z)`)
				for _, matches := range pyFromRe.FindAllStringSubmatch(contentStr, -1) {
					spec := matches[1]
					clause := matches[2]

					aliasRe := regexp.MustCompile(`\b([a-zA-Z0-9_]+)\s+as\s+` + regexp.QuoteMeta(query) + `\b`)
					if m := aliasRe.FindStringSubmatch(clause); len(m) == 2 {
						matchedSpecifier = strings.ReplaceAll(spec, ".", "/")
						targetSymbol = m[1]
						break
					}

					wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(query) + `\b`)
					if wordRe.MatchString(clause) {
						matchedSpecifier = strings.ReplaceAll(spec, ".", "/")
						targetSymbol = query
						break
					}

					if query == spec || query == filepath.Base(strings.ReplaceAll(spec, ".", "/")) {
						matchedSpecifier = strings.ReplaceAll(spec, ".", "/")
						targetSymbol = ""
						break
					}
				}
			}

			if matchedSpecifier == "" {
				pyImpRe := regexp.MustCompile(`\bimport\s+([a-zA-Z0-9_.]+)(?:\s+as\s+([a-zA-Z0-9_]+))?`)
				for _, matches := range pyImpRe.FindAllStringSubmatch(contentStr, -1) {
					mod := matches[1]
					alias := matches[2]
					base := filepath.Base(strings.ReplaceAll(mod, ".", "/"))
					if (alias != "" && alias == query) || mod == query || base == query {
						matchedSpecifier = strings.ReplaceAll(mod, ".", "/")
						targetSymbol = ""
						break
					}
				}
			}

			// 2e. Go imports
			if matchedSpecifier == "" && strings.HasSuffix(currentFile, ".go") {
				goImpRe := regexp.MustCompile(`(?:([a-zA-Z0-9_]+)\s+)?"([^"]+)"`)
				for _, matches := range goImpRe.FindAllStringSubmatch(contentStr, -1) {
					alias := matches[1]
					path := matches[2]
					lastComp := filepath.Base(path)
					if (alias != "" && alias == query) || (alias == "" && lastComp == query) || path == query {
						matchedSpecifier = path
						targetSymbol = ""
						break
					}
				}
			}

			if matchedSpecifier != "" {
				resolved := resolveFilePath(currentDir, root, matchedSpecifier)
				if resolved != "" {
					lineNo := 1
					colNo := 1
					var snippet string
					if targetSymbol != "" {
						lineNo, colNo, snippet = findDefinitionInFile(resolved, targetSymbol)
						fromQuoteRe := regexp.MustCompile(`\bfrom\s+['"][^'"]+['"]`)
						isForwardingLine := fromQuoteRe.MatchString(snippet) && (strings.HasPrefix(strings.TrimSpace(snippet), "export") || strings.HasPrefix(strings.TrimSpace(snippet), "import"))
						if (snippet == "" || isForwardingLine) && (strings.HasSuffix(resolved, ".ts") || strings.HasSuffix(resolved, ".tsx") || strings.HasSuffix(resolved, ".js") || strings.HasSuffix(resolved, ".jsx")) {
							if subDef := a.ResolveDefinition(resolved, targetSymbol); subDef != nil && subDef.File != "" && subDef.File != resolved {
								return subDef
							}
						}
					}
					return &DefinitionLocation{
						File:    resolved,
						Line:    lineNo,
						Column:  colNo,
						Snippet: snippet,
						Kind:    "import",
					}
				}
			}
		}
	}

	// 3. Sibling files in same directory (especially for Go packages or same-folder JS/TS/Python)
	if currentDir != "" {
		if entries, err := os.ReadDir(currentDir); err == nil {
			isGo := strings.HasSuffix(currentFile, ".go")
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				fullPath := filepath.Join(currentDir, name)
				if fullPath == currentFile {
					continue
				}
				if isGo && !strings.HasSuffix(name, ".go") {
					continue
				}
				lineNo, colNo, snippet := findDefinitionInFile(fullPath, query)
				if snippet != "" {
					return &DefinitionLocation{
						File:    fullPath,
						Line:    lineNo,
						Column:  colNo,
						Snippet: snippet,
						Kind:    "definition",
					}
				}
			}
		}
	}

	// 4. RakitKode Index fallback
	if idx := a.ensureIndexStore(); idx != nil {
		defs := idx.Definition(query)
		for _, d := range defs {
			filePath := d.File
			if filePath != "" {
				if !filepath.IsAbs(filePath) && root != "" {
					filePath = filepath.Join(root, filePath)
				}
				if fi, err := os.Stat(filePath); err == nil && !fi.IsDir() {
					return &DefinitionLocation{
						File:   filePath,
						Line:   d.Line,
						Column: d.Column,
						Kind:   "index",
					}
				}
			}
		}
	}

	// 5. Workspace search fallback
	if a.searchMgr != nil && root != "" {
		matches, err := a.searchMgr.SearchContent(query, 25)
		if err == nil {
			defRe := regexp.MustCompile(`\b(?:export\s+)?(?:default\s+)?(?:function|class|interface|type|enum|const|let|var|def|func|struct)\s+(?:(?:\([^)]+\)\s+)?` + regexp.QuoteMeta(query) + `\b`)
			for _, m := range matches {
				if defRe.MatchString(m.Content) {
					targetPath := m.Path
					if !filepath.IsAbs(targetPath) {
						targetPath = filepath.Join(root, targetPath)
					}
					return &DefinitionLocation{
						File:    targetPath,
						Line:    m.Line,
						Column:  1,
						Snippet: strings.TrimSpace(m.Content),
						Kind:    "search",
					}
				}
			}
		}
	}

	return nil
}

// ResolveImportPath resolves an import path or specifier relative to currentFile.
func (a *App) ResolveImportPath(currentFile, importPath string) string {
	var root string
	if a.workspaceMgr != nil {
		if ws := a.workspaceMgr.Current(); ws != nil {
			folders := ws.GetFolders()
			if len(folders) > 0 {
				root = folders[0]
			}
		}
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	currentDir := filepath.Dir(currentFile)
	return resolveFilePath(currentDir, root, importPath)
}
