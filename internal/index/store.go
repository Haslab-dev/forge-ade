package index

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Index is the public FWI API (RFC §18).
type Index interface {
	Build() error
	Update(file string) error
	Remove(file string) error
	Symbols() []Symbol
	Find(name string) []Symbol
	Definition(name string) []Symbol
	Completion(prefix string) []Symbol
}

// Store is an in-memory workspace index with optional snapshot persistence
// (RFC §10). It is safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	root    string
	files   map[uint32]*File
	byPath  map[string]*File
	symbols []Symbol
	byName  map[string][]int // name → indices into symbols
	imports map[uint32][]Import
	exports map[uint32][]Export
	// member-completion binding facts (RFC §7): instance → class/keys/function
	newExprs    map[uint32][]NewExpr
	funcReturns map[uint32][]FuncReturn
	// instLang: which language created each instance binding, so member
	// completion never leaks symbols across languages.
	instLang    map[string]Language
	classByInst map[string]string
	keysByInst  map[string][]string
	fnByInst    map[string]string
	keysByFn    map[string][]string
	classByFn   map[string]string
	aliasByInst map[string]string
	subByInst   map[string]map[string][]string // object literal: dotted subpath → keys
	// depCache: bare import specs already resolved into dependency symbols.
	// A file edit only resolves new specifiers, never rescans node_modules.
	depCache map[string]bool
	nextID   uint32
}

// New creates an empty index rooted at dir.
func New(dir string) *Store {
	return &Store{
		root:        dir,
		files:       map[uint32]*File{},
		byPath:      map[string]*File{},
		byName:      map[string][]int{},
		imports:     map[uint32][]Import{},
		exports:     map[uint32][]Export{},
		newExprs:    map[uint32][]NewExpr{},
		funcReturns: map[uint32][]FuncReturn{},
		classByInst: map[string]string{},
		keysByInst:  map[string][]string{},
		fnByInst:    map[string]string{},
		keysByFn:    map[string][]string{},
		classByFn:   map[string]string{},
		aliasByInst: map[string]string{},
		subByInst:   map[string]map[string][]string{},
		depCache:    map[string]bool{},
		nextID:      1,
	}
}

// Root returns the workspace directory.
func (s *Store) Root() string { return s.root }

// SnapshotDir is where the index persists state (RFC §17).
func (s *Store) SnapshotDir() string { return filepath.Join(s.root, ".workspace") }

func hashBytes(b []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(b)
	return h.Sum64()
}

// Build performs a full scan-and-index of the workspace (RFC §5.1).
// Files are parsed by a worker pool sized to the CPU count (RFC §21).
func (s *Store) Build() error {
	paths, err := (&Scanner{Root: s.root}).Scan()
	if err != nil {
		return err
	}
	s.reset()
	return s.indexPaths(paths)
}

func (s *Store) indexPaths(paths []string) error {
	workers := runtime.GOMAXPROCS(0)
	if workers > len(paths) {
		workers = len(paths)
	}
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				if err := s.Update(p); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("%s: %w", p, err))
					mu.Unlock()
				}
			}
		}()
	}
	for _, p := range paths {
		jobs <- p
	}
	close(jobs)
	wg.Wait()
	return errors.Join(errs...)
}

// Update indexes a single file, replacing any previous symbols for it.
// If the file content hash is unchanged, parsing is skipped (RFC §9).
func (s *Store) Update(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	hash := hashBytes(data)

	s.mu.RLock()
	existing := s.byPath[path]
	s.mu.RUnlock()
	if existing != nil && existing.Hash == hash {
		return nil // unchanged — skip parsing
	}

	if strings.Contains(path, "/node_modules/") {
		return nil // deps handled lazily by the dependency index
	}
	lang := DetectLanguage(path)
	if lang == "" || ForLang(lang) == nil {
		if existing != nil {
			return s.Remove(path) // file no longer parseable
		}
		return nil
	}

	file := &File{Path: path, Language: lang, Hash: hash}
	res, err := Parse(data, file)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if prev := s.byPath[path]; prev != nil {
		file.ID = prev.ID
		s.files[file.ID] = file
		s.byPath[path] = file
		s.removeFileSymbolsLocked(file.ID)
	} else {
		file.ID = s.nextID
		s.nextID++
		s.files[file.ID] = file
		s.byPath[path] = file
	}
	s.addResultLocked(file, res)
	s.indexDepsLocked()
	return nil
}

// indexDepsLocked resolves bare import specifiers (review: Dependency Index).
// Only the export graph of each dependency is parsed — never full .d.ts.
func (s *Store) indexDepsLocked() {
	if s.depCache == nil {
		s.depCache = map[string]bool{}
	}
	byName := map[string]bool{}
	for _, sym := range s.symbols {
		if sym.Module != "" {
			byName[sym.Module+"|"+sym.Name] = true
		}
	}
	for id, imps := range s.imports {
		from := ""
		if f, ok := s.files[id]; ok {
			from = f.Path
		}
		for _, imp := range imps {
			spec := imp.Path
			if spec == "" || !isBareSpec(spec) {
				continue
			}
			if s.depCache[spec] {
				continue
			}
			s.depCache[spec] = true
			file := resolveModuleFile(s.root, spec, from)
			if file == "" {
				continue
			}
			for _, d := range parseDepExports(file, map[string]bool{}) {
				key := spec + "|" + d.Name
				if byName[key] {
					continue
				}
				byName[key] = true
				s.symbols = append(s.symbols, Symbol{
					Name: d.Name, Kind: d.Kind, Module: spec,
					File: file, Line: d.Line, Column: 1, EndLine: d.Line, EndColumn: len(d.Name) + 1,
				})
			}
		}
	}
}

// isBareSpec reports whether an import path is a bare package specifier
// ("react", "@scope/pkg") rather than a relative/absolute/builtin path.
func isBareSpec(p string) bool {
	if p == "" || strings.HasPrefix(p, ".") || strings.HasPrefix(p, "/") {
		return false
	}
	// strip subpath: "react/jsx-runtime" → "react"
	first := p
	if i := strings.IndexByte(first, '/'); i > 0 && !strings.HasPrefix(first, "@") {
		first = first[:i]
	}
	if strings.HasPrefix(first, "@") {
		// @scope/pkg[/sub]
		parts := strings.SplitN(first, "/", 3)
		if len(parts) < 2 {
			return false
		}
		first = parts[0] + "/" + parts[1]
	}
	switch first {
	case "fs", "path", "os", "http", "https", "net", "stream", "buffer",
		"util", "crypto", "child_process", "events", "url", "zlib", "assert",
		"process", "node:", "bun", "bun:test", "react-dom/client", "node:test":
		return false
	}
	return true
}

// Remove drops a file and all its symbols from the index.
func (s *Store) Remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.byPath[path]
	if f == nil {
		return nil
	}
	s.removeFileSymbolsLocked(f.ID)
	delete(s.files, f.ID)
	delete(s.byPath, path)
	delete(s.imports, f.ID)
	delete(s.exports, f.ID)
	return nil
}

// Symbols returns all indexed symbols, sorted by file then line.
func (s *Store) Symbols() []Symbol {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Symbol, len(s.symbols))
	copy(out, s.symbols)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// Find returns symbols whose name matches exactly (RFC §18).
func (s *Store) Find(name string) []Symbol {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lookupLocked(name)
}

// Definition returns the declaration sites for name (RFC §18).
func (s *Store) Definition(name string) []Symbol {
	return s.Find(name)
}

// Completion returns symbols matching prefix (RFC §11, §18).
// Completion returns symbols for `prefix.` ranked by review priority:
// current file → workspace (same language) → dependencies. Duplicate names
// keep the highest-priority source. Keywords come from CodeMirror's own
// javascript completion source.
func (s *Store) Completion(prefix string, lang Language, path string) []Symbol {
	if prefix == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	curID := uint32(0)
	if f, ok := s.byPath[path]; ok {
		curID = f.ID
	}
	seen := map[string]bool{}
	var out []Symbol
	add := func(sym Symbol) {
		if sym.Name == "" || seen[sym.Name] {
			return
		}
		if !strings.HasPrefix(strings.ToLower(sym.Name), strings.ToLower(prefix)) {
			return
		}
		seen[sym.Name] = true
		out = append(out, sym)
	}
	// tier 1: current file
	for _, sym := range s.symbols {
		if sym.FileID == curID && sym.Module == "" {
			add(sym)
		}
	}
	// tier 2: workspace, same language, other files
	for _, sym := range s.symbols {
		if sym.Module != "" || sym.FileID == curID {
			continue
		}
		if lang != "" {
			if f, ok := s.files[sym.FileID]; ok && f.Language != lang {
				continue
			}
		}
		add(sym)
	}
	// tier 3: dependencies — only meaningful for JS-family files
	if lang == LangJavaScript || lang == LangTypeScript || lang == LangJSX || lang == LangTSX || lang == "" {
		for _, sym := range s.symbols {
			if sym.Module != "" {
				add(sym)
			}
		}
	}
	return out
}

// Search returns symbols matching query using exact, prefix, camel-case and
// fuzzy matching, most precise first (RFC §15).
func (s *Store) Search(query string) []Symbol {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if query == "" {
		out := make([]Symbol, len(s.symbols))
		copy(out, s.symbols)
		return out
	}
	seen := map[int]bool{}
	var out []Symbol
	for _, mode := range []matchMode{matchExact, matchPrefix, matchCamel, matchFuzzy} {
		for _, idx := range s.byNameIndexLocked(query, mode) {
			if !seen[idx] {
				seen[idx] = true
				out = append(out, s.symbols[idx])
			}
		}
	}
	return out
}

// Outline returns the symbols declared in a single file, sorted by line
// (RFC §14).
func (s *Store) Outline(file string) []Symbol {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f := s.byPath[file]
	if f == nil {
		return nil
	}
	var out []Symbol
	for _, sym := range s.symbols {
		if sym.FileID == f.ID {
			out = append(out, sym)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

// Imports returns the import statements of a file (RFC §5.3).
func (s *Store) Imports(file string) []Import {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f := s.byPath[file]
	if f == nil {
		return nil
	}
	return s.imports[f.ID]
}

// Exports returns the export statements of a file (RFC §5.3).
func (s *Store) Exports(file string) []Export {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f := s.byPath[file]
	if f == nil {
		return nil
	}
	return s.exports[f.ID]
}

func (s *Store) lookupLocked(name string) []Symbol {
	var out []Symbol
	for _, idx := range s.byName[name] {
		out = append(out, s.symbols[idx])
	}
	return out
}

func (s *Store) byNameIndexLocked(query string, mode matchMode) []int {
	var idxs []int
	// exact and prefix can use byName keys; others scan all symbols
	switch mode {
	case matchExact, matchPrefix:
		for name, list := range s.byName {
			if match(name, query, mode) {
				idxs = append(idxs, list...)
			}
		}
		sort.Ints(idxs)
	default:
		for i, sym := range s.symbols {
			if match(sym.Name, query, mode) {
				idxs = append(idxs, i)
			}
		}
	}
	return idxs
}

func (s *Store) addResultLocked(f *File, res *ParseResult) {
	for _, sym := range res.Symbols {
		sym.FileID = f.ID
		sym.File = f.Path
		if sym.ID == "" {
			sym.ID = fmt.Sprintf("%s:%d:%d", f.Path, sym.Line, sym.Column)
		}
		idx := len(s.symbols)
		s.symbols = append(s.symbols, sym)
		s.byName[sym.Name] = append(s.byName[sym.Name], idx)
	}
	s.imports[f.ID] = res.Imports
	s.exports[f.ID] = res.Exports
	s.newExprs[f.ID] = res.NewExprs
	s.funcReturns[f.ID] = res.FuncReturn
	s.rebuildBindingsLocked()
}

// rebuildBindingsLocked recomputes instance/function resolution maps from all
// files' NewExprs and FuncReturn facts.
func (s *Store) rebuildBindingsLocked() {
	s.classByInst = map[string]string{}
	s.keysByInst = map[string][]string{}
	s.fnByInst = map[string]string{}
	s.keysByFn = map[string][]string{}
	s.classByFn = map[string]string{}
	s.aliasByInst = map[string]string{}
	s.instLang = map[string]Language{}
	for id, exprs := range s.newExprs {
		lang := s.files[id].Language
		for _, ne := range exprs {
			s.instLang[ne.Name] = lang
			if ne.Alias != "" {
				s.instLang[ne.Alias] = lang
			}
			if ne.Class != "" {
				s.classByInst[ne.Name] = ne.Class
			} else if len(ne.Keys) > 0 {
				s.keysByInst[ne.Name] = ne.Keys
			} else if ne.Fn != "" {
				s.fnByInst[ne.Name] = ne.Fn
			} else if ne.Alias != "" {
				s.aliasByInst[ne.Name] = ne.Alias
			}
			if len(ne.Sub) > 0 {
				s.subByInst[ne.Name] = ne.Sub
			}
		}
	}
	for _, frs := range s.funcReturns {
		for _, fr := range frs {
			if fr.Class != "" {
				s.classByFn[fr.Name] = fr.Class
			} else if len(fr.Keys) > 0 {
				s.keysByFn[fr.Name] = fr.Keys
			}
		}
	}
}

func (s *Store) removeFileSymbolsLocked(id uint32) {
	keep := s.symbols[:0]
	for _, sym := range s.symbols {
		if sym.FileID != id {
			keep = append(keep, sym)
		}
	}
	s.symbols = keep
	delete(s.newExprs, id)
	delete(s.funcReturns, id)
	s.rebuildBindingsLocked()
	// rebuild name index
	s.byName = map[string][]int{}
	for i, sym := range s.symbols {
		s.byName[sym.Name] = append(s.byName[sym.Name], i)
	}
}

func (s *Store) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = map[uint32]*File{}
	s.byPath = map[string]*File{}
	s.symbols = nil
	s.byName = map[string][]int{}
	s.imports = map[uint32][]Import{}
	s.exports = map[uint32][]Export{}
	s.newExprs = map[uint32][]NewExpr{}
	s.funcReturns = map[uint32][]FuncReturn{}
	s.classByInst = map[string]string{}
	s.keysByInst = map[string][]string{}
	s.fnByInst = map[string]string{}
	s.keysByFn = map[string][]string{}
	s.classByFn = map[string]string{}
	s.aliasByInst = map[string]string{}
	s.subByInst = map[string]map[string][]string{}
	s.depCache = map[string]bool{}
	s.nextID = 1
}

// snapshot is the persisted form of the index (RFC §17 index.bin).
type snapshot struct {
	Version     int
	Files       map[uint32]*File
	Symbols     []Symbol
	Imports     map[uint32][]Import
	Exports     map[uint32][]Export
	NewExprs    map[uint32][]NewExpr
	FuncReturns map[uint32][]FuncReturn
	NextID      uint32
}

// snapshotVersion gates gob snapshots: bump when the schema changes so stale
// snapshots are rebuilt instead of loaded half-empty.
const snapshotVersion = 4 // 4: Symbol.Module (dependency index)

// Save serializes the index to .workspace/index.bin and writes metadata.json.
func (s *Store) Save() error {
	s.mu.RLock()
	snap := snapshot{
		Version:     snapshotVersion,
		Files:       s.files,
		Symbols:     s.symbols,
		Imports:     s.imports,
		Exports:     s.exports,
		NewExprs:    s.newExprs,
		FuncReturns: s.funcReturns,
		NextID:      s.nextID,
	}
	root := s.root
	s.mu.RUnlock()

	dir := filepath.Join(root, ".workspace")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "index.bin"))
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(&snap); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return s.writeMetadata(root)
}

// Load restores the index from .workspace/index.bin if present.
func (s *Store) Load() error {
	path := filepath.Join(s.root, ".workspace", "index.bin")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	var snap snapshot
	if err := gob.NewDecoder(f).Decode(&snap); err != nil {
		return err
	}
	if snap.Version < snapshotVersion {
		return nil // stale schema → caller rebuilds
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = snap.Files
	if s.files == nil {
		s.files = map[uint32]*File{}
	}
	s.byPath = map[string]*File{}
	for id, fl := range s.files {
		s.byPath[fl.Path] = fl
		if id >= s.nextID {
			s.nextID = id + 1
		}
	}
	s.symbols = snap.Symbols
	s.imports = snap.Imports
	s.exports = snap.Exports
	if s.imports == nil {
		s.imports = map[uint32][]Import{}
	}
	if s.exports == nil {
		s.exports = map[uint32][]Export{}
	}
	s.byName = map[string][]int{}
	for i, sym := range s.symbols {
		s.byName[sym.Name] = append(s.byName[sym.Name], i)
	}
	if s.nextID == 0 {
		s.nextID = 1
	}
	return nil
}

type metadata struct {
	Root          string         `json:"root"`
	Files         int            `json:"files"`
	Symbols       int            `json:"symbols"`
	BuiltAt       string         `json:"built_at"`
	Version       int            `json:"version"`
	Languages     map[string]int `json:"languages"`
	SymbolsByLang map[string]int `json:"symbols_by_language"`
}

// LanguageStats groups indexed files and symbols by language.
func (s *Store) LanguageStats() (filesByLang, symsByLang map[string]int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filesByLang = map[string]int{}
	symsByLang = map[string]int{}
	fileLang := map[uint32]string{}
	for id, f := range s.files {
		if f.Language == "" {
			continue
		}
		l := string(f.Language)
		filesByLang[l]++
		fileLang[id] = l
	}
	for _, sym := range s.symbols {
		if l, ok := fileLang[sym.FileID]; ok {
			symsByLang[l]++
		}
	}
	return filesByLang, symsByLang
}

func (s *Store) writeMetadata(root string) error {
	filesByLang, symsByLang := s.LanguageStats()
	s.mu.RLock()
	m := metadata{
		Root:          root,
		Files:         len(s.files),
		Symbols:       len(s.symbols),
		BuiltAt:       time.Now().UTC().Format(time.RFC3339),
		Version:       2,
		Languages:     filesByLang,
		SymbolsByLang: symsByLang,
	}
	s.mu.RUnlock()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ".workspace", "metadata.json"), data, 0o644)
}

// Members resolves member suggestions for `instance.` where instance was
// bound via `new Foo()` (class members), `{ a, b }` (object keys), `foo()`
// (function return shape), or `: Foo` (interface/type members).
// symsFromNames converts a raw key list into member symbols.
func symsFromNames(names []string) []Symbol {
	out := make([]Symbol, 0, len(names))
	for _, n := range names {
		out = append(out, Symbol{Name: n, Kind: Variable, Scope: "member"})
	}
	return out
}

func (s *Store) Members(instance, lang string) []Symbol {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// dotted chain: `nested.a` / `services[0]` → resolve base then subpath
	if i := strings.IndexAny(instance, ".["); i > 0 {
		base, rest := instance[:i], instance[i:]
		if instance[i] == '.' {
			rest = instance[i+1:]
		}
		canon := base
		if s.aliasByInst[base] != "" {
			canon = s.aliasByInst[base]
		}
		if !s.langOK(canon, lang) {
			return nil
		}
		keys, ok := s.subByInst[canon]
		if !ok {
			keys, ok = s.subByInst[base]
		}
		if !ok {
			return nil
		}
		mk := map[string][]string{}
		for k, v := range keys {
			if k == rest || strings.HasPrefix(k, rest+".") {
				mk[k] = v
			}
		}
		merged := map[string]bool{}
		for _, list := range mk {
			for _, n := range list {
				merged[n] = true
			}
		}
		var out []Symbol
		for n := range merged {
			out = append(out, Symbol{Name: n, Kind: Variable, Scope: "member"})
		}
		return out
	}
	if keys, ok := s.keysByInst[instance]; ok && s.langOK(instance, lang) {
		return symsFromNames(keys)
	}
	if cls, ok := s.classByInst[instance]; ok && s.langOK(instance, lang) {
		return s.classMembersLocked(cls, lang)
	}
	if fn, ok := s.fnByInst[instance]; ok && s.langOK(instance, lang) {
		if cls := s.classByFn[fn]; cls != "" {
			return s.classMembersLocked(cls, lang)
		}
		if keys := s.keysByFn[fn]; len(keys) > 0 {
			return symsFromNames(keys)
		}
	}
	if alias := s.aliasByInst[instance]; alias != "" && s.langOK(instance, lang) {
		if cls := s.classByInst[alias]; cls != "" && s.langOK(alias, lang) {
			return s.classMembersLocked(cls, lang)
		}
		if keys := s.keysByInst[alias]; len(keys) > 0 && s.langOK(alias, lang) {
			return symsFromNames(keys)
		}
	}
	// type-name direct access: `Color.`, `Point.` — no binding needed
	return s.classMembersLocked(instance, lang)
}

// langOK reports whether the instance binding `name` belongs to `lang`.
// An empty lang means "no filter" (all languages allowed).
func (s *Store) langOK(name, lang string) bool {
	if lang == "" {
		return true
	}
	if l, ok := s.instLang[name]; ok {
		return string(l) == lang
	}
	return true // no binding recorded → fall through to symbol-level filter
}

// classMembersLocked returns methods and fields declared inside class/interface
// cls, sorted by file then line.
func (s *Store) classMembersLocked(cls, lang string) []Symbol {
	var out []Symbol
	seen := map[string]bool{}
	clsLang := ""
	for _, sym := range s.symbols {
		if sym.Name == cls && (sym.Kind == Class || sym.Kind == Interface ||
			sym.Kind == Enum || sym.Kind == Struct || sym.Kind == TypeAlias) {
			if f, ok := s.files[sym.FileID]; ok {
				clsLang = string(f.Language)
			}
			break
		}
	}
	if lang != "" && clsLang != "" && clsLang != lang {
		return nil
	}
	for _, sym := range s.symbols {
		if sym.Scope != cls {
			continue
		}
		if lang != "" {
			if f, ok := s.files[sym.FileID]; ok && string(f.Language) != lang {
				continue
			}
		}
		if (sym.Kind == Variable || sym.Kind == Method || sym.Kind == Constant) && sym.Name != "constructor" && !seen[sym.Name] {
			seen[sym.Name] = true
			out = append(out, sym)
		}
	}
	return out
}
