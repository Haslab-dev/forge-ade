package main

import (
	"fmt"

	"github.com/hasdev/forge-ade/internal/index"
	"github.com/hasdev/forge-ade/internal/search"
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

// GetSymbols returns all indexed symbols.
func (a *App) GetSymbols() []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Symbols()
}

// FindSymbol returns declarations matching name exactly (go-to-definition).
func (a *App) FindSymbol(name string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Definition(name)
}

// SearchIndexSymbols finds symbols by exact/prefix/camel/fuzzy query (RFC §15).
func (a *App) SearchIndexSymbols(query string) []index.Symbol {
	if a.indexStore == nil {
		return nil
	}
	return a.indexStore.Search(query)
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
