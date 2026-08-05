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
	nextID  uint32
}

// New creates an empty index rooted at dir.
func New(dir string) *Store {
	return &Store{
		root:    dir,
		files:   map[uint32]*File{},
		byPath:  map[string]*File{},
		byName:  map[string][]int{},
		imports: map[uint32][]Import{},
		exports: map[uint32][]Export{},
		nextID:  1,
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
	return nil
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
func (s *Store) Completion(prefix string) []Symbol {
	return s.Search(prefix)
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
}

func (s *Store) removeFileSymbolsLocked(id uint32) {
	keep := s.symbols[:0]
	for _, sym := range s.symbols {
		if sym.FileID != id {
			keep = append(keep, sym)
		}
	}
	s.symbols = keep
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
	s.nextID = 1
}

// snapshot is the persisted form of the index (RFC §17 index.bin).
type snapshot struct {
	Files   map[uint32]*File
	Symbols []Symbol
	Imports map[uint32][]Import
	Exports map[uint32][]Export
	NextID  uint32
}

// Save serializes the index to .workspace/index.bin and writes metadata.json.
func (s *Store) Save() error {
	s.mu.RLock()
	snap := snapshot{
		Files:   s.files,
		Symbols: s.symbols,
		Imports: s.imports,
		Exports: s.exports,
		NextID:  s.nextID,
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
	Root    string `json:"root"`
	Files   int    `json:"files"`
	Symbols int    `json:"symbols"`
	BuiltAt string `json:"built_at"`
	Version int    `json:"version"`
}

func (s *Store) writeMetadata(root string) error {
	s.mu.RLock()
	m := metadata{
		Root:    root,
		Files:   len(s.files),
		Symbols: len(s.symbols),
		BuiltAt: time.Now().UTC().Format(time.RFC3339),
		Version: 1,
	}
	s.mu.RUnlock()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ".workspace", "metadata.json"), data, 0o644)
}
