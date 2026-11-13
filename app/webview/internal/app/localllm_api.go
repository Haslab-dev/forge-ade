package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// LocalLLMService represents a locally running LLM server (e.g. Ollama or LM Studio).
type LocalLLMService struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	BaseURL string   `json:"baseUrl"`
	Running bool     `json:"running"`
	Models  []string `json:"models"`
	Error   string   `json:"error,omitempty"`
}

// DiscoverLocalLLMs checks known local LLM endpoints (Ollama on 11434, LM Studio on 1234)
// and returns their running status and available local models.
func (a *App) DiscoverLocalLLMs() []LocalLLMService {
	return ProbeLocalLLMs(800 * time.Millisecond)
}

// ProbeLocalLLMs probes Ollama and LM Studio endpoints concurrently with the given timeout.
func ProbeLocalLLMs(timeout time.Duration) []LocalLLMService {
	return ProbeLocalLLMsWithURLs(
		"http://127.0.0.1:11434",
		"http://127.0.0.1:1234",
		timeout,
	)
}

// ProbeLocalLLMsWithURLs probes custom Ollama and LM Studio URLs (useful for unit tests).
func ProbeLocalLLMsWithURLs(ollamaURL, lmStudioURL string, timeout time.Duration) []LocalLLMService {
	results := make([]LocalLLMService, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	// 1. Probe Ollama
	go func() {
		defer wg.Done()
		service := LocalLLMService{
			ID:      "prov-ollama",
			Name:    "Ollama (Local Offline)",
			BaseURL: ollamaURL,
			Running: false,
			Models:  []string{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaURL+"/api/tags", nil)
		if err == nil {
			client := &http.Client{Timeout: timeout}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					var tagResp struct {
						Models []struct {
							Name  string `json:"name"`
							Model string `json:"model"`
						} `json:"models"`
					}
					if json.Unmarshal(body, &tagResp) == nil {
						service.Running = true
						service.Error = ""
						seen := make(map[string]bool)
						for _, m := range tagResp.Models {
							name := m.Name
							if name == "" {
								name = m.Model
							}
							if name != "" && !seen[name] {
								seen[name] = true
								service.Models = append(service.Models, name)
							}
						}
					}
				} else {
					service.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
			} else {
				service.Error = err.Error()
			}
		} else {
			service.Error = err.Error()
		}

		results[0] = service
	}()

	// 2. Probe LM Studio
	go func() {
		defer wg.Done()
		service := LocalLLMService{
			ID:      "prov-lmstudio",
			Name:    "LM Studio (Local Offline)",
			BaseURL: lmStudioURL + "/v1",
			Running: false,
			Models:  []string{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, lmStudioURL+"/v1/models", nil)
		if err == nil {
			client := &http.Client{Timeout: timeout}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					var modelResp struct {
						Data []struct {
							ID string `json:"id"`
						} `json:"data"`
					}
					if json.Unmarshal(body, &modelResp) == nil {
						service.Running = true
						service.Error = ""
						seen := make(map[string]bool)
						for _, m := range modelResp.Data {
							if m.ID != "" && !seen[m.ID] {
								seen[m.ID] = true
								service.Models = append(service.Models, m.ID)
							}
						}
					}
				} else {
					service.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
			} else {
				service.Error = err.Error()
			}
		} else {
			service.Error = err.Error()
		}

		results[1] = service
	}()

	wg.Wait()
	return results
}
