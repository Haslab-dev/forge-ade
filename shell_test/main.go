package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// ============================================================================
// shell_test — a terminal animation stress-test CLI.
//
// Exercises every major capability real terminals support for live animation:
//   - \r in-place redraws (spinners, progress, clocks, typewriters)
//   - ANSI cursor movement (cursor up/down/col, arbitrary positioning)
//   - clear-line / clear-screen escape sequences
//   - truecolor (24-bit) foreground/background gradients
//   - 256-color palette blocks
//   - alt-screen buffer (full-screen TUIs like htop/less)
//   - synchronized output mode (DEC 2026)
//   - box-drawing + Unicode glyphs (including multi-byte UTF-8)
//   - hide/show cursor, reverse video, blink, bold, dim, italic
//
// If stdout is not a TTY the CLI refuses to run animations (it would just
// spam lines) — the same guard real TUIs use.
// ============================================================================

// ANSI helpers
const (
	esc      = "\x1b["
	hideCur  = esc + "?25l"
	showCur  = esc + "?25h"
	reset    = esc + "0m"
	clearScr = esc + "2J"
	home     = esc + "H"
	clearLn  = esc + "2K"
	clearEOL = esc + "K"
	altOn    = esc + "?1049h" + clearScr + home
	altOff   = esc + "?1049l"
	syncOn   = esc + "?2026h"
	syncOff  = esc + "?2026l"
	dimOn    = esc + "2m"
	dimOff   = esc + "22m"
)

func curUp(n int) string   { return fmt.Sprintf("%s%dA", esc, n) }
func curDown(n int) string { return fmt.Sprintf("%s%dB", esc, n) }
func curCol(n int) string  { return fmt.Sprintf("%s%dG", esc, n) }
func curPos(r, c int) string {
	return fmt.Sprintf("%s%d;%dH", esc, r, c)
}

func fg(r, g, b int) string  { return fmt.Sprintf("%s38;2;%d;%d;%dm", esc, r, g, b) }
func bg(r, g, b int) string  { return fmt.Sprintf("%s48;2;%d;%d;%dm", esc, r, g, b) }
func fg256(n int) string     { return fmt.Sprintf("%s38;5;%dm", esc, n) }
func bold(s string) string   { return esc + "1m" + s + esc + "22m" }
func dim(s string) string    { return esc + "2m" + s + esc + "22m" }
func italic(s string) string { return esc + "3m" + s + esc + "23m" }
func reverse(s string) string {
	return esc + "7m" + s + esc + "27m"
}
func blink(s string) string { return esc + "5m" + s + esc + "25m" }

// isTTY reports whether stdout is a character device (a real terminal).
func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// size returns the terminal size via TIOCGWINSZ, falling back to 80x24.
func size() (cols, rows int) {
	cols, rows = 80, 24
	type winsize struct {
		Row, Col, X, Y uint16
	}
	ws := &winsize{}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if errno == 0 && ws.Col > 0 && ws.Row > 0 {
		cols, rows = int(ws.Col), int(ws.Row)
	}
	return
}

// termios struct + raw-mode helpers. The menu must read keys one at a time
// (arrow keys, Enter, q) WITHOUT the terminal echoing them or buffering until
// Enter — that requires putting stdin into raw mode via tcsetattr(TCSANOW).

type termios struct {
	Iflag  uint64
	Oflag  uint64
	Cflag  uint64
	Lflag  uint64
	Cc     [20]byte
	Ispeed uint64
	Ospeed uint64
}

const (
	tcgets = 0x40487413 // TIOCGETA on darwin
	tcsets = 0x80487414 // TIOCSETA on darwin
	icanon = 0x00000100 // ICANON (canonical mode)
	echo   = 0x00000008 // ECHO
	isig   = 0x00000080 // ISIG
	icrnl  = 0x00000100 // ICRNL
	ixon   = 0x00000200 // IXON
	opost  = 0x00000001 // OPOST
	vmin   = 16         // VMIN index in Cc[]
	vtime  = 17         // VTIME index in Cc[]
)

func getTermios() (*termios, error) {
	t := &termios{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		os.Stdin.Fd(), uintptr(tcgets), uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return nil, errno
	}
	return t, nil
}

func setTermios(t *termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		os.Stdin.Fd(), uintptr(tcsets), uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

// rawMode turns off canonical mode, echo, ISIG, ICRNL, IXON, OPOST so each
// keystroke arrives immediately without echo or Enter-buffering. Returns a
// restore function.
func rawMode() (func(), error) {
	orig, err := getTermios()
	if err != nil {
		return nil, err
	}
	raw := *orig
	raw.Lflag &^= uint64(icanon | echo | isig)
	raw.Iflag &^= uint64(icrnl | ixon)
	raw.Oflag &^= uint64(opost)
	raw.Cc[vmin] = 1
	raw.Cc[vtime] = 0
	if err := setTermios(&raw); err != nil {
		return nil, err
	}
	return func() { _ = setTermios(orig) }, nil
}

// sceneFunc renders one animation scene for the given duration.
type sceneFunc func(dur time.Duration)

// scenes registry, in menu order.
var scenes []struct {
	name string
	desc string
	fn   sceneFunc
}

func registerScene(name, desc string, fn sceneFunc) {
	scenes = append(scenes, struct {
		name string
		desc string
		fn   sceneFunc
	}{name, desc, fn})
}

// ============================================================================
// Pause-aware animation helpers.
//
// A single goroutine reads stdin in raw mode and flips the global `paused`
// flag on Space. Scenes use tick() instead of time.Sleep and elapsedSince()
// instead of time.Since so that:
//   - time keeps running while playing
//   - pressing Space freezes the frame immediately (tick blocks until resume)
//   - the elapsed timer stops advancing while paused (elapsedSince subtracts
//     accumulated pause time)
//   - q quits, any other key advances to the next scene immediately
// ============================================================================

var (
	paused         bool
	pauseAccum     time.Duration
	pauseStartTime time.Time
	sceneAbort     = make(chan struct{}, 1)
)

// setPaused flips the global pause state and tracks accumulated pause time.
func setPaused(p bool) {
	if p == paused {
		return
	}
	paused = p
	if p {
		pauseStartTime = time.Now()
	} else {
		pauseAccum += time.Since(pauseStartTime)
	}
}

// elapsedSince returns the wall-clock duration since start, minus any time
// spent paused — scene timers freeze while paused. Never negative: if the
// accumulated pause time exceeds elapsed (e.g. pause carried across scene
// boundaries), report 0.
func elapsedSince(start time.Time) time.Duration {
	if paused {
		// freeze: report duration as of the pause moment
		d := pauseStartTime.Sub(start) - pauseAccum
		if d < 0 {
			return 0
		}
		return d
	}
	d := time.Since(start) - pauseAccum
	if d < 0 {
		return 0
	}
	return d
}

// tick sleeps for d, but while paused it blocks until Space resumes — the
// animation frame freezes in place. The deadline is measured in "unpaused
// time" only: pausing suspends the countdown entirely.
func tick(d time.Duration) {
	remaining := d
	for remaining > 0 {
		if paused {
			// Freeze: wait for resume, no time counted against remaining.
			select {
			case <-sceneAbort:
				return
			case <-time.After(20 * time.Millisecond):
			}
			continue
		}
		sleep := 10 * time.Millisecond
		if remaining < sleep {
			sleep = remaining
		}
		select {
		case <-sceneAbort:
			return
		case <-time.After(sleep):
		}
		remaining -= sleep
	}
}

// inputLoop runs in a goroutine reading stdin in raw mode: Space toggles
// pause, q quits, any other key aborts the current scene early.
func inputLoop(restore func()) {
	b := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			continue
		}
		switch b[0] {
		case ' ':
			setPaused(!paused)
		case 'q', 'Q':
			// Exit WITHOUT touching stdout — the scene goroutine may be
			// mid-write holding the stdout lock; writing here would deadlock.
			// os.Exit bypasses everything; the terminal driver will restore
			// the original termios when the PTY closes.
			os.Exit(0)
		default:
			// skip current scene
			select {
			case sceneAbort <- struct{}{}:
			default:
			}
		}
	}
}

func main() {
	dur := 3 * time.Second
	direct := ""
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--duration", "-d":
			if i+1 < len(args) {
				if secs, err := time.ParseDuration(args[i+1]); err == nil {
					dur = secs
				}
				i++
			}
		case "--scene", "-s":
			if i+1 < len(args) {
				direct = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println("shell_test — terminal animation stress-test CLI")
			fmt.Println()
			fmt.Println("Usage: shell_test [--scene NAME] [--duration 3s]")
			fmt.Println()
			fmt.Println("Scenes:")
			for _, s := range scenes {
				fmt.Printf("  %-14s %s\n", s.name, s.desc)
			}
			os.Exit(0)
		}
	}

	if !isTTY() {
		fmt.Fprintln(os.Stderr, "shell_test: stdout is not a TTY — animations need a real terminal.")
		fmt.Fprintln(os.Stderr, "Run it inside the ForgeADE shell (or any real terminal).")
		os.Exit(1)
	}

	// Clean restore on Ctrl+C / kill
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Print(showCur + reset + altOff + "\r\n")
		os.Exit(130)
	}()

	if direct != "" {
		for _, s := range scenes {
			if s.name == direct {
				runScene(s, dur)
				return
			}
		}
		fmt.Fprintf(os.Stderr, "shell_test: unknown scene %q\n", direct)
		os.Exit(1)
	}

	autoPlay(dur)
}

// runScene renders one scene with a title header, honoring pause.
func runScene(s struct {
	name string
	desc string
	fn   sceneFunc
}, dur time.Duration) {
	// Reset accumulated pause time so each scene starts its own timer fresh.
	// Otherwise pause time from earlier scenes leaks into elapsedSince() and
	// progress bars/strings.Repeat go negative.
	pauseAccum = 0

	cols, _ := size()
	title := fmt.Sprintf(" ╭─ %s ─ %s ", s.name, s.desc)
	// Truncate the plain title so it can never wrap on a narrow terminal,
	// then apply dim() — avoids splitting ANSI escapes mid-sequence.
	if w := displayWidth(title); w > cols-1 {
		title = truncateToWidth(title, cols-1)
	}
	fmt.Print(clearScr + home + hideCur + reset)
	fmt.Println(strings.Repeat("─", cols))
	fmt.Println(dim(title))
	fmt.Println(strings.Repeat("─", cols))
	fmt.Println()
	tick(300 * time.Millisecond)
	s.fn(dur)
	fmt.Print(showCur + reset + altOff + "\r\n\r\n")
}

// displayWidth returns the visible width of s.
func displayWidth(s string) int {
	return len([]rune(s))
}

// truncateToWidth cuts s to at most max runes (visible columns).
func truncateToWidth(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// autoPlay plays every scene in sequence. Space pauses/resumes, q quits,
// any other key skips to the next scene.
func autoPlay(dur time.Duration) {
	restore, err := rawMode()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shell_test: failed to enter raw mode:", err)
		os.Exit(1)
	}
	defer restore()

	go inputLoop(restore)

	for i := 0; i < len(scenes); i++ {
		s := scenes[i]
		// show a "next up" splash
		fmt.Print(clearScr + home + hideCur + reset)
		fmt.Printf("%s%d;%dH%s", esc, 1, 1, clearEOL)
		title := fmt.Sprintf(" ▶ %s (%d/%d)  %s", s.name, i+1, len(scenes), s.desc)
		fmt.Println(fg(90, 200, 255) + title + reset)
		fmt.Println(dim("  space: pause/resume · any other key: skip · q: quit"))
		fmt.Println()
		tick(700 * time.Millisecond)
		runScene(s, dur)
		// drain any pending abort so the next scene starts fresh
		select {
		case <-sceneAbort:
		default:
		}
	}
	fmt.Print(showCur + reset + altOff + clearScr + home)
}
