package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// isImageOrSVG returns true if the file is a viewable image or vector SVG.
func isImageOrSVG(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".webp") ||
		strings.HasSuffix(lower, ".bmp") ||
		strings.HasSuffix(lower, ".ico") ||
		strings.HasSuffix(lower, ".svg") ||
		strings.HasSuffix(lower, ".tiff") ||
		strings.HasSuffix(lower, ".tif") ||
		strings.HasSuffix(lower, ".avif")
}

// isSVG reports whether the filename ends with .svg.
func isSVG(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".svg")
}

// cachedImage keeps decoded image/SVG state in memory to prevent re-decoding every frame.
type cachedImage struct {
	fullPath string
	modTime  int64
	src      ui.ImageSource
	width    int
	height   int
	size     int64
	format   string
	err      error
}

// imageCacheCap bounds the decoded-image cache; decoded bitmaps are the
// largest objects the app keeps, so an unbounded map grows without limit.
const imageCacheCap = 32

// imageLRUState tracks insertion/recency order for the image cache. It is
// UI-thread-only state (render calls getImage), like the rest of AppUI.
var (
	imageLRUMu    sync.Mutex
	imageLRUOrder []string
)

// imageCacheTouch records a use (move-to-front) and evicts the
// oldest-inserted entries beyond the cap.
func imageCacheTouch(cache map[string]*cachedImage, fullPath string) {
	imageLRUMu.Lock()
	defer imageLRUMu.Unlock()
	for i, p := range imageLRUOrder {
		if p == fullPath {
			imageLRUOrder = append(imageLRUOrder[:i], imageLRUOrder[i+1:]...)
			break
		}
	}
	imageLRUOrder = append([]string{fullPath}, imageLRUOrder...)
	for len(imageLRUOrder) > imageCacheCap {
		evict := imageLRUOrder[len(imageLRUOrder)-1]
		imageLRUOrder = imageLRUOrder[:len(imageLRUOrder)-1]
		delete(cache, evict)
	}
}

// getImage retrieves or decodes the image source, cached by fullPath and file modTime.
func (a *AppUI) getImage(fullPath string) *cachedImage {
	if a.imageCache == nil {
		a.imageCache = make(map[string]*cachedImage)
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return &cachedImage{fullPath: fullPath, err: err}
	}
	mod := info.ModTime().UnixNano()
	if cached, ok := a.imageCache[fullPath]; ok && cached.modTime == mod && cached.src != nil {
		imageCacheTouch(a.imageCache, fullPath)
		return cached
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return &cachedImage{fullPath: fullPath, err: err}
	}

	ci := &cachedImage{
		fullPath: fullPath,
		modTime:  mod,
		size:     info.Size(),
	}

	lower := strings.ToLower(fullPath)
	if strings.HasSuffix(lower, ".svg") {
		ci.format = "SVG"
		svg, sErr := ui.ParseSVG(data)
		if sErr == nil && svg != nil {
			ci.src = svg
			sw, sh := svg.Size()
			ci.width = int(sw)
			ci.height = int(sh)
		} else {
			// Fallback decode
			src, dErr := ui.DecodeImage(data)
			if dErr == nil && src != nil {
				ci.src = src
			} else {
				ci.err = sErr
			}
		}
	} else {
		// Bitmaps: PNG, JPEG, GIF, WebP, BMP, ICO
		bmp, bErr := ui.DecodeBitmap(data)
		if bErr == nil && bmp != nil {
			ci.src = bmp
			ci.width, ci.height = bmp.Size()
			switch {
			case len(data) > 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
				ci.format = "PNG"
			case len(data) > 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
				ci.format = "JPEG"
			case len(data) > 4 && string(data[:3]) == "GIF":
				ci.format = "GIF"
			case len(data) > 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
				ci.format = "WEBP"
			case len(data) > 2 && string(data[:2]) == "BM":
				ci.format = "BMP"
			case strings.HasSuffix(lower, ".ico"):
				ci.format = "ICO"
			default:
				ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(fullPath), "."))
				if ext != "" {
					ci.format = ext
				} else {
					ci.format = "IMAGE"
				}
			}
		} else {
			// Fallback to DecodeImage
			src, dErr := ui.DecodeImage(data)
			if dErr == nil && src != nil {
				ci.src = src
				ci.format = strings.ToUpper(strings.TrimPrefix(filepath.Ext(fullPath), "."))
			} else {
				ci.err = bErr
			}
		}
	}

	a.imageCache[fullPath] = ci
	imageCacheTouch(a.imageCache, fullPath)
	return ci
}

// formatByteSize formats byte counts into human-readable strings.
func formatByteSize(sz int64) string {
	switch {
	case sz <= 0:
		return ""
	case sz < 1024:
		return fmt.Sprintf("%d B", sz)
	case sz < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(sz)/1024)
	default:
		return fmt.Sprintf("%.2f MB", float64(sz)/(1024*1024))
	}
}

// renderImageViewer renders a full-featured Image & SVG Viewer with zoom, background canvas toggles,
// dimensions, file size, system launcher, and SVG Code/Preview switcher.
func (a *AppUI) renderImageViewer(c *ui.Context, tc theme.ThemeColors, fullPath, relPath string, doc *editor.OpenDocument, isSidePreview bool) {
	ci := a.getImage(fullPath)

	// Ensure maps initialized
	if a.ImageZoom == nil {
		a.ImageZoom = make(map[string]float32)
	}
	if a.ImageFitMode == nil {
		a.ImageFitMode = make(map[string]bool)
	}
	if a.ImageBgStyle == nil {
		a.ImageBgStyle = make(map[string]string)
	}
	if a.SVGViewMode == nil {
		a.SVGViewMode = make(map[string]string)
	}

	bgStyle := a.ImageBgStyle[fullPath]
	if bgStyle == "" {
		bgStyle = "grid" // default
	}

	zoom := a.ImageZoom[fullPath]
	if zoom <= 0 {
		if isSidePreview {
			a.ImageFitMode[fullPath] = true
			zoom = 1.0
		} else {
			zoom = 1.0
		}
		a.ImageZoom[fullPath] = zoom
	}

	fitMode := a.ImageFitMode[fullPath]

	ui.Column(c).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
		// 1. Image Viewer Toolbar
		ui.Row(c).Height(38).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Icon(c, icons.TablerPhoto).Width(15).Height(15).TextColor(tc.Accent)
			ui.Text(c, filepath.Base(relPath)).FontSize(12).FontWeight(600).SingleLine()

			if ci.format != "" {
				Badge(c, tc, ci.format, tc.Accent)
			}

			// Dimensions & file size
			if ci.width > 0 && ci.height > 0 {
				dimText := fmt.Sprintf("%d × %d px", ci.width, ci.height)
				ui.Text(c, dimText).FontSize(11).TextColor(tc.TextMuted)
			}
			if sz := formatByteSize(ci.size); sz != "" {
				ui.Text(c, sz).FontSize(11).TextColor(tc.TextMuted)
			}

			// SVG View Mode Switcher: [Preview | Code]
			if isSVG(relPath) {
				ui.Row(c).Height(24).Radius(5).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
					Padding(2, 2).Gap(2).AlignItems(ui.Center).Children(func() {
					prevBtn := ui.Row(c.Key("svg-tab-prev")).Height(20).Radius(4).Padding(0, 7).Gap(4).AlignItems(ui.Center)
					if a.SVGViewMode[fullPath] != "code" {
						prevBtn.Background(tc.Accent.Alpha(0.18))
						prevBtn.Children(func() {
							ui.Icon(c, icons.TablerEye).Width(11).Height(11).TextColor(tc.Accent)
							ui.Text(c, "Preview").FontSize(10).FontWeight(600).TextColor(tc.Accent)
						})
					} else {
						prevBtn.Children(func() {
							ui.Icon(c, icons.TablerEye).Width(11).Height(11).TextColor(tc.TextMuted)
							ui.Text(c, "Preview").FontSize(10).TextColor(tc.TextMuted)
						})
						if prevBtn.Clicked() {
							a.SVGViewMode[fullPath] = "preview"
						}
					}

					codeBtn := ui.Row(c.Key("svg-tab-code")).Height(20).Radius(4).Padding(0, 7).Gap(4).AlignItems(ui.Center)
					if a.SVGViewMode[fullPath] == "code" {
						codeBtn.Background(tc.Accent.Alpha(0.18))
						codeBtn.Children(func() {
							ui.Icon(c, icons.TablerCode).Width(11).Height(11).TextColor(tc.Accent)
							ui.Text(c, "Code").FontSize(10).FontWeight(600).TextColor(tc.Accent)
						})
					} else {
						codeBtn.Children(func() {
							ui.Icon(c, icons.TablerCode).Width(11).Height(11).TextColor(tc.TextMuted)
							ui.Text(c, "Code").FontSize(10).TextColor(tc.TextMuted)
						})
						if codeBtn.Clicked() {
							a.SVGViewMode[fullPath] = "code"
						}
					}
				})
			}

			ui.Box(c).Grow(1)

			// Zoom Controls: [-] [Zoom %] [+] [Fit]
			ui.Row(c).Height(26).Radius(6).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
				Padding(0, 4).Gap(2).AlignItems(ui.Center).Children(func() {
				// Zoom Out
				zoomOut := ui.Row(c.Key("img-zoom-out")).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
				zoomOut.Children(func() {
					ui.Icon(c, icons.TablerZoomOut).Width(12).Height(12).TextColor(tc.IconMuted)
				})
				if zoomOut.Hovered() {
					zoomOut.Background(tc.Text.Alpha(0.06))
				}
				if zoomOut.Clicked() {
					a.ImageFitMode[fullPath] = false
					if zoom > 0.25 {
						a.ImageZoom[fullPath] = zoom - 0.25
					}
				}

				// Zoom label / Reset to 100%
				zoomLabel := "Fit"
				if !fitMode {
					zoomLabel = fmt.Sprintf("%d%%", int(zoom*100+0.5))
				}
				zoomPill := ui.Row(c.Key("img-zoom-reset")).Height(20).Radius(4).Padding(0, 6).AlignItems(ui.Center)
				zoomPill.Children(func() {
					ui.Text(c, zoomLabel).FontSize(10).FontWeight(600).TextColor(tc.Text)
				})
				if zoomPill.Hovered() {
					zoomPill.Background(tc.Text.Alpha(0.06))
				}
				if zoomPill.Clicked() {
					a.ImageFitMode[fullPath] = false
					a.ImageZoom[fullPath] = 1.0
				}

				// Zoom In
				zoomIn := ui.Row(c.Key("img-zoom-in")).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
				zoomIn.Children(func() {
					ui.Icon(c, icons.TablerZoomIn).Width(12).Height(12).TextColor(tc.IconMuted)
				})
				if zoomIn.Hovered() {
					zoomIn.Background(tc.Text.Alpha(0.06))
				}
				if zoomIn.Clicked() {
					a.ImageFitMode[fullPath] = false
					if zoom < 10.0 {
						a.ImageZoom[fullPath] = zoom + 0.25
					}
				}

				// Fit Toggle
				fitBtn := ui.Row(c.Key("img-fit-btn")).Height(20).Radius(4).Padding(0, 6).AlignItems(ui.Center)
				if fitMode {
					fitBtn.Background(tc.Accent.Alpha(0.18))
					fitBtn.Children(func() {
						ui.Text(c, "Fit").FontSize(10).FontWeight(600).TextColor(tc.Accent)
					})
				} else {
					fitBtn.Children(func() {
						ui.Text(c, "Fit").FontSize(10).TextColor(tc.TextMuted)
					})
				}
				if fitBtn.Hovered() && !fitMode {
					fitBtn.Background(tc.Text.Alpha(0.06))
				}
				if fitBtn.Clicked() {
					a.ImageFitMode[fullPath] = !fitMode
				}
			})

			// Background Canvas Toggle: [Grid / Dark / Light]
			bgBtn := ui.Row(c.Key("img-bg-btn")).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			bgBtn.Children(func() {
				label := "Grid"
				switch bgStyle {
				case "dark":
					label = "Dark"
				case "light":
					label = "Light"
				}
				ui.Text(c, "Bg: "+label).FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
			})
			if bgBtn.Hovered() {
				bgBtn.Background(tc.SurfaceRaised)
			}
			if bgBtn.Clicked() {
				switch bgStyle {
				case "grid":
					a.ImageBgStyle[fullPath] = "dark"
				case "dark":
					a.ImageBgStyle[fullPath] = "light"
				case "light":
					a.ImageBgStyle[fullPath] = "grid"
				default:
					a.ImageBgStyle[fullPath] = "grid"
				}
			}

			// Open in Default App
			openAppBtn := ui.Row(c.Key("img-open-app")).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			openAppBtn.Children(func() {
				ui.Icon(c, icons.TablerExternalLink).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.Text(c, "Open").FontSize(10).FontWeight(500)
			})
			if openAppBtn.Hovered() {
				openAppBtn.Background(tc.SurfaceRaised)
			}
			if openAppBtn.Clicked() {
				_ = mygo.Shell.OpenPath(fullPath)
			}

			// In Right Panel Preview: "Open in Editor" link
			if isSidePreview {
				openInEditorBtn := ui.Row(c.Key("img-open-in-editor")).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
				openInEditorBtn.Children(func() {
					ui.Text(c, "Open in Editor").FontSize(10).FontWeight(600).TextColor(tc.Accent)
				})
				if openInEditorBtn.Hovered() {
					openInEditorBtn.Background(tc.Accent.Alpha(0.2))
				}
				if openInEditorBtn.Clicked() {
					_, _ = a.Editor.OpenFile(fullPath, relPath)
					a.Mode = ModeEditor
				}
			}
		})

		// 2. Interactive Image Canvas. Backdrops come from the active theme's
		// semantic tokens (never raw hex): canvas for the default grid tone,
		// the code background for the dark swatch, the overlay surface for
		// the light swatch — whatever the current palette defines them to be.
		canvasBg := tc.Canvas
		switch bgStyle {
		case "dark":
			canvasBg = tc.CodeBackground
		case "light":
			canvasBg = tc.SurfaceOverlay
		case "grid":
			canvasBg = tc.Canvas
		}

		if ci.err != nil || ci.src == nil {
			// Error state
			ui.Column(c).Grow(1).Center().Gap(14).Padding(24).Children(func() {
				ui.Box(c).Size(56, 56).Radius(12).Background(tc.Error.Alpha(0.12)).
					AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerAlertTriangle).Width(28).Height(28).TextColor(tc.Error)
				})
				ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
					ui.Text(c, filepath.Base(relPath)).FontSize(14).FontWeight(600)
					if ci.err != nil {
						ui.Text(c, fmt.Sprintf("Failed to load image: %v", ci.err)).FontSize(12).TextColor(tc.Error)
					}
					ui.Text(c, fullPath).FontSize(11).TextColor(tc.TextMuted.Alpha(0.7)).MaxWidth(380).SingleLine()
				})
				ui.Row(c).Gap(10).Children(func() {
					cta := ui.PrimaryButton(c, "Open with System Viewer").FontSize(12)
					if cta.Clicked() {
						_ = mygo.Shell.OpenPath(fullPath)
					}
					rf := ui.Button(c, "Reveal in Finder").FontSize(12)
					if rf.Clicked() {
						mygo.Shell.ShowItemInFolder(fullPath)
					}
				})
			})
			return
		}

		// Centered Scrollable Canvas
		ui.ScrollBoth(c).Grow(1).FillWidth().Background(canvasBg).Children(func() {
			centerBox := ui.Column(c).Grow(1).FillWidth().Center().Padding(28)
			centerBox.Children(func() {
				// Outer card around image to give distinct shadow and border
				imgCard := ui.Column(c.Key("img-card-"+fullPath)).Radius(6).Clip().
					BorderWidth(1).BorderColor(tc.Border.Alpha(0.3)).
					Shadow(0, 10, 32, -4, ui.RGBA(0, 0, 0, 0.45))

				// Give the card a tone related to the chosen backdrop.
				if bgStyle == "grid" {
					imgCard.Background(tc.SurfaceRaised)
				} else if bgStyle == "light" {
					imgCard.Background(tc.SurfaceOverlay)
				} else {
					imgCard.Background(tc.CodeBackground)
				}

				imgCard.Children(func() {
					if fitMode {
						maxDim := float32(720)
						if isSidePreview {
							maxDim = float32(360)
						}
						ui.Image(c, ci.src).Fit(ui.Contain).MaxWidth(maxDim).MaxHeight(maxDim)
					} else {
						if ci.width > 0 && ci.height > 0 {
							targetW := float32(ci.width) * zoom
							targetH := float32(ci.height) * zoom
							ui.Image(c, ci.src).Size(targetW, targetH)
						} else {
							ui.Image(c, ci.src).Fit(ui.Contain).Grow(1)
						}
					}
				})
			})
		})

		// 3. Footer Status Bar
		ui.Row(c).Height(24).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, relPath).FontSize(10).TextColor(tc.TextMuted).SingleLine()
			ui.Box(c).Grow(1)
			if ci.width > 0 && ci.height > 0 {
				ui.Text(c, fmt.Sprintf("%d × %d px", ci.width, ci.height)).FontSize(10).TextColor(tc.TextMuted)
			}
			zoomText := fmt.Sprintf("%.0f%%", zoom*100)
			if fitMode {
				zoomText = "Fit"
			}
			ui.Text(c, "Zoom: "+zoomText).FontSize(10).TextColor(tc.TextMuted)
		})
	})
}
