package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTurnMachineTransitions(t *testing.T) {
	m := NewTurnMachine("s1", 1, "hello")
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.StartModelRequest("gpt-x"); err != nil {
		t.Fatal(err)
	}
	if err := m.ReceiveModelResponse("partial"); err != nil {
		t.Fatal(err)
	}
	// illegal: idle from streaming
	if _, err := m.State().transition(PhaseIdle); err == nil {
		t.Fatal("expected illegal transition error")
	}
	if err := m.AddStreamingContent(" more"); err != nil {
		t.Fatal(err)
	}
	calls := []ToolCall{{ID: "c1", Name: "Read"}}
	if err := m.ScheduleTools(calls); err != nil {
		t.Fatal(err)
	}
	if err := m.StartToolExecution(); err != nil {
		t.Fatal(err)
	}
	if m.State().Phase != PhaseExecutingTools {
		t.Fatalf("phase = %s, want executing_tools", m.State().Phase)
	}
	if err := m.CompleteTool("c1", ToolResult{CallID: "c1", Success: true, Content: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := m.AggregateResults(); err != nil {
		t.Fatal(err)
	}
	next, ok := m.NextPhase()
	if !ok || next != PhaseAwaitingModel {
		t.Fatalf("next = %s, want awaiting_model_response", next)
	}
	if err := m.Complete("done", ResultSuccess); err != nil {
		t.Fatal(err)
	}
	if m.State().ResultType != ResultSuccess {
		t.Fatal("bad result type")
	}
	if err := m.Reset(); err != nil {
		t.Fatal(err)
	}
}

func TestTurnMachinePermissionFlow(t *testing.T) {
	m := NewTurnMachine("s1", 1, "hi")
	_ = m.Start()
	_ = m.StartModelRequest("m")
	_ = m.ReceiveModelResponse("text")
	_ = m.ScheduleTools([]ToolCall{{ID: "c1", Name: "Bash"}})
	// start with a permission request goes to awaiting_permission
	if err := m.RequestPermission(PermissionRequest{ID: "p1", CallID: "c1", ToolName: "Bash"}); err != nil {
		t.Fatal(err)
	}
	if m.State().Phase != PhaseAwaitingPermission {
		t.Fatalf("phase = %s", m.State().Phase)
	}
	if err := m.ResolvePermission("p1", PermAllow, nil); err != nil {
		t.Fatal(err)
	}
	if m.State().ToolCalls[0].Status != CallScheduled {
		t.Fatalf("call status = %s", m.State().ToolCalls[0].Status)
	}
	if err := m.StartToolExecution(); err != nil {
		t.Fatal(err)
	}
	if err := m.AggregateResults(); err != nil {
		t.Fatal(err)
	}
	// deny path
	_ = m.ReceiveModelResponse("more")
	_ = m.ScheduleTools([]ToolCall{{ID: "c2", Name: "Bash"}})
	if err := m.RequestPermission(PermissionRequest{ID: "p2", CallID: "c2", ToolName: "Bash"}); err != nil {
		t.Fatal(err)
	}
	if err := m.ResolvePermission("p2", PermDeny, nil); err != nil {
		t.Fatal(err)
	}
	if m.State().ToolCalls[1].Status != CallPermissionDenied {
		t.Fatal("expected permission_denied")
	}
}

func TestSchedulerParallelGroups(t *testing.T) {
	reg := NewToolRegistry()
	ro := true
	_ = reg.Register(&ToolEntry{Metadata: ToolMetadata{Name: "Read", ReadOnly: true}})
	_ = reg.Register(&ToolEntry{Metadata: ToolMetadata{Name: "Bash", Destructive: true}})
	_ = reg.Register(&ToolEntry{Metadata: ToolMetadata{Name: "Grep", ReadOnly: true, ConcurrentSafe: &ro}})
	calls := []ToolCall{
		{ID: "1", Name: "Read"}, {ID: "2", Name: "Read"}, {ID: "3", Name: "Bash"}, {ID: "4", Name: "Grep"},
	}
	sched, err := ScheduleTools(reg, calls, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sched.ParallelGroups) < 2 {
		t.Fatalf("expected >=2 groups, got %d", len(sched.ParallelGroups))
	}
	// destructive Bash must be a singleton group
	bashGroupSize := 0
	for _, g := range sched.ParallelGroups {
		if len(g) == 1 && calls[g[0]].Name == "Bash" {
			bashGroupSize = 1
		}
	}
	if bashGroupSize != 1 {
		t.Fatal("Bash should run in a singleton group")
	}
}

func TestExecutorStopTurnTruncatesLaterGroups(t *testing.T) {
	reg := NewToolRegistry()
	_ = reg.Register(&ToolEntry{Metadata: ToolMetadata{Name: "A", ReadOnly: true}, Execute: func(ctx context.Context, i map[string]any) (ToolOutput, error) {
		return ToolOutput{Content: "a", StopTurn: true}, nil
	}})
	_ = reg.Register(&ToolEntry{Metadata: ToolMetadata{Name: "B", ReadOnly: true}, Execute: func(ctx context.Context, i map[string]any) (ToolOutput, error) {
		return ToolOutput{Content: "b"}, nil
	}})
	sched, _ := ScheduleTools(reg, []ToolCall{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}, 10)
	x := &Executor{Registry: reg}
	var results []ToolResult
	x.ExecuteSchedule(context.Background(), sched, &SessionContext{}, func(r ToolResult) { results = append(results, r) })
	if len(results) != 2 {
		t.Fatalf("want 2 results (cancelled sibling synthesized), got %d", len(results))
	}
}

func TestSkillsScanAndRender(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".forge", "skills", "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: demo\ndescription: A demo skill\nwhen_to_use: when testing\n---\nBody ${CLAUDE_SKILL_DIR}\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := DiscoverSkillRoots(dir, nil, "")
	skills, diags := ScanSkills(roots)
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	var found *SkillEntry
	for i := range skills {
		if skills[i].Name == "demo" {
			found = &skills[i]
		}
	}
	if found == nil {
		t.Fatal("demo skill not found")
	}
	out, err := RenderSkillContent(*found, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "<skill_content name=\"demo\">") || !contains(out, "Base directory for this skill:") {
		t.Fatalf("bad skill envelope: %s", out)
	}
	if !contains(out, skillDir) {
		t.Fatal("skill dir not expanded")
	}
}

func TestPluginsManifestAndDiscovery(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".forge-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge-plugin", "plugin.json"),
		[]byte(`{"name":"example","version":"1.2.3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(root, "skills", "x"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "skills", "x", "SKILL.md"), []byte("---\nname: x\ndescription: X\n---\nhi\n"), 0o644)

	comp, err := DiscoverPlugin(root, "test-marketplace", true, true, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if comp.ID != "example@test-marketplace" || comp.Priority != 1000 {
		t.Fatalf("comp = %+v", comp)
	}
	if len(comp.Skills) != 1 {
		t.Fatalf("want 1 skill root, got %d", len(comp.Skills))
	}

	// state files round-trip
	home := t.TempDir()
	pm := NewPluginManager(home)
	if err := pm.AddMarketplace("test-marketplace", MarketplaceSource{Source: "directory", Path: root}, "d"); err != nil {
		t.Fatal(err)
	}
	if err := pm.InstallPlugin("example@test-marketplace", "user", func(dst string) error {
		return copyDir(root, dst)
	}); err != nil {
		t.Fatal(err)
	}
	ip, err := pm.LoadInstalledPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if len(ip.Plugins) != 1 || ip.Plugins[0].Version != "1.2.3" {
		t.Fatalf("installed = %+v", ip.Plugins)
	}
	comps, diags := pm.DiscoverInstalled(map[string]bool{"example@test-marketplace": true})
	if len(diags) != 0 || len(comps) != 1 {
		t.Fatalf("comps=%d diags=%v", len(comps), diags)
	}
	if err := pm.UninstallPlugin("example@test-marketplace", true); err != nil {
		t.Fatal(err)
	}
	ip, _ = pm.LoadInstalledPlugins()
	if len(ip.Plugins) != 0 {
		t.Fatal("expected empty installed list")
	}
}

func TestPermissionPipelineOrdering(t *testing.T) {
	svc := &PermissionService{}
	reg := NewToolRegistry()
	_ = reg.Register(&ToolEntry{Metadata: ToolMetadata{Name: "Read", ReadOnly: true}})
	_ = reg.Register(&ToolEntry{Metadata: ToolMetadata{Name: "Bash", Destructive: true}})

	read, _ := reg.Lookup("Read")
	bash, _ := reg.Lookup("Bash")

	// yolo allows everything
	s := &SessionContext{Mode: ModeYolo, Disallowed: []string{"Bash"}}
	d, _ := svc.Check(context.Background(), s, bash, "Bash", nil)
	if d.Action != PermActionAllow {
		t.Fatal("yolo should precede disallowedTools")
	}
	// plan mode denies non-read-only
	s = &SessionContext{Mode: ModePlan, PlanEnabled: true}
	d, _ = svc.Check(context.Background(), s, bash, "Bash", nil)
	if d.Action != PermActionDeny {
		t.Fatal("plan mode should deny destructive tools")
	}
	d, _ = svc.Check(context.Background(), s, read, "Read", nil)
	if d.Action != PermActionAllow {
		t.Fatal("plan mode should allow read-only")
	}
	// build mode asks for destructive
	s = &SessionContext{Mode: ModeBuild}
	d, _ = svc.Check(context.Background(), s, bash, "Bash", map[string]any{"command": "rm -rf /"})
	if d.Action != PermActionAsk {
		t.Fatal("build mode should ask for destructive tools")
	}
	// rule matching: prefix
	s = &SessionContext{Mode: ModeBuild, Rules: Ruleset{Allow: []PermissionRule{{ToolName: "Bash", RuleContent: "npm test:*"}}}}
	d, _ = svc.Check(context.Background(), s, bash, "Bash", map[string]any{"command": "npm test -- --watch"})
	if d.Action != PermActionAllow {
		t.Fatal("prefix rule should allow")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
