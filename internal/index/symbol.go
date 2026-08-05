// Package index implements ForgeADE Workspace Index (FWI): a lightweight
// indexing engine for editor features (go-to-definition, completion, outline)
// without depending on LSP or AI. It does no type checking or semantic
// analysis — those stay in the LSP domain.
package index

// SymbolKind categorizes a symbol (RFC §7).
type SymbolKind int

const (
	Function SymbolKind = iota
	Class
	Interface
	Enum
	Struct
	TypeAlias
	Variable
	Constant
	Method
	Package
)

// String returns the stable lowercase name of the kind.
func (k SymbolKind) String() string {
	switch k {
	case Function:
		return "function"
	case Class:
		return "class"
	case Interface:
		return "interface"
	case Enum:
		return "enum"
	case Struct:
		return "struct"
	case TypeAlias:
		return "type"
	case Variable:
		return "variable"
	case Constant:
		return "constant"
	case Method:
		return "method"
	case Package:
		return "package"
	}
	return "unknown"
}

// Language identifies a programming language (RFC §6).
type Language string

const (
	LangJavaScript Language = "javascript"
	LangTypeScript Language = "typescript"
	LangJSX        Language = "jsx"
	LangTSX        Language = "tsx"
	LangGo         Language = "go"
	LangKotlin     Language = "kotlin"
	LangSwift      Language = "swift"
)

// File is a source file tracked by the index.
type File struct {
	ID       uint32
	Path     string
	Language Language
	Hash     uint64
}

// Symbol is a single indexed declaration (RFC §7).
type Symbol struct {
	ID        string
	Name      string
	Kind      SymbolKind
	FileID    uint32
	File      string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
	Scope     string // enclosing declaration name; empty at top level
	Exported  bool
}

// Import is one import/require statement in a file (RFC §5.3).
type Import struct {
	FileID uint32
	Path   string // module path or relative path
	Names  []string
	Line   int
	Column int
}

// Export is one export statement in a file (RFC §5.3).
type Export struct {
	FileID uint32
	Name   string
	Line   int
	Column int
}

// ParseResult is what a Parser produces for a single file.
// Only symbols are stored in the index; AST is not retained (RFC §5.3).
type ParseResult struct {
	Symbols []Symbol
	Imports []Import
	Exports []Export
	Hash    uint64
}
