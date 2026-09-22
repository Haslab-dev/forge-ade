package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Plugin system ported from the reference plugin adapter. Storage layout mirrors
// under ~/.forge:
//
//	~/.forge/cli/plugins/
//	  known_marketplaces.json
//	  installed_plugins.json
//	  cache/<marketplace>/<plugin>/<version>/
//	  marketplaces/<marketplace>/marketplace.json
//	  data/<plugin-id>/
const (
	OfficialPluginMarketplace = "forge-plugins-official"
	InlinePluginMarketplace   = "inline"
	DefaultMarketplaceURL     = "https://plugins.forgeade.dev/marketplace.json"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

// SanitizePluginID replaces [^a-zA-Z0-9_.@-] with "-".
func SanitizePluginID(s string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9_.@-]`).ReplaceAllString(s, "-")
}

// PluginID joins name@marketplace.
func MakePluginID(name, marketplace string) (string, error) {
	if !namePattern.MatchString(name) || !namePattern.MatchString(marketplace) {
		return "", fmt.Errorf("invalid plugin or marketplace name")
	}
	return name + "@" + marketplace, nil
}

// MarketplaceSource is the discriminated union (url|github|git|file|directory).
type MarketplaceSource struct {
	Source      string            `json:"source"`
	URL         string            `json:"url,omitempty"`
	Repo        string            `json:"repo,omitempty"`
	Ref         string            `json:"ref,omitempty"`
	Path        string            `json:"path,omitempty"`
	SparsePaths []string          `json:"sparsePaths,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// KnownMarketplaceRecord / known_marketplaces.json.
type KnownMarketplaceRecord struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Source      MarketplaceSource `json:"source"`
	Description string            `json:"description,omitempty"`
	AddedAt     string            `json:"addedAt,omitempty"`
	LastUpdated string            `json:"lastUpdated,omitempty"`
	PluginCount int               `json:"pluginCount"`
}

type KnownMarketplaces struct {
	Version     int                       `json:"version"`
	Marketplaces []KnownMarketplaceRecord `json:"marketplaces"`
}

// InstalledPluginRecord / installed_plugins.json.
type InstalledPluginRecord struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Marketplace string            `json:"marketplace"`
	Version     string            `json:"version"`
	InstallPath string            `json:"installPath"`
	InstalledAt string            `json:"installedAt,omitempty"`
	UpdatedAt   string            `json:"updatedAt,omitempty"`
	Scope       string            `json:"scope"` // user | workspace
	Dependencies json.RawMessage  `json:"dependencies,omitempty"`
	Source      json.RawMessage   `json:"source,omitempty"`
}

type InstalledPlugins struct {
	Version int                      `json:"version"`
	Plugins []InstalledPluginRecord  `json:"plugins"`
}

// PluginManifest (.forge-plugin/plugin.json, with .claude/.codex compat).
type PluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	// Component overrides: string or string[] paths.
	Skills    json.RawMessage `json:"skills,omitempty"`
	Commands  json.RawMessage `json:"commands,omitempty"`
	Hooks     json.RawMessage `json:"hooks,omitempty"`
	Agents    json.RawMessage `json:"agents,omitempty"`
	MCPServers json.RawMessage `json:"mcpServers,omitempty"`
}

// manifestLookupOrder per spec §1.6.
var manifestLookupOrder = []string{
	".forge-plugin/plugin.json",
	".claude-plugin/plugin.json",
	".codex-plugin/plugin.json",
}

// PluginManager owns plugin storage, install/uninstall and discovery.
type PluginManager struct {
	storageRoot string
}

func NewPluginManager(homeDir string) *PluginManager {
	return &PluginManager{storageRoot: filepath.Join(homeDir, ".forge", "cli", "plugins")}
}

func (p *PluginManager) StorageRoot() string { return p.storageRoot }

func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (p *PluginManager) KnownMarketplacesPath() string {
	return filepath.Join(p.storageRoot, "known_marketplaces.json")
}
func (p *PluginManager) InstalledPluginsPath() string {
	return filepath.Join(p.storageRoot, "installed_plugins.json")
}

// LoadKnownMarketplaces tolerates a missing file (seeded with default).
func (p *PluginManager) LoadKnownMarketplaces() (*KnownMarketplaces, error) {
	km := &KnownMarketplaces{Version: 1}
	if err := readJSON(p.KnownMarketplacesPath(), km); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		km.Marketplaces = []KnownMarketplaceRecord{{
			ID: OfficialPluginMarketplace, Name: OfficialPluginMarketplace,
			Source: MarketplaceSource{Source: "url", URL: DefaultMarketplaceURL},
		}}
	}
	return km, nil
}

func (p *PluginManager) SaveKnownMarketplaces(km *KnownMarketplaces) error {
	return writeJSONAtomic(p.KnownMarketplacesPath(), km)
}

// LoadInstalledPlugins tolerates missing file.
func (p *PluginManager) LoadInstalledPlugins() (*InstalledPlugins, error) {
	ip := &InstalledPlugins{Version: 1}
	if err := readJSON(p.InstalledPluginsPath(), ip); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return ip, nil
}

func (p *PluginManager) SaveInstalledPlugins(ip *InstalledPlugins) error {
	return writeJSONAtomic(p.InstalledPluginsPath(), ip)
}

// AddMarketplace registers a marketplace from a parsed source input.
func (p *PluginManager) AddMarketplace(name string, src MarketplaceSource, description string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid marketplace name %q", name)
	}
	if name == OfficialPluginMarketplace {
		return fmt.Errorf("the official marketplace id is reserved")
	}
	km, err := p.LoadKnownMarketplaces()
	if err != nil {
		return err
	}
	for _, m := range km.Marketplaces {
		if m.ID == name {
			return fmt.Errorf("marketplace %s already known", name)
		}
	}
	km.Marketplaces = append(km.Marketplaces, KnownMarketplaceRecord{
		ID: name, Name: name, Source: src, Description: description,
		AddedAt: nowISO(),
	})
	return p.SaveKnownMarketplaces(km)
}

// RemoveMarketplace also removes installed plugins from that marketplace.
func (p *PluginManager) RemoveMarketplace(name string) error {
	km, err := p.LoadKnownMarketplaces()
	if err != nil {
		return err
	}
	out := km.Marketplaces[:0]
	found := false
	for _, m := range km.Marketplaces {
		if m.ID == name {
			found = true
			continue
		}
		out = append(out, m)
	}
	if !found {
		return fmt.Errorf("unknown marketplace %s", name)
	}
	km.Marketplaces = out
	if err := p.SaveKnownMarketplaces(km); err != nil {
		return err
	}
	ip, err := p.LoadInstalledPlugins()
	if err != nil {
		return err
	}
	remaining := ip.Plugins[:0]
	for _, rec := range ip.Plugins {
		if rec.Marketplace == name {
			_ = os.RemoveAll(rec.InstallPath)
			continue
		}
		remaining = append(remaining, rec)
	}
	ip.Plugins = remaining
	return p.SaveInstalledPlugins(ip)
}

// InstallPlugin installs name@marketplace into the cache and records it.
// Source fetch is delegated to Fetcher (git clone / archive download /
// filesystem copy), keeping this file host-agnostic.
func (p *PluginManager) InstallPlugin(pluginID, scope string, fetch func(dst string) error) error {
	at := strings.LastIndex(pluginID, "@")
	if at <= 0 || at == len(pluginID)-1 {
		return fmt.Errorf("invalid plugin id %q (want name@marketplace)", pluginID)
	}
	name, marketplace := pluginID[:at], pluginID[at+1:]
	ip, err := p.LoadInstalledPlugins()
	if err != nil {
		return err
	}
	target := p.CachePath(marketplace, name, "latest")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if fetch != nil {
		if err := fetch(target); err != nil {
			return fmt.Errorf("fetch %s: %w", pluginID, err)
		}
	}
	manifest, err := LoadPluginManifest(target)
	if err != nil {
		return err
	}
	if manifest.Name != "" && manifest.Name != name {
		return fmt.Errorf("manifest name %q does not match %q", manifest.Name, name)
	}
	version := manifest.Version
	if version == "" {
		version = "0.0.0"
	}
	final := p.CachePath(marketplace, name, version)
	if final != target {
		if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
			return err
		}
		_ = os.RemoveAll(final)
		if err := os.Rename(target, final); err != nil {
			return err
		}
	}
	now := nowISO()
	replaced := false
	for i := range ip.Plugins {
		if ip.Plugins[i].ID == pluginID {
			ip.Plugins[i].Version = version
			ip.Plugins[i].InstallPath = final
			ip.Plugins[i].UpdatedAt = now
			replaced = true
		}
	}
	if !replaced {
		ip.Plugins = append(ip.Plugins, InstalledPluginRecord{
			ID: pluginID, Name: name, Marketplace: marketplace, Version: version,
			InstallPath: final, InstalledAt: now, UpdatedAt: now, Scope: scope,
		})
	}
	return p.SaveInstalledPlugins(ip)
}

// UninstallPlugin removes the record; removeCache deletes the install path.
func (p *PluginManager) UninstallPlugin(pluginID string, removeCache bool) error {
	ip, err := p.LoadInstalledPlugins()
	if err != nil {
		return err
	}
	out := ip.Plugins[:0]
	found := false
	for _, rec := range ip.Plugins {
		if rec.ID == pluginID {
			found = true
			if removeCache {
				_ = os.RemoveAll(rec.InstallPath)
			}
			_ = os.RemoveAll(filepath.Join(p.storageRoot, "data", SanitizePluginID(pluginID)))
			continue
		}
		out = append(out, rec)
	}
	if !found {
		return fmt.Errorf("plugin %s is not installed", pluginID)
	}
	ip.Plugins = out
	return p.SaveInstalledPlugins(ip)
}

func (p *PluginManager) CachePath(marketplace, plugin, version string) string {
	return filepath.Join(p.storageRoot, "cache", SanitizePluginID(marketplace), SanitizePluginID(plugin), SanitizePluginID(version))
}

func (p *PluginManager) DataPath(pluginID string) string {
	return filepath.Join(p.storageRoot, "data", SanitizePluginID(pluginID))
}

// LoadPluginManifest finds and parses the plugin manifest with compat lookup.
func LoadPluginManifest(root string) (*PluginManifest, error) {
	for _, rel := range manifestLookupOrder {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			var m PluginManifest
			if err := readJSON(path, &m); err != nil {
				return nil, fmt.Errorf("plugin_manifest_invalid: %s: %w", path, err)
			}
			if m.Name == "" {
				return nil, fmt.Errorf("plugin_manifest_invalid: %s missing name", path)
			}
			if !namePattern.MatchString(m.Name) {
				return nil, fmt.Errorf("plugin_manifest_invalid: bad name %q", m.Name)
			}
			return &m, nil
		}
	}
	return nil, fmt.Errorf("plugin_manifest_not_found: %s", root)
}

// jsonPathList decodes manifest component fields that may be string,
// []string, or absent.
func jsonPathList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return []string{s}
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	return nil
}

// PluginComponents is the discovery result for one plugin.
type PluginComponents struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Root     string       `json:"root"`
	Enabled  bool         `json:"enabled"`
	Priority int          `json:"priority"`
	Skills   []SkillRoot  `json:"-"`
	Commands []string     `json:"commands"` // markdown command dirs/files
	Agents   []string     `json:"agents"`
	MCP      map[string]json.RawMessage `json:"mcp"`
	Hooks    json.RawMessage `json:"hooks,omitempty"`
}

// DiscoverPlugin loads a plugin root and enumerates its components
// (spec §1.7). priority follows first-plugin=1000, step +10; skill roots get
// p, command roots p+1.
func DiscoverPlugin(root, marketplace string, defaultEnabled, followsSymlinks bool, priority int) (*PluginComponents, error) {
	manifest, err := LoadPluginManifest(root)
	if err != nil {
		return nil, err
	}
	id, _ := MakePluginID(manifest.Name, marketplace)
	comp := &PluginComponents{
		ID: id, Name: manifest.Name, Root: root, Enabled: defaultEnabled, Priority: priority,
		MCP: map[string]json.RawMessage{},
	}
	// skills/: default dir + manifest overrides
	skillDirs := jsonPathList(manifest.Skills)
	if len(skillDirs) == 0 {
		if fi, err := os.Stat(filepath.Join(root, "skills")); err == nil && fi.IsDir() {
			skillDirs = []string{"skills"}
		}
	}
	for _, d := range skillDirs {
		p := filepath.Join(root, filepath.FromSlash(d))
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			comp.Skills = append(comp.Skills, SkillRoot{Path: p, Scope: "system", Source: "plugin",
				Priority: priority, PluginID: id})
		}
	}
	// commands/
	cmdDirs := jsonPathList(manifest.Commands)
	if len(cmdDirs) == 0 {
		if fi, err := os.Stat(filepath.Join(root, "commands")); err == nil && fi.IsDir() {
			cmdDirs = []string{"commands"}
		}
	}
	for _, d := range cmdDirs {
		p := filepath.Join(root, filepath.FromSlash(d))
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			comp.Commands = append(comp.Commands, p)
		}
	}
	// agents/
	agentDirs := jsonPathList(manifest.Agents)
	if len(agentDirs) == 0 {
		if fi, err := os.Stat(filepath.Join(root, "agents")); err == nil && fi.IsDir() {
			agentDirs = []string{"agents"}
		}
	}
	for _, d := range agentDirs {
		p := filepath.Join(root, filepath.FromSlash(d))
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			comp.Agents = append(comp.Agents, p)
		}
	}
	// hooks/hooks.json + manifest hooks path
	hooksPath := filepath.Join(root, "hooks", "hooks.json")
	if fi, err := os.Stat(hooksPath); err == nil && !fi.IsDir() {
		comp.Hooks, _ = os.ReadFile(hooksPath)
	} else if p := jsonPathList(manifest.Hooks); len(p) > 0 {
		hp := filepath.Join(root, filepath.FromSlash(p[0]))
		if data, err := os.ReadFile(hp); err == nil {
			comp.Hooks = data
		}
	}
	// .mcp.json merged under manifest mcpServers (manifest wins)
	mcpServers := map[string]json.RawMessage{}
	dotMcp := filepath.Join(root, ".mcp.json")
	if data, err := os.ReadFile(dotMcp); err == nil {
		var parsed struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		}
		if json.Unmarshal(data, &parsed) == nil {
			for k, v := range parsed.MCPServers {
				mcpServers[k] = v
			}
		}
	}
	if len(manifest.MCPServers) > 0 {
		var obj map[string]json.RawMessage
		if json.Unmarshal(manifest.MCPServers, &obj) == nil {
			for k, v := range obj {
				mcpServers[k] = v
			}
		}
	}
	comp.MCP = mcpServers
	return comp, nil
}

// DiscoverInstalled scans installed_plugins.json records, first plugin at
// priority 1000 with step +10.
func (p *PluginManager) DiscoverInstalled(enabled map[string]bool) ([]*PluginComponents, []string) {
	var out []*PluginComponents
	var diags []string
	ip, err := p.LoadInstalledPlugins()
	if err != nil {
		return nil, []string{"plugin_state_read_failed"}
	}
	// deterministic order
	ids := make([]string, 0, len(ip.Plugins))
	byID := map[string]InstalledPluginRecord{}
	for _, rec := range ip.Plugins {
		ids = append(ids, rec.ID)
		byID[rec.ID] = rec
	}
	sortStrings(ids)
	prio := 1000
	for _, id := range ids {
		rec := byID[id]
		en, ok := enabled[id]
		comp, err := DiscoverPlugin(rec.InstallPath, rec.Marketplace, defaultWhenMissing(ok, en, id), true, prio)
		if err != nil {
			diags = append(diags, err.Error())
			continue
		}
		comp.Enabled = en
		out = append(out, comp)
		prio += 10
	}
	return out, diags
}

func defaultWhenMissing(specified, enabled bool, id string) bool {
	if specified {
		return enabled
	}
	// official plugins default-on list
	switch id {
	case "browser-use@" + OfficialPluginMarketplace, "skill-creator@" + OfficialPluginMarketplace,
		"documents@" + OfficialPluginMarketplace, "pdf@" + OfficialPluginMarketplace:
		return true
	}
	return false
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func nowISO() string {
	return timeNowUTC().Format("2006-01-02T15:04:05Z")
}

func timeNowUTC() time.Time { return time.Now().UTC() }
