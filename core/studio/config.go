package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// StudioConfig stores custom API settings for Image, Video, and Voice generation.
type StudioConfig struct {
	// Image Generation Custom API
	ImageProvider string `json:"imageProvider"` // "custom", "openai", "stability", "replicate", "local"
	ImageEndpoint string `json:"imageEndpoint"` // e.g. "http://localhost:8000/v1/images/generations"
	ImageAPIKey   string `json:"imageApiKey"`
	ImageModel    string `json:"imageModel"` // e.g. "flux-1-schnell", "dall-e-3"

	// Video Generation Custom API
	VideoProvider string `json:"videoProvider"` // "custom", "replicate", "luma", "runway", "local"
	VideoEndpoint string `json:"videoEndpoint"`
	VideoAPIKey   string `json:"videoApiKey"`
	VideoModel    string `json:"videoModel"`

	// TTS Audio Custom API (e.g. ElevenLabs)
	TTSProvider string `json:"ttsProvider"` // "elevenlabs", "openai", "local"
	TTSEndpoint string `json:"ttsEndpoint"`
	TTSAPIKey   string `json:"ttsApiKey"`
	TTSVoiceID  string `json:"ttsVoiceId"` // e.g. "21m00Tcm4TlvDq8ikWAM"
}

// DefaultStudioConfig returns baseline defaults for Studio settings.
func DefaultStudioConfig() StudioConfig {
	return StudioConfig{
		ImageProvider: "custom",
		ImageEndpoint: "http://localhost:8000/v1/images/generations",
		ImageModel:    "flux-1-schnell",
		VideoProvider: "custom",
		VideoEndpoint: "http://localhost:8000/v1/videos/generations",
		VideoModel:    "animate-diff-v3",
		TTSProvider:   "elevenlabs",
		TTSEndpoint:   "https://api.elevenlabs.io/v1/text-to-speech",
		TTSVoiceID:    "21m00Tcm4TlvDq8ikWAM",
	}
}

var (
	configMu     sync.RWMutex
	cachedConfig *StudioConfig
)

// ConfigFilePath returns the path to studio_settings.json.
func ConfigFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge-ade", "studio_settings.json")
}

// LoadStudioConfig reads configuration from ~/.forge-ade/studio_settings.json.
func LoadStudioConfig() StudioConfig {
	configMu.Lock()
	defer configMu.Unlock()

	if cachedConfig != nil {
		return *cachedConfig
	}

	cfg := DefaultStudioConfig()
	data, err := os.ReadFile(ConfigFilePath())
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	cachedConfig = &cfg
	return cfg
}

// SaveStudioConfig writes configuration to ~/.forge-ade/studio_settings.json.
func SaveStudioConfig(cfg StudioConfig) error {
	configMu.Lock()
	defer configMu.Unlock()

	p := ConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(p, data, 0644); err != nil {
		return err
	}

	cachedConfig = &cfg
	return nil
}
