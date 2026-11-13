package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeLocalLLMs_Success(t *testing.T) {
	// Mock Ollama server
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		data := map[string]interface{}{
			"models": []map[string]interface{}{
				{"name": "qwen2.5-coder:latest", "model": "qwen2.5-coder:latest"},
				{"name": "llama3.2:latest", "model": "llama3.2:latest"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(data)
	}))
	defer ollamaServer.Close()

	// Mock LM Studio server
	lmStudioServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		data := map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "qwen2.5-coder-7b-instruct", "object": "model"},
				{"id": "deepseek-r1-distill-qwen-7b", "object": "model"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(data)
	}))
	defer lmStudioServer.Close()

	services := ProbeLocalLLMsWithURLs(ollamaServer.URL, lmStudioServer.URL, 500*time.Millisecond)
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	// Verify Ollama
	ollama := services[0]
	if ollama.ID != "prov-ollama" || !ollama.Running {
		t.Errorf("expected ollama to be running, got %+v", ollama)
	}
	if len(ollama.Models) != 2 || ollama.Models[0] != "qwen2.5-coder:latest" {
		t.Errorf("unexpected ollama models: %v", ollama.Models)
	}

	// Verify LM Studio
	lmStudio := services[1]
	if lmStudio.ID != "prov-lmstudio" || !lmStudio.Running {
		t.Errorf("expected lmstudio to be running, got %+v", lmStudio)
	}
	if len(lmStudio.Models) != 2 || lmStudio.Models[0] != "qwen2.5-coder-7b-instruct" {
		t.Errorf("unexpected lmstudio models: %v", lmStudio.Models)
	}
}

func TestProbeLocalLLMs_Unreachable(t *testing.T) {
	// Points to ports not listening
	services := ProbeLocalLLMsWithURLs("http://127.0.0.1:59998", "http://127.0.0.1:59999", 50*time.Millisecond)
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].Running {
		t.Errorf("expected ollama to not be running")
	}
	if services[1].Running {
		t.Errorf("expected lmstudio to not be running")
	}
}
