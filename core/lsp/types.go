// Package lsp implements a lightweight, lazy-spawned Language Server Protocol
// (LSP) client over JSON-RPC 2.0 stdio for pure-Go native editor integration.
package lsp

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// Position represents a 0-based line and character offset in an LSP document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a start-to-end span in an LSP document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a file URI and range.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies an open text document by URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// VersionedTextDocumentIdentifier identifies a specific version of a document.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentItem is sent in textDocument/didOpen.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentPositionParams is used by hover, completion, definition, etc.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ContentChangeEvent represents a full-document or incremental change.
type ContentChangeEvent struct {
	Text string `json:"text"`
}

// DiagnosticSeverity matches LSP DiagnosticSeverity constants.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// Diagnostic represents an LSP diagnostic from textDocument/publishDiagnostics.
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

// PublishDiagnosticsParams is the payload of textDocument/publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// CompletionItem represents a single LSP completion candidate.
type CompletionItem struct {
	Label            string `json:"label"`
	Kind             int    `json:"kind,omitempty"`
	Detail           string `json:"detail,omitempty"`
	InsertText       string `json:"insertText,omitempty"`
	InsertTextFormat int    `json:"insertTextFormat,omitempty"`
	SortText         string `json:"sortText,omitempty"`
}

// CompletionList is returned by some servers instead of []CompletionItem.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// MarkupContent represents LSP hover documentation.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Hover represents the response to textDocument/hover.
type Hover struct {
	Contents json.RawMessage `json:"contents"`
	Range    *Range          `json:"range,omitempty"`
}

// ExtractHoverText extracts plain/markdown text from LSP Hover.Contents
// (which may be MarkupContent, MarkedString, string, or []MarkedString).
func ExtractHoverText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var mc MarkupContent
	if err := json.Unmarshal(raw, &mc); err == nil && mc.Value != "" {
		return strings.TrimSpace(mc.Value)
	}
	var obj struct {
		Language string `json:"language"`
		Value    string `json:"value"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Value != "" {
		return strings.TrimSpace(obj.Value)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		parts := make([]string, 0, len(arr))
		for _, item := range arr {
			if txt := ExtractHoverText(item); txt != "" {
				parts = append(parts, txt)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// TextEdit represents a single text replacement in a document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit represents edits across multiple files (e.g. textDocument/rename).
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// FormattingOptions configures textDocument/formatting.
type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

// ServerSpec describes a language server binary and its supported extensions.
type ServerSpec struct {
	Name       string
	LanguageID string
	Command    string
	Args       []string
	Extensions []string
}

// DefaultServerSpecs returns built-in language server definitions. Servers are
// spawned lazily only when a matching file is opened and the binary exists.
func DefaultServerSpecs() []ServerSpec {
	return []ServerSpec{
		{
			Name:       "gopls",
			LanguageID: "go",
			Command:    "gopls",
			Args:       []string{"serve"},
			Extensions: []string{".go"},
		},
		{
			Name:       "typescript-language-server",
			LanguageID: "typescript",
			Command:    "typescript-language-server",
			Args:       []string{"--stdio"},
			Extensions: []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"},
		},
		{
			Name:       "kotlin-language-server",
			LanguageID: "kotlin",
			Command:    "kotlin-language-server",
			Args:       nil,
			Extensions: []string{".kt", ".kts"},
		},
		{
			Name:       "sourcekit-lsp",
			LanguageID: "swift",
			Command:    "sourcekit-lsp",
			Args:       nil,
			Extensions: []string{".swift"},
		},
		{
			Name:       "pyright",
			LanguageID: "python",
			Command:    "pyright-langserver",
			Args:       []string{"--stdio"},
			Extensions: []string{".py", ".pyi"},
		},
	}
}

// LanguageIDForPath returns the LSP languageId for a file path.
func LanguageIDForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".mts", ".cts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".py", ".pyi":
		return "python"
	default:
		return "plaintext"
	}
}

// PathToURI converts a local filesystem path into a file:// URI.
func PathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	slashed := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	u := url.URL{Scheme: "file", Path: slashed}
	return u.String()
}

// URIToPath converts a file:// URI back to a local filesystem path.
func URIToPath(rawURI string) string {
	u, err := url.Parse(rawURI)
	if err != nil || u.Scheme != "file" {
		return rawURI
	}
	p := u.Path
	if runtime.GOOS == "windows" && strings.HasPrefix(p, "/") && len(p) > 2 && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}
