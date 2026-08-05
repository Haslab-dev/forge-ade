package index

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Parser extracts symbols, imports and exports from a single source file
// (RFC §5.3). Parsers must be stateless: Parse may be called concurrently.
// New languages plug in via Register without touching the core indexer.
type Parser interface {
	Language() Language
	Parse(src []byte, file *File) (*ParseResult, error)
}

var registry = map[Language]Parser{}

// Register adds a parser for a language. Last registration wins.
func Register(p Parser) {
	registry[p.Language()] = p
}

// ForLang returns the parser registered for lang, or nil.
func ForLang(lang Language) Parser {
	return registry[lang]
}

// DetectLanguage maps a file path to a language, or "" if unsupported.
func DetectLanguage(path string) Language {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs":
		return LangJavaScript
	case ".jsx":
		return LangJSX
	case ".ts":
		return LangTypeScript
	case ".tsx":
		return LangTSX
	case ".go":
		return LangGo
	case ".kt", ".kts":
		return LangKotlin
	case ".swift":
		return LangSwift
	}
	return ""
}

// Parse dispatches to the registered parser for file's language.
func Parse(src []byte, file *File) (*ParseResult, error) {
	p := ForLang(file.Language)
	if p == nil {
		return nil, fmt.Errorf("index: no parser for language %q", file.Language)
	}
	return p.Parse(src, file)
}

func init() {
	Register(&jsParser{lang: LangJavaScript})
	Register(&jsParser{lang: LangTypeScript})
	Register(&jsParser{lang: LangJSX})
	Register(&jsParser{lang: LangTSX})
}
