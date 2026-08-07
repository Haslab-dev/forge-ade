package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatWithProviderStream_StripsCacheControlForNonAnthropic(t *testing.T) {
	var reqBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/completions" {
			var err error
			reqBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer ts.Close()

	client := NewLLMClient(t.TempDir())
	_ = client.SaveProviderProfiles([]ProviderProfile{
		{
			ID:             "openai",
			Name:           "OpenAI",
			BaseURL:        ts.URL,
			Enabled:        true,
			SelectedModels: []string{"gpt-4"},
		},
	})

	messages := []LLMMessage{
		{
			Role:    RoleSystem,
			Content: "You are helpful.",
			CacheControl: &CacheControl{
				Type: "ephemeral",
			},
		},
	}

	_, err := client.ChatWithProvider(context.Background(), "openai", "gpt-4", messages, nil)
	if err != nil {
		t.Fatalf("ChatWithProvider: %v", err)
	}

	if strings.Contains(string(reqBody), "cache_control") {
		t.Fatalf("expected cache_control to be stripped for non-Anthropic provider, got: %s", string(reqBody))
	}
}

func TestChatWithProviderStream_KeepsCacheControlForAnthropic(t *testing.T) {
	var reqBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/completions" {
			var err error
			reqBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer ts.Close()

	client := NewLLMClient(t.TempDir())
	_ = client.SaveProviderProfiles([]ProviderProfile{
		{
			ID:             "anthropic",
			Name:           "Anthropic",
			BaseURL:        ts.URL,
			Enabled:        true,
			SelectedModels: []string{"claude-3"},
		},
	})

	messages := []LLMMessage{
		{
			Role:    RoleSystem,
			Content: "You are helpful.",
			CacheControl: &CacheControl{
				Type: "ephemeral",
			},
		},
	}

	_, err := client.ChatWithProvider(context.Background(), "anthropic", "claude-3", messages, nil)
	if err != nil {
		t.Fatalf("ChatWithProvider: %v", err)
	}

	if !strings.Contains(string(reqBody), "cache_control") {
		t.Fatalf("expected cache_control to be present for Anthropic provider, got: %s", string(reqBody))
	}
}

func stripCacheControl(msgs []LLMMessage) []LLMMessage {
	out := make([]LLMMessage, len(msgs))
	for i, m := range msgs {
		msg := m
		msg.CacheControl = nil
		out[i] = msg
	}
	return out
}

func TestStripCacheControl_NilCacheControl(t *testing.T) {
	messages := []LLMMessage{
		{Role: RoleUser, Content: "hello"},
	}
	result := stripCacheControl(messages)
	if len(result) != 1 || result[0].CacheControl != nil {
		t.Fatalf("expected unchanged message without cache_control")
	}
}

func TestStripCacheControl_MixedMessages(t *testing.T) {
	messages := []LLMMessage{
		{Role: RoleSystem, Content: "sys", CacheControl: &CacheControl{Type: "ephemeral"}},
		{Role: RoleUser, Content: "user", CacheControl: &CacheControl{Type: "ephemeral", TTL: "5m"}},
		{Role: RoleAssistant, Content: "asst"},
	}
	result := stripCacheControl(messages)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	for i, m := range result {
		if m.CacheControl != nil {
			t.Fatalf("message %d: expected nil CacheControl, got %+v", i, m.CacheControl)
		}
	}
}

func TestStripCacheControl_PreservesOtherFields(t *testing.T) {
	messages := []LLMMessage{
		{
			Role:    RoleUser,
			Content: "hello",
			Name:    "user1",
			CacheControl: &CacheControl{Type: "ephemeral"},
		},
	}
	result := stripCacheControl(messages)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].Name != "user1" {
		t.Fatalf("expected Name preserved, got %q", result[0].Name)
	}
	if result[0].Content != "hello" {
		t.Fatalf("expected Content preserved, got %q", result[0].Content)
	}
	if result[0].CacheControl != nil {
		t.Fatalf("expected CacheControl stripped")
	}
}

func TestLLMMessageJSON_RoundTripPreservesCacheControl(t *testing.T) {
	msgs := []LLMMessage{
		{
			Role:    RoleSystem,
			Content: "prompt",
			CacheControl: &CacheControl{Type: "ephemeral", TTL: "5m"},
		},
	}
	b, err := json.Marshal(msgs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out []LLMMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out[0].CacheControl == nil || out[0].CacheControl.Type != "ephemeral" || out[0].CacheControl.TTL != "5m" {
		t.Fatalf("cache_control lost in JSON round-trip: %+v", out[0])
	}
}
