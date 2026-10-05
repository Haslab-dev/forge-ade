package main

import "github.com/hasdev/forge-ade/internal/llm"

func (a *App) GetProviderProfiles() []llm.ProviderProfile {
	return a.llmClient.GetProviderProfiles()
}

// SaveProviderProfiles saves configured provider profiles.
func (a *App) SaveProviderProfiles(profiles []llm.ProviderProfile) error {
	err := a.llmClient.SaveProviderProfiles(profiles)
	if err != nil {
		return err
	}
	a.emitAgentConfigChanged()
	return nil
}

// SyncAgentProviders merges the frontend's provider list (localStorage) into
// the Go-side provider profiles so the internal agent harness authenticates
// with the same keys and model lists the UI configured. Matching is by ID
// first, then base URL; settings-configured profiles are never clobbered.
// Returns the merged profile list.
func (a *App) SyncAgentProviders(profiles []llm.ProviderProfile) ([]llm.ProviderProfile, error) {
	merged, err := a.llmClient.SyncProviderProfiles(profiles)
	if err != nil {
		return nil, err
	}
	a.emitAgentConfigChanged()
	return merged, nil
}

// emitAgentConfigChanged notifies open agent chats that provider/model/agent
// config changed so they refresh their model list and active model.
func (a *App) emitAgentConfigChanged() {
	if a.ctx == nil {
		return
	}
	a.emitEvent("agent:config:changed", map[string]interface{}{})
}

// FetchProviderModels fetches model list from provider endpoint.
func (a *App) FetchProviderModels(apiKey string, baseURL string) ([]string, error) {
	return a.llmClient.FetchModels(a.ctx, apiKey, baseURL)
}

// SetActiveModel sets active LLM model.
func (a *App) SetActiveModel(providerID string, model string) error {
	a.llmClient.SetActiveModel(providerID, model)
	return nil
}

// SaveLLMProfile updates active LLM provider profile.
func (a *App) SaveLLMProfile(providerID, apiKey, baseURL, model string) error {
	return a.llmClient.SaveProfile(providerID, apiKey, baseURL, model)
}

// GetLLMConfig gets active LLM profile config.
func (a *App) GetLLMConfig() llm.Profile {
	return a.llmClient.GetConfig()
}

// ListLLMProviders lists all supported LLM providers.
func (a *App) ListLLMProviders() []llm.ProviderConfig {
	return a.llmClient.ListProviders()
}

// ListSkills returns loaded SKILL.md skills.
