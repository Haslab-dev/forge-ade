package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GetAgentQuota returns account, tier, and real-time quota usage for agents (e.g. Command Code, Antigravity, etc.)
func (a *App) GetAgentQuota(agentID string) (map[string]interface{}, error) {
	norm := strings.ToLower(agentID)
	if strings.Contains(norm, "commandcode") || norm == "cmd" {
		return fetchCommandCodeQuota()
	}
	if strings.Contains(norm, "agy") || strings.Contains(norm, "antigravity") {
		return fetchAntigravityQuota()
	}

	// Generic/Internal fallback: return usage summary totals
	summary := a.GetUsageSummary()
	return map[string]interface{}{
		"agentId":   agentID,
		"agentName": "Internal Harness",
		"totals":    summary["totals"],
	}, nil
}

func fetchCommandCodeQuota() (map[string]interface{}, error) {
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
	fiveHourReset := formatDurationMillis(creditsResp.WindowLimits.FiveHour.ResetAt)

	weeklyCap := creditsResp.WindowLimits.Weekly.Cap
	weeklyUsed := creditsResp.WindowLimits.Weekly.Used
	weeklyPct := 0.0
	if weeklyCap > 0 {
		weeklyPct = (weeklyUsed / weeklyCap) * 100
	}
	weeklyReset := formatDurationMillis(creditsResp.WindowLimits.Weekly.ResetAt)

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

func formatDurationMillis(resetAtMillis int64) string {
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

func formatAntigravityResetDuration(resetTimeStr string) string {
	if resetTimeStr == "" {
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

func fetchAntigravityQuota() (map[string]interface{}, error) {
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

	if authErr == nil && accessToken != "" {
		client := &http.Client{Timeout: 8 * time.Second}

		endpoints := []string{
			"https://daily-cloudcode-pa.googleapis.com",
			"https://cloudcode-pa.googleapis.com",
		}

		for _, base := range endpoints {
			// 1. fetchAvailableModels for weekly quotas
			req1, err := http.NewRequest("POST", base+"/v1internal:fetchAvailableModels", bytes.NewBuffer([]byte("{}")))
			if err == nil {
				req1.Header.Set("Authorization", "Bearer "+accessToken)
				req1.Header.Set("Content-Type", "application/json")
				req1.Header.Set("User-Agent", "Antigravity/1.4.5")

				resp1, doErr := client.Do(req1)
				if doErr == nil {
					defer resp1.Body.Close()
					if resp1.StatusCode == http.StatusOK {
						var modelsResp struct {
							Models map[string]struct {
								DisplayName string `json:"displayName"`
								QuotaInfo   *struct {
									RemainingFraction float64 `json:"remainingFraction"`
									ResetTime         string  `json:"resetTime"`
								} `json:"quotaInfo"`
							} `json:"models"`
						}
						if json.NewDecoder(resp1.Body).Decode(&modelsResp) == nil {
							for id, m := range modelsResp.Models {
								if strings.Contains(id, "gemini") && m.QuotaInfo != nil && m.QuotaInfo.RemainingFraction > 0 {
									weeklyRemaining = math.Round(m.QuotaInfo.RemainingFraction*10000) / 100
									weeklyReset = formatAntigravityResetDuration(m.QuotaInfo.ResetTime)
									break
								}
							}
							for id, m := range modelsResp.Models {
								if (strings.Contains(id, "claude") || strings.Contains(id, "gpt")) && m.QuotaInfo != nil {
									claudeWeeklyRemaining = math.Round(m.QuotaInfo.RemainingFraction*10000) / 100
									claudeWeeklyReset = formatAntigravityResetDuration(m.QuotaInfo.ResetTime)
									break
								}
							}
						}
					}
				}
			}

			// 2. retrieveUserQuota for 5-hour limit
			req2, err := http.NewRequest("POST", base+"/v1internal:retrieveUserQuota", bytes.NewBuffer([]byte("{}")))
			if err == nil {
				req2.Header.Set("Authorization", "Bearer "+accessToken)
				req2.Header.Set("Content-Type", "application/json")
				req2.Header.Set("User-Agent", "Antigravity/1.4.5")

				resp2, doErr := client.Do(req2)
				if doErr == nil {
					defer resp2.Body.Close()
					if resp2.StatusCode == http.StatusOK {
						var quotaResp struct {
							Buckets []struct {
								ModelId           string  `json:"modelId"`
								RemainingFraction float64 `json:"remainingFraction"`
								ResetTime         string  `json:"resetTime"`
							} `json:"buckets"`
						}
						if json.NewDecoder(resp2.Body).Decode(&quotaResp) == nil {
							for _, b := range quotaResp.Buckets {
								if strings.Contains(b.ModelId, "gemini") && b.RemainingFraction < 1.0 && b.ResetTime != "" {
									t, err := time.Parse(time.RFC3339, b.ResetTime)
									if err == nil && time.Until(t) < 6*time.Hour {
										fiveHourRemaining = math.Round(b.RemainingFraction*10000) / 100
										fiveHourReset = formatAntigravityResetDuration(b.ResetTime)
										break
									}
								}
							}
						}
					}
				}
			}

			if weeklyRemaining < 100.0 || weeklyReset != "Quota available" {
				break
			}
		}
	}

	return map[string]interface{}{
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
	}, nil
}
