package engine

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AgyAuthStatus is a read-only snapshot of the Antigravity (agy) login the
// active ACP session's CLI process authenticates with. It resolves the exact
// stores the agy CLI itself uses (macOS keychain entry "gemini/antigravity",
// then ~/.gemini/oauth_creds.json), so the usage modal always reflects the
// account that actually backs the running sessions.
type AgyAuthStatus struct {
	Authenticated bool
	Account       string
	Source        string // "keychain", "file", or "" when nothing was found
}

// agyStoredCredentials mirrors the credential payloads in both stores.
type agyStoredCredentials struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	Expiry       string
}

// agyAuthLookup is the seam tests override to avoid touching the real
// keychain or home directory.
var agyAuthLookup = agyAuthSnapshot

// agyAuthSnapshot resolves the stored Antigravity login without spawning the
// agy CLI or calling any network endpoint: spawning the CLI unauthenticated
// launches its OAuth login page, so a quota fetch must never be the thing
// that starts it. Everything here is a plain file/keychain read.
func agyAuthSnapshot() AgyAuthStatus {
	home, err := os.UserHomeDir()
	if home == "" || err != nil {
		home = ""
	}
	return agyAuthFromHome(home, lookupAgyKeychain)
}

// lookupAgyKeychain reads the macOS generic password the Antigravity stack
// stores (service "gemini", account "antigravity"). `security find-generic-password`
// is a read-only query — it never triggers a login flow. On non-macOS (or
// when the entry is absent) it returns false.
func lookupAgyKeychain() (string, bool) {
	out, err := exec.Command("security", "find-generic-password", "-s", "gemini", "-a", "antigravity", "-w").Output()
	if err != nil {
		return "", false
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return "", false
	}
	return raw, true
}

// agyAuthFromHome is the testable resolver: home may be a fixture directory
// and keychain a fake value source.
func agyAuthFromHome(home string, keychain func() (string, bool)) AgyAuthStatus {
	var creds agyStoredCredentials
	source := ""

	// 1. Keychain first — the same precedence the agy CLI uses.
	keychainFound := false
	if keychain != nil {
		if raw, ok := keychain(); ok && raw != "" {
			if decodeAgyKeychainValue(raw, &creds) {
				source = "keychain"
				keychainFound = true
			}
		}
	}

	// 2. Fall back to the CLI's own OAuth credentials file — also when the
	// keychain entry exists but is stale beyond non-interactive use.
	if (!keychainFound || !agyCredentialsUsable(creds)) && home != "" {
		if data, err := os.ReadFile(filepath.Join(home, ".gemini", "oauth_creds.json")); err == nil {
			var fileToken struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				IDToken      string `json:"id_token"`
				Expiry       string `json:"expiry"`
			}
			if json.Unmarshal(data, &fileToken) == nil && fileToken.AccessToken != "" {
				creds = agyStoredCredentials{
					AccessToken:  fileToken.AccessToken,
					RefreshToken: fileToken.RefreshToken,
					IDToken:      fileToken.IDToken,
					Expiry:       fileToken.Expiry,
				}
				source = "file"
			}
		}
	}

	status := AgyAuthStatus{Source: source}
	status.Authenticated = creds.AccessToken != "" && agyCredentialsUsable(creds)
	if status.Authenticated {
		status.Account = agyAccountEmail(home, creds.IDToken)
	}
	return status
}

// decodeAgyKeychainValue parses a keychain secret: either the go-keyring
// base64 envelope ("go-keyring-base64:<base64 json>") or raw JSON.
func decodeAgyKeychainValue(raw string, creds *agyStoredCredentials) bool {
	payload := raw
	if strings.HasPrefix(payload, "go-keyring-base64:") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(payload, "go-keyring-base64:"))
		if err != nil {
			return false
		}
		payload = string(decoded)
	}
	var stored struct {
		Token struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			Expiry       string `json:"expiry"`
		} `json:"token"`
		IDToken string `json:"id_token"`
	}
	if json.Unmarshal([]byte(payload), &stored) != nil {
		return false
	}
	if stored.Token.AccessToken == "" {
		return false
	}
	creds.AccessToken = stored.Token.AccessToken
	creds.RefreshToken = stored.Token.RefreshToken
	creds.IDToken = stored.IDToken
	creds.Expiry = stored.Token.Expiry
	return true
}

// agyCredentialsUsable reports whether the stored token can be used
// non-interactively: a refresh token lets the CLI renew silently; without
// one, an already-expired access token would force the interactive login
// page (which a quota fetch must never do).
func agyCredentialsUsable(creds agyStoredCredentials) bool {
	if creds.RefreshToken != "" {
		return true
	}
	if strings.TrimSpace(creds.Expiry) == "" {
		// Unknown expiry: assume the CLI can decide; it holds the token, so
		// worst case its own refresh path handles it.
		return true
	}
	expiry, err := time.Parse(time.RFC3339, creds.Expiry)
	if err != nil {
		// Unparseable expiry with no refresh token: the CLI could be in the
		// exact state this guard exists for (expired access, nothing to
		// refresh) — assuming "usable" is what launches the OAuth page
		// from a quota fetch. Report unusable; a working login rewrites the
		// store with a parseable expiry on the next successful auth.
		return false
	}
	return time.Now().Before(expiry)
}

// agyAccountEmail resolves the logged-in account email from the ID token
// claims, falling back to the CLI's active-account file.
func agyAccountEmail(home, idToken string) string {
	if email := emailFromAgyIDToken(idToken); email != "" {
		return email
	}
	if home == "" {
		return ""
	}
	if data, err := os.ReadFile(filepath.Join(home, ".gemini", "google_accounts.json")); err == nil {
		var acc struct {
			Active string `json:"active"`
		}
		if json.Unmarshal(data, &acc) == nil {
			return strings.TrimSpace(acc.Active)
		}
	}
	return ""
}

// emailFromAgyIDToken decodes the JWT payload claims (no signature check —
// the token was read from the credential store the CLI itself owns).
func emailFromAgyIDToken(idToken string) string {
	parts := strings.Split(strings.TrimSpace(idToken), ".")
	if len(parts) < 2 {
		return ""
	}
	payload := parts[1]
	if pad := len(payload) % 4; pad > 0 {
		payload += strings.Repeat("=", 4-pad)
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(decoded, &claims) != nil {
		return ""
	}
	return strings.TrimSpace(claims.Email)
}
