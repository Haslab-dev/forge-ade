package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Executor runs plugin tools safely with timeouts and workspace context.
type Executor struct {
	inspector Inspector
}

func NewExecutor(inspector ...Inspector) *Executor {
	var insp Inspector
	if len(inspector) > 0 {
		insp = inspector[0]
	}
	return &Executor{inspector: insp}
}

// SetInspector sets or updates the inspector interface.
func (e *Executor) SetInspector(inspector Inspector) {
	e.inspector = inspector
}

// Execute runs a plugin tool with the provided arguments and context.
func (e *Executor) Execute(ctx context.Context, p *Plugin, tool *PluginToolDef, args map[string]interface{}, workspaceDir string) (*PluginToolResult, error) {
	timeout := 30 * time.Second
	if tool.TimeoutSeconds > 0 {
		timeout = time.Duration(tool.TimeoutSeconds) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	workDir := workspaceDir
	if workDir == "" {
		workDir = p.Dir
	}
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	argsJSON, _ := json.Marshal(args)

	switch tool.HandlerType {
	case HandlerCommand:
		return e.executeCommand(execCtx, tool.Command, p.Dir, workDir, args, string(argsJSON))
	case HandlerScript:
		return e.executeScript(execCtx, tool.Script, p.Dir, workDir, args, string(argsJSON))
	case HandlerInline:
		return e.executeInline(execCtx, tool, p, workDir, args)
	default:
		// Default to command if command is set
		if tool.Command != "" {
			return e.executeCommand(execCtx, tool.Command, p.Dir, workDir, args, string(argsJSON))
		}
		if tool.Script != "" {
			return e.executeScript(execCtx, tool.Script, p.Dir, workDir, args, string(argsJSON))
		}
		return e.executeInline(execCtx, tool, p, workDir, args)
	}
}

func (e *Executor) executeCommand(ctx context.Context, cmdTemplate string, pluginDir string, workDir string, args map[string]interface{}, argsJSON string) (*PluginToolResult, error) {
	// Template replacement: replace {{key}} with string value of args[key]
	cmdStr := cmdTemplate
	for k, v := range args {
		placeholder := fmt.Sprintf("{{%s}}", k)
		cmdStr = strings.ReplaceAll(cmdStr, placeholder, fmt.Sprint(v))
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("FORGE_PLUGIN_DIR=%s", pluginDir),
		fmt.Sprintf("FORGE_WORKSPACE_DIR=%s", workDir),
		fmt.Sprintf("FORGE_TOOL_ARGS=%s", argsJSON),
	)

	// Also inject args as individual env vars (e.g. FORGE_ARG_FOO)
	for k, v := range args {
		envKey := fmt.Sprintf("FORGE_ARG_%s", strings.ToUpper(strings.ReplaceAll(k, "-", "_")))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%v", envKey, v))
	}

	// Stdin carries args JSON in case command reads stdin
	cmd.Stdin = bytes.NewReader([]byte(argsJSON))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	res := &PluginToolResult{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: exitCode,
		Success:  err == nil,
	}
	if err != nil {
		res.Error = err.Error()
	}

	return res, nil
}

func (e *Executor) executeScript(ctx context.Context, scriptPath string, pluginDir string, workDir string, args map[string]interface{}, argsJSON string) (*PluginToolResult, error) {
	fullPath := scriptPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(pluginDir, scriptPath)
	}

	if _, err := os.Stat(fullPath); err != nil {
		return nil, fmt.Errorf("script not found: %s", fullPath)
	}

	ext := strings.ToLower(filepath.Ext(fullPath))
	var cmd *exec.Cmd

	switch ext {
	case ".py":
		pythonBin := "python3"
		if _, err := exec.LookPath("python3"); err != nil {
			pythonBin = "python"
		}
		cmd = exec.CommandContext(ctx, pythonBin, fullPath)
	case ".js", ".mjs", ".cjs":
		cmd = exec.CommandContext(ctx, "node", fullPath)
	case ".sh":
		cmd = exec.CommandContext(ctx, "bash", fullPath)
	default:
		// Check executable permission
		cmd = exec.CommandContext(ctx, fullPath)
	}

	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("FORGE_PLUGIN_DIR=%s", pluginDir),
		fmt.Sprintf("FORGE_WORKSPACE_DIR=%s", workDir),
		fmt.Sprintf("FORGE_TOOL_ARGS=%s", argsJSON),
	)
	for k, v := range args {
		envKey := fmt.Sprintf("FORGE_ARG_%s", strings.ToUpper(strings.ReplaceAll(k, "-", "_")))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%v", envKey, v))
	}
	cmd.Stdin = bytes.NewReader([]byte(argsJSON))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	res := &PluginToolResult{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: exitCode,
		Success:  err == nil,
	}
	if err != nil {
		res.Error = err.Error()
	}

	return res, nil
}

func (e *Executor) executeInline(ctx context.Context, tool *PluginToolDef, p *Plugin, workDir string, args map[string]interface{}) (*PluginToolResult, error) {
	switch tool.Name {
	case "run_code":
		return e.executeRunCode(ctx, workDir, args)
	case "eval_expression":
		return e.executeEvalExpression(ctx, workDir, args)
	case "cordis_inspect":
		return e.executeCordisInspect(ctx, args)
	case "validate_plugin":
		return e.executeValidatePlugin(ctx, workDir, args)
	case "verify_changes":
		return e.executeVerifyChanges(ctx, workDir, args)
	case "test_runner":
		return e.executeTestRunner(ctx, workDir, args)
	case "persistent_shell":
		cmdStr, _ := args["command"].(string)
		if strings.TrimSpace(cmdStr) == "" {
			return nil, fmt.Errorf("command parameter is required")
		}
		argsJSON, _ := json.Marshal(args)
		return e.executeCommand(ctx, cmdStr, p.Dir, workDir, args, string(argsJSON))
	default:
		// Fallback to command or script if configured
		if tool.Command != "" {
			argsJSON, _ := json.Marshal(args)
			return e.executeCommand(ctx, tool.Command, p.Dir, workDir, args, string(argsJSON))
		}
		if tool.Script != "" {
			argsJSON, _ := json.Marshal(args)
			return e.executeScript(ctx, tool.Script, p.Dir, workDir, args, string(argsJSON))
		}
		return &PluginToolResult{
			Stdout:   fmt.Sprintf("Inline plugin tool executed successfully: %s", tool.Name),
			ExitCode: 0,
			Success:  true,
		}, nil
	}
}

func (e *Executor) executeRunCode(ctx context.Context, workDir string, args map[string]interface{}) (*PluginToolResult, error) {
	code, _ := args["code"].(string)
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("code parameter is required")
	}
	lang, _ := args["language"].(string)
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		lang = "javascript"
	}

	tempDir := os.TempDir()
	var cmd *exec.Cmd
	var scriptFile string

	switch lang {
	case "javascript", "js", "node":
		f, err := os.CreateTemp(tempDir, "forge_run_code_*.mjs")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp script: %w", err)
		}
		scriptFile = f.Name()
		_, _ = f.WriteString(code)
		_ = f.Close()
		defer os.Remove(scriptFile)

		cmd = exec.CommandContext(ctx, "node", scriptFile)

	case "typescript", "ts":
		f, err := os.CreateTemp(tempDir, "forge_run_code_*.ts")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp script: %w", err)
		}
		scriptFile = f.Name()
		_, _ = f.WriteString(code)
		_ = f.Close()
		defer os.Remove(scriptFile)

		if _, err := exec.LookPath("npx"); err == nil {
			cmd = exec.CommandContext(ctx, "npx", "tsx", scriptFile)
		} else if _, err := exec.LookPath("bun"); err == nil {
			cmd = exec.CommandContext(ctx, "bun", scriptFile)
		} else {
			cmd = exec.CommandContext(ctx, "node", scriptFile)
		}

	case "python", "py", "python3":
		f, err := os.CreateTemp(tempDir, "forge_run_code_*.py")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp script: %w", err)
		}
		scriptFile = f.Name()
		_, _ = f.WriteString(code)
		_ = f.Close()
		defer os.Remove(scriptFile)

		pyBin := "python3"
		if _, err := exec.LookPath("python3"); err != nil {
			pyBin = "python"
		}
		cmd = exec.CommandContext(ctx, pyBin, scriptFile)

	case "bash", "sh", "shell":
		cmd = exec.CommandContext(ctx, "bash", "-c", code)

	default:
		return nil, fmt.Errorf("unsupported language %q; supported: javascript, typescript, python, bash", lang)
	}

	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("FORGE_WORKSPACE_DIR=%s", workDir),
		"NODE_PATH="+filepath.Join(workDir, "node_modules"),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	res := &PluginToolResult{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: exitCode,
		Success:  err == nil,
	}
	if err != nil {
		res.Error = err.Error()
	}

	return res, nil
}

func (e *Executor) executeEvalExpression(ctx context.Context, workDir string, args map[string]interface{}) (*PluginToolResult, error) {
	expr, _ := args["expression"].(string)
	if strings.TrimSpace(expr) == "" {
		return nil, fmt.Errorf("expression parameter is required")
	}
	lang, _ := args["language"].(string)
	if lang == "" {
		lang = "javascript"
	}
	lang = strings.ToLower(strings.TrimSpace(lang))

	var cmd *exec.Cmd
	switch lang {
	case "python", "py":
		pyBin := "python3"
		if _, err := exec.LookPath("python3"); err != nil {
			pyBin = "python"
		}
		script := fmt.Sprintf("import sys, json\nres = eval(%q)\nif isinstance(res, (dict, list)):\n    print(json.dumps(res, indent=2))\nelse:\n    print(res)", expr)
		cmd = exec.CommandContext(ctx, pyBin, "-c", script)
	default:
		script := fmt.Sprintf("try { const res = eval(%q); console.log(typeof res === 'object' ? JSON.stringify(res, null, 2) : res); } catch (e) { console.error(e.message); process.exit(1); }", expr)
		cmd = exec.CommandContext(ctx, "node", "-e", script)
	}

	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return &PluginToolResult{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: exitCode,
		Success:  err == nil,
	}, nil
}

func (e *Executor) executeCordisInspect(ctx context.Context, args map[string]interface{}) (*PluginToolResult, error) {
	if e.inspector == nil {
		return &PluginToolResult{
			Stdout:   `{"info": "Plugin inspector initialized with no active plugins"}`,
			ExitCode: 0,
			Success:  true,
		}, nil
	}

	target, _ := args["target"].(string)
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = "all"
	}

	summary := make(map[string]interface{})
	if target == "all" || target == "plugins" {
		plugins := e.inspector.List()
		var pList []map[string]interface{}
		for _, p := range plugins {
			pList = append(pList, map[string]interface{}{
				"id":           p.ID,
				"name":         p.Name,
				"description":  p.Description,
				"version":      p.Version,
				"enabled":      p.Enabled,
				"source":       p.Source,
				"tools_count":  len(p.Tools),
				"skills_count": len(p.Skills),
			})
		}
		summary["plugins"] = pList
	}

	if target == "all" || target == "tools" {
		activeTools := e.inspector.ActiveTools()
		var tList []map[string]interface{}
		for _, at := range activeTools {
			tList = append(tList, map[string]interface{}{
				"name":        at.Tool.Name,
				"description": at.Tool.Description,
				"plugin_id":   at.PluginID,
				"handler":     at.Tool.HandlerType,
			})
		}
		summary["active_tools"] = tList
	}

	if target == "all" || target == "skills" {
		activeSkills := e.inspector.ActiveSkills()
		var sList []map[string]interface{}
		for _, as := range activeSkills {
			sList = append(sList, map[string]interface{}{
				"name":        as.Skill.Name,
				"description": as.Skill.Description,
				"plugin_id":   as.PluginID,
			})
		}
		summary["active_skills"] = sList
	}

	if target == "all" || target == "system_prompt" {
		summary["active_system_prompts"] = e.inspector.ActiveSystemPrompts()
	}

	b, _ := json.MarshalIndent(summary, "", "  ")
	return &PluginToolResult{
		Stdout:   string(b),
		ExitCode: 0,
		Success:  true,
	}, nil
}

func (e *Executor) executeValidatePlugin(ctx context.Context, workDir string, args map[string]interface{}) (*PluginToolResult, error) {
	path, _ := args["path"].(string)
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	targetPath := path
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(workDir, targetPath)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return &PluginToolResult{
			Stdout:   fmt.Sprintf(`{"valid": false, "error": "Path does not exist: %s"}`, targetPath),
			ExitCode: 1,
			Success:  false,
		}, nil
	}

	checks := make([]string, 0)
	var pluginID, pluginName string
	valid := true

	dir := targetPath
	if !info.IsDir() {
		dir = filepath.Dir(targetPath)
	}

	manifestJSON := filepath.Join(dir, "plugin.json")
	presetYAML := filepath.Join(dir, "preset.yml")
	cordisYAML := filepath.Join(dir, "cordis.yml")
	agentCordisYAML := filepath.Join(dir, "agent.cordis.yml")

	hasManifest := false
	if data, err := os.ReadFile(manifestJSON); err == nil {
		hasManifest = true
		checks = append(checks, "Found plugin.json")
		var p Plugin
		if err := json.Unmarshal(data, &p); err != nil {
			valid = false
			checks = append(checks, fmt.Sprintf("Error parsing plugin.json: %v", err))
		} else {
			pluginID = p.ID
			pluginName = p.Name
			checks = append(checks, fmt.Sprintf("Valid plugin.json (ID: %s, Tools: %d, Skills: %d)", p.ID, len(p.Tools), len(p.Skills)))
		}
	}

	if data, err := os.ReadFile(presetYAML); err == nil {
		hasManifest = true
		checks = append(checks, "Found preset.yml (Cordis Preset)")
		var py struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}
		if err := yaml.Unmarshal(data, &py); err != nil {
			checks = append(checks, fmt.Sprintf("Warning: Failed parsing preset.yml: %v", err))
		} else {
			if pluginName == "" {
				pluginName = py.Name
			}
			checks = append(checks, fmt.Sprintf("Valid preset.yml (Name: %s)", py.Name))
		}
	}

	if _, err := os.ReadFile(agentCordisYAML); err == nil {
		hasManifest = true
		checks = append(checks, "Found agent.cordis.yml (Cordis Agent Composition)")
	}

	if _, err := os.ReadFile(cordisYAML); err == nil {
		hasManifest = true
		checks = append(checks, "Found cordis.yml (Cordis Module Entry)")
	}

	if !hasManifest {
		valid = false
		checks = append(checks, "No recognized manifest found (expected plugin.json, preset.yml, or cordis.yml)")
	}

	// Check skills directory
	skillsDir := filepath.Join(dir, "skills")
	if sInfo, err := os.Stat(skillsDir); err == nil && sInfo.IsDir() {
		subEntries, _ := os.ReadDir(skillsDir)
		skillCount := 0
		for _, se := range subEntries {
			if se.IsDir() {
				if _, err := os.Stat(filepath.Join(skillsDir, se.Name(), "SKILL.md")); err == nil {
					skillCount++
				}
			}
		}
		checks = append(checks, fmt.Sprintf("Found skills/ directory with %d valid SKILL.md entries", skillCount))
	}

	resultMap := map[string]interface{}{
		"valid":  valid,
		"path":   targetPath,
		"id":     pluginID,
		"name":   pluginName,
		"checks": checks,
	}
	b, _ := json.MarshalIndent(resultMap, "", "  ")
	return &PluginToolResult{
		Stdout:   string(b),
		ExitCode: 0,
		Success:  valid,
	}, nil
}

func (e *Executor) executeVerifyChanges(ctx context.Context, workDir string, args map[string]interface{}) (*PluginToolResult, error) {
	var sb strings.Builder
	sb.WriteString("=== Git Status Snapshot ===\n")
	gitCmd := exec.CommandContext(ctx, "git", "status", "--short")
	gitCmd.Dir = workDir
	gitOut, err := gitCmd.CombinedOutput()
	if err == nil {
		sb.WriteString(string(gitOut))
	}

	sb.WriteString("\n=== Git Diff Stat ===\n")
	diffCmd := exec.CommandContext(ctx, "git", "diff", "--stat")
	diffCmd.Dir = workDir
	diffOut, err := diffCmd.CombinedOutput()
	if err == nil {
		sb.WriteString(string(diffOut))
	}

	runTests := true
	if v, ok := args["run_tests"].(bool); ok {
		runTests = v
	}

	if runTests {
		sb.WriteString("\n=== Test Verification ===\n")
		testRes, _ := e.executeTestRunner(ctx, workDir, nil)
		if testRes != nil {
			sb.WriteString(testRes.Stdout)
			if testRes.Stderr != "" {
				sb.WriteString("\n" + testRes.Stderr)
			}
		}
	}

	return &PluginToolResult{
		Stdout:   sb.String(),
		ExitCode: 0,
		Success:  true,
	}, nil
}

func (e *Executor) executeTestRunner(ctx context.Context, workDir string, args map[string]interface{}) (*PluginToolResult, error) {
	var cmd *exec.Cmd

	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		cmdArgs := []string{"test", "-v"}
		if filter, ok := args["filter"].(string); ok && filter != "" {
			cmdArgs = append(cmdArgs, "-run", filter)
		}
		path := "./..."
		if p, ok := args["path"].(string); ok && p != "" {
			path = p
		}
		cmdArgs = append(cmdArgs, path)
		cmd = exec.CommandContext(ctx, "go", cmdArgs...)
	} else if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		cmd = exec.CommandContext(ctx, "npm", "test")
	} else if _, err := os.Stat(filepath.Join(workDir, "Cargo.toml")); err == nil {
		cmd = exec.CommandContext(ctx, "cargo", "test")
	} else if _, err := os.Stat(filepath.Join(workDir, "pytest.ini")); err == nil || hasPythonFiles(workDir) {
		cmd = exec.CommandContext(ctx, "pytest")
	} else {
		return &PluginToolResult{
			Stdout:   "No standard test suite runner detected in workspace (go.mod, package.json, Cargo.toml, pytest)",
			ExitCode: 0,
			Success:  true,
		}, nil
	}

	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &PluginToolResult{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: exitCode,
		Success:  err == nil,
	}, nil
}

func hasPythonFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".py") {
			return true
		}
	}
	return false
}
