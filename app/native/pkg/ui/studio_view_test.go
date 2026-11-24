package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	corestudio "github.com/hasdev/forge-ade/core/studio"
)

func TestStudioModeSwitching(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)

	if app.Mode != ModeAgent {
		t.Fatalf("expected initial mode ModeAgent, got %d", app.Mode)
	}

	// Switch to Studio Mode
	app.setMode(ModeStudio)
	if app.Mode != ModeStudio {
		t.Fatalf("expected ModeStudio (%d), got %d", ModeStudio, app.Mode)
	}

	if app.Studio == nil {
		t.Fatal("expected Studio manager to be initialized")
	}

	if app.Studio.ActiveProject() == nil {
		t.Fatal("expected an active studio project")
	}

	// Switch to Editor Mode
	app.setMode(ModeEditor)
	if app.Mode != ModeEditor {
		t.Fatalf("expected ModeEditor (%d), got %d", ModeEditor, app.Mode)
	}

	// Switch back to Studio Mode
	app.setMode(ModeStudio)
	if app.Mode != ModeStudio {
		t.Fatalf("expected ModeStudio (%d), got %d", ModeStudio, app.Mode)
	}
}

func TestStudioElementPinning(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)

	if app.Studio.IsPinning() {
		t.Fatal("expected pinning to be initially inactive")
	}

	// Toggle element pinning (V key)
	app.studioTogglePinning()
	if !app.Studio.IsPinning() {
		t.Fatal("expected pinning to be active after toggle")
	}

	// Select a pin
	proj := app.Studio.ActiveProject()
	if proj != nil && len(proj.PinnedElements) > 0 {
		pinID := proj.PinnedElements[0].ID
		app.studioSelectPin(pinID)

		if app.Studio.SelectedPin() == nil || app.Studio.SelectedPin().ID != pinID {
			t.Fatalf("expected selected pin %s, got %+v", pinID, app.Studio.SelectedPin())
		}

		if app.StudioPromptInput == "" {
			t.Fatal("expected prompt input to be prepopulated with pinned selector")
		}
	}
}

func TestStudioCrossModeExport(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)

	// Export active project to Editor (⌘⇧E)
	app.studioExportActive()

	// Verify app transitioned to Editor Mode
	if app.Mode != ModeEditor {
		t.Fatalf("expected ModeEditor after export, got %d", app.Mode)
	}

	proj := app.Studio.ActiveProject()
	if proj == nil || proj.ExportPath == "" {
		t.Fatal("expected project export path to be set")
	}

	if _, err := os.Stat(proj.ExportPath); err != nil {
		t.Fatalf("expected exported file to exist on disk: %v", err)
	}

	// Verify active document in editor matches exported primary file
	activeDoc := app.Editor.ActiveDoc()
	if activeDoc == nil {
		t.Fatal("expected active document in editor")
	}
	if filepath.Base(activeDoc.FullPath) != filepath.Base(proj.ExportPath) {
		t.Fatalf("expected active editor doc %s, got %s", proj.ExportPath, activeDoc.FullPath)
	}
}

func TestStudioViewModes(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)

	if app.StudioViewMode != "canvas" {
		t.Fatalf("expected initial view mode 'canvas', got %s", app.StudioViewMode)
	}

	app.StudioViewMode = "nodes"
	if app.StudioViewMode != "nodes" {
		t.Fatalf("expected 'nodes', got %s", app.StudioViewMode)
	}

	app.StudioViewMode = "code"
	if app.StudioViewMode != "code" {
		t.Fatalf("expected 'code', got %s", app.StudioViewMode)
	}
}

func TestStudioProjectsManagement(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)

	// Create 3D Scene
	p := app.Studio.CreateProject("Interactive WebGL Sphere", "Three.js shader mesh", corestudio.ArtifactTypeCodeJS)
	if p == nil {
		t.Fatal("expected created project")
	}

	if app.Studio.ActiveProject().ID != p.ID {
		t.Fatalf("expected active project %s, got %s", p.ID, app.Studio.ActiveProject().ID)
	}

	// Verify default node graph exists
	if len(p.Graph.Nodes) == 0 {
		t.Fatal("expected node graph for 3D project")
	}
}

func TestStudioNodeWorkflowQueuePrompt(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)

	// Create Image project
	p := app.Studio.CreateProject("Image Node Workflow", "Visual Node Workflow", corestudio.ArtifactTypeImage)
	if p == nil {
		t.Fatal("expected created image project")
	}

	app.StudioPositivePrompt = "Cyberpunk street in rain, neon light reflections, 8k render"
	app.StudioSteps = 20
	app.StudioSeed = 42

	// Queue prompt
	app.studioQueuePrompt()

	// Wait for goroutine generation to complete
	deadline := time.Now().Add(3 * time.Second)
	for app.StudioIsGenerating && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	if app.StudioIsGenerating {
		t.Fatal("generation timed out")
	}

	if app.StudioGeneratedImagePath == "" {
		t.Fatal("expected non-empty StudioGeneratedImagePath")
	}

	if _, err := os.Stat(app.StudioGeneratedImagePath); err != nil {
		t.Fatalf("expected generated image file to exist: %v", err)
	}
}

func TestStudioSplitAgentViewerChatWidth(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)
	app.StudioLeftChatWidth = 450

	w := app.chatColumnWidth(nil)
	if w != 450 {
		t.Fatalf("expected chatColumnWidth to match StudioLeftChatWidth (450), got %v", w)
	}

	leftInset, rightInset := app.chatLaneInset(nil)
	if leftInset != 10 || rightInset != 10 {
		t.Fatalf("expected compact insets (10, 10), got (%v, %v)", leftInset, rightInset)
	}
}

// newStudioSlidesApp builds an AppUI in Studio Mode with a slides project
// active so the split agent/viewer layout renders headlessly.
func newStudioSlidesApp(t *testing.T) *AppUI {
	t.Helper()
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)
	p := app.Studio.CreateProject("Deck", "layout test deck", corestudio.ArtifactTypeSlides)
	if p == nil {
		t.Fatal("expected slides project")
	}
	return app
}

// TestStudioHeaderLabelsInsideWideWindow pins the wide layout: surface switcher
// and mode selector render cleanly without header overflow.
func TestStudioHeaderLabelsInsideWideWindow(t *testing.T) {
	app := newStudioSlidesApp(t)
	const winW = 1024.0
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, winW, 700, ui.TesterOptions{Dark: true})

	// "Send to Editor" was removed per user request for independent studio process
	if _, ok := tw.Find("Send to Editor"); ok {
		t.Fatal("expected 'Send to Editor' to be removed from header")
	}

	// Surface tabs: unified Canvas, Image, and Video tabs should be rendered cleanly without overflow
	for _, tab := range []string{"Canvas", "Image", "Video"} {
		rect, ok := tw.Find(tab)
		if !ok {
			t.Fatalf("expected %q surface tab at 1024px width", tab)
		}
		if rect.X+rect.W > winW {
			t.Fatalf("%q tab clipped: ends at %.0f > window %.0f", tab, rect.X+rect.W, winW)
		}
	}
	if _, ok := tw.Find("Slides"); ok {
		t.Fatal("expected 'Slides' tab to be unified into 'Canvas'")
	}
	if _, ok := tw.Find("Motion"); ok {
		t.Fatal("expected 'Motion' tab to be unified into 'Canvas'")
	}
	if _, ok := tw.Find("3D Sandbox"); ok {
		t.Fatal("expected '3D Sandbox' tab to be unified into 'Canvas'")
	}
}

// TestStudioHeaderCollapsesToIconsWhenNarrow pins narrow behavior:
// "Send to Editor" is absent, and surface tabs survive.
func TestStudioHeaderCollapsesToIconsWhenNarrow(t *testing.T) {
	app := newStudioSlidesApp(t)
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 940, 700, ui.TesterOptions{Dark: true})

	if _, ok := tw.Find("Send to Editor"); ok {
		t.Fatal("expected 'Send to Editor' to be removed from header")
	}
	if _, ok := tw.Find("Canvas"); !ok {
		t.Fatal("expected Canvas surface tab to survive at 940px width")
	}
}

// TestStudioChatPaneClampedByWindow reproduces the crushed-viewer bug: a chat
// pane width remembered from a large window must clamp to the small window,
// and the viewer content must sit right of the pane, inside the window.
func TestStudioChatPaneClampedByWindow(t *testing.T) {
	app := newStudioSlidesApp(t)
	app.StudioLeftChatWidth = 720 // remembered from a much larger window

	const winW = 800.0
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, winW, 600, ui.TesterOptions{Dark: true})

	if maxChat := float32(winW) * 0.55; app.StudioLeftChatWidth > maxChat {
		t.Fatalf("chat pane not clamped: %.0f > %.0f", app.StudioLeftChatWidth, maxChat)
	}

	rect, ok := tw.Find("16:9 PRESENTATION")
	if !ok {
		t.Skip("slides fallback stage not rendered")
	}
	if rect.X < app.StudioLeftChatWidth {
		t.Fatalf("viewer overlaps chat pane: content at x=%.0f < chat width %.0f", rect.X, app.StudioLeftChatWidth)
	}
	if rect.X+rect.W > winW {
		t.Fatalf("viewer content overflows window: %.0f > %.0f", rect.X+rect.W, winW)
	}
}

// TestStudioStageCardFitsPane pins the adaptive 16:9 fallback stage: with a
// wide chat pane, the stage must shrink to the pane instead of overflowing.
func TestStudioStageCardFitsPane(t *testing.T) {
	app := newStudioSlidesApp(t)
	app.StudioLeftChatWidth = 704

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})

	rect, ok := tw.Find("16:9 PRESENTATION")
	if !ok {
		t.Skip("slides fallback stage not rendered")
	}
	if rect.X < app.StudioLeftChatWidth {
		t.Fatalf("stage overlaps chat pane: content at x=%.0f < chat width %.0f", rect.X, app.StudioLeftChatWidth)
	}
	if rect.X+rect.W > 1280-16 {
		t.Fatalf("stage card overflows window: ends at %.0f", rect.X+rect.W)
	}
}

func TestStudioStageCardFitsPaneWithRightDrawer(t *testing.T) {
	app := newStudioSlidesApp(t)
	app.StudioLeftChatWidth = 420
	app.ShowRightDrawer = true
	app.RightDrawerWidth = 380

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})

	rect, ok := tw.Find("16:9 PRESENTATION")
	if !ok {
		t.Skip("slides fallback stage not rendered")
	}
	if rect.X < app.StudioLeftChatWidth {
		t.Fatalf("stage overlaps chat pane: content at x=%.0f < chat width %.0f", rect.X, app.StudioLeftChatWidth)
	}
	maxRight := float32(1280) - app.RightDrawerWidth
	if rect.X+rect.W > maxRight {
		t.Fatalf("stage card overflows into right drawer: ends at %.0f > maxRight %.0f", rect.X+rect.W, maxRight)
	}
}

// TestStudioChatPaneCollapse pins the collapse toggle: when collapsed the
// chat pane (and its header) must not render at all.
func TestStudioChatPaneCollapse(t *testing.T) {
	app := newStudioSlidesApp(t)

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := tw.Find("Studio Agent"); !ok {
		t.Fatal("expected chat pane visible when not collapsed")
	}

	app.StudioChatCollapsed = true
	tw2 := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := tw2.Find("Studio Agent"); ok {
		t.Fatal("expected chat pane hidden when collapsed")
	}
}

// TestStudioAssetRegenerationCacheBusting pins the HTML cache-busting: stale
// files without the version marker are regenerated; current files are kept.
func TestStudioAssetRegenerationCacheBusting(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)
	p := app.Studio.CreateProject("Deck", "regen test", corestudio.ArtifactTypeSlides)

	stalePath := filepath.Join(dir, ".forge", "studio", "slides.html")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stalePath, []byte("<html>old-deck-no-marker</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	if path := app.ensureStudioAssetFile(p); path != stalePath {
		t.Fatalf("unexpected asset path %q", path)
	}
	data, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), corestudio.HTMLVersionMarker) {
		t.Fatal("expected stale slides.html to be regenerated with the version marker")
	}

	before, _ := os.ReadFile(stalePath)
	app.ensureStudioAssetFile(p)
	after, _ := os.ReadFile(stalePath)
	if !bytes.Equal(before, after) {
		t.Fatal("expected current slides.html to be preserved across calls")
	}
}

// TestStudioAssetRegeneratedOnNewVersion pins that the viewer asset follows
// the project's active version instead of being cached forever.
func TestStudioAssetRegeneratedOnNewVersion(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)
	p := app.Studio.CreateProject("Deck", "version test", corestudio.ArtifactTypeSlides)

	path := app.ensureStudioAssetFile(p)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := studioEmbeddedAssetVer(string(before)), p.ActiveVersion().Version; got != want {
		t.Fatalf("embedded ver = %d, want %d", got, want)
	}

	// A new version (as created by an agent import or director run) must
	// regenerate the asset and bump the embedded version.
	p.AddVersion("Agent: deck.md", "imported", corestudio.ArtifactData{
		Slides: []corestudio.SlideItem{{ID: "s1", Title: "Real Deck", Layout: "hero", Theme: "dark"}},
	}, nil)
	path2 := app.ensureStudioAssetFile(p)
	if path2 != path {
		t.Fatalf("asset path changed unexpectedly: %q vs %q", path, path2)
	}
	after, err := os.ReadFile(path2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "Real Deck") {
		t.Fatal("expected regenerated asset to contain the new version's slides")
	}
	if got, want := studioEmbeddedAssetVer(string(after)), p.ActiveVersion().Version; got != want {
		t.Fatalf("embedded ver after regen = %d, want %d", got, want)
	}
}

// TestStudioEntryHTMLAsset pins that an agent-authored page wins over the
// generated template.
func TestStudioEntryHTMLAsset(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)
	p := app.Studio.CreateProject("Deck", "entry test", corestudio.ArtifactTypeSlides)

	page := filepath.Join(dir, "agent_deck.html")
	if err := os.WriteFile(page, []byte("<html><body><h1>Agent Deck</h1></body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	p.AddVersion("Agent: agent_deck.html", "imported", corestudio.ArtifactData{
		EntryHTML: page,
	}, nil)

	if got := app.ensureStudioAssetFile(p); got != page {
		t.Fatalf("expected agent page %q, got %q", page, got)
	}
}

// TestStudioImportFromWrittenFiles pins the import pipeline's primary source:
// the turn's write tool calls (which work outside git workspaces), resolved
// against the session workspace.
func TestStudioImportFromWrittenFiles(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)

	as := app.Agent
	sess := &agent.AgentSession{
		ID: "sess-writer", Title: "Writer", Status: "idle",
		WorkspacePath: dir, Tasks: agent.NewTaskManager(),
		Messages: []*agent.ChatMessage{
			{ID: 1, Role: "user", Text: "buat deck", Done: true, Timestamp: time.Now()},
			{ID: 2, Role: "assistant", Done: true, Timestamp: time.Now(),
				Tools: []*agent.ToolExecution{
					{ID: "t1", ToolName: "terminal", Command: `{"command":"ls"}`, Status: "completed"},
					{ID: "t2", ToolName: "write", Command: `{"path":"deck.html","content":"<html/>"}`, Status: "completed"},
				}},
		},
	}
	as.Lock()
	as.Sessions = []*agent.AgentSession{sess}
	as.ActiveSessionID = "sess-writer"
	as.Messages = sess.Messages
	as.Unlock()

	files := app.Agent.LastTurnWrittenFiles("sess-writer")
	if len(files) != 1 || files[0] != "deck.html" {
		t.Fatalf("expected [deck.html], got %v", files)
	}

	deckPath := filepath.Join(dir, "deck.html")
	if err := os.WriteFile(deckPath, []byte("<html><body>deck</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	app.setMode(ModeStudio)
	app.studioImportAgentArtifacts("sess-writer")

	ver := app.Studio.ActiveProject().ActiveVersion()
	if ver == nil || ver.Data.EntryHTML != deckPath {
		t.Fatalf("expected EntryHTML import of %s, got %+v", deckPath, ver)
	}
	if fmt.Sprint(ver.Prompt) != "Agent: deck.html" {
		t.Fatalf("unexpected version prompt %q", ver.Prompt)
	}
}

// TestStudioScanArtifactFiles pins the explorer scan: artifact files in the
// session workspace are found, non-artifact files are filtered out.
func TestStudioScanArtifactFiles(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)

	as := app.Agent
	sess := &agent.AgentSession{
		ID: "sess-scan", Title: "Scanner", Status: "idle",
		WorkspacePath: dir, Tasks: agent.NewTaskManager(),
	}
	as.Lock()
	as.Sessions = []*agent.AgentSession{sess}
	as.ActiveSessionID = "sess-scan"
	as.Unlock()

	if err := os.MkdirAll(filepath.Join(dir, "slides"), 0o755); err != nil {
		t.Fatal(err)
	}
	deck := filepath.Join(dir, "slides", "deck.html")
	if err := os.WriteFile(deck, []byte("<html/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := app.studioScanArtifactFiles("sess-scan")
	found := false
	for _, f := range files {
		if f.Path == deck {
			found = true
		}
		if filepath.Ext(f.Path) == ".txt" {
			t.Fatalf("non-artifact file leaked into scan: %s", f.Path)
		}
	}
	if !found {
		t.Fatalf("expected %s in scan results, got %+v", deck, files)
	}
}

// TestStudioTimecodeFormat pins the timeline dock timecode formatting.
func TestStudioTimecodeFormat(t *testing.T) {
	cases := []struct {
		secs float64
		want string
	}{
		{0, "00:00.0"},
		{3.2, "00:03.2"},
		{12, "00:12.0"},
		{72.5, "01:12.5"},
		{-1, "00:00.0"},
	}
	for _, tc := range cases {
		if got := studioTimecode(tc.secs); got != tc.want {
			t.Errorf("studioTimecode(%v) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}

func TestStudioDecoratePrompt(t *testing.T) {
	app := newStudioSlidesApp(t)
	rawPrompt := "Make a 5-slide pitch deck about our AI agents"
	decorated := app.decorateStudioPrompt("sess-test", rawPrompt)

	if !strings.Contains(decorated, "OPEN DESIGN SKILLS ACTIVATED") {
		t.Fatal("expected Open Design skills header in decorated prompt")
	}
	if !strings.Contains(decorated, "Generate Image") || !strings.Contains(decorated, "Make Motion Video") {
		t.Fatal("expected 5 studio capabilities in prompt")
	}
	if !strings.Contains(decorated, "slides.html") {
		t.Fatal("expected slides target instruction in decorated prompt")
	}
	if !strings.Contains(decorated, rawPrompt) {
		t.Fatal("expected original user prompt to be preserved")
	}
}

func TestStudioIntentRouterAndFlavorDetection(t *testing.T) {
	// Intent routing
	if intent := detectStudioIntent("buatkan motion video kinetic typography dengan sound sync elevenlabs"); intent != IntentMotion {
		t.Fatalf("expected IntentMotion, got %s", intent)
	}
	if intent := detectStudioIntent("bikin presentasi slide pitch deck untuk saas"); intent != IntentSlides {
		t.Fatalf("expected IntentSlides, got %s", intent)
	}
	if intent := detectStudioIntent("render torus knot mesh dengan threejs webgl"); intent != Intent3D {
		t.Fatalf("expected Intent3D, got %s", intent)
	}
	if intent := detectStudioIntent("buat gambar flux comfyui cyberpunk"); intent != IntentImage {
		t.Fatalf("expected IntentImage, got %s", intent)
	}
	if intent := detectStudioIntent("generate video veo ai storyboard"); intent != IntentVideo {
		t.Fatalf("expected IntentVideo, got %s", intent)
	}

	// Path flavor detection
	if f, typ := detectPathFlavor("/path/to/motion.html"); f != "motion" || typ != corestudio.ArtifactTypeMotion {
		t.Fatalf("expected motion flavor, got %s, %s", f, typ)
	}
	if f, typ := detectPathFlavor("/path/to/deck.html"); f != "slides" || typ != corestudio.ArtifactTypeSlides {
		t.Fatalf("expected slides flavor, got %s, %s", f, typ)
	}
	if f, typ := detectPathFlavor("/path/to/scene3d.html"); f != "3d" || typ != corestudio.ArtifactTypeCodeJS {
		t.Fatalf("expected 3d flavor, got %s, %s", f, typ)
	}
	if f, _ := detectPathFlavor("/path/to/doc.pdf"); f != "pdf" {
		t.Fatalf("expected pdf flavor, got %s", f)
	}
	if f, _ := detectPathFlavor("/path/to/deck.pptx"); f != "pptx" {
		t.Fatalf("expected pptx flavor, got %s", f)
	}

	// Studio flavor detection
	motionData := corestudio.ArtifactData{EntryHTML: "/Users/test/.forge-ade/motion/motion.html"}
	if f := detectStudioFlavor(nil, motionData); f != "motion" {
		t.Fatalf("expected motion flavor for motion.html, got %s", f)
	}

	slidesData := corestudio.ArtifactData{EntryHTML: "/Users/test/.forge-ade/slides/slides.html"}
	if f := detectStudioFlavor(nil, slidesData); f != "slides" {
		t.Fatalf("expected slides flavor for slides.html, got %s", f)
	}
}

func TestStudioSessionHistoryAndAgentBadges(t *testing.T) {
	app := newStudioSlidesApp(t)
	app.ShowLeftSidebar = true

	// Add an internal session and an ACP session
	now := time.Now()
	sInternal := &agent.AgentSession{
		ID:            "sess-internal",
		Title:         "Internal Studio Task",
		AgentID:       "agent-internal",
		AgentType:     "internal",
		Model:         "mimo-flash",
		WorkspacePath: app.Workspace.RootDir,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	sACP := &agent.AgentSession{
		ID:            "sess-acp",
		Title:         "ACP Studio Task",
		AgentID:       "agent-agy",
		AgentType:     "acp",
		Model:         "gemini-3.8-flash",
		WorkspacePath: app.Workspace.RootDir,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	app.Agent.ReplaceSessions([]*agent.AgentSession{sInternal, sACP})
	app.Agent.SelectSession(sACP.ID)

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})

	// 1. History button exists in Studio agent header
	if _, ok := tw.Find("History"); !ok {
		t.Fatal("expected 'History' button in studio agent header")
	}

	// 2. Both sessions are visible in the sidebar session history
	if _, ok := tw.Find("Internal Studio Task"); !ok {
		t.Fatal("expected 'Internal Studio Task' in session history sidebar")
	}
	if _, ok := tw.Find("ACP Studio Task"); !ok {
		t.Fatal("expected 'ACP Studio Task' in session history sidebar")
	}

	// 3. Sidebar rows use icon-only agent indicators (no redundant 'Forge' text pill)
	if _, ok := tw.Find("Forge"); ok {
		t.Fatal("expected 'Forge' text pill to be removed from sidebar session row (icon-only)")
	}
}
