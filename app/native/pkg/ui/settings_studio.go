package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	corestudio "github.com/hasdev/forge-ade/core/studio"
)

// StudioSettingsDraft holds transient form values during editing.
type StudioSettingsDraft struct {
	Loaded        bool
	ImageProvider string
	ImageEndpoint string
	ImageAPIKey   string
	ImageModel    string

	VideoProvider string
	VideoEndpoint string
	VideoAPIKey   string
	VideoModel    string

	TTSProvider string
	TTSEndpoint string
	TTSAPIKey   string
	TTSVoiceID  string

	SavedNotice bool
}

var studioDraft StudioSettingsDraft

func loadStudioDraft() {
	if studioDraft.Loaded {
		return
	}
	cfg := corestudio.LoadStudioConfig()
	studioDraft = StudioSettingsDraft{
		Loaded:        true,
		ImageProvider: cfg.ImageProvider,
		ImageEndpoint: cfg.ImageEndpoint,
		ImageAPIKey:   cfg.ImageAPIKey,
		ImageModel:    cfg.ImageModel,
		VideoProvider: cfg.VideoProvider,
		VideoEndpoint: cfg.VideoEndpoint,
		VideoAPIKey:   cfg.VideoAPIKey,
		VideoModel:    cfg.VideoModel,
		TTSProvider:   cfg.TTSProvider,
		TTSEndpoint:   cfg.TTSEndpoint,
		TTSAPIKey:     cfg.TTSAPIKey,
		TTSVoiceID:    cfg.TTSVoiceID,
	}
}

// renderSettingsTabStudio renders the configuration screen for Studio generative media & APIs.
func (a *AppUI) renderSettingsTabStudio(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Studio & Generative APIs",
		"Configure custom endpoints, API credentials, and models for Image, Video, and Voice generation.")

	loadStudioDraft()

	ui.Column(c.Key("settings-studio-pane")).Gap(20).Children(func() {

		// 1. Image Generation Custom API Card
		renderSettingSectionCard(c, tc, "IMAGE GENERATION (NODE WORKFLOW API)", icons.TablerPhoto, func() {
			ui.Column(c).Gap(12).Children(func() {
				// Provider & Model Row
				ui.Row(c).Gap(12).Children(func() {
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "API Provider").FontSize(11).FontWeight(600).TextColor(tc.Text)
						ui.TextInput(c, &studioDraft.ImageProvider).Placeholder("custom, openai, stability, or local").Grow(1)
					})
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "Model Identifier").FontSize(11).FontWeight(600).TextColor(tc.Text)
						ui.TextInput(c, &studioDraft.ImageModel).Placeholder("e.g. flux-1-schnell, dall-e-3, sdxl").Grow(1)
					})
				})

				// Endpoint URL
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Endpoint URL").FontSize(11).FontWeight(600).TextColor(tc.Text)
					ui.TextInput(c, &studioDraft.ImageEndpoint).Placeholder("http://localhost:8000/v1/images/generations").Grow(1)
				})

				// API Key
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "API Key (Optional for local endpoints)").FontSize(11).FontWeight(600).TextColor(tc.Text)
					ui.TextInput(c, &studioDraft.ImageAPIKey).Placeholder("sk-...").Grow(1)
				})
			})
		})

		// 2. Video Generation Custom API Card
		renderSettingSectionCard(c, tc, "VIDEO GENERATION WORKFLOW API", icons.TablerMovie, func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.Row(c).Gap(12).Children(func() {
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "Video Provider").FontSize(11).FontWeight(600).TextColor(tc.Text)
						ui.TextInput(c, &studioDraft.VideoProvider).Placeholder("custom, replicate, luma, or local").Grow(1)
					})
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "Video Model").FontSize(11).FontWeight(600).TextColor(tc.Text)
						ui.TextInput(c, &studioDraft.VideoModel).Placeholder("e.g. animate-diff-v3, luma-ray").Grow(1)
					})
				})

				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Endpoint URL").FontSize(11).FontWeight(600).TextColor(tc.Text)
					ui.TextInput(c, &studioDraft.VideoEndpoint).Placeholder("http://localhost:8000/v1/videos/generations").Grow(1)
				})

				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "API Key").FontSize(11).FontWeight(600).TextColor(tc.Text)
					ui.TextInput(c, &studioDraft.VideoAPIKey).Placeholder("Bearer token or API Key").Grow(1)
				})
			})
		})

		// 3. TTS Voice Synthesis API Card (ElevenLabs)
		renderSettingSectionCard(c, tc, "TEXT-TO-SPEECH (TTS) & VOICE SYNTHESIS", icons.TablerPlayerPlay, func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.Row(c).Gap(12).Children(func() {
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "TTS Provider").FontSize(11).FontWeight(600).TextColor(tc.Text)
						ui.TextInput(c, &studioDraft.TTSProvider).Placeholder("elevenlabs, openai, or local").Grow(1)
					})
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "Default Voice ID").FontSize(11).FontWeight(600).TextColor(tc.Text)
						ui.TextInput(c, &studioDraft.TTSVoiceID).Placeholder("e.g. 21m00Tcm4TlvDq8ikWAM (Rachel)").Grow(1)
					})
				})

				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "ElevenLabs / TTS API Key").FontSize(11).FontWeight(600).TextColor(tc.Text)
					ui.TextInput(c, &studioDraft.TTSAPIKey).Placeholder("xi-api-key-...").Grow(1)
				})
			})
		})

		// Save Action Row
		ui.Row(c.Key("studio-settings-actions")).AlignItems(ui.Center).Gap(12).Children(func() {
			saveBtn := ui.Row(c.Key("btn-save-studio-settings")).Height(32).Padding(0, 16).Radius(6).
				Background(tc.Accent).AlignItems(ui.Center).Gap(6).Cursor(ui.CursorPointer)
			saveBtn.Children(func() {
				ui.Icon(c, icons.TablerCheck).Width(13).Height(13).TextColor(tc.AccentForeground)
				ui.Text(c, "Save Studio API Settings").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
			})
			if saveBtn.Clicked() {
				cfg := corestudio.StudioConfig{
					ImageProvider: studioDraft.ImageProvider,
					ImageEndpoint: studioDraft.ImageEndpoint,
					ImageAPIKey:   studioDraft.ImageAPIKey,
					ImageModel:    studioDraft.ImageModel,
					VideoProvider: studioDraft.VideoProvider,
					VideoEndpoint: studioDraft.VideoEndpoint,
					VideoAPIKey:   studioDraft.VideoAPIKey,
					VideoModel:    studioDraft.VideoModel,
					TTSProvider:   studioDraft.TTSProvider,
					TTSEndpoint:   studioDraft.TTSEndpoint,
					TTSAPIKey:     studioDraft.TTSAPIKey,
					TTSVoiceID:    studioDraft.TTSVoiceID,
				}
				_ = corestudio.SaveStudioConfig(cfg)
				studioDraft.SavedNotice = true
			}

			if studioDraft.SavedNotice {
				ui.Row(c).Padding(4, 10).Radius(4).Background(tc.Success.Alpha(0.15)).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerCircleCheck).Width(12).Height(12).TextColor(tc.Success)
					ui.Text(c, "Studio API settings saved successfully").FontSize(11).FontWeight(500).TextColor(tc.Success)
				})
			}
		})
	})
}

func renderSettingSectionCard(c *ui.Context, tc theme.ThemeColors, title string, icon *ui.SVG, content func()) {
	ui.Column(c.Key("card-"+title)).Radius(10).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
		Padding(16, 18).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Icon(c, icon).Width(14).Height(14).TextColor(tc.Accent)
			ui.Text(c, title).FontSize(11).FontWeight(700).LetterSpacing(0.5).TextColor(tc.Text)
		})
		ui.Box(c).Size(1, 1).Background(tc.Border)
		content()
	})
}
