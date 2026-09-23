// Package marketplace implements the plugin marketplace (plugin store):
// marketplace sources (official CDN catalog, git repos, local folders),
// catalog parsing, and install / uninstall / update of plugins into the
// user-scope plugin directory.
package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/plugins"
)

// OfficialMarketplaceID is the single public store marketplace. Custom
// (personal) sources get generated ids and never render in the public segment.
const OfficialMarketplaceID = "zcode-plugins-official"

// OfficialMarketplaceSource is the seeded CDN catalog URL.
const OfficialMarketplaceSource = "https://cdn-zcode.z.ai/zcode/official-plugin/marketplace.json"

// SourceKind enumerates how a marketplace manifest is fetched.
type SourceKind string

const (
	SourceURL   SourceKind = "url"
	SourceGit   SourceKind = "git"
	SourceLocal SourceKind = "local"
)

// InstallSource describes how an individual plugin is fetched.
type InstallSource struct {
	Type   string `json:"type,omitempty"`   // zip | git | local
	URL    string `json:"url,omitempty"`    // zip or git URL
	SHA256 string `json:"sha256,omitempty"` // zip integrity check
	Path   string `json:"path,omitempty"`   // subpath inside a git repo / local dir / archive
	Ref    string `json:"ref,omitempty"`    // git ref
}

// StoreListing is display-only metadata attached to catalog entries.
type StoreListing struct {
	DisplayName      string            `json:"displayName,omitempty"`
	Icon             string            `json:"icon,omitempty"`
	Category         string            `json:"category,omitempty"`
	Author           string            `json:"author,omitempty"`
	AuthorURL        string            `json:"authorUrl,omitempty"`
	Homepage         string            `json:"homepage,omitempty"`
	PrivacyPolicy    string            `json:"privacyPolicy,omitempty"`
	TermsOfService   string            `json:"termsOfService,omitempty"`
	HeroImage        string            `json:"heroImage,omitempty"`
	ExamplePrompts   []string          `json:"examplePrompts,omitempty"`
	RequiresPaidPlan bool              `json:"requiresPaidPlan,omitempty"`
	DescriptionI18n  map[string]string `json:"descriptionI18n,omitempty"`
}

// catalogAuthor is the shared author object in manifests.
type catalogAuthor struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// catalogEntry is one plugin row inside a marketplace manifest.
type catalogEntry struct {
	Name             string            `json:"name"`
	Source           *InstallSource    `json:"source,omitempty"`
	Description      string            `json:"description,omitempty"`
	DescriptionI18n  map[string]string `json:"descriptionI18n,omitempty"`
	Version          string            `json:"version,omitempty"`
	Author           *catalogAuthor    `json:"author,omitempty"`
	Icon             string   `json:"icon,omitempty"`
	Category         string   `json:"category,omitempty"`
	Homepage         string   `json:"homepage,omitempty"`
	Repository       string   `json:"repository,omitempty"`
	PrivacyPolicy    string   `json:"privacyPolicy,omitempty"`
	TermsOfService   string   `json:"termsOfService,omitempty"`
	HeroImage        string   `json:"heroImage,omitempty"`
	ExamplePrompts   []string `json:"examplePrompts,omitempty"`
	RequiresPaidPlan bool     `json:"requiresPaidPlan,omitempty"`
	Keywords         []string `json:"keywords,omitempty"`
	DisplayName      string   `json:"displayName,omitempty"`
}

// catalog is the parsed marketplace manifest.
type catalog struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	DescriptionI18 map[string]any `json:"descriptionI18n,omitempty"`
	Owner          *catalogAuthor `json:"owner,omitempty"`
	Featured []string       `json:"featured,omitempty"`
	Plugins  []catalogEntry `json:"plugins"`
}

// RefreshFailure records the last failed marketplace refresh.
type RefreshFailure struct {
	Code     string `json:"code"`
	FailedAt string `json:"failedAt"`
	Message  string `json:"message"`
}

// Marketplace is the UI-facing marketplace summary.
type Marketplace struct {
	ID                             string          `json:"id"`
	Name                           string          `json:"name"`
	Source                         string          `json:"source"`
	Description                    string          `json:"description,omitempty"`
	LastUpdated                    string          `json:"lastUpdated,omitempty"`
	PluginCount                    int             `json:"pluginCount"`
	IsOfficial                     bool            `json:"isOfficial,omitempty"`
	Featured                       []string        `json:"featured,omitempty"`
	RefreshFailure                 *RefreshFailure `json:"refreshFailure,omitempty"`
	Available                      bool            `json:"available"`

	// internal persistence fields (not projected to the store UI payloads
	// beyond what the UI reads above)
	Kind    SourceKind `json:"kind,omitempty"`
	GitPath string     `json:"gitPath,omitempty"`

	catalog *catalog
}

// InstalledRecord is the central install ledger entry.
type InstalledRecord struct {
	ID          string `json:"id"` // name@marketplace
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"`
	Version     string `json:"version,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
}

// AvailablePlugin is one browsable store entry.
type AvailablePlugin struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Marketplace    string        `json:"marketplace"`
	Description    string        `json:"description,omitempty"`
	Version        string        `json:"version,omitempty"`
	Installed      bool          `json:"installed"`
	ComponentTypes []string      `json:"componentTypes,omitempty"`
	Listing        *StoreListing `json:"listing,omitempty"`
}

// InstalledMeta is the UI-facing installed record with update status.
type InstalledMeta struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Marketplace   string `json:"marketplace"`
	Version       string `json:"version,omitempty"`
	Description   string `json:"description,omitempty"`
	InstalledAt   string `json:"installedAt,omitempty"`
	UpdateStatus  string `json:"updateStatus,omitempty"` // none | update-available
	LatestVersion string `json:"latestVersion,omitempty"`
}

// PluginComponentItem is one component row inside a detail section.
type PluginComponentItem struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PluginComponentGroup groups described components by kind (mcp/skill/command/agent/hook).
type PluginComponentGroup struct {
	Kind  string               `json:"kind"`
	Items []PluginComponentItem `json:"items"`
}

// DescribeMetadata carries manifest fallback fields for the detail info section.
type DescribeMetadata struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Author      string `json:"author,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
}

// DescribeResult is the on-demand component listing for an uninstalled candidate.
type DescribeResult struct {
	Metadata   DescribeMetadata       `json:"metadata"`
	Components []PluginComponentGroup `json:"components"`
}

// PluginRuntimeInfo mirrors the subset of plugins.Plugin the store UI reads.
type PluginRuntimeInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Author      string `json:"author,omitempty"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"`
	Marketplace string `json:"marketplace"`
	RootPath    string `json:"rootPath,omitempty"`
	Dir         string `json:"dir,omitempty"`
}

// Overview is the joined store payload for the marketplace page.
type Overview struct {
	Marketplaces                 []Marketplace       `json:"marketplaces"`
	AvailablePlugins             []AvailablePlugin   `json:"availablePlugins"`
	InstalledPlugins             []InstalledMeta     `json:"installedPlugins"`
	Plugins                      []PluginRuntimeInfo `json:"plugins"`
	MarketplaceAvailabilityKnown bool                `json:"marketplaceAvailabilityKnown"`
	Diagnostics                  []string            `json:"diagnostics,omitempty"`
}

// installMarker is the sidecar file written into an installed plugin dir.
type installMarker struct {
	Marketplace string `json:"marketplace"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
}

// Manager owns the marketplace registry, catalog caches and install ledger.
type Manager struct {
	mu            sync.Mutex
	dataDir       string
	plugins       *plugins.Manager
	registryPath  string
	installedPath string
	gitDir        string
	httpClient    *http.Client
}

// NewManager creates the marketplace manager and seeds the official source.
func NewManager(dataDir string, pm *plugins.Manager) *Manager {
	dir := filepath.Join(dataDir, "marketplaces")
	_ = os.MkdirAll(filepath.Join(dir, "git"), 0755)
	m := &Manager{
		dataDir:       dataDir,
		plugins:       pm,
		registryPath:  filepath.Join(dir, "registry.json"),
		installedPath: filepath.Join(dir, "installed.json"),
		gitDir:        filepath.Join(dir, "git"),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
	m.seedOfficial()
	return m
}

func (m *Manager) seedOfficial() {
	list := m.loadRegistry()
	for _, mp := range list {
		if mp.ID == OfficialMarketplaceID {
			return
		}
	}
	list = append(list, Marketplace{
		ID:         OfficialMarketplaceID,
		Name:       OfficialMarketplaceID,
		Source:     OfficialMarketplaceSource,
		IsOfficial: true,
		Kind:       SourceURL,
	})
	m.saveRegistry(list)
}

// ── persistence ─────────────────────────────────────────────────────────────

func (m *Manager) loadRegistry() []Marketplace {
	data, err := os.ReadFile(m.registryPath)
	if err != nil {
		return nil
	}
	var list []Marketplace
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return list
}

func (m *Manager) saveRegistry(list []Marketplace) {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.registryPath, data, 0644)
}

func (m *Manager) loadInstalled() []InstalledRecord {
	data, err := os.ReadFile(m.installedPath)
	if err != nil {
		return nil
	}
	var list []InstalledRecord
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return list
}

func (m *Manager) saveInstalled(list []InstalledRecord) {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.installedPath, data, 0644)
}

// cachePath is where fetched URL catalogs are cached.
func (m *Manager) cachePath(id string) string {
	return filepath.Join(m.dataDir, "marketplaces", "cache", id+".json")
}

// ── catalog fetching ────────────────────────────────────────────────────────

var marketplacesManifestNames = []string{
	"marketplace.json",
	".forge-marketplace.json",
	".zcode-marketplace.json",
	".claude-plugin/marketplace.json",
}

func (m *Manager) fetchCatalog(mp *Marketplace) (*catalog, error) {
	switch mp.Kind {
	case SourceURL:
		data, err := m.httpGet(mp.Source)
		if err != nil {
			return nil, err
		}
		return parseCatalog(data)
	case SourceGit, SourceLocal:
		root := mp.GitPath
		if mp.Kind == SourceGit {
			if err := gitCloneOrPull(mp.Source, mp.GitPath, ""); err != nil {
				return nil, err
			}
		}
		for _, name := range marketplacesManifestNames {
			data, err := os.ReadFile(filepath.Join(root, name))
			if err == nil {
				cat, perr := parseCatalog(data)
				if perr != nil {
					return nil, perr
				}
				return cat, nil
			}
		}
		// No manifest: treat each direct child with a plugin.json as a plugin.
		return scanDirectoryCatalog(root)
	default:
		return nil, fmt.Errorf("unknown marketplace source kind %q", mp.Kind)
	}
}

func parseCatalog(data []byte) (*catalog, error) {
	var cat catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("invalid marketplace manifest: %w", err)
	}
	if cat.Name == "" {
		cat.Name = "unnamed-marketplace"
	}
	return &cat, nil
}

// scanDirectoryCatalog builds a catalog from a bare folder of plugin dirs.
func scanDirectoryCatalog(root string) (*catalog, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	cat := &catalog{Name: strings.TrimPrefix(filepath.Base(root), ".")}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "plugin.json")); err != nil {
			continue
		}
		manifest, _ := os.ReadFile(filepath.Join(dir, "plugin.json"))
		var p struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Version     string `json:"version"`
			Author      string `json:"author"`
			Icon        string `json:"icon"`
		}
		_ = json.Unmarshal(manifest, &p)
		name := p.Name
		if name == "" {
			name = e.Name()
		}
		cat.Plugins = append(cat.Plugins, catalogEntry{
			Name:        name,
			Description: p.Description,
			Version:     p.Version,
			Author:      &catalogAuthor{Name: p.Author},
			Icon:        p.Icon,
			Source:      &InstallSource{Type: "local", Path: e.Name()},
		})
	}
	return cat, nil
}

// classifySource maps a raw user-provided source string to a kind + normalized URL/path.
func classifySource(raw string) (SourceKind, string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", errors.New("empty marketplace source")
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return SourceURL, s, nil
	}
	if strings.HasPrefix(s, "git@") || strings.HasSuffix(s, ".git") ||
		strings.HasPrefix(s, "ssh://git@") || strings.HasPrefix(s, "git://") {
		return SourceGit, s, nil
	}
	// owner/repo GitHub shorthand
	if regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(s) {
		return SourceGit, "https://github.com/" + s + ".git", nil
	}
	// local path
	if p := expandHome(s); p != "" {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return SourceLocal, p, nil
		}
	}
	return "", "", fmt.Errorf("cannot resolve marketplace source: %s", raw)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return ""
}

func (m *Manager) httpGet(url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	limit := int64(64 << 20) // 64MB catalog cap
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func gitCloneOrPull(url, dest, ref string) error {
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		args := []string{"-C", dest, "pull", "--ff-only"}
		if ref != "" {
			args = []string{"-C", dest, "fetch", "origin", ref}
		}
		if out, err := runGit(args...); err != nil {
			return fmt.Errorf("git pull failed: %s", out)
		}
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(dest), 0755)
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, dest)
	if out, err := runGit(args...); err != nil {
		return fmt.Errorf("git clone failed: %s", out)
	}
	return nil
}

func runGit(args ...string) (string, error) {
	out, err := execGit(args...)
	return string(out), err
}

// ── public API ──────────────────────────────────────────────────────────────

func (m *Manager) Overview() *Overview {
	m.mu.Lock()
	defer m.mu.Unlock()

	registry := m.loadRegistry()
	overviews := make([]Marketplace, 0, len(registry))
	available := make([]AvailablePlugin, 0, 64)
	known := true

	// runtime plugins joined by install marker
	markerByDir := map[string]installMarker{}
	runtime := m.plugins.List()
	for _, p := range runtime {
		if p.Dir == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.Dir, ".forge-marketplace.json"))
		if err != nil {
			continue
		}
		var mk installMarker
		if json.Unmarshal(data, &mk) == nil && mk.Marketplace != "" {
			markerByDir[p.Dir] = mk
		}
	}

	ledger := map[string]InstalledRecord{}
	for _, rec := range m.loadInstalled() {
		ledger[rec.ID] = rec
	}

	for _, mp := range registry {
		view := mp
		view.IsOfficial = mp.ID == OfficialMarketplaceID
		cat, err := m.loadCatalogForView(&mp)
		if err != nil || cat == nil {
			view.Available = false
			if err != nil {
				view.RefreshFailure = &RefreshFailure{
					Code:     "load",
					FailedAt: time.Now().UTC().Format(time.RFC3339),
					Message:  err.Error(),
				}
			}
		} else {
			view.Available = true
			view.catalog = cat
			view.PluginCount = len(cat.Plugins)
			if mp.Name == "" || mp.Name == mp.ID {
				view.Name = cat.Name
			}
			if cat.Description != "" && view.Description == "" {
				view.Description = cat.Description
			}
			view.Featured = cat.Featured
			for i := range cat.Plugins {
				entry := &cat.Plugins[i]
				id := entry.Name + "@" + mp.ID
				listing := listingFromEntry(entry)
				types := componentTypesFromEntry(entry)
				installed := installedByAnyRecord(id, entry.Name, mp.ID, markerByDir, ledger)
				available = append(available, AvailablePlugin{
					ID:             id,
					Name:           entry.Name,
					Marketplace:    mp.ID,
					Description:    entry.Description,
					Version:        entry.Version,
					Installed:      installed,
					ComponentTypes: types,
					Listing:        listing,
				})
			}
		}
		overviews = append(overviews, view)
	}

	installedList := make([]InstalledMeta, 0, len(ledger))
	for _, rec := range ledger {
		meta := InstalledMeta{
			ID:          rec.ID,
			Name:        rec.Name,
			Marketplace: rec.Marketplace,
			Version:     rec.Version,
			InstalledAt: rec.InstalledAt,
			UpdateStatus: "none",
		}
		// find latest version from its marketplace catalog
		for i := range overviews {
			if overviews[i].catalog == nil || overviews[i].ID != rec.Marketplace {
				continue
			}
			for _, entry := range overviews[i].catalog.Plugins {
				if entry.Name == rec.Name && entry.Version != rec.Version {
					meta.UpdateStatus = "update-available"
					meta.LatestVersion = entry.Version
				}
			}
		}
		installedList = append(installedList, meta)
	}
	sort.Slice(installedList, func(i, j int) bool { return installedList[i].ID < installedList[j].ID })

	runtimeInfos := make([]PluginRuntimeInfo, 0, len(runtime))
	for _, p := range runtime {
		mk := markerByDir[p.Dir]
		runtimeInfos = append(runtimeInfos, PluginRuntimeInfo{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Version:     p.Version,
			Author:      p.Author,
			Enabled:     p.Enabled,
			Source:      string(p.Source),
			Marketplace: mk.Marketplace,
			RootPath:    p.Path,
			Dir:         p.Dir,
		})
	}

	return &Overview{
		Marketplaces:                 overviews,
		AvailablePlugins:             available,
		InstalledPlugins:             installedList,
		Plugins:                      runtimeInfos,
		MarketplaceAvailabilityKnown: known,
	}
}

// loadCatalogForView reads the cached catalog for overview rendering without network.
func (m *Manager) loadCatalogForView(mp *Marketplace) (*catalog, error) {
	switch mp.Kind {
	case SourceURL:
		data, err := os.ReadFile(m.cachePath(mp.ID))
		if err != nil {
			return nil, nil // not fatal: catalog just not cached yet
		}
		return parseCatalog(data)
	case SourceGit:
		if mp.GitPath == "" {
			mp.GitPath = filepath.Join(m.gitDir, slugify(mp.ID))
		}
		return m.fetchCatalog(mp)
	case SourceLocal:
		return m.fetchCatalog(mp)
	}
	return nil, fmt.Errorf("unknown source kind")
}

// installedByAnyRecord checks install state via ledger and marker files.
func installedByAnyRecord(id, name, marketplace string, markerByDir map[string]installMarker, ledger map[string]InstalledRecord) bool {
	if _, ok := ledger[id]; ok {
		return true
	}
	for _, mk := range markerByDir {
		if mk.Name == name && mk.Marketplace == marketplace {
			return true
		}
	}
	return false
}

func listingFromEntry(e *catalogEntry) *StoreListing {
	if e == nil {
		return nil
	}
	l := &StoreListing{
		DisplayName:      e.DisplayName,
		Icon:             e.Icon,
		Category:         e.Category,
		Homepage:         e.Homepage,
		PrivacyPolicy:    e.PrivacyPolicy,
		TermsOfService:   e.TermsOfService,
		HeroImage:        e.HeroImage,
		ExamplePrompts:   e.ExamplePrompts,
		RequiresPaidPlan: e.RequiresPaidPlan,
		DescriptionI18n:  e.DescriptionI18n,
	}
	if e.Author != nil {
		l.Author = e.Author.Name
		l.AuthorURL = e.Author.URL
	}
	if l.DisplayName == "" && l.Icon == "" && l.Category == "" && l.Author == "" &&
		l.Homepage == "" && l.HeroImage == "" && len(l.ExamplePrompts) == 0 && !l.RequiresPaidPlan &&
		len(l.DescriptionI18n) == 0 {
		return nil
	}
	return l
}

func componentTypesFromEntry(e *catalogEntry) []string {
	// unknown until describe; leave empty — UI describes on demand
	return nil
}

// AddSource registers a new marketplace from a URL / git repo / local directory.
func (m *Manager) AddSource(raw string) (*Marketplace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	kind, normalized, err := classifySource(raw)
	if err != nil {
		return nil, err
	}

	entry := Marketplace{Source: normalized, Kind: kind}
	switch kind {
	case SourceURL:
		data, err := m.httpGet(normalized)
		if err != nil {
			return nil, err
		}
		cat, err := parseCatalog(data)
		if err != nil {
			return nil, err
		}
		entry.catalog = cat
		entry.Name = cat.Name
		entry.Description = cat.Description
	case SourceGit:
		id := slugify(filepath.Base(strings.TrimSuffix(normalized, ".git")))
		entry.ID = id
		entry.GitPath = filepath.Join(m.gitDir, id)
		cat, err := m.fetchCatalog(&entry)
		if err != nil {
			return nil, err
		}
		entry.catalog = cat
		entry.Name = cat.Name
		entry.Description = cat.Description
	case SourceLocal:
		entry.GitPath = normalized
		cat, err := m.fetchCatalog(&entry)
		if err != nil {
			return nil, err
		}
		entry.catalog = cat
		entry.Name = cat.Name
		entry.Description = cat.Description
	}

	// unique id from catalog name
	base := slugify(entry.Name)
	if base == "" {
		base = fmt.Sprintf("marketplace-%d", time.Now().Unix())
	}
	id, suffix := base, 1
	for _, existing := range m.loadRegistry() {
		if existing.ID == id {
			if existing.Source == normalized {
				return nil, fmt.Errorf("marketplace already registered: %s", id)
			}
			id = fmt.Sprintf("%s-%d", base, suffix)
			suffix++
		}
	}
	entry.ID = id
	entry.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	if entry.Kind == SourceURL {
		_ = os.MkdirAll(filepath.Dir(m.cachePath(id)), 0755)
		if data, err := json.Marshal(entry.catalog); err == nil {
			_ = os.WriteFile(m.cachePath(id), data, 0644)
		}
	}

	list := m.loadRegistry()
	list = append(list, entry)
	m.saveRegistry(list)

	view := entry
	view.IsOfficial = false
	return &view, nil
}

// Update re-fetches one marketplace catalog (empty id = all).
func (m *Manager) Update(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	list := m.loadRegistry()
	var lastErr error
	updatedAny := false
	for i := range list {
		mp := &list[i]
		if id != "" && mp.ID != id {
			continue
		}
		cat, err := m.fetchCatalog(mp)
		if err != nil {
			lastErr = err
			mp.RefreshFailure = &RefreshFailure{
				Code:     "update",
				FailedAt: time.Now().UTC().Format(time.RFC3339),
				Message:  err.Error(),
			}
			continue
		}
		mp.RefreshFailure = nil
		mp.LastUpdated = time.Now().UTC().Format(time.RFC3339)
		mp.catalog = cat
		mp.PluginCount = len(cat.Plugins)
		if mp.Kind == SourceURL {
			_ = os.MkdirAll(filepath.Dir(m.cachePath(mp.ID)), 0755)
			if data, err := json.Marshal(cat); err == nil {
				_ = os.WriteFile(m.cachePath(mp.ID), data, 0644)
			}
		}
		updatedAny = true
	}
	m.saveRegistry(list)
	if lastErr != nil && !updatedAny {
		return lastErr
	}
	return nil
}

// Remove unregisters a marketplace. The official source cannot be removed.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == OfficialMarketplaceID {
		return errors.New("the official marketplace cannot be removed")
	}
	list := m.loadRegistry()
	out := list[:0]
	removed := false
	for _, mp := range list {
		if mp.ID == id {
			removed = true
			if mp.Kind == SourceGit && mp.GitPath != "" {
				_ = os.RemoveAll(mp.GitPath)
			}
			_ = os.Remove(m.cachePath(id))
			continue
		}
		out = append(out, mp)
	}
	if !removed {
		return fmt.Errorf("unknown marketplace: %s", id)
	}
	m.saveRegistry(out)

	// installed plugins from this marketplace become orphaned records;
	// keep the ledger so the store can still show them as installed-but-orphaned.
	return nil
}

// globalPluginsDir mirrors plugins.Manager's global directory.
func (m *Manager) globalPluginsDir() string {
	return filepath.Join(m.dataDir, "plugins")
}

// Install fetches a plugin from its marketplace into the user plugin directory.
func (m *Manager) Install(name, marketplaceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var mp *Marketplace
	var entry *catalogEntry
	list := m.loadRegistry()
	for i := range list {
		if list[i].ID != marketplaceID {
			continue
		}
		cat, err := m.fetchCatalog(&list[i])
		if err != nil {
			return fmt.Errorf("refresh marketplace %s: %w", marketplaceID, err)
		}
		list[i].catalog = cat
		mp = &list[i]
		for j := range cat.Plugins {
			if cat.Plugins[j].Name == name {
				entry = &cat.Plugins[j]
			}
		}
		if entry == nil {
			return fmt.Errorf("plugin %q not found in marketplace %s", name, marketplaceID)
		}
		break
	}
	if mp == nil {
		return fmt.Errorf("unknown marketplace: %s", marketplaceID)
	}

	dest := filepath.Join(m.globalPluginsDir(), name)
	tmp, err := os.MkdirTemp("", "forge-plugin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	src, err := m.materialize(entry, mp, tmp)
	if err != nil {
		return err
	}
	if !hasPluginManifest(src) {
		return fmt.Errorf("plugin %s has no plugin.json at its root", name)
	}

	// replace any previous install
	_ = os.RemoveAll(dest)
	if err := copyDir(src, dest); err != nil {
		return fmt.Errorf("install %s: %w", name, err)
	}

	marker, _ := json.MarshalIndent(map[string]string{
		"marketplace": marketplaceID,
		"name":        name,
		"version":     entry.Version,
		"installedAt": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(dest, ".forge-marketplace.json"), marker, 0644)

	// update ledger
	records := m.loadInstalled()
	id := name + "@" + marketplaceID
	found := false
	for i := range records {
		if records[i].ID == id {
			records[i].Version = entry.Version
			records[i].InstalledAt = time.Now().UTC().Format(time.RFC3339)
			found = true
		}
	}
	if !found {
		records = append(records, InstalledRecord{
			ID: id, Name: name, Marketplace: marketplaceID,
			Version: entry.Version, InstalledAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	m.saveInstalled(records)

	m.plugins.Reload()
	return nil
}

// materialize resolves a catalog entry source into a local plugin root under tmp.
func (m *Manager) materialize(entry *catalogEntry, mp *Marketplace, tmp string) (string, error) {
	src := entry.Source
	// Git/local marketplaces with plain path references install from the checkout.
	if src == nil || (src.Type == "" && src.Path != "") {
		if mp.Kind == SourceGit || mp.Kind == SourceLocal {
			root := mp.GitPath
			sub := src.Path
			if sub == "" && entry.Name != "" {
				sub = entry.Name
			}
			candidate := filepath.Join(root, sub)
			if hasPluginManifest(candidate) {
				return candidate, nil
			}
			if hasPluginManifest(root) {
				return root, nil
			}
			return "", fmt.Errorf("plugin %q not found in source", entry.Name)
		}
		return "", fmt.Errorf("plugin %q has no installable source", entry.Name)
	}

	switch src.Type {
	case "zip", "archive", "url":
		if src.URL == "" {
			return "", fmt.Errorf("zip source missing url")
		}
		data, err := m.httpGet(src.URL)
		if err != nil {
			return "", err
		}
		if src.SHA256 != "" {
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != strings.ToLower(src.SHA256) {
				return "", fmt.Errorf("checksum mismatch for %s", entry.Name)
			}
		}
		zipPath := filepath.Join(tmp, "plugin.zip")
		if err := os.WriteFile(zipPath, data, 0644); err != nil {
			return "", err
		}
		extractDir := filepath.Join(tmp, "unpacked")
		if err := unzip(zipPath, extractDir); err != nil {
			return "", fmt.Errorf("unzip %s: %w", entry.Name, err)
		}
		return findPluginRoot(extractDir, src.Path)
	case "git":
		cloneDir := filepath.Join(tmp, "repo")
		if err := gitCloneOrPull(src.URL, cloneDir, src.Ref); err != nil {
			return "", err
		}
		sub := src.Path
		if sub == "" {
			sub = entry.Name
		}
		candidate := filepath.Join(cloneDir, sub)
		if hasPluginManifest(candidate) {
			return candidate, nil
		}
		if hasPluginManifest(cloneDir) {
			return cloneDir, nil
		}
		return "", fmt.Errorf("plugin %q not found in git source", entry.Name)
	case "local":
		p := expandHome(src.Path)
		if p == "" || !hasPluginManifest(p) {
			return "", fmt.Errorf("local plugin source not found: %s", src.Path)
		}
		return p, nil
	}
	return "", fmt.Errorf("unsupported install source type %q", src.Type)
}

func hasPluginManifest(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "plugin.json"))
	return err == nil
}

// findPluginRoot locates the directory containing plugin.json inside an unpacked
// archive, optionally descending into a declared subpath first.
func findPluginRoot(extractDir, subPath string) (string, error) {
	if subPath != "" {
		candidate := filepath.Join(extractDir, subPath)
		if hasPluginManifest(candidate) {
			return candidate, nil
		}
	}
	if hasPluginManifest(extractDir) {
		return extractDir, nil
	}
	// search two levels deep for a nested root
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(extractDir, e.Name())
		if hasPluginManifest(candidate) {
			return candidate, nil
		}
		nested, err := os.ReadDir(candidate)
		if err != nil {
			continue
		}
		for _, n := range nested {
			if !n.IsDir() {
				continue
			}
			deep := filepath.Join(candidate, n.Name())
			if hasPluginManifest(deep) {
				return deep, nil
			}
		}
	}
	return "", errors.New("no plugin.json found in archive")
}

// Uninstall removes a marketplace-installed plugin.
func (m *Manager) Uninstall(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	records := m.loadInstalled()
	var rec *InstalledRecord
	out := records[:0]
	for _, r := range records {
		if r.ID == id {
			r := r
			rec = &r
			continue
		}
		out = append(out, r)
	}
	if rec == nil {
		// fall back to marker scan
		name, marketplace, ok := strings.Cut(id, "@")
		if !ok {
			return fmt.Errorf("unknown plugin: %s", id)
		}
		rec = &InstalledRecord{ID: id, Name: name, Marketplace: marketplace}
	} else {
		m.saveInstalled(out)
	}

	// find the installed dir via marker files
	runtime := m.plugins.List()
	removed := false
	for _, p := range runtime {
		if p.Dir == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.Dir, ".forge-marketplace.json"))
		if err != nil {
			continue
		}
		var mk struct {
			Marketplace string `json:"marketplace"`
			Name        string `json:"name"`
		}
		if json.Unmarshal(data, &mk) == nil && mk.Name == rec.Name && mk.Marketplace == rec.Marketplace {
			_ = os.RemoveAll(p.Dir)
			removed = true
		}
	}
	// also try the canonical global dir
	if !removed {
		candidate := filepath.Join(m.globalPluginsDir(), rec.Name)
		if _, err := os.Stat(filepath.Join(candidate, ".forge-marketplace.json")); err == nil {
			_ = os.RemoveAll(candidate)
		}
	}

	m.plugins.Reload()
	return nil
}

// UpdatePlugin reinstalls the latest version of an installed plugin.
func (m *Manager) UpdatePlugin(id string) error {
	records := m.loadInstalled()
	for _, r := range records {
		if r.ID == id {
			return m.Install(r.Name, r.Marketplace)
		}
	}
	name, marketplace, ok := strings.Cut(id, "@")
	if !ok {
		return fmt.Errorf("unknown plugin: %s", id)
	}
	return m.Install(name, marketplace)
}

// Describe fetches an uninstalled candidate's manifest and component listing
// into a temp dir without touching the plugin directory.
func (m *Manager) Describe(name, marketplaceID string) (*DescribeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var mp *Marketplace
	var entry *catalogEntry
	list := m.loadRegistry()
	for i := range list {
		if list[i].ID != marketplaceID {
			continue
		}
		cat, err := m.fetchCatalog(&list[i])
		if err != nil {
			return nil, err
		}
		mp = &list[i]
		for j := range cat.Plugins {
			if cat.Plugins[j].Name == name {
				entry = &cat.Plugins[j]
			}
		}
		break
	}
	if mp == nil || entry == nil {
		return nil, fmt.Errorf("plugin %s@%s not found", name, marketplaceID)
	}

	tmp, err := os.MkdirTemp("", "forge-describe-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	root, err := m.materialize(entry, mp, tmp)
	if err != nil {
		return nil, err
	}
	return describeDir(root, entry)
}

// DescribeInstalled reads component groups from an already-installed plugin dir.
func (m *Manager) DescribeInstalled(dir string) (*DescribeResult, error) {
	if dir == "" {
		return nil, errors.New("plugin dir unknown")
	}
	return describeDir(dir, nil)
}

// ── component discovery ─────────────────────────────────────────────────────

var frontmatterNameRe = regexp.MustCompile(`(?m)^name:\s*(.+)$`)
var frontmatterDescRe = regexp.MustCompile(`(?m)^description:\s*(.+)$`)

func describeDir(root string, entry *catalogEntry) (*DescribeResult, error) {
	res := &DescribeResult{}
	if entry != nil {
		res.Metadata = DescribeMetadata{
			Name:        entry.Name,
			Description: entry.Description,
			Version:     entry.Version,
			Homepage:    entry.Homepage,
		}
		if entry.Author != nil {
			res.Metadata.Author = entry.Author.Name
		}
	}

	// manifest fallbacks
	if data, err := os.ReadFile(filepath.Join(root, "plugin.json")); err == nil {
		var p struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Version     string `json:"version"`
			Author      string `json:"author"`
			Homepage    string `json:"homepage"`
			MCPServers  map[string]json.RawMessage `json:"mcpServers"`
			Tools       []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		}
		if json.Unmarshal(data, &p) == nil {
			if res.Metadata.Name == "" {
				res.Metadata.Name = p.Name
			}
			if res.Metadata.Description == "" {
				res.Metadata.Description = p.Description
			}
			if res.Metadata.Version == "" {
				res.Metadata.Version = p.Version
			}
			if res.Metadata.Author == "" {
				res.Metadata.Author = p.Author
			}
			if res.Metadata.Homepage == "" {
				res.Metadata.Homepage = p.Homepage
			}
			if len(p.MCPServers) > 0 {
				group := PluginComponentGroup{Kind: "mcp"}
				names := make([]string, 0, len(p.MCPServers))
				for serverName := range p.MCPServers {
					names = append(names, serverName)
				}
				sort.Strings(names)
				for _, serverName := range names {
					group.Items = append(group.Items, PluginComponentItem{Name: serverName})
				}
				res.Components = append(res.Components, group)
			}
			if len(p.Tools) > 0 {
				group := PluginComponentGroup{Kind: "command"}
				for _, t := range p.Tools {
					group.Items = append(group.Items, PluginComponentItem{Name: t.Name, Description: t.Description})
				}
				res.Components = append(res.Components, group)
			}
		}
	}

	// skills: skills/*/SKILL.md
	if items := describeSkillDirs(filepath.Join(root, "skills")); len(items) > 0 {
		res.Components = append(res.Components, PluginComponentGroup{Kind: "skill", Items: items})
	}
	// commands + agents: markdown files
	if items := describeMarkdownDir(filepath.Join(root, "commands")); len(items) > 0 {
		res.Components = append(res.Components, PluginComponentGroup{Kind: "command", Items: items})
	}
	if items := describeMarkdownDir(filepath.Join(root, "agents")); len(items) > 0 {
		res.Components = append(res.Components, PluginComponentGroup{Kind: "agent", Items: items})
	}
	// hooks: hooks/hooks.json
	if items := describeHooks(filepath.Join(root, "hooks", "hooks.json")); len(items) > 0 {
		res.Components = append(res.Components, PluginComponentGroup{Kind: "hook", Items: items})
	}

	return res, nil
}

func describeSkillDirs(skillsRoot string) []PluginComponentItem {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil
	}
	items := []PluginComponentItem{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(skillsRoot, e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		item := PluginComponentItem{Name: e.Name()}
		if name := frontmatterNameRe.FindSubmatch(data); name != nil {
			item.Name = strings.Trim(strings.TrimSpace(string(name[1])), `"'`)
		}
		if desc := frontmatterDescRe.FindSubmatch(data); desc != nil {
			item.Description = strings.Trim(strings.TrimSpace(string(desc[1])), `"'`)
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func describeMarkdownDir(dir string) []PluginComponentItem {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	items := []PluginComponentItem{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") ||
			strings.EqualFold(e.Name(), "README.md") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		item := PluginComponentItem{Name: strings.TrimSuffix(e.Name(), ".md")}
		if name := frontmatterNameRe.FindSubmatch(data); name != nil {
			item.Name = strings.Trim(strings.TrimSpace(string(name[1])), `"'`)
		}
		if desc := frontmatterDescRe.FindSubmatch(data); desc != nil {
			item.Description = strings.Trim(strings.TrimSpace(string(desc[1])), `"'`)
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func describeHooks(path string) []PluginComponentItem {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		Hooks []struct {
			Matcher string `json:"matcher"`
			Event   string `json:"event"`
		} `json:"hooks"`
	}
	if json.Unmarshal(data, &doc) != nil || len(doc.Hooks) == 0 {
		return nil
	}
	items := []PluginComponentItem{}
	for _, h := range doc.Hooks {
		name := h.Matcher
		if name == "" {
			name = h.Event
		}
		if name == "" {
			continue
		}
		items = append(items, PluginComponentItem{Name: name})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

// ── fs helpers ──────────────────────────────────────────────────────────────

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

func unzip(zipPath, dest string) error {
	return unzipFile(zipPath, dest)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = regexp.MustCompile(`[^a-z0-9-_]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
