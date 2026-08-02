package main

import (
	"fmt"
	"strings"
	"time"
)

func init() {
	// Classic line spinners — these redraw in place with \r (no newline),
	// the most common terminal animation primitive.
	registerScene("spinners", "braille + line + dot spinners on \r redraws", sceneSpinners)

	registerScene("progress", "multiple progress bars + percents", sceneProgress)

	registerScene("status", "live updating status line (CPU-ish + labels)", sceneStatus)

	registerScene("clock", "live clock + elapsed + blinking colon", sceneClock)

	registerScene("typewriter", "typewriter text with \r rewrite", sceneTypewriter)
}

// Frame sets for the various spinner styles.
var brailleFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
var lineFrames = []string{"|", "/", "-", "\\"}
var dotFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
var blockFrames = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▂"}

func sceneSpinners(dur time.Duration) {
	start := time.Now()
	i := 0
	t := 0
	for elapsedSince(start) < dur {
		sp := brailleFrames[i%len(brailleFrames)]
		line := lineFrames[(i/2)%len(lineFrames)]
		dot := dotFrames[(i/3)%len(dotFrames)]
		block := blockFrames[(i/2)%len(blockFrames)]
		elapsed := int(elapsedSince(start).Seconds())

		fmt.Printf("\r %s%s%s %s%s%s %s⌛ Crystallizing • %ds • ↓ %d ",
			fg(180, 140, 255), sp, reset,
			fg(120, 180, 255), line, reset,
			fg(255, 180, 120),
			elapsed, t%97,
		)
		fmt.Printf(" %s%s%s %sIndexing%s  %s%s%s", fg(255, 180, 120), dot, reset, fg(120, 255, 180), reset, fg(120, 255, 180), block, reset)
		fmt.Printf(" %s%s%s", fg(255, 120, 120), blockFrames[(i/4)%len(blockFrames)], reset)
		fmt.Print(clearEOL)
		tick(60 * time.Millisecond)
		i++
		t += 7
	}
	fmt.Print("\r" + clearEOL)
}

func sceneProgress(dur time.Duration) {
	start := time.Now()
	barW := 30
	for elapsedSince(start) < dur {
		elapsed := float64(elapsedSince(start).Seconds())
		total := float64(dur.Seconds())
		pct := elapsed / total
		if pct > 1 {
			pct = 1
		}

		fmt.Print("\r\x1b[2A")
		// Bar 1 — downloading
		drawBar("Downloading", pct*0.9, barW, fg(120, 200, 255), "MB/s", 12.4)
		// Bar 2 — building
		drawBar("Building", pct, barW, fg(255, 180, 90), "files", 847)
		fmt.Print(clearEOL)

		// Side stats
		fmt.Printf("\r\x1b[2B")
		fmt.Printf(" %s●%s phase %d/5  %s%5.1f%%%s  %sETA %ds%s%s",
			fg(80, 255, 160), reset,
			int(pct*5)+1,
			fg(255, 255, 120), pct*100, reset,
			fg(255, 120, 120), int(total-elapsed), reset, clearEOL)
		tick(40 * time.Millisecond)
	}
	fmt.Print("\r\x1b[2A" + clearEOL + "\r" + clearEOL + "\r" + clearEOL)
}

func drawBar(label string, frac float64, width int, color, unit string, val float64) {
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	pct := int(frac * 100)
	fmt.Printf("%s%dG%s %-12s%s [%s%s%s] %3d%% %s%.1f %s%s",
		esc, 1, reset, label, color, color, bar, reset, pct, color, val, unit, reset)
}
func sceneStatus(dur time.Duration) {
	start := time.Now()
	labels := []string{"agent", "search", "git", "watcher", "lsp"}
	states := []bool{true, false, true, true, false}
	i := 0
	for elapsedSince(start) < dur {
		// simulate changing states
		if i%12 == 0 {
			states[i/12%len(states)] = !states[i/12%len(states)]
		}
		fmt.Print("\r" + clearEOL)
		fmt.Printf(" %sSTATUS%s ", bold(fg(90, 200, 255)), reset)
		for j, l := range labels {
			on := states[j]
			col := fg(120, 120, 120)
			mark := "○"
			if on {
				col = fg(80, 255, 160)
				mark = "●"
			}
			fmt.Printf(" %s%s%s %s", col, mark, reset, l)
		}
		fmt.Printf("  %s%smem %s%5.1f MB%s  %s%scpu %s%3d%%%s",
			dimOn, reset, reset, float64(300+i*3%100), reset,
			dimOn, reset, reset, 10+i%60, reset)
		fmt.Printf("  %s%s%s", fg(255, 120, 120), brailleFrames[i%len(brailleFrames)], reset)
		fmt.Print(clearEOL)
		tick(80 * time.Millisecond)
		i++
	}
	fmt.Print("\r" + clearEOL)
}

func sceneClock(dur time.Duration) {
	start := time.Now()
	blinkOn := true
	for elapsedSince(start) < dur {
		elapsed := elapsedSince(start)
		colon := ":"
		if !blinkOn {
			colon = " "
		}
		blinkOn = !blinkOn

		fmt.Print("\r" + clearEOL)
		fmt.Printf(" %s██%s %s%s%s %s%d:%02d:%02d%s %s%s%s  %smillis%s %04d%s",
			fg(120, 200, 255), reset,
			fg(255, 255, 120), "CLOCK", reset,
			fg(180, 140, 255), int(elapsed.Hours())%24, int(elapsed.Minutes())%60, int(elapsed.Seconds())%60, reset,
			fg(255, 180, 90), colon, reset,
			dimOn, dimOff, int((elapsed.Milliseconds()%1000)/10), reset)
		fmt.Printf("  %s%selapsed%s %s%s%s", dimOn, reset, reset, fg(120, 255, 180), elapsed.Round(time.Millisecond).String(), reset)
		fmt.Print(clearEOL)
		tick(100 * time.Millisecond)
	}
	fmt.Print("\r" + clearEOL)
}

func sceneTypewriter(dur time.Duration) {
	lines := []string{
		"In the forge where the neon rivers flow,",
		"An editor wakes with a softer glow —",
		"The terminal breathes, a xterm vein,",
		"Raw bytes racing through the pane.",
	}
	start := time.Now()
	lineIdx := 0
	col := 0
	for elapsedSince(start) < dur {
		if lineIdx >= len(lines) {
			lineIdx = 0
			col = 0
			fmt.Print("\r\x1b[4A" + clearEOL)
			fmt.Print("\r\x1b[3A" + clearEOL)
			fmt.Print("\r\x1b[2A" + clearEOL)
			fmt.Print("\r\x1b[1A" + clearEOL)
		}
		line := lines[lineIdx]
		if col >= len(line) {
			col = 0
			lineIdx++
			continue
		}
		col++
		// redraw the current line up to col with a block cursor
		prefix := line[:col]
		cursor := "▌"
		if col >= len(line) {
			cursor = " "
		}
		fmt.Printf("\r%s%s%s%s%s", fg(140, 220, 180), prefix, reset, reverse(cursor), clearEOL)
		tick(45 * time.Millisecond)
	}
	fmt.Print("\r" + clearEOL)
}
