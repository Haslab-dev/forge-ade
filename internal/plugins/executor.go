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
)

// Executor runs plugin tools safely with timeouts and workspace context.
type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
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
		return &PluginToolResult{
			Stdout:   fmt.Sprintf("Inline plugin tool executed: %s", tool.Name),
			ExitCode: 0,
			Success:  true,
		}, nil
	default:
		// Default to command if command is set
		if tool.Command != "" {
			return e.executeCommand(execCtx, tool.Command, p.Dir, workDir, args, string(argsJSON))
		}
		if tool.Script != "" {
			return e.executeScript(execCtx, tool.Script, p.Dir, workDir, args, string(argsJSON))
		}
		return nil, fmt.Errorf("unsupported handler type %q for tool %s", tool.HandlerType, tool.Name)
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
