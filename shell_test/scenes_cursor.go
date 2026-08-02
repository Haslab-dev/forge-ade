package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func init() {
	registerScene("sinewave", "sine wave traced with cursor movement", sceneSineWave)
	registerScene("bounce", "ball bouncing off terminal edges", sceneBounce)
	registerScene("matrix", "matrix rain with random glyphs", sceneMatrix)
	registerScene("sparkline", "live-updating sparkline/bar chart", sceneSparkline)
	registerScene("concurrent", "multiple scenes updating at once", sceneConcurrent)
}

func sceneSineWave(dur time.Duration) {
	cols, rows := size()
	amp := rows/2 - 4
	if amp < 3 {
		amp = 3
	}
	mid := rows / 2
	start := time.Now()
	t := 0.0
	for elapsedSince(start) < dur {
		fmt.Print(clearScr + home)
		// draw axis
		for r := 1; r <= rows-2; r++ {
			if r == mid {
				fmt.Printf("%s%d;1H%s%s%s", esc, r, fg(60, 60, 60), "─"+strings.Repeat("─", cols-2), reset)
			}
		}
		// draw wave
		for x := 0; x < cols-1; x++ {
			y := mid + int(math.Sin(float64(x)*0.2+t)*float64(amp))
			if y < 1 || y > rows-2 {
				continue
			}
			fmt.Printf("%s%d;%dH%s●%s", esc, y, x+1, fg(120, 255, 160), reset)
		}
		// draw tracer head
		hx := int(t*8) % (cols - 1)
		hy := mid + int(math.Sin(float64(hx)*0.2+t)*float64(amp))
		if hy >= 1 && hy <= rows-2 {
			fmt.Printf("%s%d;%dH%s★%s", esc, hy, hx+1, fg(255, 255, 120), reset)
		}
		fmt.Print(curPos(rows, 1))
		fmt.Printf("%s sine wave  t=%.1f  %s%s(q to exit)%s%s", fg(255, 180, 90), t, dimOn, reset, reset, clearEOL)
		t += 0.15
		tick(30 * time.Millisecond)
	}
	fmt.Print(clearScr + home)
}

func sceneBounce(dur time.Duration) {
	cols, rows := size()
	// ball
	bx, by := 5.0, 5.0
	vx, vy := 1.7, 1.3
	start := time.Now()
	t := 0
	for elapsedSince(start) < dur {
		// physics
		bx += vx
		by += vy
		if bx >= float64(cols-2) {
			bx = float64(cols - 2)
			vx = -vx
		}
		if bx <= 1 {
			bx = 1
			vx = -vx
		}
		if by >= float64(rows-2) {
			by = float64(rows - 2)
			vy = -vy
		}
		if by <= 1 {
			by = 1
			vy = -vy
		}
		fmt.Print(clearScr + home)
		// trail
		for i := 1; i <= 8; i++ {
			tx := bx - vx*float64(i)*0.8
			ty := by - vy*float64(i)*0.8
			if tx >= 1 && tx < float64(cols-1) && ty >= 1 && ty < float64(rows-1) {
				fmt.Printf("%s%d;%dH%s·%s", esc, int(ty), int(tx), fg(60, 120, 200), reset)
			}
		}
		fmt.Printf("%s%d;%dH%s●%s", esc, int(by), int(bx), fg(255, 120, 120), reset)
		fmt.Print(curPos(rows, 1))
		fmt.Printf("%s bounce  v=(%.1f,%.1f)  %s%s%dbounces%s%s",
			fg(255, 180, 90), vx, vy, dimOn, reset, t, reset, clearEOL)
		if t%20 == 0 {
			// randomize velocity slightly
			vx += (float64(t%3) - 1) * 0.1
			vy += (float64(t%5) - 2) * 0.05
			if math.Abs(vx) < 0.5 {
				vx = 1.5
			}
			if math.Abs(vy) < 0.3 {
				vy = 1.2
			}
		}
		t++
		tick(30 * time.Millisecond)
	}
	fmt.Print(clearScr + home)
}

var matrixGlyphs = []rune("アイウエオカキクケコサシスセソ0123456789ABCDEF")

func sceneMatrix(dur time.Duration) {
	cols, rows := size()
	// columns of falling glyphs with per-column speed
	speeds := make([]float64, cols)
	positions := make([]float64, cols)
	for i := range speeds {
		speeds[i] = 0.3 + float64(i%7)*0.2
		positions[i] = float64(i % rows)
	}
	start := time.Now()
	for elapsedSince(start) < dur {
		fmt.Print(clearScr + home)
		for c := 0; c < cols; c++ {
			positions[c] += speeds[c]
			head := int(positions[c]) % rows
			// trail of fading glyphs
			for j := 0; j < 4; j++ {
				r := (head - j + rows*4) % rows
				glyph := string(matrixGlyphs[(c*7+j*3+int(time.Now().UnixNano()/1000000))%len(matrixGlyphs)])
				col := fg(20, 255, 120)
				if j == 0 {
					col = fg(200, 255, 255)
				} else if j == 1 {
					col = fg(120, 255, 160)
				}
				fmt.Printf("%s%d;%dH%s%s%s", esc, r, c+1, col, glyph, reset)
			}
		}
		fmt.Print(curPos(rows, 1))
		fmt.Printf("%s MATRIX %s%s%s", fg(80, 255, 160), fg(120, 255, 180), strings.Repeat("▄", (cols-20)/2), reset)
		fmt.Print(clearEOL)
		tick(45 * time.Millisecond)
	}
	fmt.Print(clearScr + home)
}

func sceneSparkline(dur time.Duration) {
	// rolling buffer of values
	const maxN = 60
	var vals []int
	start := time.Now()
	phase := 0.0
	rows := 12
	for elapsedSince(start) < dur {
		// generate a value with a few sine waves + noise
		v := int(5 +
			math.Sin(phase)*3 +
			math.Sin(phase*0.5+1.3)*2 +
			math.Sin(phase*0.2)*1.5)
		if v < 0 {
			v = 0
		}
		if v > 10 {
			v = 10
		}
		vals = append(vals, v)
		if len(vals) > maxN {
			vals = vals[1:]
		}
		phase += 0.2

		// render vertical bars
		fmt.Print("\r\x1b[" + fmt.Sprint(rows) + "A")
		for r := rows; r >= 1; r-- {
			fmt.Print("\r" + clearEOL)
			for _, val := range vals {
				if val >= r {
					h := float64(val) / 10
					c := fg(int(120+135*h), int(80+175*h), 255)
					fmt.Print(c + "█" + reset)
				} else {
					fmt.Print(dim("·"))
				}
			}
			fmt.Print(clearEOL)
		}
		// baseline
		fmt.Print("\r" + clearEOL)
		for range vals {
			fmt.Print(fg(60, 60, 60) + "─" + reset)
		}
		fmt.Print(clearEOL)
		fmt.Printf("%s sparkline  peak=%d  n=%d%s",
			fg(255, 180, 90), maxInt(vals), len(vals), reset)
		fmt.Print(clearEOL)
		tick(80 * time.Millisecond)
	}
	fmt.Print("\r\x1b[" + fmt.Sprint(rows+2) + "B" + clearEOL)
}

func maxInt(vals []int) int {
	m := 0
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}

func sceneConcurrent(dur time.Duration) {
	start := time.Now()
	i := 0
	// three independent "panels" updated each frame on the same screen
	cols, rows := size()
	panelW := cols / 3
	if panelW < 15 {
		panelW = 15
	}
	progress := 0.0
	for elapsedSince(start) < dur {
		progress = float64(elapsedSince(start).Seconds()) / float64(dur.Seconds())
		if progress > 1 {
			progress = 1
		}
		fmt.Print(clearScr + home)
		// Panel 1: spinner + percent
		fmt.Printf("%s%d;%dH%s SPINNER %s\n", esc, 1, 1, fg(90, 200, 255), reset)
		fmt.Printf("%s%d;%dH %s%s%s  %3.0f%%", esc, 2, 1,
			fg(255, 180, 120), brailleFrames[i%len(brailleFrames)], reset, progress*100)
		fmt.Printf("%s%d;%dH %s%s%s", esc, 3, 1, fg(120, 200, 255), strings.Repeat("█", int(progress*float64(panelW-2))), reset)
		// Panel 2: mini bars
		fmt.Printf("%s%d;%dH%s BARS %s", esc, 1, panelW+2, fg(120, 255, 180), reset)
		for r := 0; r < 4; r++ {
			val := int(math.Sin(float64(i)/8+float64(r))*2 + 3)
			if val < 0 {
				val = 0
			}
			fmt.Printf("%s%d;%dH %s%s%s", esc, 2+r, panelW+2, fg(255, 200, 100), strings.Repeat("▍", val), reset)
		}
		// Panel 3: digital clock
		fmt.Printf("%s%d;%dH%s CLOCK %s", esc, 1, panelW*2+3, fg(255, 255, 120), reset)
		el := elapsedSince(start)
		fmt.Printf("%s%d;%dH %s%02d:%02d:%02d.%03d%s", esc, 2, panelW*2+3,
			fg(180, 140, 255),
			int(el.Minutes())%60, int(el.Seconds())%60, (el.Milliseconds()%1000)/100, el.Milliseconds()%100, reset)
		// progress bar spanning full width
		fmt.Printf("%s%d;%dH%s%s%s%s", esc, rows-1, 1, fg(80, 160, 255),
			strings.Repeat("█", int(progress*float64(cols-2))), strings.Repeat("░", cols-2-int(progress*float64(cols-2))), reset)
		fmt.Printf("%s%d;%dH%s %s%s %ds%s", esc, rows, 1, dimOn,
			fmt.Sprintf("concurrent scene  t+%d", int(elapsedSince(start).Seconds())), reset,
			int(dur.Seconds()), reset)
		i++
		tick(40 * time.Millisecond)
	}
	fmt.Print(clearScr + home)
}
