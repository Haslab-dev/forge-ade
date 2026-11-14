package ui

import (
	"fmt"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// openPdfViewer initializes or switches the PDF viewer for a file.
func (a *AppUI) openPdfViewer(fullPath string) {
	if fullPath == "" {
		return
	}
	if a.PDFWebView != nil && !a.PDFWebView.IsDestroyed() {
		if a.PDFCurrentPath == fullPath {
			return
		}
		a.PDFWebView.Destroy()
		a.PDFWebView = nil
	}
	a.PDFCurrentPath = fullPath
	if a.Window != nil && mygo.WebViewAvailable() {
		wv, err := a.Window.NewWebView(mygo.WebViewOptions{URL: "file://" + fullPath})
		if err == nil {
			a.PDFWebView = wv
		}
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// closePdfViewer tears down the PDF webview.
func (a *AppUI) closePdfViewer() {
	if a.PDFWebView != nil && !a.PDFWebView.IsDestroyed() {
		a.PDFWebView.Destroy()
	}
	a.PDFWebView = nil
	a.PDFCurrentPath = ""
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// renderPdfViewerPane renders the PDF Document viewer with toolbar, system integration, and native WKWebView PDF preview.
func (a *AppUI) renderPdfViewerPane(c *ui.Context, tc theme.ThemeColors, fullPath, fileName string) {
	if a.PDFCurrentPath != fullPath {
		a.openPdfViewer(fullPath)
	}

	var fileSizeStr string
	if fi, err := os.Stat(fullPath); err == nil {
		sz := fi.Size()
		switch {
		case sz < 1024:
			fileSizeStr = fmt.Sprintf("%d B", sz)
		case sz < 1024*1024:
			fileSizeStr = fmt.Sprintf("%.1f KB", float64(sz)/1024)
		default:
			fileSizeStr = fmt.Sprintf("%.1f MB", float64(sz)/(1024*1024))
		}
	}

	ui.Column(c).Grow(1).FillHeight().Background(tc.Canvas).Children(func() {
		// 1. Toolbar
		ui.Row(c).Height(38).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Icon(c, icons.TablerFileText).Width(15).Height(15).TextColor(tc.Error)
			ui.Text(c, fileName).FontSize(12).FontWeight(600).SingleLine()
			Badge(c, tc, "PDF", tc.Error)
			if fileSizeStr != "" {
				ui.Text(c, fileSizeStr).FontSize(11).TextColor(tc.TextMuted)
			}

			ui.Box(c).Grow(1)

			// Open in Default App (Preview / Acrobat)
			openAppBtn := ui.Row(c.Key("pdf-open-app")).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			openAppBtn.Children(func() {
				ui.Icon(c, icons.TablerExternalLink).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.Text(c, "Open in Default App").FontSize(11).FontWeight(500)
			})
			if openAppBtn.Hovered() {
				openAppBtn.Background(tc.SurfaceRaised)
			}
			if openAppBtn.Clicked() {
				_ = mygo.Shell.OpenPath(fullPath)
			}

			// Reveal in Finder
			finderBtn := ui.Row(c.Key("pdf-reveal-finder")).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
			finderBtn.Children(func() {
				ui.Icon(c, icons.TablerFolder).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.Text(c, "Reveal in Finder").FontSize(11).FontWeight(500)
			})
			if finderBtn.Hovered() {
				finderBtn.Background(tc.SurfaceRaised)
			}
			if finderBtn.Clicked() {
				mygo.Shell.ShowItemInFolder(fullPath)
			}
		})

		// 2. Center Content
		if a.PDFWebView != nil && !a.PDFWebView.IsDestroyed() {
			ui.WebView(c, a.PDFWebView).Grow(1).FillWidth()
		} else {
			ui.Column(c).Grow(1).Center().Gap(14).Padding(24).Children(func() {
				ui.Box(c).Size(56, 56).Radius(12).Background(tc.Error.Alpha(0.12)).
					AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerFileText).Width(28).Height(28).TextColor(tc.Error)
				})
				ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
					ui.Text(c, fileName).FontSize(14).FontWeight(600)
					if fileSizeStr != "" {
						ui.Text(c, fmt.Sprintf("PDF Document · %s", fileSizeStr)).FontSize(12).TextColor(tc.TextMuted)
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
		}

		// 3. Status Bar
		ui.Row(c).Height(22).Padding(0, 10).Gap(6).AlignItems(ui.Center).
			Background(tc.Surface.Alpha(0.4)).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, fullPath).FontSize(10).TextColor(tc.TextMuted).Grow(1).SingleLine()
			if fileSizeStr != "" {
				ui.Text(c, fileSizeStr).FontSize(10).TextColor(tc.TextMuted)
			}
		})
	})
}
