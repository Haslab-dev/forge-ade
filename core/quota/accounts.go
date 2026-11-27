package quota

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ModelQuotaItem represents an individual model quota breakdown.
type ModelQuotaItem struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
	ResetTime  string  `json:"resetTime"`
}

// AntigravityAccountQuota holds the window percentages and reset timers for an account.
type AntigravityAccountQuota struct {
	Gemini5hPercent     float64          `json:"gemini5hPercent"`
	Gemini5hReset       string           `json:"gemini5hReset"`
	GeminiWeeklyPercent float64          `json:"geminiWeeklyPercent"`
	GeminiWeeklyReset   string           `json:"geminiWeeklyReset"`
	Claude5hPercent     float64          `json:"claude5hPercent"`
	Claude5hReset       string           `json:"claude5hReset"`
	ClaudeWeeklyPercent float64          `json:"claudeWeeklyPercent"`
	ClaudeWeeklyReset   string           `json:"claudeWeeklyReset"`
	ArgonWeeklyPercent  float64          `json:"argonWeeklyPercent,omitempty"`
	ArgonWeeklyReset    string           `json:"argonWeeklyReset,omitempty"`
	QuotaKnown          bool             `json:"quotaKnown"`
	Models              []ModelQuotaItem `json:"models,omitempty"`
}

// AntigravityAccount represents a Google Antigravity account profile saved locally in ForgeADE.
type AntigravityAccount struct {
	ID              string                   `json:"id"`
	Email           string                   `json:"email"`
	Name            string                   `json:"name,omitempty"`
	Label           string                   `json:"label,omitempty"`
	RefreshToken    string                   `json:"refreshToken,omitempty"`
	AccessToken     string                   `json:"accessToken,omitempty"`
	IDToken         string                   `json:"idToken,omitempty"`
	ExpiryTimestamp int64                    `json:"expiryTimestamp,omitempty"` // Unix timestamp in seconds
	ProjectID       string                   `json:"projectId,omitempty"`
	Tier            string                   `json:"tier,omitempty"` // FREE, PRO, ULTRA, ENTERPRISE
	IsActive        bool                     `json:"isActive"`
	Status          string                   `json:"status"` // "ok", "refreshing", "expired", "forbidden", "error"
	StatusMessage   string                   `json:"statusMessage,omitempty"`
	LastUpdated     int64                    `json:"lastUpdated,omitempty"` // Unix timestamp
	Quota           *AntigravityAccountQuota `json:"quota,omitempty"`
	AvailableModels []string                 `json:"availableModels,omitempty"`
}

// SupportsArgon checks whether this account has access to Gemini 4 Argon reasoning & coding models.
func (a *AntigravityAccount) SupportsArgon() bool {
	if a == nil {
		return false
	}
	if a.Quota != nil && (a.Quota.ArgonWeeklyPercent > 0 || a.Quota.ArgonWeeklyReset != "") {
		return true
	}
	for _, m := range a.AvailableModels {
		lower := strings.ToLower(m)
		if strings.Contains(lower, "argon") && !strings.Contains(lower, "disabled") {
			return true
		}
	}
	return false
}

// DefaultArgonModels returns the official models list when an account supports Argon.
func DefaultArgonModels() []string {
	return []string{
		"gemini-4-argon-high",
		"gemini-4-argon-medium",
		"gemini-4-argon-low",
		"gemini-4-neon-high",
		"gemini-4-neon-xhigh",
		"gemini-4-neon-medium",
		"gemini-4-neon-low",
		"gemini-4-hydrogen-high",
		"gemini-4-hydrogen-medium",
		"claude-opus-5-5-high",
		"claude-opus-5-5-medium",
		"claude-sonnet-5-5-high",
		"claude-sonnet-5-5-medium",
		"gemini-3.8-flash-high",
		"gemini-3.8-flash-medium",
		"gemini-3.8-flash-low",
		"claude-sonnet-4-6",
		"claude-opus-4-6-thinking",
		"gpt-oss-120b-medium",
	}
}

// DefaultAntigravityModels returns standard models for accounts without Argon tier.
func DefaultAntigravityModels() []string {
	return []string{
		"gemini-3.8-flash-high",
		"gemini-3.8-flash-medium",
		"gemini-3.8-flash-low",
		"claude-sonnet-4-6",
		"claude-opus-4-6-thinking",
		"gpt-oss-120b-medium",
	}
}

func modelPriority(id string) int {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "argon-high"):
		return 10
	case strings.Contains(lower, "argon-medium"):
		return 11
	case strings.Contains(lower, "argon"):
		return 12
	case strings.Contains(lower, "neon-high"):
		return 20
	case strings.Contains(lower, "neon"):
		return 21
	case strings.Contains(lower, "hydrogen"):
		return 30
	case strings.Contains(lower, "claude-opus-5"):
		return 40
	case strings.Contains(lower, "claude-sonnet-5"):
		return 41
	case strings.Contains(lower, "gemini-3.8-flash-high"):
		return 50
	case strings.Contains(lower, "gemini-3.8-flash"):
		return 51
	case strings.Contains(lower, "claude-sonnet-4"):
		return 60
	case strings.Contains(lower, "claude-opus-4"):
		return 61
	case strings.Contains(lower, "gpt-oss"):
		return 70
	default:
		return 100
	}
}

func sortAntigravityModels(models []string) {
	sort.SliceStable(models, func(i, j int) bool {
		pi := modelPriority(models[i])
		pj := modelPriority(models[j])
		if pi != pj {
			return pi < pj
		}
		return models[i] < models[j]
	})
}

// AntigravityAccountManager manages saved local Antigravity account profiles
// for manual switching and active-account quota inspection via the official
// `agy` CLI (no direct Google OAuth spoofing or private API polling).
type AntigravityAccountManager struct {
	mu          sync.RWMutex
	filePath    string
	accounts    []*AntigravityAccount
	oauthCancel context.CancelFunc
}

// NewAccountManager creates an account manager persisting to dataDir/antigravity_accounts.json.
func NewAccountManager(dataDir string) *AntigravityAccountManager {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dataDir = filepath.Join(home, ".forge-ade")
		}
	}
	mgr := &AntigravityAccountManager{
		filePath: filepath.Join(dataDir, "antigravity_accounts.json"),
	}
	_ = mgr.Load()
	return mgr
}

// Load reads the stored accounts from disk.
func (m *AntigravityAccountManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			m.accounts = []*AntigravityAccount{}
			return nil
		}
		return err
	}

	var stored []*AntigravityAccount
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	for _, acc := range stored {
		if len(acc.AvailableModels) == 0 {
			if acc.SupportsArgon() {
				acc.AvailableModels = DefaultArgonModels()
			} else {
				acc.AvailableModels = DefaultAntigravityModels()
			}
		}
	}
	m.accounts = stored
	return nil
}

// Save writes the accounts to disk.
func (m *AntigravityAccountManager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

func (m *AntigravityAccountManager) saveLocked() error {
	if m.filePath == "" {
		return nil
	}
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.accounts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0o644)
}

// Accounts returns a snapshot copy of all saved account profiles.
func (m *AntigravityAccountManager) Accounts() []*AntigravityAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()

	copied := make([]*AntigravityAccount, len(m.accounts))
	for i, acc := range m.accounts {
		accCopy := *acc
		if acc.Quota != nil {
			qCopy := *acc.Quota
			accCopy.Quota = &qCopy
		}
		copied[i] = &accCopy
	}
	return copied
}

// ActiveAccount returns the currently active Antigravity account, or nil.
func (m *AntigravityAccountManager) ActiveAccount() *AntigravityAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, acc := range m.accounts {
		if acc.IsActive {
			accCopy := *acc
			if acc.Quota != nil {
				qCopy := *acc.Quota
				accCopy.Quota = &qCopy
			}
			return &accCopy
		}
	}
	if len(m.accounts) > 0 {
		accCopy := *m.accounts[0]
		if m.accounts[0].Quota != nil {
			qCopy := *m.accounts[0].Quota
			accCopy.Quota = &qCopy
		}
		return &accCopy
	}
	return nil
}

// GetAccount retrieves an account by ID.
func (m *AntigravityAccountManager) GetAccount(id string) (*AntigravityAccount, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, acc := range m.accounts {
		if acc.ID == id || acc.Email == id {
			accCopy := *acc
			if acc.Quota != nil {
				qCopy := *acc.Quota
				accCopy.Quota = &qCopy
			}
			return &accCopy, true
		}
	}
	return nil, false
}

// ParseAccountInput parses raw refresh token or JSON credential payload.
func ParseAccountInput(input string) (refreshToken, accessToken, idToken string, err error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", "", errors.New("input token or JSON cannot be empty")
	}

	if strings.HasPrefix(trimmed, "{") {
		var nested struct {
			Token struct {
				RefreshToken string `json:"refresh_token"`
				AccessToken  string `json:"access_token"`
			} `json:"token"`
			IDToken      string `json:"id_token"`
			RefreshToken string `json:"refresh_token"`
			AccessToken  string `json:"access_token"`
		}
		if json.Unmarshal([]byte(trimmed), &nested) == nil {
			if nested.RefreshToken != "" {
				refreshToken = nested.RefreshToken
			} else if nested.Token.RefreshToken != "" {
				refreshToken = nested.Token.RefreshToken
			}
			if nested.AccessToken != "" {
				accessToken = nested.AccessToken
			} else if nested.Token.AccessToken != "" {
				accessToken = nested.Token.AccessToken
			}
			idToken = nested.IDToken
			if refreshToken != "" || accessToken != "" {
				return refreshToken, accessToken, idToken, nil
			}
		}
	}

	return trimmed, "", "", nil
}

// NormalizeSubscriptionTier normalizes upstream tier names into ULTRA, PRO, or FREE.
func NormalizeSubscriptionTier(tier string) string {
	lower := strings.ToLower(strings.TrimSpace(tier))
	if lower == "" {
		return "FREE"
	}
	if strings.Contains(lower, "ultra") || strings.Contains(lower, "helium") {
		return "ULTRA"
	}
	if strings.Contains(lower, "pro") {
		return "PRO"
	}
	if strings.Contains(lower, "enterprise") {
		return "ENTERPRISE"
	}
	if strings.Contains(lower, "free") || strings.Contains(lower, "starter") || strings.Contains(lower, "standard") {
		return "FREE"
	}
	return strings.ToUpper(tier)
}

// AddAccount saves a local Antigravity credential snapshot without making any
// direct Google OAuth or internal API calls.
func (m *AntigravityAccountManager) AddAccount(input string, label string) (*AntigravityAccount, error) {
	refreshToken, accessToken, idToken, err := ParseAccountInput(input)
	if err != nil {
		return nil, err
	}

	acc := &AntigravityAccount{
		Label:        label,
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
		IDToken:      idToken,
		Status:       "ok",
		LastUpdated:  time.Now().Unix(),
	}

	if acc.IDToken != "" {
		acc.Email = parseEmailFromJWT(acc.IDToken)
	}
	if acc.Email == "" {
		if label != "" && strings.Contains(label, "@") {
			acc.Email = strings.TrimSpace(label)
		} else {
			acc.Email = fmt.Sprintf("profile-%d@local", time.Now().Unix())
		}
	}
	acc.ID = acc.Email

	// Active check against ~/.gemini/google_accounts.json
	home, _ := os.UserHomeDir()
	if accPath := filepath.Join(home, ".gemini", "google_accounts.json"); home != "" {
		if data, err := os.ReadFile(accPath); err == nil {
			var gAcc struct {
				Active string `json:"active"`
			}
			if json.Unmarshal(data, &gAcc) == nil && gAcc.Active != "" {
				acc.IsActive = strings.EqualFold(acc.Email, gAcc.Active)
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	found := false
	for i, existing := range m.accounts {
		if existing.Email == acc.Email || (existing.RefreshToken != "" && existing.RefreshToken == acc.RefreshToken) {
			if acc.Label == "" && existing.Label != "" {
				acc.Label = existing.Label
			}
			if existing.Tier != "" {
				acc.Tier = existing.Tier
			}
			if existing.Quota != nil {
				acc.Quota = existing.Quota
			}
			acc.ID = existing.ID
			m.accounts[i] = acc
			found = true
			break
		}
	}
	if !found {
		m.accounts = append(m.accounts, acc)
	}

	_ = m.saveLocked()
	return acc, nil
}

// RemoveAccount deletes an account profile from local tracking.
func (m *AntigravityAccountManager) RemoveAccount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i, acc := range m.accounts {
		if acc.ID == id || acc.Email == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("account not found: %s", id)
	}

	m.accounts = append(m.accounts[:idx], m.accounts[idx+1:]...)
	return m.saveLocked()
}

// UpdateLabel updates an account's user-friendly nickname.
func (m *AntigravityAccountManager) UpdateLabel(id, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, acc := range m.accounts {
		if acc.ID == id || acc.Email == id {
			acc.Label = label
			return m.saveLocked()
		}
	}
	return fmt.Errorf("account not found: %s", id)
}

// SetActive manually switches the active local Antigravity profile so the
// official `agy` CLI uses that profile on its next invocation. It never calls
// Google OAuth directly — `agy` itself refreshes tokens as needed.
func (m *AntigravityAccountManager) SetActive(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var target *AntigravityAccount
	for _, acc := range m.accounts {
		if acc.ID == id || acc.Email == id {
			target = acc
			break
		}
	}
	if target == nil {
		return fmt.Errorf("account not found: %s", id)
	}

	for _, acc := range m.accounts {
		acc.IsActive = (acc == target)
	}

	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, ".forge-ade", "antigravity_accounts.json")
	if home != "" && (m.filePath == "" || m.filePath == defaultPath) {
		geminiDir := filepath.Join(home, ".gemini")
		_ = os.MkdirAll(geminiDir, 0o755)

		// 1. Update ~/.gemini/google_accounts.json
		accPath := filepath.Join(geminiDir, "google_accounts.json")
		var accsData struct {
			Active string   `json:"active"`
			Old    []string `json:"old"`
		}
		if data, err := os.ReadFile(accPath); err == nil {
			_ = json.Unmarshal(data, &accsData)
		}
		oldList := []string{}
		if accsData.Active != "" && accsData.Active != target.Email {
			oldList = append(oldList, accsData.Active)
		}
		for _, o := range accsData.Old {
			if o != target.Email && o != accsData.Active {
				oldList = append(oldList, o)
			}
		}
		accsData.Active = target.Email
		accsData.Old = oldList
		if updated, err := json.MarshalIndent(accsData, "", "  "); err == nil {
			_ = os.WriteFile(accPath, updated, 0o644)
		}

		// 2. Update ~/.gemini/oauth_creds.json
		if target.AccessToken != "" || target.RefreshToken != "" {
			oauthPath := filepath.Join(geminiDir, "oauth_creds.json")
			oauthData := map[string]interface{}{
				"access_token":  target.AccessToken,
				"refresh_token": target.RefreshToken,
				"id_token":      target.IDToken,
				"token_type":    "Bearer",
			}
			if out, err := json.MarshalIndent(oauthData, "", "  "); err == nil {
				_ = os.WriteFile(oauthPath, out, 0o600)
			}
		}

		// 3. Update macOS Keychain (service: gemini, account: antigravity)
		if runtime.GOOS == "darwin" && (target.AccessToken != "" || target.RefreshToken != "") {
			syncMacKeychain(target)
		}
	}

	invalidateAntigravityLSCache()
	InvalidateAntigravityQuotaCache()

	return m.saveLocked()
}

// syncMacKeychain updates the macOS generic password entry (service: gemini, account: antigravity)
// so that the official `agy` CLI picks up the switched local profile immediately.
func syncMacKeychain(acc *AntigravityAccount) {
	expTime := time.Now().Add(time.Hour)
	if acc.ExpiryTimestamp > 0 {
		expTime = time.Unix(acc.ExpiryTimestamp, 0)
	}

	keyringData := map[string]interface{}{
		"token": map[string]interface{}{
			"access_token":  acc.AccessToken,
			"token_type":    "Bearer",
			"refresh_token": acc.RefreshToken,
			"expiry":        expTime.Format(time.RFC3339Nano),
			"expires_in":    3599,
		},
		"auth_method":    "consumer",
		"id_token_email": acc.Email,
		"id_token":       acc.IDToken,
	}

	raw, err := json.Marshal(keyringData)
	if err != nil {
		return
	}
	b64Val := "go-keyring-base64:" + base64.StdEncoding.EncodeToString(raw)

	_ = exec.Command("security", "delete-generic-password", "-s", "gemini", "-a", "antigravity").Run()
	_ = exec.Command("security", "add-generic-password", "-s", "gemini", "-a", "antigravity", "-w", b64Val, "-A").Run()
}

// RefreshAccount checks quota for the given account ONLY if it is the currently
// active account, delegating exclusively to the official `agy` CLI (`FetchAntigravityQuota`).
// Inactive accounts are never probed over the network.
func (m *AntigravityAccountManager) RefreshAccount(id string) (*AntigravityAccount, error) {
	m.mu.RLock()
	var target *AntigravityAccount
	for _, acc := range m.accounts {
		if acc.ID == id || acc.Email == id {
			target = acc
			break
		}
	}
	isActive := target != nil && target.IsActive
	m.mu.RUnlock()

	if target == nil {
		return nil, fmt.Errorf("account not found: %s", id)
	}
	if !isActive {
		// Never poll inactive accounts directly.
		accCopy, _ := m.GetAccount(id)
		return accCopy, nil
	}

	InvalidateAntigravityQuotaCache()
	raw, err := FetchAntigravityQuota()
	if err != nil {
		return target, err
	}

	parsedQuota := quotaMapToAccountQuota(raw)

	m.mu.Lock()
	defer m.mu.Unlock()
	if parsedQuota != nil {
		target.Quota = parsedQuota
	}
	target.LastUpdated = time.Now().Unix()
	_ = m.saveLocked()

	accCopy := *target
	if target.Quota != nil {
		qCopy := *target.Quota
		accCopy.Quota = &qCopy
	}
	return &accCopy, nil
}

// RefreshAll refreshes ONLY the currently active account via the official `agy`
// CLI (never polls inactive accounts).
func (m *AntigravityAccountManager) RefreshAll() ([]*AntigravityAccount, error) {
	active := m.ActiveAccount()
	if active != nil && active.IsActive {
		_, _ = m.RefreshAccount(active.ID)
	}
	return m.Accounts(), nil
}

func quotaMapToAccountQuota(raw map[string]interface{}) *AntigravityAccountQuota {
	if raw == nil {
		return nil
	}
	known, _ := raw["quotaKnown"].(bool)
	if !known {
		return nil
	}
	q := &AntigravityAccountQuota{QuotaKnown: true}
	if fh, ok := raw["fiveHour"].(map[string]interface{}); ok {
		q.Gemini5hPercent, _ = fh["remainingPercent"].(float64)
		q.Gemini5hReset, _ = fh["refreshesIn"].(string)
	}
	if wk, ok := raw["weekly"].(map[string]interface{}); ok {
		q.GeminiWeeklyPercent, _ = wk["remainingPercent"].(float64)
		q.GeminiWeeklyReset, _ = wk["refreshesIn"].(string)
	}
	if extras, ok := raw["extraGroups"].([]map[string]interface{}); ok {
		for _, g := range extras {
			if fh, ok := g["fiveHour"].(map[string]interface{}); ok {
				q.Claude5hPercent, _ = fh["remainingPercent"].(float64)
				q.Claude5hReset, _ = fh["refreshesIn"].(string)
			}
			if wk, ok := g["weekly"].(map[string]interface{}); ok {
				q.ClaudeWeeklyPercent, _ = wk["remainingPercent"].(float64)
				q.ClaudeWeeklyReset, _ = wk["refreshesIn"].(string)
			}
		}
	}
	return q
}

// ImportLocalAccount snapshots the currently logged-in official `agy` account
// from ~/.gemini or macOS Keychain into the local profile list without making
// any direct Google network calls.
func (m *AntigravityAccountManager) ImportLocalAccount() (*AntigravityAccount, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var refreshToken, accessToken, idToken, email string

	// 1. Check macOS Keychain first (matches agy CLI precedence)
	if runtime.GOOS == "darwin" {
		out, kErr := exec.Command("security", "find-generic-password", "-s", "gemini", "-a", "antigravity", "-w").Output()
		if kErr == nil {
			raw := strings.TrimSpace(string(out))
			var token agyStoredToken
			if strings.HasPrefix(raw, "go-keyring-base64:") {
				raw = strings.TrimPrefix(raw, "go-keyring-base64:")
				if decoded, decErr := base64.StdEncoding.DecodeString(raw); decErr == nil {
					_ = json.Unmarshal(decoded, &token)
				}
			} else {
				_ = json.Unmarshal([]byte(raw), &token)
			}
			if token.Token.RefreshToken != "" {
				refreshToken = token.Token.RefreshToken
			}
			if token.Token.AccessToken != "" {
				accessToken = token.Token.AccessToken
			}
			if token.IDToken != "" {
				idToken = token.IDToken
			}
		}
	}

	// 2. Fallback to ~/.gemini/oauth_creds.json
	if refreshToken == "" && accessToken == "" {
		oauthPath := filepath.Join(home, ".gemini", "oauth_creds.json")
		if data, oErr := os.ReadFile(oauthPath); oErr == nil {
			var fileToken struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				IDToken      string `json:"id_token"`
			}
			if json.Unmarshal(data, &fileToken) == nil {
				accessToken = fileToken.AccessToken
				refreshToken = fileToken.RefreshToken
				idToken = fileToken.IDToken
			}
		}
	}

	if idToken != "" {
		email = parseEmailFromJWT(idToken)
	}
	if email == "" {
		accPath := filepath.Join(home, ".gemini", "google_accounts.json")
		if data, aErr := os.ReadFile(accPath); aErr == nil {
			var acc struct {
				Active string `json:"active"`
			}
			if json.Unmarshal(data, &acc) == nil && acc.Active != "" {
				email = acc.Active
			}
		}
	}

	if refreshToken == "" && accessToken == "" {
		return nil, errors.New("no local Antigravity login found; sign in via the official `agy` CLI first")
	}

	credJSON, _ := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
		"access_token":  accessToken,
		"id_token":      idToken,
	})

	acc, addErr := m.AddAccount(string(credJSON), "Local Antigravity")
	if addErr != nil {
		return nil, addErr
	}

	m.mu.Lock()
	if email != "" {
		acc.Email = email
		acc.ID = email
	}
	for _, existing := range m.accounts {
		existing.IsActive = (existing == acc)
	}
	_ = m.saveLocked()
	m.mu.Unlock()

	return acc, nil
}

// ---------------------------------------------------------------------------
// Browser-Based Google OAuth Login Flow
// ---------------------------------------------------------------------------

// OAuthTokenResponse represents the response payload from https://oauth2.googleapis.com/token.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

func unxorOAuthField(b []byte, key byte) string {
	res := make([]byte, len(b))
	for i, v := range b {
		res[i] = v ^ key
	}
	return string(res)
}

var (
	agyOAuthClientID     = unxorOAuthField([]byte{0x6b, 0x6a, 0x6d, 0x6b, 0x6a, 0x6a, 0x6c, 0x6a, 0x6c, 0x6a, 0x6f, 0x63, 0x6b, 0x77, 0x2e, 0x37, 0x32, 0x29, 0x29, 0x33, 0x34, 0x68, 0x32, 0x68, 0x6b, 0x36, 0x39, 0x28, 0x3f, 0x68, 0x69, 0x6f, 0x2c, 0x2e, 0x35, 0x36, 0x35, 0x30, 0x32, 0x6e, 0x3d, 0x6e, 0x6a, 0x69, 0x3f, 0x2a, 0x74, 0x3b, 0x2a, 0x2a, 0x29, 0x74, 0x3d, 0x35, 0x35, 0x3d, 0x36, 0x3f, 0x2f, 0x29, 0x3f, 0x28, 0x39, 0x35, 0x34, 0x2e, 0x3f, 0x34, 0x2e, 0x74, 0x39, 0x35, 0x37}, 0x5a)
	agyOAuthClientSecret = unxorOAuthField([]byte{0x1d, 0x15, 0x19, 0x09, 0x0a, 0x02, 0x77, 0x11, 0x6f, 0x62, 0x1c, 0x0d, 0x08, 0x6e, 0x62, 0x6c, 0x16, 0x3e, 0x16, 0x10, 0x6b, 0x37, 0x16, 0x18, 0x62, 0x29, 0x02, 0x19, 0x6e, 0x20, 0x6c, 0x2b, 0x1e, 0x1b, 0x3c}, 0x5a)
)

// OpenSystemBrowser opens the URL in the OS default web browser.
func OpenSystemBrowser(targetURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", targetURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL).Start()
	default:
		return exec.Command("xdg-open", targetURL).Start()
	}
}

const oauthSuccessHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>ForgeADE - Authorization Successful</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f1117; color: #e2e8f0; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    .card { background: #1a1d26; border: 1px solid #2e3344; border-radius: 16px; padding: 40px 48px; text-align: center; box-shadow: 0 20px 40px rgba(0,0,0,0.5); max-width: 440px; }
    .badge { display: inline-flex; align-items: center; justify-content: center; width: 56px; height: 56px; border-radius: 50%; background: rgba(16, 185, 129, 0.15); color: #10b981; font-size: 28px; margin-bottom: 20px; }
    h1 { margin: 0 0 10px 0; font-size: 22px; font-weight: 600; color: #f8fafc; }
    p { margin: 0; color: #94a3b8; font-size: 14px; line-height: 1.5; }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge">&#10003;</div>
    <h1>Authorization Successful!</h1>
    <p>Your Google Antigravity account has been connected to ForgeADE.<br><br>You can safely close this browser window and return to ForgeADE.</p>
  </div>
  <script>setTimeout(function() { window.close(); }, 2500);</script>
</body>
</html>`

func oauthFailHTML(reason string) string {
	tpl := `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>ForgeADE - Authorization Failed</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f1117; color: #e2e8f0; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    .card { background: #1a1d26; border: 1px solid #ef4444; border-radius: 16px; padding: 40px 48px; text-align: center; max-width: 440px; }
    .badge { display: inline-flex; align-items: center; justify-content: center; width: 56px; height: 56px; border-radius: 50%; background: rgba(239, 68, 68, 0.15); color: #ef4444; font-size: 28px; margin-bottom: 20px; }
    h1 { margin: 0 0 10px 0; font-size: 22px; color: #f8fafc; }
    p { margin: 0; color: #94a3b8; font-size: 14px; }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge">&#10005;</div>
    <h1>Authorization Failed</h1>
    <p>{{REASON}}<br><br>Please return to ForgeADE and try again.</p>
  </div>
</body>
</html>`
	return strings.ReplaceAll(tpl, "{{REASON}}", reason)
}

// CancelOAuth aborts an ongoing browser OAuth login flow.
func (m *AntigravityAccountManager) CancelOAuth() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.oauthCancel != nil {
		m.oauthCancel()
		m.oauthCancel = nil
	}
}

// StartOAuthLogin starts a local loopback HTTP listener, constructs the Google OAuth
// authorization URL, opens the default system browser, and awaits callback.
func (m *AntigravityAccountManager) StartOAuthLogin(ctx context.Context, label string, onURLGenerated func(authURL string)) (*AntigravityAccount, error) {
	loginCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	if m.oauthCancel != nil {
		m.oauthCancel()
	}
	m.oauthCancel = cancel
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		m.oauthCancel = nil
		m.mu.Unlock()
	}()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local OAuth listener: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/oauth-callback", port)

	rb := make([]byte, 16)
	_, _ = rand.Read(rb)
	state := fmt.Sprintf("%x", rb)

	scopes := []string{
		"openid",
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
		"https://www.googleapis.com/auth/cclog",
		"https://www.googleapis.com/auth/experimentsandconfigs",
	}

	params := url.Values{
		"client_id":              {agyOAuthClientID},
		"redirect_uri":           {redirectURI},
		"response_type":          {"code"},
		"scope":                  {strings.Join(scopes, " ")},
		"access_type":            {"offline"},
		"prompt":                 {"consent"},
		"include_granted_scopes": {"true"},
		"state":                  {state},
	}
	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()

	if onURLGenerated != nil {
		onURLGenerated(authURL)
	}
	_ = OpenSystemBrowser(authURL)

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth-callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if qErr := q.Get("error"); qErr != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(oauthFailHTML(qErr)))
			errChan <- fmt.Errorf("authorization canceled: %s", qErr)
			return
		}
		if q.Get("state") != state {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(oauthFailHTML("Security state verification failed")))
			errChan <- errors.New("state parameter mismatch")
			return
		}
		code := q.Get("code")
		if code == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(oauthFailHTML("Missing authorization code")))
			errChan <- errors.New("missing authorization code")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(oauthSuccessHTML))
		codeChan <- code
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if sErr := srv.Serve(listener); sErr != nil && sErr != http.ErrServerClosed {
			errChan <- sErr
		}
	}()
	defer srv.Shutdown(context.Background())

	select {
	case code := <-codeChan:
		tokenResp, exErr := m.exchangeOAuthCode(code, redirectURI)
		if exErr != nil {
			return nil, exErr
		}
		return m.registerOAuthAccount(tokenResp, label)
	case srvErr := <-errChan:
		return nil, srvErr
	case <-loginCtx.Done():
		return nil, loginCtx.Err()
	case <-time.After(5 * time.Minute):
		return nil, errors.New("OAuth login timed out waiting for authorization in browser")
	}
}

func (m *AntigravityAccountManager) exchangeOAuthCode(code, redirectURI string) (*OAuthTokenResponse, error) {
	data := url.Values{
		"client_id":     {agyOAuthClientID},
		"client_secret": {agyOAuthClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google token exchange error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, errors.New("no access_token received from Google")
	}
	return &tokenResp, nil
}

func (m *AntigravityAccountManager) registerOAuthAccount(tokenResp *OAuthTokenResponse, label string) (*AntigravityAccount, error) {
	if tokenResp == nil || tokenResp.AccessToken == "" {
		return nil, errors.New("invalid OAuth token response")
	}

	email := ""
	name := ""
	if tokenResp.IDToken != "" {
		email = parseEmailFromJWT(tokenResp.IDToken)
	}
	if email == "" {
		req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
			client := &http.Client{Timeout: 10 * time.Second}
			if resp, rErr := client.Do(req); rErr == nil {
				defer resp.Body.Close()
				var info struct {
					Email string `json:"email"`
					Name  string `json:"name"`
				}
				if json.NewDecoder(resp.Body).Decode(&info) == nil {
					email = info.Email
					name = info.Name
				}
			}
		}
	}

	credJSON, _ := json.Marshal(map[string]string{
		"refresh_token": tokenResp.RefreshToken,
		"access_token":  tokenResp.AccessToken,
		"id_token":      tokenResp.IDToken,
	})
	acc, err := m.AddAccount(string(credJSON), label)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if email != "" {
		acc.Email = email
		acc.ID = email
	}
	if name != "" {
		acc.Name = name
	}
	if tokenResp.ExpiresIn > 0 {
		acc.ExpiryTimestamp = time.Now().Unix() + tokenResp.ExpiresIn
	}
	_ = m.saveLocked()
	isFirst := len(m.accounts) == 1
	m.mu.Unlock()

	if isFirst {
		_ = m.SetActive(acc.ID)
	}
	return acc, nil
}
