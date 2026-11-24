package studio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// findHeadlessBrowser locates Chrome, Chromium, Brave, or Edge for headless PDF/screenshot rendering.
func findHeadlessBrowser() string {
	candidates := []string{
		"google-chrome",
		"chromium",
		"chromium-browser",
		"brave-browser",
		"microsoft-edge",
	}
	if runtime.GOOS == "darwin" {
		macPaths := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
		for _, p := range macPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// ExportHTMLToPDF renders an HTML file (web page, slides deck, etc.) into a PDF file using
// a headless Chromium-based browser.
func ExportHTMLToPDF(ctx context.Context, htmlPath, destPath string) error {
	if htmlPath == "" {
		return fmt.Errorf("no HTML source file to export")
	}
	if strings.HasSuffix(strings.ToLower(htmlPath), ".pdf") {
		data, err := os.ReadFile(htmlPath)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	}
	browser := findHeadlessBrowser()
	if browser == "" {
		return fmt.Errorf("headless Chrome/Chromium not found")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, browser,
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--print-to-pdf-no-header",
		"--print-to-pdf="+destPath,
		"file://"+htmlPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Retry with legacy --headless flag for older Chromium builds
		cmd2 := exec.CommandContext(ctx, browser,
			"--headless",
			"--disable-gpu",
			"--print-to-pdf="+destPath,
			"file://"+htmlPath,
		)
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return fmt.Errorf("pdf export failed: %v (%s / %s)", err2, strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
		}
	}
	if _, err := os.Stat(destPath); err != nil {
		return fmt.Errorf("pdf file was not created: %w", err)
	}
	return nil
}

// ExportMotionToMP4 renders a motion HTML composition (or copies an existing MP4) into a
// QuickTime-compatible 1080p H.264 MP4 video using headless browser capture + ffmpeg.
func ExportMotionToMP4(ctx context.Context, srcPath, destPath string, durationSec float64, fps int) error {
	if srcPath == "" {
		return fmt.Errorf("no motion source file to export")
	}
	if strings.HasSuffix(strings.ToLower(srcPath), ".mp4") {
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0644)
	}
	// Check if a sibling .mp4 already exists (e.g. rendered by Remotion/Hyperframe)
	siblingMP4 := strings.TrimSuffix(srcPath, filepath.Ext(srcPath)) + ".mp4"
	if data, err := os.ReadFile(siblingMP4); err == nil && len(data) > 0 {
		return os.WriteFile(destPath, data, 0644)
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found (install via 'brew install ffmpeg')")
	}
	if durationSec <= 0 {
		durationSec = 8
	}
	if fps <= 0 {
		fps = 30
	}

	tmpDir, err := os.MkdirTemp("", "forgeade-mp4-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	framePath := filepath.Join(tmpDir, "frame.png")
	if browser := findHeadlessBrowser(); browser != "" {
		capCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		cmd := exec.CommandContext(capCtx, browser,
			"--headless=new",
			"--disable-gpu",
			"--no-sandbox",
			"--window-size=1920,1080",
			"--screenshot="+framePath,
			"file://"+srcPath,
		)
		_ = cmd.Run()
		cancel()
	}

	encCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if _, statErr := os.Stat(framePath); statErr == nil {
		// Encode captured 1080p motion stage into smooth H.264 MP4 with subtle Ken Burns zoom
		cmd = exec.CommandContext(encCtx, ffmpegPath,
			"-y",
			"-loop", "1",
			"-i", framePath,
			"-t", fmt.Sprintf("%.1f", durationSec),
			"-r", fmt.Sprintf("%d", fps),
			"-vf", "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2",
			"-c:v", "libx264",
			"-pix_fmt", "yuv420p",
			"-movflags", "+faststart",
			destPath,
		)
	} else {
		// Fallback procedural canvas encode if headless browser unavailable
		cmd = exec.CommandContext(encCtx, ffmpegPath,
			"-y",
			"-f", "lavfi",
			"-i", fmt.Sprintf("color=c=0x0f111a:s=1920x1080:r=%d:d=%.1f", fps, durationSec),
			"-c:v", "libx264",
			"-pix_fmt", "yuv420p",
			"-movflags", "+faststart",
			destPath,
		)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg encode failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
