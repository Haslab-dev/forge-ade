package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/usage"
	"github.com/google/uuid"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type LLMMessage struct {
	Role         Role       `json:"role"`
	Content      string     `json:"content"`
	Name         string     `json:"name,omitempty"`
	ToolCallID   string     `json:"tool_call_id,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl marks a message for provider prompt caching (Anthropic-style
// `{"type":"ephemeral"}`). Applied to the system message so the stable prefix
// (system prompt + tool defs) stays cached across turns. OpenAI-compatible
// gateways ignore unknown message fields, so this is safe to send broadly.
type CacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

type FunctionSpec struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// MCPTool is a tool discovered from an MCP server, registered into the tool
// registry under its full "server/tool" name.
type MCPTool struct {
	ServerName  string                 `json:"server_name"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type TokenStats struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CachedTokens     int `json:"cached_tokens"`
	// PromptCacheHitTokens / PromptCacheMissTokens: DeepSeek's context-cache
	// accounting (hit+miss == prompt). Anthropic-style providers only set
	// CachedTokens; DeepSeek providers leave it 0.
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

type LLMResponse struct {
	Content      string     `json:"content"`
	Reasoning    string     `json:"reasoning,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	TokenUsage   TokenStats `json:"token_usage"`
}

type ProviderConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	EnvKey       string `json:"env_key"`
	DefaultModel string `json:"default_model"`
	RequiresKey  bool   `json:"requires_key"`
}

type ProviderProfile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	APIKey          string   `json:"api_key"`
	BaseURL         string   `json:"base_url"`
	Enabled         bool     `json:"enabled"`
	AvailableModels []string `json:"available_models"`
	SelectedModels  []string `json:"selected_models"`
}

var SupportedProviders = map[string]ProviderConfig{
	"openai": {
		ID:           "openai",
		Name:         "OpenAI",
		BaseURL:      "https://api.openai.com/v1",
		EnvKey:       "OPENAI_API_KEY",
		DefaultModel: "gpt-4o",
		RequiresKey:  true,
	},
	"deepseek": {
		ID:           "deepseek",
		Name:         "DeepSeek",
		BaseURL:      "https://api.deepseek.com/v1",
		EnvKey:       "DEEPSEEK_API_KEY",
		DefaultModel: "deepseek-chat",
		RequiresKey:  true,
	},
	"gemini": {
		ID:           "gemini",
		Name:         "Gemini",
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta/openai",
		EnvKey:       "GEMINI_API_KEY",
		DefaultModel: "gemini-2.0-flash",
		RequiresKey:  true,
	},
	"anthropic": {
		ID:           "anthropic",
		Name:         "Anthropic",
		BaseURL:      "https://api.anthropic.com/v1",
		EnvKey:       "ANTHROPIC_API_KEY",
		DefaultModel: "claude-3-5-sonnet-latest",
		RequiresKey:  true,
	},
	"ollama": {
		ID:           "ollama",
		Name:         "Ollama (Local)",
		BaseURL:      "http://localhost:11434/v1",
		EnvKey:       "OLLAMA_API_KEY",
		DefaultModel: "llama3.1:8b",
		RequiresKey:  false,
	},
	"groq": {
		ID:           "groq",
		Name:         "Groq",
		BaseURL:      "https://api.groq.com/openai/v1",
		EnvKey:       "GROQ_API_KEY",
		DefaultModel: "llama-3.3-70b-versatile",
		RequiresKey:  true,
	},
	"openrouter": {
		ID:           "openrouter",
		Name:         "OpenRouter",
		BaseURL:      "https://openrouter.ai/api/v1",
		EnvKey:       "OPENROUTER_API_KEY",
		DefaultModel: "openai/gpt-4o",
		RequiresKey:  true,
	},
	"mistral": {
		ID:           "mistral",
		Name:         "Mistral",
		BaseURL:      "https://api.mistral.ai/v1",
		EnvKey:       "MISTRAL_API_KEY",
		DefaultModel: "mistral-large-latest",
		RequiresKey:  true,
	},
}

type Profile struct {
	ProviderID string `json:"provider_id"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
}

type LLMClient struct {
	mu            sync.RWMutex
	providerID    string
	apiKey        string
	baseURL       string
	model         string
	httpClient    *http.Client
	profilePath   string
	providersPath string
	profiles      []ProviderProfile

	// usageStore records per-request usage telemetry (observability).
	usageStore *usage.Store
	// usageMeta is the static context attached to each recorded event
	// (workspace/session/agent identity) — set by the agent layer per turn.
	usageMeta UsageMeta
}

// UsageMeta carries the identity context for a batch of LLM calls.
type UsageMeta struct {
	WorkspaceID   string
	WorkspaceName string
	SessionID     string
	Agent         string
}

// SetUsageStore wires the observability store.
func (c *LLMClient) SetUsageStore(s *usage.Store) {
	c.mu.Lock()
	c.usageStore = s
	c.mu.Unlock()
}

// SetUsageMeta sets the current identity context for recorded events.
func (c *LLMClient) SetUsageMeta(meta UsageMeta) {
	c.mu.Lock()
	c.usageMeta = meta
	c.mu.Unlock()
}

// UsageMetaSnapshot returns a copy of the current usage metadata.
func (c *LLMClient) UsageMetaSnapshot() UsageMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.usageMeta
}

func NewLLMClient(dataDir string) *LLMClient {
	c := &LLMClient{
		providerID:    "openai",
		baseURL:       SupportedProviders["openai"].BaseURL,
		model:         SupportedProviders["openai"].DefaultModel,
		httpClient:    &http.Client{Timeout: 120 * time.Second},
		profilePath:   filepath.Join(dataDir, "profiles.json"),
		providersPath: filepath.Join(dataDir, "providers_config.json"),
		profiles:      make([]ProviderProfile, 0),
	}
	c.loadProfiles()
	return c
}

func (c *LLMClient) loadProfiles() {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.providersPath)
	if err == nil {
		var list []ProviderProfile
		if json.Unmarshal(data, &list) == nil {
			c.profiles = list
			for _, p := range list {
				if p.Enabled && len(p.SelectedModels) > 0 {
					c.providerID = p.ID
					c.apiKey = p.APIKey
					c.baseURL = p.BaseURL
					c.model = p.SelectedModels[0]
					return
				}
			}
			return
		}
	}

	// No default fallback profiles; all profiles are custom user-managed providers
	c.profiles = make([]ProviderProfile, 0)
}

func (c *LLMClient) GetProviderProfiles() []ProviderProfile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.profiles
}

func (c *LLMClient) SaveProviderProfiles(profiles []ProviderProfile) error {
	c.mu.Lock()
	c.profiles = profiles

	for _, p := range profiles {
		if p.Enabled && len(p.SelectedModels) > 0 {
			c.providerID = p.ID
			c.apiKey = p.APIKey
			c.baseURL = p.BaseURL
			c.model = p.SelectedModels[0]
			break
		}
	}
	c.mu.Unlock()

	_ = os.MkdirAll(filepath.Dir(c.providersPath), 0755)
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.providersPath, data, 0644)
}

func (c *LLMClient) SetActiveModel(providerID string, model string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.profiles {
		p := &c.profiles[i]
		if p.ID == providerID {
			c.providerID = p.ID
			c.apiKey = p.APIKey
			c.baseURL = p.BaseURL
			c.model = model
			// Move the selected model to the front of SelectedModels so the
			// active choice persists with the profile.
			var next []string
			next = append(next, model)
			for _, m := range p.SelectedModels {
				if m != model {
					next = append(next, m)
				}
			}
			p.SelectedModels = next
			break
		}
	}

	// Persist so the active model survives restarts.
	_ = os.MkdirAll(filepath.Dir(c.providersPath), 0755)
	if data, err := json.MarshalIndent(c.profiles, "", "  "); err == nil {
		_ = os.WriteFile(c.providersPath, data, 0644)
	}
	// Persist the active profile file too.
	_ = c.saveActiveProfileLocked()
}

// saveActiveProfileLocked writes the active profile to disk. Caller must hold c.mu.
func (c *LLMClient) saveActiveProfileLocked() error {
	p := Profile{
		ProviderID: c.providerID,
		APIKey:     c.apiKey,
		BaseURL:    c.baseURL,
		Model:      c.model,
	}
	_ = os.MkdirAll(filepath.Dir(c.profilePath), 0755)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.profilePath, data, 0644)
}

// SetActiveModelByID sets the active provider/model from an opencode-style
// "provider/model" id (e.g. "anthropic/claude-sonnet-4-5").
func (c *LLMClient) SetActiveModelByID(fullID string) {
	providerID, model := splitModelID(fullID)
	if providerID == "" || model == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Match the longest provider prefix in known profiles.
	var best *ProviderProfile
	for i := range c.profiles {
		p := &c.profiles[i]
		if fullID == p.ID+"/"+model || providerID == p.ID {
			best = p
			break
		}
	}
	if best == nil {
		return
	}
	c.providerID = best.ID
	c.apiKey = best.APIKey
	c.baseURL = best.BaseURL
	c.model = model
}

// splitModelID splits "provider/model" into its parts, handling model ids that
// themselves contain slashes by matching against known provider prefixes.
func splitModelID(fullID string) (string, string) {
	idx := strings.Index(fullID, "/")
	if idx <= 0 || idx == len(fullID)-1 {
		return "", fullID
	}
	return fullID[:idx], fullID[idx+1:]
}

func (c *LLMClient) FetchModels(ctx context.Context, apiKey string, baseURL string) ([]string, error) {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	var reqURL string
	if strings.Contains(baseURL, "11434") || strings.Contains(baseURL, "ollama") {
		reqURL = "http://localhost:11434/api/tags"
	} else {
		reqURL = baseURL + "/models"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var models []string

	// Try Ollama structure
	var ollamaResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err == nil && len(ollamaResp.Models) > 0 {
		for _, m := range ollamaResp.Models {
			models = append(models, m.Name)
		}
		return models, nil
	}

	// Try OpenAI structure
	var openAIResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openAIResp); err == nil && len(openAIResp.Data) > 0 {
		for _, m := range openAIResp.Data {
			models = append(models, m.ID)
		}
		return models, nil
	}

	return []string{}, nil
}

func (c *LLMClient) SaveProfile(providerID, apiKey, baseURL, model string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.providerID = providerID
	c.apiKey = apiKey
	c.baseURL = baseURL
	c.model = model

	return c.saveActiveProfileLocked()
}

func (c *LLMClient) GetConfig() Profile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Profile{
		ProviderID: c.providerID,
		APIKey:     c.apiKey,
		BaseURL:    c.baseURL,
		Model:      c.model,
	}
}

func (c *LLMClient) ListProviders() []ProviderConfig {
	list := make([]ProviderConfig, 0, len(SupportedProviders))
	for _, p := range SupportedProviders {
		list = append(list, p)
	}
	return list
}

func (c *LLMClient) Chat(ctx context.Context, messages []LLMMessage, tools []ToolDefinition) (*LLMResponse, error) {
	c.mu.RLock()
	providerID := c.providerID
	model := c.model
	c.mu.RUnlock()
	return c.ChatWithProvider(ctx, providerID, model, messages, tools)
}

func (c *LLMClient) ChatWithStream(ctx context.Context, messages []LLMMessage, tools []ToolDefinition, onChunk func(deltaContent string, deltaReasoning string)) (*LLMResponse, error) {
	c.mu.RLock()
	providerID := c.providerID
	model := c.model
	c.mu.RUnlock()
	return c.ChatWithProviderStream(ctx, providerID, model, messages, tools, onChunk, nil)
}

// ChatWithStreamDetailed is ChatWithStream plus a streamed tool-call fragment
// callback. Each fragment carries the accumulating index so the caller can
// track partial tool-call arguments as they arrive
// toolcall_delta events).
type ToolCallDelta struct {
	Index       int
	ID          string
	Name        string
	ArgFragment string
}

func (c *LLMClient) ChatWithStreamDetailed(ctx context.Context, messages []LLMMessage, tools []ToolDefinition, onChunk func(deltaContent string, deltaReasoning string), onToolCallDelta func(delta ToolCallDelta)) (*LLMResponse, error) {
	c.mu.RLock()
	providerID := c.providerID
	model := c.model
	c.mu.RUnlock()
	return c.ChatWithProviderStream(ctx, providerID, model, messages, tools, onChunk, onToolCallDelta)
}

func (c *LLMClient) ChatWithProvider(ctx context.Context, targetProviderID string, targetModel string, messages []LLMMessage, tools []ToolDefinition) (*LLMResponse, error) {
	return c.ChatWithProviderStream(ctx, targetProviderID, targetModel, messages, tools, nil, nil)
}

func (c *LLMClient) ChatWithProviderStream(
	ctx context.Context,
	targetProviderID string,
	targetModel string,
	messages []LLMMessage,
	tools []ToolDefinition,
	onChunk func(deltaContent string, deltaReasoning string),
	onToolCallDelta func(delta ToolCallDelta),
) (respOut *LLMResponse, errOut error) {
	start := time.Now()
	// Estimate input tokens from the request (used on error paths where the
	// provider returns no usage).
	inputEst := int64(0)
	for _, msg := range messages {
		inputEst += estimateTokens(msg.Content)
		for _, tc := range msg.ToolCalls {
			inputEst += estimateTokens(tc.Function.Arguments)
		}
	}
	// Record usage telemetry on every exit path (success or failure).
	defer func() {
		c.recordUsage(targetProviderID, targetModel, start, inputEst, respOut, errOut)
	}()
	c.mu.RLock()
	var baseURL, apiKey string
	model := targetModel

	for _, p := range c.profiles {
		if p.ID == targetProviderID {
			baseURL = p.BaseURL
			apiKey = p.APIKey
			if model == "" && len(p.SelectedModels) > 0 {
				model = p.SelectedModels[0]
			}
			break
		}
	}
	if baseURL == "" {
		baseURL = c.baseURL
		apiKey = c.apiKey
		if model == "" {
			model = c.model
		}
	}
	c.mu.RUnlock()

	if apiKey == "" && SupportedProviders[targetProviderID].EnvKey != "" {
		apiKey = os.Getenv(SupportedProviders[targetProviderID].EnvKey)
	}

	// Deduplicate system messages and strip CacheControl for non-Anthropic providers.
	// Many OpenAI-compatible gateways (OpenRouter, Gemini, Groq) strictly enforce
	// exactly 1 system message and reject unknown fields (e.g. CacheControl).
	var sanitizedMessages []LLMMessage
	var firstSystemSeen bool
	for _, m := range messages {
		msg := m
		if targetProviderID != "anthropic" {
			msg.CacheControl = nil
		}
		if msg.Role == RoleSystem {
			if firstSystemSeen {
				// Combine extra system message into a user message or append to first system message
				if len(sanitizedMessages) > 0 && sanitizedMessages[0].Role == RoleSystem {
					sanitizedMessages[0].Content += "\n\n" + msg.Content
				}
				continue
			}
			firstSystemSeen = true
		}
		sanitizedMessages = append(sanitizedMessages, msg)
	}

	isStreaming := onChunk != nil
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": sanitizedMessages,
		"stream":   isStreaming,
	}

	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[llm] HTTP error status %d provider=%q model=%q: %s", resp.StatusCode, targetProviderID, model, string(respBody))
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if isStreaming {
		// Some providers ignore stream:true and reply with a plain JSON body;
		// normal providers stream SSE incrementally. Peek the first non-space
		// byte to route WITHOUT buffering the whole body — reading everything
		// first would destroy real-time streaming.
		br := bufio.NewReader(resp.Body)
		// Peek past leading whitespace to find the first byte.
		var first byte
		for {
			b, err := br.Peek(1)
			if err != nil || len(b) == 0 {
				break
			}
			if b[0] == ' ' || b[0] == '\n' || b[0] == '\r' || b[0] == '\t' {
				br.ReadByte()
				continue
			}
			first = b[0]
			break
		}
		if first == '{' {
			// Provider ignored stream:true — read the JSON body and parse it.
			body, readErr := io.ReadAll(br)
			if readErr != nil {
				return nil, fmt.Errorf("read response body: %w", readErr)
			}
			if len(bytes.TrimSpace(body)) == 0 {
				log.Printf("[llm] WARNING: empty response body (status 200) for streaming request; provider=%q model=%q", targetProviderID, model)
				return nil, fmt.Errorf("empty response body (status 200) — provider returned nothing")
			}
			return parseOpenAIJSON(body)
		}
		// Normal SSE stream — parse INCREMENTALLY so chunks reach the UI in
		// real time (thinking + content streaming).
		sseResp, sseErr := parseSSEStream(ctx, br, onChunk, onToolCallDelta)
		if sseErr != nil {
			return nil, fmt.Errorf("parse stream response: %w", sseErr)
		}
		if sseResp.Content == "" && sseResp.Reasoning == "" && len(sseResp.ToolCalls) == 0 {
			log.Printf("[llm] WARNING: SSE stream produced no content")
		}
		return sseResp, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if len(bytes.TrimSpace(respBody)) == 0 {
		// Status 200 with an empty body: some providers/proxies only answer
		// streaming requests and return an empty body to stream:false calls.
		// Surface this as an error instead of silently returning empty content.
		log.Printf("[llm] WARNING: empty response body (status 200) for non-streaming request; provider=%q model=%q", targetProviderID, model)
		return nil, fmt.Errorf("empty response body (status 200) — provider may only support streaming requests")
	}

	var openAIResp openAICompletionResp

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		log.Printf("[llm] unmarshal failed: %v; body=%s", err, truncateBody(respBody))
		// Some OpenAI-compatible providers/proxies reply in SSE format even
		// when asked for a non-streaming response. Fall back to parsing the
		// body as an SSE stream so calls like AI commit generation don't fail
		// with "unmarshal response: unexpected json input".
		if sseResp, sseErr := parseSSEStream(ctx, bytes.NewReader(respBody), nil, nil); sseErr == nil {
			return sseResp, nil
		}
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return buildLLMResponse(openAIResp, respBody)
}

// openAICompletionResp mirrors the OpenAI chat-completions response shape
// (also used by compatible providers/proxies).
type openAICompletionResp struct {
	Choices []struct {
		Message struct {
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			Reasoning        string     `json:"reasoning"`
			ToolCalls        []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage TokenStats `json:"usage"`
}

// parseOpenAIJSON parses a plain JSON chat-completions body (non-SSE).
func parseOpenAIJSON(body []byte) (*LLMResponse, error) {
	var openAIResp openAICompletionResp
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return buildLLMResponse(openAIResp, body)
}

// buildLLMResponse converts a parsed OpenAI-style chat completion into an
// LLMResponse, logging the raw body when choices are missing so silent empty
// responses are diagnosable.
func buildLLMResponse(openAIResp openAICompletionResp, body []byte) (*LLMResponse, error) {
	if len(openAIResp.Choices) == 0 {
		log.Printf("[llm] WARNING: response has no choices; body=%s", truncateBody(body))
		return &LLMResponse{Content: ""}, nil
	}

	choice := openAIResp.Choices[0]
	cached := openAIResp.Usage.CachedTokens
	if openAIResp.Usage.PromptCacheHitTokens > 0 {
		cached = openAIResp.Usage.PromptCacheHitTokens
	}

	return &LLMResponse{
		Content:   choice.Message.Content,
		Reasoning: choice.Message.Reasoning,
		ToolCalls: choice.Message.ToolCalls,
		TokenUsage: TokenStats{
			PromptTokens:          openAIResp.Usage.PromptTokens,
			CompletionTokens:      openAIResp.Usage.CompletionTokens,
			CachedTokens:          cached,
			PromptCacheHitTokens:  openAIResp.Usage.PromptCacheHitTokens,
			PromptCacheMissTokens: openAIResp.Usage.PromptCacheMissTokens,
			TotalTokens:           openAIResp.Usage.TotalTokens,
		},
	}, nil
}

// recordUsage writes a usage.Event for a completed LLM request (observability).
func (c *LLMClient) recordUsage(provider string, model string, start time.Time, inputEst int64, resp *LLMResponse, err error) {
	c.mu.RLock()
	store := c.usageStore
	meta := c.usageMeta
	// Resolve the provider's human-readable name (custom profiles carry a
	// user-set Name; built-ins come from SupportedProviders). Falls back to
	// the raw id (e.g. "custom-1785427998771") when unknown.
	providerName := provider
	for _, p := range c.profiles {
		if p.ID == provider && p.Name != "" {
			providerName = p.Name
			break
		}
	}
	if providerName == provider {
		if cfg, ok := SupportedProviders[provider]; ok && cfg.Name != "" {
			providerName = cfg.Name
		}
	}
	c.mu.RUnlock()
	if store == nil {
		return
	}

	ev := &usage.Event{
		ID:            uuid.NewString(),
		Timestamp:     time.Now(),
		WorkspaceID:   meta.WorkspaceID,
		WorkspaceName: meta.WorkspaceName,
		SessionID:     meta.SessionID,
		Agent:         meta.Agent,
		Provider:      providerName,
		Model:         model,
		LatencyMS:     time.Since(start).Milliseconds(),
		Success:       err == nil,
	}
	if resp != nil {
		ev.InputTokens = int64(resp.TokenUsage.PromptTokens)
		ev.OutputTokens = int64(resp.TokenUsage.CompletionTokens)
		ev.CachedTokens = int64(resp.TokenUsage.CachedTokens)
		if resp.TokenUsage.PromptCacheHitTokens > 0 {
			ev.CachedTokens = int64(resp.TokenUsage.PromptCacheHitTokens)
		}
		ev.ThinkingTokens = estimateTokens(resp.Reasoning)
		ev.ToolCalls = len(resp.ToolCalls)
		ev.CostUSD = estimateCost(model, int64(resp.TokenUsage.PromptTokens), int64(resp.TokenUsage.CompletionTokens))
	}
	if err != nil {
		// Failed calls still count input tokens from the request estimate.
		ev.InputTokens = inputEst
	}
	store.Record(ev)
}

// estimateTokens approximates token count from byte length (~4 chars/token).
func estimateTokens(s string) int64 {
	if s == "" {
		return 0
	}
	return int64((len(s) + 3) / 4)
}

// estimateCost approximates USD cost. Rough per-1M-token rates — good enough
// for observability. input $3/1M, output $15/1M, cached $0.3/1M.
func estimateCost(model string, in, out int64) float64 {
	const inRate = 3.0 / 1_000_000
	const outRate = 15.0 / 1_000_000
	return float64(in)*inRate + float64(out)*outRate
}
func parseSSEStream(ctx context.Context, r io.Reader, onChunk func(deltaContent string, deltaReasoning string), onToolCallDelta func(delta ToolCallDelta)) (*LLMResponse, error) {
	var fullContent strings.Builder
	var fullReasoning strings.Builder
	toolCallMap := make(map[int]*ToolCall)
	var toolCallOrder []int
	var usage TokenStats

	// Capture a few raw SSE lines so an empty result is diagnosable (the
	// provider may use a different event shape than "data: {...}").
	var rawSample []string
	const rawSampleMax = 6

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(rawSample) < rawSampleMax {
			rawSample = append(rawSample, line)
		}
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					Reasoning        string `json:"reasoning"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens          int `json:"prompt_tokens"`
				CompletionTokens      int `json:"completion_tokens"`
				CachedTokens          int `json:"cached_tokens"` // Anthropic-style: cached input subset
				PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`  // DeepSeek
				PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"` // DeepSeek
				TotalTokens           int `json:"total_tokens"`
			} `json:"usage"`
			Error *struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err == nil {
			// Provider sent an error payload inside the SSE stream — surface
			// it instead of silently treating the stream as empty.
			if chunk.Error != nil && chunk.Error.Message != "" {
				msg := chunk.Error.Message
				if chunk.Error.Type != "" {
					msg += " (" + chunk.Error.Type + ")"
				}
				log.Printf("[llm] SSE provider error payload received: %s (raw data: %s)", msg, data)
				return nil, fmt.Errorf("provider error: %s", msg)
			}
			if chunk.Usage != nil {
				if chunk.Usage.TotalTokens > 0 {
					usage.PromptTokens = chunk.Usage.PromptTokens
					usage.CompletionTokens = chunk.Usage.CompletionTokens
					usage.CachedTokens = chunk.Usage.CachedTokens
					usage.PromptCacheHitTokens = chunk.Usage.PromptCacheHitTokens
					usage.PromptCacheMissTokens = chunk.Usage.PromptCacheMissTokens
					usage.TotalTokens = chunk.Usage.TotalTokens
				}
			}
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				contentDelta := delta.Content
				reasoningDelta := delta.ReasoningContent
				if reasoningDelta == "" {
					reasoningDelta = delta.Reasoning
				}

				if contentDelta != "" || reasoningDelta != "" {
					fullContent.WriteString(contentDelta)
					fullReasoning.WriteString(reasoningDelta)
					if onChunk != nil {
						onChunk(contentDelta, reasoningDelta)
					}
				}

				for _, tcChunk := range delta.ToolCalls {
					idx := tcChunk.Index
					existing, ok := toolCallMap[idx]
					if !ok {
						toolCallMap[idx] = &ToolCall{
							ID:   tcChunk.ID,
							Type: tcChunk.Type,
							Function: ToolFunction{
								Name:      tcChunk.Function.Name,
								Arguments: tcChunk.Function.Arguments,
							},
						}
						toolCallOrder = append(toolCallOrder, idx)
					} else {
						if tcChunk.ID != "" {
							existing.ID = tcChunk.ID
						}
						if tcChunk.Type != "" {
							existing.Type = tcChunk.Type
						}
						if tcChunk.Function.Name != "" {
							existing.Function.Name += tcChunk.Function.Name
						}
						existing.Function.Arguments += tcChunk.Function.Arguments
					}
					if onToolCallDelta != nil {
						onToolCallDelta(ToolCallDelta{
							Index:       idx,
							ID:          tcChunk.ID,
							Name:        tcChunk.Function.Name,
							ArgFragment: tcChunk.Function.Arguments,
						})
					}
				}
			}
		}
	}

	var accumulatedToolCalls []ToolCall
	for _, idx := range toolCallOrder {
		if tc, ok := toolCallMap[idx]; ok {
			accumulatedToolCalls = append(accumulatedToolCalls, *tc)
		}
	}

	// Diagnose empty streams: dump the first raw lines so we can see the
	// provider's actual SSE shape (it may use "event:" fields, different
	// deltas, or an error payload that our parser skips).
	if fullContent.Len() == 0 && fullReasoning.Len() == 0 && len(accumulatedToolCalls) == 0 {
		log.Printf("[llm] SSE stream empty; first raw lines:\n%s", strings.Join(rawSample, "\n"))
	}

	return &LLMResponse{
		Content:    fullContent.String(),
		Reasoning:  fullReasoning.String(),
		ToolCalls:  accumulatedToolCalls,
		TokenUsage: usage,
	}, nil
}

// truncateBody caps a raw response body for logging so a huge or binary body
// doesn't flood the log.
func truncateBody(b []byte) string {
	const max = 2000
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + fmt.Sprintf("... (%d more bytes)", len(b)-max)
}
