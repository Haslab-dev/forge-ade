package main

import (
	"fmt"
	"strings"
	"time"
)

func init() {
	registerScene("gradient", "truecolor gradient sweep + rainbow bar", sceneGradient)
	registerScene("alt-screen", "full-screen alt-buffer TUI (like htop)", sceneAltScreen)
	registerScene("boxdraw", "animated box-drawing + Unicode glyphs", sceneBoxDraw)
	registerScene("sync", "DEC 2026 synchronized output burst", sceneSync)
	registerScene("palette", "256-color + truecolor palette blocks", scenePalette)
}

func sceneGradient(dur time.Duration) {
	cols, rows := size()
	start := time.Now()
	t := 0.0
	for elapsedSince(start) < dur {
		fmt.Print(clearScr + home)
		// full-screen smooth gradient
		for r := 1; r <= rows-2; r++ {
			line := ""
			for c := 0; c < cols-1; c++ {
				// rainbow across width, breathing with time
				hue := float64(c)/float64(cols) + t*0.02
				rgb := hsvToRGB(hue, 0.8, 1.0)
				line += fg(rgb[0], rgb[1], rgb[2]) + "▄"
			}
			fmt.Print(line + reset + "\n")
		}
		// breathing rainbow bar at bottom
		fmt.Printf("%s%d;%dH%s%s%s%s", esc, rows-1, 1, reset, fg(255, 255, 255), strings.Repeat("█", cols-1), reset)
		fmt.Printf("%s%d;%dH%s", esc, rows, 1, reset)
		fmt.Printf("%s truecolor sweep  t=%.2f %s", fg(255, 255, 255), t, reset)
		fmt.Print(clearEOL)
		t += 0.5
		tick(30 * time.Millisecond)
	}
	fmt.Print(clearScr + home)
}

func hsvToRGB(h, s, v float64) [3]int {
	i := int(h * 6)
	f := h*6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - s*f)
	tt := v * (1 - s*(1-f))
	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, tt, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, tt
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = tt, p, v
	case 5:
		r, g, b = v, p, q
	}
	return [3]int{int(r * 255), int(g * 255), int(b * 255)}
}

func sceneAltScreen(dur time.Duration) {
	// Enter alt-screen buffer — the same thing htop/less/vim do.
	fmt.Print(altOn + hideCur)
	cols, rows := size()
	start := time.Now()
	i := 0
	progress := 0.0
	for elapsedSince(start) < dur {
		progress = float64(elapsedSince(start).Seconds()) / float64(dur.Seconds())
		fmt.Print(clearScr + home)
		// header bar (full width)
		header := " FORGEADE  shell_test full-screen TUI — alt-screen buffer (like htop) "
		fmt.Printf("%s%s%-*s%s", fg(0, 0, 0), bg(90, 200, 255), cols-1, header, reset)
		// fake process list — each row drawn with cursor control, no \n flow
		procs := []struct {
			name string
			cpu  int
			spin int
		}{
			{"cmd", 40 + i%30, i % 10},
			{"node", 20 + i%15, (i / 2) % 10},
			{"git", 5 + i%8, (i / 3) % 10},
			{"zsh", 2 + i%4, (i / 4) % 10},
			{"forge-ade", 15 + i%12, (i / 5) % 10},
		}
		for j, p := range procs {
			row := j + 3
			// name column
			fmt.Printf("%s%d;%dH %s%-10s%s", esc, row, 1, fg(120, 200, 255), p.name, reset)
			// cpu bar column (width aware: barW scales with terminal width)
			barW := (cols - 34) / 2
			if barW < 5 {
				barW = 5
			}
			filled := p.cpu * barW / 100
			if filled > barW {
				filled = barW
			}
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)
			fmt.Printf("%s%d;%dH %s%s%s %3d%%", esc, row, 13, fg(80, 255, 160), bar, reset, p.cpu)
			// spinner at right
			fmt.Printf("%s%d;%dH %s%s%s", esc, row, cols-3, fg(255, 180, 120), brailleFrames[p.spin%len(brailleFrames)], reset)
		}
		// progress bar (full width, width aware)
		loadW := cols - 12
		if loadW < 10 {
			loadW = 10
		}
		loadFilled := int(progress * float64(loadW))
		fmt.Printf("%s%d;%dH %sload:%s %s%s%s %3d%%",
			esc, rows-2, 1,
			fg(255, 255, 120), reset,
			fg(255, 120, 120), strings.Repeat("█", loadFilled)+strings.Repeat("░", loadW-loadFilled), reset,
			int(progress*100))
		// bottom status bar (reverse video, full width)
		status := " ForgeADE shell_test "
		fmt.Printf("%s%d;%dH%s%s %s t+%d %s%sESC=exit%s",
			esc, rows, 1,
			reverse(status), reset,
			fg(120, 255, 180), int(elapsedSince(start).Seconds()),
			dimOn, reset, dimOff)
		fmt.Printf("%s%d;%dH%s", esc, rows, cols, clearEOL)
		i++
		tick(50 * time.Millisecond)
	}
	fmt.Print(showCur + altOff)
}

func sceneBoxDraw(dur time.Duration) {
	cols, rows := size()
	start := time.Now()
	i := 0
	for elapsedSince(start) < dur {
		fmt.Print(clearScr + home)
		// animated box — width/height clamp to the real terminal so no
		// line can wrap. Drawn with absolute cursor positioning (ESC[r;cH)
		// from a fixed origin — never \n flow, so the box cannot drift or
		// accumulate indentation even on a narrow terminal.
		originRow, originCol := 1, 1
		maxW := cols - originCol - 2
		if maxW < 12 {
			maxW = 12
		}
		w := 26 + i%8
		if w > maxW {
			w = maxW
		}
		if w < 12 {
			w = 12
		}
		maxH := rows - originRow - 6
		if maxH < 4 {
			maxH = 4
		}
		h := 6 + (i/3)%3
		if h > maxH {
			h = maxH
		}
		if h < 4 {
			h = 4
		}
		inner := w - 2

		// top border
		fmt.Printf("%s%d;%dH%s%s%s%s",
			esc, originRow, originCol, fg(180, 140, 255), "╭", strings.Repeat("─", inner), "╮")
		// box body rows — each row at its own absolute position
		for r := 0; r < h-2; r++ {
			row := originRow + 1 + r
			// left border
			fmt.Printf("%s%d;%dH%s│%s", esc, row, originCol, fg(180, 140, 255), reset)
			if r == 0 {
				pad := inner - 12
				if pad < 1 {
					pad = 1
				}
				fmt.Printf("%s%s%s%s%s%s", " ", bold(fg(255, 255, 120)), "BOX DRAWING", reset, strings.Repeat(" ", pad), "")
			} else if r == h-3 {
				// animated inner line with a moving marker
				maxPos := inner - 3
				if maxPos < 1 {
					maxPos = 1
				}
				pos := i % maxPos
				line := strings.Repeat("░", pos) + "●" + strings.Repeat("░", inner-pos-3)
				fmt.Printf("%s%s", fg(120, 255, 180), line)
			} else {
				// diagonal fill
				fmt.Printf("%s", strings.Repeat("░", inner))
			}
			// right border at the exact column
			fmt.Printf("%s%d;%dH%s│%s", esc, row, originCol+inner+1, fg(180, 140, 255), reset)
		}
		// bottom border
		fmt.Printf("%s%d;%dH%s%s%s%s",
			esc, originRow+h-1, originCol, fg(180, 140, 255), "╰", strings.Repeat("─", inner), "╮")

		// Unicode glyph row — exercises multi-byte UTF-8, truncated to fit
		glyphs := []string{"⠶", "⣿", "█", "▓", "▒", "░", "◉", "◆", "★", "♥", "♫", "☀", "⌘", "∑", "∞", "≈", "♻", "⚙", "✈", "⚡", "🎀", "🚀", "🌊", "⛰"}
		glyphRow := originRow + h
		avail := cols - originCol - 2
		shown := 0
		fmt.Printf("%s%d;%dH ", esc, glyphRow, originCol)
		for _, g := range glyphs {
			shown += 2 // glyph + space
			if shown > avail {
				break
			}
			h2 := hsvToRGB(float64(i%len(glyphs))/float64(len(glyphs)), 0.8, 1.0)
			fmt.Print(fg(h2[0], h2[1], h2[2]) + g + reset + " ")
		}
		fmt.Printf("%d glyphs ", len(glyphs))
		fmt.Print(reset + clearEOL)
		i++
		tick(80 * time.Millisecond)
	}
	fmt.Print(clearScr + home)
}

func sceneSync(dur time.Duration) {
	// DEC 2026 synchronized output — batches many small writes into one
	// atomic frame so the terminal doesn't flicker mid-frame.
	start := time.Now()
	cols, _ := size()
	i := 0
	for elapsedSince(start) < dur {
		fmt.Print(clearScr + home + syncOn)
		// build a big frame as one string
		var sb strings.Builder
		sb.WriteString(fg(90, 200, 255) + " SYNC " + reset + dim(" DEC 2026 ") + reset)
		sb.WriteString(fmt.Sprintf(" frame %d\n", i))
		for r := 0; r < 8; r++ {
			offset := (i + r*3) % 20
			sb.WriteString(" ")
			for c := 0; c < cols-2; c++ {
				if (c+offset)%20 < 10 {
					sb.WriteString(fg(120, 255, 160) + "█" + reset)
				} else {
					sb.WriteString(fg(60, 60, 60) + "░" + reset)
				}
			}
			sb.WriteString("\n")
		}
		// spinner
		sb.WriteString(" " + fg(255, 180, 120) + brailleFrames[i%len(brailleFrames)] + reset + " synchronized write burst\n")
		fmt.Print(sb.String())
		fmt.Print(syncOff)
		fmt.Print(clearEOL)
		i++
		tick(50 * time.Millisecond)
	}
	fmt.Print(clearScr + home)
}

func scenePalette(dur time.Duration) {
	// Static palette but with a moving highlight column to prove color
	// fidelity across a wide swath of the ANSI/256/truecolor space.
	start := time.Now()
	i := 0
	for elapsedSince(start) < dur {
		fmt.Print(clearScr + home)
		// 16 basic colors
		fmt.Println(bold(" 16-COLOR "))
		for c := 0; c < 8; c++ {
			fmt.Printf(" %s███%s", fg256(c), reset)
		}
		fmt.Println()
		for c := 8; c < 16; c++ {
			fmt.Printf(" %s███%s", fg256(c), reset)
		}
		fmt.Println()
		// 256-color cube rows (sample)
		fmt.Println(bold(" 256-COLOR CUBE "))
		for row := 16; row < 256; row += 8 {
			for c := row; c < row+8 && c < 256; c++ {
				fmt.Printf(" %s██%s", fg256(c), reset)
			}
			fmt.Println()
		}
		// truecolor sweep
		fmt.Println(bold(" TRUECOLOR "))
		for c := 0; c < 40; c++ {
			col := hsvToRGB(float64((c+i)/40.0), 1.0, 1.0)
			fmt.Printf("%s█%s", fg(col[0], col[1], col[2]), reset)
		}
		fmt.Println()
		// highlight label
		hl := i % 40
		fmt.Printf("%s%d;%dH%s▲%s", esc, 4+2+4, hl+1, fg(255, 255, 255), reset)
		fmt.Print(curPos(24, 1))
		fmt.Printf(" palette scan  %s%04d%s", fg(255, 180, 90), i, reset)
		fmt.Print(clearEOL)
		i++
		tick(60 * time.Millisecond)
	}
	fmt.Print(clearScr + home)
}
