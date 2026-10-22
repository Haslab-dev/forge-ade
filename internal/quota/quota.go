package quota

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// GetAgentQuota returns account, tier, and real-time quota usage for agents.
func GetAgentQuota(agentID string, fallbackTotals func() map[string]interface{}) (map[string]interface{}, error) {
	norm := strings.ToLower(agentID)
	if strings.Contains(norm, "commandcode") || norm == "cmd" {
		return FetchCommandCodeQuota()
	}
	if strings.Contains(norm, "agy") || strings.Contains(norm, "antigravity") {
		return FetchAntigravityQuota()
	}

	totals := map[string]interface{}{}
	if fallbackTotals != nil {
		if summary := fallbackTotals(); summary != nil {
			totals = summary
		}
	}

	return map[string]interface{}{
		"agentId":   agentID,
		"agentName": "Internal Harness",
		"totals":    totals["totals"],
	}, nil
}

// FetchCommandCodeQuota fetches live credits and limits from Command Code API.
func FetchCommandCodeQuota() (map[string]interface{}, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	authPath := filepath.Join(home, ".commandcode", "auth.json")
	data, err := os.ReadFile(authPath)
	if err != nil {
		return nil, fmt.Errorf("command code auth not found at %s: %w", authPath, err)
	}

	var auth struct {
		APIKey   string `json:"apiKey"`
		UserName string `json:"userName"`
		UserID   string `json:"userId"`
	}
	if err := json.Unmarshal(data, &auth); err != nil || auth.APIKey == "" {
		return nil, fmt.Errorf("invalid command code auth credentials")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Fetch live credits & window limits
	reqCredits, err := http.NewRequest("GET", "https://api.commandcode.ai/alpha/billing/credits", nil)
	if err != nil {
		return nil, err
	}
	reqCredits.Header.Set("Authorization", "Bearer "+auth.APIKey)
	reqCredits.Header.Set("Accept", "application/json")

	respCredits, err := client.Do(reqCredits)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch credits: %w", err)
	}
	defer respCredits.Body.Close()

	var creditsResp struct {
		Credits struct {
			MonthlyCredits float64 `json:"monthlyCredits"`
			TotalRemaining float64 `json:"totalRemaining"`
		} `json:"credits"`
		WindowLimits struct {
			FiveHour struct {
				Cap     float64 `json:"cap"`
				Used    float64 `json:"used"`
				ResetAt int64   `json:"resetAt"`
			} `json:"fiveHour"`
			Weekly struct {
				Cap     float64 `json:"cap"`
				Used    float64 `json:"used"`
				ResetAt int64   `json:"resetAt"`
			} `json:"weekly"`
		} `json:"windowLimits"`
	}
	if err := json.NewDecoder(respCredits.Body).Decode(&creditsResp); err != nil {
		return nil, fmt.Errorf("failed to decode credits response: %w", err)
	}

	// 2. Fetch subscription details (plan name, renew date)
	planTitle := "Go Plan"
	status := "active"
	daysToRenew := 17

	reqSub, err := http.NewRequest("GET", "https://api.commandcode.ai/alpha/billing/subscriptions", nil)
	if err == nil {
		reqSub.Header.Set("Authorization", "Bearer "+auth.APIKey)
		reqSub.Header.Set("Accept", "application/json")
		if respSub, err := client.Do(reqSub); err == nil {
			defer respSub.Body.Close()
			var subResp struct {
				Subscriptions []struct {
					Status             string `json:"status"`
					CurrentPeriodEnd   int64  `json:"currentPeriodEnd"`
					CurrentPeriodStart int64  `json:"currentPeriodStart"`
					Plan               struct {
						Title string `json:"title"`
					} `json:"plan"`
				} `json:"subscriptions"`
			}
			if json.NewDecoder(respSub.Body).Decode(&subResp) == nil && len(subResp.Subscriptions) > 0 {
				sub := subResp.Subscriptions[0]
				if sub.Plan.Title != "" {
					planTitle = sub.Plan.Title
				}
				if sub.Status != "" {
					status = sub.Status
				}
				if sub.CurrentPeriodEnd > 0 {
					days := int(time.Until(time.UnixMilli(sub.CurrentPeriodEnd)).Hours() / 24)
					if days >= 0 {
						daysToRenew = days
					}
				}
			}
		}
	}

	// 3. Fetch usage summary total requests count
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	endOfMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	summaryURL := fmt.Sprintf("https://api.commandcode.ai/alpha/usage/summary?startDate=%s&endDate=%s", startOfMonth, endOfMonth)

	var summaryResp struct {
		TotalCount int `json:"totalCount"`
	}
	reqSum, err := http.NewRequest("GET", summaryURL, nil)
	if err == nil {
		reqSum.Header.Set("Authorization", "Bearer "+auth.APIKey)
		reqSum.Header.Set("Accept", "application/json")
		if respSum, err := client.Do(reqSum); err == nil {
			defer respSum.Body.Close()
			_ = json.NewDecoder(respSum.Body).Decode(&summaryResp)
		}
	}

	fiveHourCap := creditsResp.WindowLimits.FiveHour.Cap
	fiveHourUsed := creditsResp.WindowLimits.FiveHour.Used
	fiveHourPct := 0.0
	if fiveHourCap > 0 {
		fiveHourPct = (fiveHourUsed / fiveHourCap) * 100
	}
	fiveHourReset := FormatDurationMillis(creditsResp.WindowLimits.FiveHour.ResetAt)

	weeklyCap := creditsResp.WindowLimits.Weekly.Cap
	weeklyUsed := creditsResp.WindowLimits.Weekly.Used
	weeklyPct := 0.0
	if weeklyCap > 0 {
		weeklyPct = (weeklyUsed / weeklyCap) * 100
	}
	weeklyReset := FormatDurationMillis(creditsResp.WindowLimits.Weekly.ResetAt)

	usagePct := 5.0
	if creditsResp.Credits.MonthlyCredits > 0 && weeklyCap > 0 {
		usagePct = (weeklyUsed / weeklyCap) * 100
	}
	if usagePct > 100 {
		usagePct = 100
	}

	userName := auth.UserName
	if userName == "" {
		userName = "lutfi-haslab"
	}

	return map[string]interface{}{
		"agentId":       "agent-commandcode",
		"agentName":     "Command Code",
		"plan":          planTitle,
		"status":        status,
		"userName":      userName,
		"totalRequests": summaryResp.TotalCount,
		"daysLeft":      daysToRenew,
		"usagePercent":  usagePct,
		"fiveHour": map[string]interface{}{
			"used":     fiveHourUsed,
			"cap":      fiveHourCap,
			"percent":  fiveHourPct,
			"resetsIn": fiveHourReset,
		},
		"weekly": map[string]interface{}{
			"used":     weeklyUsed,
			"cap":      weeklyCap,
			"percent":  weeklyPct,
			"resetsIn": weeklyReset,
		},
		"breakdownUrl": fmt.Sprintf("https://commandcode.ai/%s/settings/usage", userName),
	}, nil
}

// FormatDurationMillis formats a duration until target timestamp in milliseconds.
func FormatDurationMillis(resetAtMillis int64) string {
	if resetAtMillis <= 0 {
		return "soon"
	}
	diff := time.Until(time.UnixMilli(resetAtMillis))
	if diff <= 0 {
		return "now"
	}
	if diff > 24*time.Hour {
		days := int(diff.Hours()) / 24
		hours := int(diff.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if diff > time.Hour {
		hours := int(diff.Hours())
		mins := int(diff.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	mins := int(diff.Minutes())
	if mins <= 0 {
		mins = 1
	}
	return fmt.Sprintf("%dm", mins)
}

type agyStoredToken struct {
	Token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Expiry       string `json:"expiry"`
	} `json:"token"`
	IDToken    string `json:"id_token"`
	AuthMethod string `json:"auth_method"`
}

func parseEmailFromJWT(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload := parts[1]
	if pad := len(payload) % 4; pad > 0 {
		payload += strings.Repeat("=", 4-pad)
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(decoded, &claims)
	return claims.Email
}

func getAntigravityCredentials() (accessToken string, email string, err error) {
	var token agyStoredToken

	// 1. Read dynamically from macOS Keychain (service: "gemini", account: "antigravity")
	out, kErr := exec.Command("security", "find-generic-password", "-s", "gemini", "-a", "antigravity", "-w").Output()
	if kErr == nil {
		raw := strings.TrimSpace(string(out))
		if strings.HasPrefix(raw, "go-keyring-base64:") {
			raw = strings.TrimPrefix(raw, "go-keyring-base64:")
			decoded, decErr := base64.StdEncoding.DecodeString(raw)
			if decErr == nil {
				_ = json.Unmarshal(decoded, &token)
			}
		}
	}

	// 2. Fallback to ~/.gemini/oauth_creds.json if keychain was not present or empty
	if token.Token.AccessToken == "" {
		home, _ := os.UserHomeDir()
		oauthPath := filepath.Join(home, ".gemini", "oauth_creds.json")
		if data, oErr := os.ReadFile(oauthPath); oErr == nil {
			var fileToken struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				IDToken      string `json:"id_token"`
			}
			if json.Unmarshal(data, &fileToken) == nil {
				token.Token.AccessToken = fileToken.AccessToken
				token.Token.RefreshToken = fileToken.RefreshToken
				token.IDToken = fileToken.IDToken
			}
		}
	}

	// 3. Extract active account email from ID token
	if token.IDToken != "" {
		email = parseEmailFromJWT(token.IDToken)
	}

	// 4. Fallback email from ~/.gemini/google_accounts.json
	if email == "" {
		home, _ := os.UserHomeDir()
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

	if token.Token.AccessToken == "" {
		return "", email, fmt.Errorf("no valid Antigravity access token found")
	}

	return token.Token.AccessToken, email, nil
}

// FormatAntigravityResetDuration formats reset duration for Antigravity quotas.
func FormatAntigravityResetDuration(remainingFraction float64, resetTimeStr string) string {
	if remainingFraction >= 1.0 || resetTimeStr == "" {
		return "Quota available"
	}
	t, err := time.Parse(time.RFC3339, resetTimeStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, resetTimeStr)
		if err != nil {
			return "Quota available"
		}
	}
	diff := time.Until(t)
	if diff <= 0 {
		return "Quota available"
	}
	hours := int(diff.Hours())
	mins := int(diff.Minutes()) % 60
	return fmt.Sprintf("Refreshes in %dh %dm", hours, mins)
}

var (
	agyLSMu      sync.RWMutex
	cachedLSAddr string
	cachedLSCSRF string
)

func discoverAntigravityLS() (addr string, csrf string) {
	// 1. Direct environment variables
	addr = os.Getenv("ANTIGRAVITY_LS_ADDRESS")
	csrf = os.Getenv("ANTIGRAVITY_CSRF_TOKEN")
	if addr != "" && csrf != "" {
		agyLSMu.Lock()
		cachedLSAddr = addr
		cachedLSCSRF = csrf
		agyLSMu.Unlock()
		return addr, csrf
	}

	// 2. Inspect active processes on macOS / Unix
	out, err := exec.Command("ps", "-A", "-ww", "-E").Output()
	if err == nil {
		text := string(out)
		if addr == "" {
			if m := regexp.MustCompile(`ANTIGRAVITY_LS_ADDRESS=([a-zA-Z0-9.:_-]+)`).FindStringSubmatch(text); len(m) > 1 {
				addr = m[1]
			}
		}
		if csrf == "" {
			if m := regexp.MustCompile(`ANTIGRAVITY_CSRF_TOKEN=([a-zA-Z0-9_-]+)`).FindStringSubmatch(text); len(m) > 1 {
				csrf = m[1]
			}
		}
	}

	agyLSMu.Lock()
	defer agyLSMu.Unlock()
	if addr != "" && csrf != "" {
		cachedLSAddr = addr
		cachedLSCSRF = csrf
		return addr, csrf
	}

	if cachedLSAddr != "" && cachedLSCSRF != "" {
		return cachedLSAddr, cachedLSCSRF
	}

	return addr, csrf
}

func invalidateAntigravityLSCache() {
	agyLSMu.Lock()
	cachedLSAddr = ""
	cachedLSCSRF = ""
	agyLSMu.Unlock()
}

func findAgyBinary() string {
	if p := os.Getenv("ANTIGRAVITY_AGENTAPI_EXE"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("agy"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "agy"),
		filepath.Join(home, ".gemini", "antigravity-cli", "bin", "agy"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

var (
	agyQuotaCacheMu   sync.RWMutex
	agyQuotaCacheVal  map[string]interface{}
	agyQuotaCacheTime time.Time
)

// FetchAntigravityQuota retrieves real-time quotas for Google Antigravity.
func FetchAntigravityQuota() (map[string]interface{}, error) {
	agyQuotaCacheMu.RLock()
	if agyQuotaCacheVal != nil && time.Since(agyQuotaCacheTime) < 15*time.Second {
		cached := make(map[string]interface{}, len(agyQuotaCacheVal))
		for k, v := range agyQuotaCacheVal {
			cached[k] = v
		}
		agyQuotaCacheMu.RUnlock()
		return cached, nil
	}
	agyQuotaCacheMu.RUnlock()

	accessToken, accountEmail, authErr := getAntigravityCredentials()
	if accountEmail == "" {
		accountEmail = "lutfiikbalmajid2@gmail.com"
	}

	weeklyRemaining := 100.0
	weeklyReset := "Quota available"
	fiveHourRemaining := 100.0
	fiveHourReset := "Quota available"

	claudeWeeklyRemaining := 100.0
	claudeWeeklyReset := "Quota available"
	claude5hRemaining := 100.0
	claude5hReset := "Quota available"

	type quotaSummaryBucket struct {
		BucketID          string  `json:"bucketId"`
		DisplayName       string  `json:"displayName"`
		Description       string  `json:"description"`
		Window            string  `json:"window"`
		RemainingFraction float64 `json:"remainingFraction"`
		ResetTime         string  `json:"resetTime"`
	}

	type quotaSummaryGroup struct {
		DisplayName string               `json:"displayName"`
		Description string               `json:"description"`
		Buckets     []quotaSummaryBucket `json:"buckets"`
	}

	type quotaSummaryEnvelope struct {
		Groups   []quotaSummaryGroup `json:"groups"`
		Response *struct {
			Groups []quotaSummaryGroup `json:"groups"`
		} `json:"response"`
	}

	applyGroups := func(groups []quotaSummaryGroup) {
		for _, g := range groups {
			gName := strings.ToLower(g.DisplayName)
			if strings.Contains(gName, "gemini") {
				for _, b := range g.Buckets {
					if b.Window == "weekly" || strings.Contains(b.BucketID, "weekly") {
						weeklyRemaining = math.Round(b.RemainingFraction*10000) / 100
						weeklyReset = FormatAntigravityResetDuration(b.RemainingFraction, b.ResetTime)
					} else if b.Window == "5h" || strings.Contains(b.BucketID, "5h") {
						fiveHourRemaining = math.Round(b.RemainingFraction*10000) / 100
						fiveHourReset = FormatAntigravityResetDuration(b.RemainingFraction, b.ResetTime)
					}
				}
			} else if strings.Contains(gName, "claude") || strings.Contains(gName, "gpt") || strings.Contains(gName, "3p") {
				for _, b := range g.Buckets {
					if b.Window == "weekly" || strings.Contains(b.BucketID, "weekly") {
						claudeWeeklyRemaining = math.Round(b.RemainingFraction*10000) / 100
						claudeWeeklyReset = FormatAntigravityResetDuration(b.RemainingFraction, b.ResetTime)
					} else if b.Window == "5h" || strings.Contains(b.BucketID, "5h") {
						claude5hRemaining = math.Round(b.RemainingFraction*10000) / 100
						claude5hReset = FormatAntigravityResetDuration(b.RemainingFraction, b.ResetTime)
					}
				}
			}
		}
	}

	gotQuota := false

	// 1. Direct query via agy CLI print mode
	if bin := findAgyBinary(); bin != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, bin, "-p", "/usage", "--output-format", "json")
		out, err := cmd.Output()
		cancel()
		if err == nil {
			var parsed struct {
				Command struct {
					Name string `json:"name"`
					Data struct {
						Groups []struct {
							Name    string `json:"name"`
							Buckets []struct {
								ID                string  `json:"id"`
								Name              string  `json:"name"`
								Window            string  `json:"window"`
								RemainingFraction float64 `json:"remaining_fraction"`
								ResetTime         string  `json:"reset_time"`
							} `json:"buckets"`
						} `json:"groups"`
					} `json:"data"`
				} `json:"command"`
			}
			if json.Unmarshal(out, &parsed) == nil && len(parsed.Command.Data.Groups) > 0 {
				var groups []quotaSummaryGroup
				for _, g := range parsed.Command.Data.Groups {
					qg := quotaSummaryGroup{DisplayName: g.Name}
					for _, b := range g.Buckets {
						qg.Buckets = append(qg.Buckets, quotaSummaryBucket{
							BucketID:          b.ID,
							DisplayName:       b.Name,
							Window:            b.Window,
							RemainingFraction: b.RemainingFraction,
							ResetTime:         b.ResetTime,
						})
					}
					groups = append(groups, qg)
				}
				applyGroups(groups)
				gotQuota = true
			}
		}
	}

	// 2. Try local Antigravity Language Server RPC (if CLI print mode was unavailable)
	if !gotQuota {
		lsAddr, csrfToken := discoverAntigravityLS()
		if lsAddr != "" && csrfToken != "" {
			lsClient := &http.Client{Timeout: 3 * time.Second}
			lsURL := fmt.Sprintf("http://%s/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary", lsAddr)
			req, err := http.NewRequest("POST", lsURL, bytes.NewBuffer([]byte(`{"force_refresh": true}`)))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("x-codeium-csrf-token", csrfToken)

				resp, doErr := lsClient.Do(req)
				if doErr == nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						var env quotaSummaryEnvelope
						if json.NewDecoder(resp.Body).Decode(&env) == nil {
							groups := env.Groups
							if len(groups) == 0 && env.Response != nil {
								groups = env.Response.Groups
							}
							if len(groups) > 0 {
								applyGroups(groups)
								gotQuota = true
							}
						}
					} else if resp.StatusCode == http.StatusUnauthorized {
						invalidateAntigravityLSCache()
					}
				} else {
					invalidateAntigravityLSCache()
				}
			}
		}
	}

	// 3. Direct CloudCode PA API queries (only if CLI and Language Server were unavailable)
	if !gotQuota && authErr == nil && accessToken != "" {
		client := &http.Client{Timeout: 8 * time.Second}

		endpoints := []string{
			"https://daily-cloudcode-pa.googleapis.com",
			"https://cloudcode-pa.googleapis.com",
		}

		for _, base := range endpoints {
			success := false

			// 2a. fetchAvailableModels for weekly quotas
			reqModels, err := http.NewRequest("POST", base+"/v1internal:fetchAvailableModels", bytes.NewBuffer([]byte("{}")))
			if err == nil {
				reqModels.Header.Set("Authorization", "Bearer "+accessToken)
				reqModels.Header.Set("Content-Type", "application/json")
				reqModels.Header.Set("User-Agent", "Antigravity/1.4.5")

				respModels, doErr := client.Do(reqModels)
				if doErr == nil {
					defer respModels.Body.Close()
					if respModels.StatusCode == http.StatusOK {
						success = true
						var modelsResp struct {
							Models map[string]struct {
								DisplayName string `json:"displayName"`
								QuotaInfo   *struct {
									RemainingFraction float64 `json:"remainingFraction"`
									ResetTime         string  `json:"resetTime"`
								} `json:"quotaInfo"`
							} `json:"models"`
						}
						if json.NewDecoder(respModels.Body).Decode(&modelsResp) == nil {
							for id, m := range modelsResp.Models {
								if strings.Contains(id, "gemini") && m.QuotaInfo != nil && m.QuotaInfo.RemainingFraction > 0 {
									weeklyRemaining = math.Round(m.QuotaInfo.RemainingFraction*10000) / 100
									weeklyReset = FormatAntigravityResetDuration(m.QuotaInfo.RemainingFraction, m.QuotaInfo.ResetTime)
									break
								}
							}
							for id, m := range modelsResp.Models {
								if (strings.Contains(id, "claude") || strings.Contains(id, "gpt")) && m.QuotaInfo != nil {
									claudeWeeklyRemaining = math.Round(m.QuotaInfo.RemainingFraction*10000) / 100
									claudeWeeklyReset = FormatAntigravityResetDuration(m.QuotaInfo.RemainingFraction, m.QuotaInfo.ResetTime)
									break
								}
							}
						}
					}
				}
			}

			// 2b. retrieveUserQuota for 5-hour limit
			reqQuota, err := http.NewRequest("POST", base+"/v1internal:retrieveUserQuota", bytes.NewBuffer([]byte("{}")))
			if err == nil {
				reqQuota.Header.Set("Authorization", "Bearer "+accessToken)
				reqQuota.Header.Set("Content-Type", "application/json")
				reqQuota.Header.Set("User-Agent", "Antigravity/1.4.5")

				respQuota, doErr := client.Do(reqQuota)
				if doErr == nil {
					defer respQuota.Body.Close()
					if respQuota.StatusCode == http.StatusOK {
						var quotaResp struct {
							Buckets []struct {
								ModelId           string  `json:"modelId"`
								RemainingFraction float64 `json:"remainingFraction"`
								ResetTime         string  `json:"resetTime"`
							} `json:"buckets"`
						}
						if json.NewDecoder(respQuota.Body).Decode(&quotaResp) == nil {
							for _, b := range quotaResp.Buckets {
								if strings.Contains(b.ModelId, "gemini") && b.RemainingFraction < 1.0 && b.ResetTime != "" {
									t, err := time.Parse(time.RFC3339, b.ResetTime)
									if err == nil && time.Until(t) < 6*time.Hour {
										fiveHourRemaining = math.Round(b.RemainingFraction*10000) / 100
										fiveHourReset = FormatAntigravityResetDuration(b.RemainingFraction, b.ResetTime)
										break
									}
								}
							}
						}
					}
				}
			}

			// If the daily endpoint succeeded, do NOT overwrite with subsequent fallback endpoint!
			if success {
				break
			}
		}
	}

	// Ensure any 100% states reflect "Quota available"
	if weeklyRemaining >= 100.0 {
		weeklyReset = "Quota available"
	}
	if fiveHourRemaining >= 100.0 {
		fiveHourReset = "Quota available"
	}
	if claudeWeeklyRemaining >= 100.0 {
		claudeWeeklyReset = "Quota available"
	}
	if claude5hRemaining >= 100.0 {
		claude5hReset = "Quota available"
	}

	result := map[string]interface{}{
		"agentId":    "agent-agy",
		"agentName":  "Google Antigravity",
		"account":    accountEmail,
		"modelGroup": "GEMINI MODELS",
		"models":     []string{"Gemini Flash", "Gemini Pro"},
		"weekly": map[string]interface{}{
			"remainingPercent": weeklyRemaining,
			"refreshesIn":      weeklyReset,
		},
		"fiveHour": map[string]interface{}{
			"remainingPercent": fiveHourRemaining,
			"refreshesIn":      fiveHourReset,
		},
		"extraGroups": []map[string]interface{}{
			{
				"modelGroup": "CLAUDE AND GPT MODELS",
				"models":     []string{"Claude Opus", "Claude Sonnet", "GPT-OSS"},
				"weekly": map[string]interface{}{
					"remainingPercent": claudeWeeklyRemaining,
					"refreshesIn":      claudeWeeklyReset,
				},
				"fiveHour": map[string]interface{}{
					"remainingPercent": claude5hRemaining,
					"refreshesIn":      claude5hReset,
				},
			},
		},
	}

	agyQuotaCacheMu.Lock()
	agyQuotaCacheVal = result
	agyQuotaCacheTime = time.Now()
	agyQuotaCacheMu.Unlock()

	return result, nil
}
