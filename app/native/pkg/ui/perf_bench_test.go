package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
)

func newBenchApp(b *testing.B) (*AppUI, *ui.Tester, func()) {
	b.Helper()
	tmpDir, err := os.MkdirTemp("", "forge-perf-*")
	if err != nil {
		b.Fatalf("mkdirtemp: %v", err)
	}
	sampleGo := filepath.Join(tmpDir, "main.go")
	_ = os.WriteFile(sampleGo, []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello forge-ade\")\n}\n"), 0o644)

	eng := engine.NewEngine(tmpDir)
	app := NewAppUI(eng, tmpDir)
	app.ShowLeftSidebar = true
	app.ShowRightDrawer = false

	// Populate a realistic agent session with messages.
	sess := &agent.AgentSession{
		ID:            "bench-sess",
		Title:         "Profile render performance",
		Status:        "idle",
		WorkspacePath: tmpDir,
		Tasks:         agent.NewTaskManager(),
		Messages: []*agent.ChatMessage{
			{ID: 1, Role: "user", Text: "Audit performance and memory footprint.", Done: true, Timestamp: time.Now()},
			{ID: 2, Role: "assistant", Text: "Running CPU and memory profiling across all modes:\n\n- Agent Mode\n- Editor Mode\n- Studio Mode\n- Settings Screen", Done: true, Timestamp: time.Now()},
		},
	}
	app.Agent.Lock()
	app.Agent.Sessions = []*agent.AgentSession{sess}
	app.Agent.ActiveSessionID = sess.ID
	app.Agent.Messages = sess.Messages
	app.Agent.Unlock()

	_, _ = app.Editor.OpenFile(sampleGo, "main.go")

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1440, 900, ui.TesterOptions{Dark: true})
	// Warmup frame
	tw.Frame()

	cleanup := func() {
		app.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return app, tw, cleanup
}

func BenchmarkRenderAgentMode(b *testing.B) {
	app, tw, cleanup := newBenchApp(b)
	defer cleanup()
	app.Mode = ModeAgent
	tw.Frame()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tw.Frame()
	}
}

func BenchmarkRenderEditorMode(b *testing.B) {
	app, tw, cleanup := newBenchApp(b)
	defer cleanup()
	app.Mode = ModeEditor
	tw.Frame()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tw.Frame()
	}
}

func BenchmarkRenderStudioMode(b *testing.B) {
	app, tw, cleanup := newBenchApp(b)
	defer cleanup()
	app.Mode = ModeStudio
	tw.Frame()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tw.Frame()
	}
}

func BenchmarkRenderSettingsScreen(b *testing.B) {
	app, tw, cleanup := newBenchApp(b)
	defer cleanup()
	app.openSettings(settings.SettingsTabModels)
	tw.Frame()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tw.Frame()
	}
}

func TestResourceFootprintSnapshot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-mem-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	goroutinesBefore := runtime.NumGoroutine()

	start := time.Now()
	eng := engine.NewEngine(tmpDir)
	app := NewAppUI(eng, tmpDir)
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1440, 900, ui.TesterOptions{Dark: true})
	tw.Frame()
	bootElapsed := time.Since(start)

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	goroutinesAfter := runtime.NumGoroutine()

	t.Logf("Boot+1stFrame=%v | HeapAlloc=%.2fMB (delta %.2fMB) | HeapInuse=%.2fMB | Sys=%.2fMB | Goroutines=%d (delta +%d)",
		bootElapsed.Round(time.Millisecond),
		float64(after.HeapAlloc)/(1024*1024),
		float64(int64(after.HeapAlloc)-int64(before.HeapAlloc))/(1024*1024),
		float64(after.HeapInuse)/(1024*1024),
		float64(after.Sys)/(1024*1024),
		goroutinesAfter,
		goroutinesAfter-goroutinesBefore,
	)

	app.Close()
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	t.Logf("After Close(): Goroutines=%d (leaked vs before: %+d)", runtime.NumGoroutine(), runtime.NumGoroutine()-goroutinesBefore)
}
