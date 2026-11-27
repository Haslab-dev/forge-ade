package ui

import (
	"embed"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

//go:embed assets/avatars/*.png
var embeddedBotAvatars embed.FS

var botAvatarPresets = []string{
	"01_grok_futuristic_bot.png",
	"02_anime_girl_ai.png",
	"03_anime_boy_ai.png",
	"04_chibi_robot.png",
	"05_western_cartoon_bot.png",
	"06_cyberpunk_android.png",
	"07_chrome_mecha_bot.png",
	"08_cat_ai_companion.png",
	"09_fox_intelligence.png",
	"10_owl_knowledge_bot.png",
	"11_panda_assistant.png",
	"12_shiba_inu_bot.png",
	"13_pixel_art_bot.png",
	"14_3d_clay_robot.png",
	"15_anime_mecha_pilot.png",
	"16_abstract_ai_entity.png",
	"17_steampunk_bot.png",
	"18_alien_ai.png",
	"19_ghost_assistant.png",
	"20_minimal_line_art_bot.png",
}

var (
	botAvatarCacheMu sync.Mutex
	botAvatarCache   = make(map[string]ui.ImageSource)
)

const defaultBotAvatarPreset = "01_grok_futuristic_bot.png"

func resolveEmbeddedBotAvatar(name string) ui.ImageSource {
	if name == "" {
		name = defaultBotAvatarPreset
	}
	botAvatarCacheMu.Lock()
	if src, ok := botAvatarCache[name]; ok {
		botAvatarCacheMu.Unlock()
		return src
	}
	botAvatarCacheMu.Unlock()

	data, err := embeddedBotAvatars.ReadFile("assets/avatars/" + name)
	if err != nil {
		return nil
	}
	src, err := ui.DecodeImage(data)
	if err != nil || src == nil {
		return nil
	}
	botAvatarCacheMu.Lock()
	botAvatarCache[name] = src
	botAvatarCacheMu.Unlock()
	return src
}

func (a *AppUI) resolveBotAvatarSource(avatar string) ui.ImageSource {
	avatar = strings.TrimSpace(avatar)
	if avatar == "" {
		return resolveEmbeddedBotAvatar(defaultBotAvatarPreset)
	}
	if !strings.ContainsAny(avatar, "/\\") {
		if src := resolveEmbeddedBotAvatar(avatar); src != nil {
			return src
		}
	}
	if ci := a.getImage(avatar); ci != nil && ci.src != nil {
		return ci.src
	}
	return resolveEmbeddedBotAvatar(defaultBotAvatarPreset)
}

func (a *AppUI) renderBotAvatarImg(c *ui.Context, tc theme.ThemeColors, avatar string, size, radius float32) {
	if src := a.resolveBotAvatarSource(avatar); src != nil {
		ui.Image(c, src).Size(size, size).Radius(radius).Fit(ui.Cover).Shrink(0)
		return
	}
	ui.Box(c).Size(size, size).Radius(radius).AlignItems(ui.Center).Justify(ui.Center).
		Background(tc.Accent.Alpha(0.14)).Shrink(0).Children(func() {
		ui.Icon(c, icons.TablerBot).Width(size * 0.55).Height(size * 0.55).TextColor(tc.Accent)
	})
}

func (a *AppUI) renderBotAvatar(c *ui.Context, tc theme.ThemeColors, bot *agent.BotProfile, size, radius float32) {
	avatar := ""
	if bot != nil {
		avatar = bot.Avatar
	}
	a.renderBotAvatarImg(c, tc, avatar, size, radius)
}

// OpenBotAvatarModal opens the avatar picker modal for botID (or for NewBotModal if botID == "").
func (a *AppUI) OpenBotAvatarModal(botID string) {
	a.BotAvatarTargetID = botID
	a.ShowBotAvatarModal = true
}

// pickBotAvatarFile opens a native file dialog to choose a custom image for a bot.
func (a *AppUI) pickBotAvatarFile(targetBotID string) {
	if a.Window == nil {
		return
	}
	win := a.Window
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win,
			Title:  "Choose Bot Avatar Image",
			Filters: []mygo.FileFilter{
				{
					Name:       "Images",
					Extensions: []string{"png", "jpg", "jpeg", "webp", "gif"},
					MIMETypes:  []string{"image/*"},
				},
			},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.runOnUI(func() {
			if targetBotID == "" {
				a.NewBotAvatar = paths[0]
			} else if a.Bots != nil {
				a.Bots.UpdateAvatar(targetBotID, paths[0])
			}
			a.ShowBotAvatarModal = false
			a.invalidateUI()
		})
	}()
}

func (a *AppUI) renderBotAvatarModal(c *ui.Context) {
	if !a.ShowBotAvatarModal {
		return
	}
	tc := a.ThemeMgr.Colors()
	currentAvatar := a.NewBotAvatar
	botName := "New Bot"
	if a.BotAvatarTargetID != "" && a.Bots != nil {
		if b := a.Bots.Get(a.BotAvatarTargetID); b != nil {
			currentAvatar = b.Avatar
			botName = b.Name
		}
	}
	if currentAvatar == "" {
		currentAvatar = defaultBotAvatarPreset
	}

	a.modal(c, &a.ShowBotAvatarModal, func() {
		modal := ui.Column(c).Width(460).Radius(14).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 20, 56, -16, ui.RGBA(0, 0, 0, 0.55)).Clip()
		modal.Children(func() {
			ui.Row(c).Padding(14, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				a.renderBotAvatarImg(c, tc, currentAvatar, 28, 8)
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "Bot Avatar · "+botName).FontSize(14).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Pick a preset avatar or upload your own image").FontSize(11).TextColor(tc.TextMuted)
				})
				xBtn := ui.Row(c.Key("bam-close")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				xBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if xBtn.Hovered() {
					xBtn.Background(tc.Text.Alpha(0.08))
				}
				if xBtn.Clicked() {
					a.ShowBotAvatarModal = false
				}
			})

			ui.Column(c).Padding(16, 18).Gap(12).Children(func() {
				ui.Row(c).Gap(8).Wrap().Children(func() {
					for _, preset := range botAvatarPresets {
						p := preset
						sel := currentAvatar == p
						tile := ui.Box(c.Key("bam-p-"+p)).Size(56, 56).Radius(12).Padding(2).
							AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).BorderWidth(2)
						if sel {
							tile.BorderColor(tc.Accent).Background(tc.Accent.Alpha(0.14))
						} else {
							tile.BorderColor(tc.Border).Background(tc.SurfaceRaised)
						}
						tile.Children(func() {
							a.renderBotAvatarImg(c, tc, p, 48, 10)
						})
						if tile.Clicked() {
							if a.BotAvatarTargetID == "" {
								a.NewBotAvatar = p
							} else if a.Bots != nil {
								a.Bots.UpdateAvatar(a.BotAvatarTargetID, p)
							}
							a.ShowBotAvatarModal = false
						}
					}
				})

				uploadBtn := ui.Row(c.Key("bam-upload")).Height(32).Radius(8).Padding(0, 12).Gap(7).
					AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
				uploadBtn.Children(func() {
					ui.Icon(c, icons.TablerUpload).Width(14).Height(14).TextColor(tc.Text)
					ui.Text(c, "Choose Custom Image…").FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if uploadBtn.Hovered() {
					uploadBtn.Background(tc.ToolbarControlHover)
				}
				if uploadBtn.Clicked() {
					a.pickBotAvatarFile(a.BotAvatarTargetID)
				}
			})
		})
	})
}
