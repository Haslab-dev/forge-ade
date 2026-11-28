package lsp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultDidChangeDebounce = 150 * time.Millisecond
	maxCompletionItems       = 100
)

type serverKey struct {
	root string
	name string
}

type docSyncState struct {
	root    string
	version int
	timer   *time.Timer
}

// Manager coordinates lazy-spawned language servers per workspace and language.
// No servers are started until a matching file is opened.
type Manager struct {
	mu          sync.Mutex
	specs       []ServerSpec
	extMap      map[string]ServerSpec
	clients     map[serverKey]*Client
	starting    map[serverKey]chan struct{}
	missingBins map[string]bool
	binCache    map[string]string
	docs        map[string]*docSyncState

	// OnDiagnostics is invoked on a background goroutine when a server
	// publishes diagnostics for a local file path.
	OnDiagnostics func(path string, diags []Diagnostic)
}

// NewManager creates an LSP Manager with the default language server registry.
func NewManager() *Manager {
	specs := DefaultServerSpecs()
	extMap := make(map[string]ServerSpec, 32)
	for _, s := range specs {
		for _, ext := range s.Extensions {
			extMap[strings.ToLower(ext)] = s
		}
	}
	return &Manager{
		specs:       specs,
		extMap:      extMap,
		clients:     make(map[serverKey]*Client),
		starting:    make(map[serverKey]chan struct{}),
		missingBins: make(map[string]bool),
		binCache:    make(map[string]string),
		docs:        make(map[string]*docSyncState),
	}
}

// RegisterClient injects a pre-built Client (used in tests).
func (m *Manager) RegisterClient(rootDir, serverName string, c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[serverKey{root: rootDir, name: serverName}] = c
}

// SupportsPath reports whether any configured server handles filePath's extension.
func (m *Manager) SupportsPath(filePath string) bool {
	if m == nil || filePath == "" || strings.HasPrefix(filePath, "ssh://") || strings.HasPrefix(filePath, "untitled://") {
		return false
	}
	_, ok := m.extMap[strings.ToLower(filepath.Ext(filePath))]
	return ok
}

func (m *Manager) specForPath(filePath string) (ServerSpec, bool) {
	if m == nil || filePath == "" || strings.HasPrefix(filePath, "ssh://") || strings.HasPrefix(filePath, "untitled://") {
		return ServerSpec{}, false
	}
	s, ok := m.extMap[strings.ToLower(filepath.Ext(filePath))]
	return s, ok
}

func (m *Manager) resolveBinary(cmd string) string {
	m.mu.Lock()
	if m.missingBins[cmd] {
		m.mu.Unlock()
		return ""
	}
	if p, ok := m.binCache[cmd]; ok {
		m.mu.Unlock()
		return p
	}
	m.mu.Unlock()

	found := ""
	if p, err := exec.LookPath(cmd); err == nil {
		found = p
	} else {
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, "go", "bin", cmd),
			filepath.Join(home, ".cargo", "bin", cmd),
			filepath.Join(home, ".bun", "bin", cmd),
			filepath.Join("/opt/homebrew/bin", cmd),
			filepath.Join("/usr/local/bin", cmd),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				found = c
				break
			}
		}
	}

	m.mu.Lock()
	if found == "" {
		m.missingBins[cmd] = true
	} else {
		m.binCache[cmd] = found
	}
	m.mu.Unlock()
	return found
}

// ensureClient returns an alive Client for (rootDir, filePath), spawning it
// lazily if needed (and coalescing concurrent startup calls).
func (m *Manager) ensureClient(ctx context.Context, rootDir, filePath string) *Client {
	spec, ok := m.specForPath(filePath)
	if !ok {
		return nil
	}
	if rootDir == "" {
		rootDir = filepath.Dir(filePath)
	}
	key := serverKey{root: rootDir, name: spec.Name}

	for {
		m.mu.Lock()
		if c := m.clients[key]; c != nil {
			if c.Alive() {
				m.mu.Unlock()
				return c
			}
			delete(m.clients, key)
		}
		if waitCh := m.starting[key]; waitCh != nil {
			m.mu.Unlock()
			select {
			case <-waitCh:
				continue
			case <-ctx.Done():
				return nil
			}
		}
		waitCh := make(chan struct{})
		m.starting[key] = waitCh
		m.mu.Unlock()

		bin := m.resolveBinary(spec.Command)
		var client *Client
		if bin != "" {
			if c, err := StartProcessClient(ctx, bin, spec.Args, rootDir, m.handleNotification); err == nil {
				client = c
			}
		}

		m.mu.Lock()
		if client != nil {
			m.clients[key] = client
		}
		delete(m.starting, key)
		close(waitCh)
		m.mu.Unlock()
		return client
	}
}

func (m *Manager) existingClient(rootDir, filePath string) *Client {
	spec, ok := m.specForPath(filePath)
	if !ok {
		return nil
	}
	if rootDir == "" {
		rootDir = filepath.Dir(filePath)
	}
	m.mu.Lock()
	c := m.clients[serverKey{root: rootDir, name: spec.Name}]
	m.mu.Unlock()
	if c != nil && !c.Alive() {
		return nil
	}
	return c
}

func (m *Manager) handleNotification(method string, params json.RawMessage) {
	if method != "textDocument/publishDiagnostics" || m.OnDiagnostics == nil {
		return
	}
	var p PublishDiagnosticsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	path := URIToPath(p.URI)
	m.OnDiagnostics(path, p.Diagnostics)
}

// DidOpen notifies (and lazily starts) the language server asynchronously.
func (m *Manager) DidOpen(rootDir, filePath, text string) {
	if !m.SupportsPath(filePath) {
		return
	}
	m.mu.Lock()
	if prev, ok := m.docs[filePath]; ok && prev.timer != nil {
		prev.timer.Stop()
	}
	m.docs[filePath] = &docSyncState{root: rootDir, version: 1}
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		c := m.ensureClient(ctx, rootDir, filePath)
		if c == nil {
			return
		}
		_ = c.Notify("textDocument/didOpen", map[string]any{
			"textDocument": TextDocumentItem{
				URI:        PathToURI(filePath),
				LanguageID: LanguageIDForPath(filePath),
				Version:    1,
				Text:       text,
			},
		})
	}()
}

// DidChange debounces buffer updates (150ms) so rapid typing never floods the
// server or serializes the full document on every keystroke.
func (m *Manager) DidChange(rootDir, filePath string, getText func() string) {
	if !m.SupportsPath(filePath) || getText == nil {
		return
	}
	m.mu.Lock()
	st, ok := m.docs[filePath]
	if !ok {
		st = &docSyncState{root: rootDir, version: 1}
		m.docs[filePath] = st
	}
	if st.timer != nil {
		st.timer.Stop()
	}
	st.version++
	ver := st.version
	st.timer = time.AfterFunc(defaultDidChangeDebounce, func() {
		c := m.existingClient(rootDir, filePath)
		if c == nil {
			return
		}
		text := getText()
		_ = c.Notify("textDocument/didChange", map[string]any{
			"textDocument": VersionedTextDocumentIdentifier{
				URI:     PathToURI(filePath),
				Version: ver,
			},
			"contentChanges": []ContentChangeEvent{{Text: text}},
		})
	})
	m.mu.Unlock()
}

// DidSave notifies the language server that filePath was saved.
func (m *Manager) DidSave(rootDir, filePath, text string) {
	c := m.existingClient(rootDir, filePath)
	if c == nil {
		return
	}
	go func() {
		_ = c.Notify("textDocument/didSave", map[string]any{
			"textDocument": TextDocumentIdentifier{URI: PathToURI(filePath)},
			"text":         text,
		})
	}()
}

// DidClose notifies the language server and cancels any pending debounce timer.
func (m *Manager) DidClose(rootDir, filePath string) {
	if m == nil || filePath == "" {
		return
	}
	m.mu.Lock()
	if st, ok := m.docs[filePath]; ok {
		if st.timer != nil {
			st.timer.Stop()
		}
		if rootDir == "" {
			rootDir = st.root
		}
		delete(m.docs, filePath)
	}
	m.mu.Unlock()

	c := m.existingClient(rootDir, filePath)
	if c == nil {
		return
	}
	go func() {
		_ = c.Notify("textDocument/didClose", map[string]any{
			"textDocument": TextDocumentIdentifier{URI: PathToURI(filePath)},
		})
	}()
}

// Completion queries textDocument/completion, capping results to maxCompletionItems.
func (m *Manager) Completion(ctx context.Context, rootDir, filePath string, line, col int) ([]CompletionItem, bool) {
	c := m.existingClient(rootDir, filePath)
	if c == nil {
		return nil, false
	}
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: PathToURI(filePath)},
		Position:     Position{Line: line, Character: col},
	}
	var raw json.RawMessage
	if err := c.Request(ctx, "textDocument/completion", params, &raw); err != nil || len(raw) == 0 {
		return nil, false
	}
	var list CompletionList
	if err := json.Unmarshal(raw, &list); err == nil && len(list.Items) > 0 {
		if len(list.Items) > maxCompletionItems {
			list.Items = list.Items[:maxCompletionItems]
		}
		return list.Items, true
	}
	var items []CompletionItem
	if err := json.Unmarshal(raw, &items); err == nil {
		if len(items) > maxCompletionItems {
			items = items[:maxCompletionItems]
		}
		return items, true
	}
	return nil, false
}

// Hover queries textDocument/hover and returns formatted hover text.
func (m *Manager) Hover(ctx context.Context, rootDir, filePath string, line, col int) (string, bool) {
	c := m.existingClient(rootDir, filePath)
	if c == nil {
		return "", false
	}
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: PathToURI(filePath)},
		Position:     Position{Line: line, Character: col},
	}
	var h Hover
	if err := c.Request(ctx, "textDocument/hover", params, &h); err != nil {
		return "", false
	}
	txt := ExtractHoverText(h.Contents)
	return txt, txt != ""
}

// Definition queries textDocument/definition.
func (m *Manager) Definition(ctx context.Context, rootDir, filePath string, line, col int) ([]Location, bool) {
	return m.queryLocations(ctx, "textDocument/definition", rootDir, filePath, line, col, nil)
}

// TypeDefinition queries textDocument/typeDefinition.
func (m *Manager) TypeDefinition(ctx context.Context, rootDir, filePath string, line, col int) ([]Location, bool) {
	return m.queryLocations(ctx, "textDocument/typeDefinition", rootDir, filePath, line, col, nil)
}

// Implementation queries textDocument/implementation.
func (m *Manager) Implementation(ctx context.Context, rootDir, filePath string, line, col int) ([]Location, bool) {
	return m.queryLocations(ctx, "textDocument/implementation", rootDir, filePath, line, col, nil)
}

// References queries textDocument/references.
func (m *Manager) References(ctx context.Context, rootDir, filePath string, line, col int) ([]Location, bool) {
	params := map[string]any{
		"textDocument": TextDocumentIdentifier{URI: PathToURI(filePath)},
		"position":     Position{Line: line, Character: col},
		"context":      map[string]bool{"includeDeclaration": true},
	}
	return m.queryLocations(ctx, "textDocument/references", rootDir, filePath, line, col, params)
}

func (m *Manager) queryLocations(ctx context.Context, method, rootDir, filePath string, line, col int, customParams any) ([]Location, bool) {
	c := m.existingClient(rootDir, filePath)
	if c == nil {
		return nil, false
	}
	params := customParams
	if params == nil {
		params = TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: PathToURI(filePath)},
			Position:     Position{Line: line, Character: col},
		}
	}
	var raw json.RawMessage
	if err := c.Request(ctx, method, params, &raw); err != nil || len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var single Location
	if err := json.Unmarshal(raw, &single); err == nil && single.URI != "" {
		return []Location{single}, true
	}
	var multi []Location
	if err := json.Unmarshal(raw, &multi); err == nil {
		return multi, true
	}
	// Handle LocationLink[] returned by some servers (e.g. typescript-language-server).
	var links []struct {
		TargetURI            string `json:"targetUri"`
		TargetSelectionRange Range  `json:"targetSelectionRange"`
		TargetRange          Range  `json:"targetRange"`
	}
	if err := json.Unmarshal(raw, &links); err == nil && len(links) > 0 {
		out := make([]Location, 0, len(links))
		for _, l := range links {
			r := l.TargetSelectionRange
			if r.Start == (Position{}) && r.End == (Position{}) {
				r = l.TargetRange
			}
			out = append(out, Location{URI: l.TargetURI, Range: r})
		}
		return out, true
	}
	return nil, false
}

// Rename queries textDocument/rename and returns the WorkspaceEdit.
func (m *Manager) Rename(ctx context.Context, rootDir, filePath string, line, col int, newName string) (*WorkspaceEdit, bool) {
	c := m.existingClient(rootDir, filePath)
	if c == nil {
		return nil, false
	}
	params := map[string]any{
		"textDocument": TextDocumentIdentifier{URI: PathToURI(filePath)},
		"position":     Position{Line: line, Character: col},
		"newName":      newName,
	}
	var edit WorkspaceEdit
	if err := c.Request(ctx, "textDocument/rename", params, &edit); err != nil {
		return nil, false
	}
	return &edit, true
}

// Formatting queries textDocument/formatting and returns the TextEdits.
func (m *Manager) Formatting(ctx context.Context, rootDir, filePath string, tabSize int, insertSpaces bool) ([]TextEdit, bool) {
	c := m.existingClient(rootDir, filePath)
	if c == nil {
		return nil, false
	}
	if tabSize <= 0 {
		tabSize = 4
	}
	params := map[string]any{
		"textDocument": TextDocumentIdentifier{URI: PathToURI(filePath)},
		"options": FormattingOptions{
			TabSize:      tabSize,
			InsertSpaces: insertSpaces,
		},
	}
	var edits []TextEdit
	if err := c.Request(ctx, "textDocument/formatting", params, &edits); err != nil {
		return nil, false
	}
	return edits, true
}

// ShutdownAll cleanly stops all active language servers and cancels pending timers.
func (m *Manager) ShutdownAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	for path, st := range m.docs {
		if st.timer != nil {
			st.timer.Stop()
		}
		delete(m.docs, path)
	}
	clients := make([]*Client, 0, len(m.clients))
	for k, c := range m.clients {
		clients = append(clients, c)
		delete(m.clients, k)
	}
	m.mu.Unlock()

	for _, c := range clients {
		c.Shutdown()
	}
}
