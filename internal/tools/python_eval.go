package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// pythonRunnerScript is executed with python3 -c, reading code from stdin.
// If the code ends with an ast.Expr, it evaluates it and prints its representation
// if not None (true REPL evaluation behavior like IPython / Jupyter / Codex).
const pythonRunnerScript = `
import sys, ast

code = sys.stdin.read()
if not code.strip():
    sys.exit(0)

try:
    tree = ast.parse(code)
except SyntaxError:
    exec(code, globals())
    sys.exit(0)

if tree.body and isinstance(tree.body[-1], ast.Expr):
    last_expr = tree.body.pop()
    if tree.body:
        code_mod = ast.Module(body=tree.body, type_ignores=[])
        exec(compile(code_mod, filename='<repl>', mode='exec'), globals())
    expr_code = compile(ast.Expression(body=last_expr.value), filename='<repl>', mode='eval')
    res = eval(expr_code, globals())
    if res is not None:
        print(repr(res))
else:
    exec(compile(tree, filename='<repl>', mode='exec'), globals())
`

func findMontyBinary() string {
	candidates := []string{
		"monty",
		"/usr/local/bin/monty",
		"/opt/homebrew/bin/monty",
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil && path != "" {
			return path
		}
	}
	return ""
}

func findBunBinary() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		"bun",
		filepath.Join(home, ".bun", "bin", "bun"),
		"/opt/homebrew/bin/bun",
		"/usr/local/bin/bun",
		"/usr/bin/bun",
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil && path != "" {
			return path
		}
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

func findPythonBinary() string {
	candidates := []string{
		"python3",
		"python",
		"/usr/local/bin/python3",
		"/opt/homebrew/bin/python3",
		"/usr/bin/python3",
		"/Library/Frameworks/Python.framework/Versions/3.12/bin/python3",
		"/Library/Frameworks/Python.framework/Versions/3.11/bin/python3",
		"/Library/Frameworks/Python.framework/Versions/3.10/bin/python3",
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil && path != "" {
			return path
		}
	}
	return ""
}

// evalTool runs code with Sandboxed Python (monty / python3) -> Bun -> Bash fallback.
func evalTool() ToolSpec {
	return ToolSpec{
		Name: "eval",
		Description: "Evaluate code or expressions dynamically. Uses Sandboxed Python runner (pydantic/monty or python3) by default; " +
			"falls back to Bun (if available for JS/TS) or Bash if neither is available. " +
			"Trailing expressions evaluate and display representation automatically in REPL mode.",
		Cost: "cheap",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code":     map[string]any{"type": "string", "description": "Code snippet or expression to evaluate"},
				"language": map[string]any{"type": "string", "description": "Optional language: 'python', 'javascript', 'typescript', 'bash', or 'auto' (default 'auto')"},
				"timeout":  map[string]any{"type": "integer", "description": "Execution timeout in seconds (default 20, max 60)"},
			},
			"required": []string{"code"},
		},
		Handler: handleCodeEval,
	}
}

// runCodeTool aliases evalTool for multi-language programmatic execution.
func runCodeTool() ToolSpec {
	spec := evalTool()
	spec.Name = "run_code"
	spec.Description = "Execute programmatic code (Python, Bun, or Bash) to analyze data, inspect structures, or evaluate expressions."
	return spec
}

func pythonEvalTool() ToolSpec {
	spec := evalTool()
	spec.Name = "python_eval"
	spec.Description = "Execute Python code in interactive REPL mode using Sandboxed Python runner (monty / python3, with bun / bash fallback). " +
		"Trailing expressions automatically evaluate and display their representation (like IPython/Jupyter)."
	return spec
}

func pythonToolAlias() ToolSpec {
	spec := evalTool()
	spec.Name = "python"
	spec.Description = "Alias for python_eval. Execute Python code in REPL mode."
	return spec
}

func handleCodeEval(ctx context.Context, args map[string]any) (any, error) {
	code := argString(args, "code")
	if code == "" {
		code = argString(args, "script")
	}
	if code == "" {
		code = argString(args, "command")
	}
	if code == "" {
		return nil, fmt.Errorf("code parameter is required")
	}

	lang := strings.ToLower(strings.TrimSpace(argString(args, "language")))
	if lang == "" {
		lang = strings.ToLower(strings.TrimSpace(argString(args, "lang")))
	}

	timeoutSec := 20
	if to, ok := args["timeout"].(float64); ok && to > 0 {
		timeoutSec = int(to)
	} else if toInt, ok := args["timeout"].(int); ok && toInt > 0 {
		timeoutSec = toInt
	}
	if timeoutSec > 60 {
		timeoutSec = 60
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cwd := defaultCwd(ctx)

	// Determine execution order:
	// 1. Sandboxed Python runner (pydantic/monty or python3)
	// 2. Bun runner (if bun is available or language is js/ts)
	// 3. Bash code fallback
	var cmd *exec.Cmd
	var runtimeUsed string

	montyBin := findMontyBinary()
	pyBin := findPythonBinary()
	bunBin := findBunBinary()

	// If language is explicitly JS/TS, prefer Bun
	if (lang == "javascript" || lang == "js" || lang == "typescript" || lang == "ts") && bunBin != "" {
		cmd = exec.CommandContext(execCtx, bunBin, "eval", code)
		runtimeUsed = "bun"
	} else if lang == "bash" || lang == "sh" || lang == "shell" {
		cmd = exec.CommandContext(execCtx, "bash", "-c", code)
		runtimeUsed = "bash"
	} else {
		// Default order: Monty (Sandboxed Python) -> Python3 -> Bun -> Bash
		if montyBin != "" {
			cmd = exec.CommandContext(execCtx, montyBin, "eval", code)
			runtimeUsed = "monty"
		} else if pyBin != "" {
			cmd = exec.CommandContext(execCtx, pyBin, "-c", pythonRunnerScript)
			cmd.Stdin = strings.NewReader(code)
			runtimeUsed = "python3"
		} else if bunBin != "" {
			cmd = exec.CommandContext(execCtx, bunBin, "eval", code)
			runtimeUsed = "bun"
		} else {
			cmd = exec.CommandContext(execCtx, "bash", "-c", code)
			runtimeUsed = "bash"
		}
	}

	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%s execution timed out after %ds", runtimeUsed, timeoutSec)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to run code via %s: %w", runtimeUsed, err)
		}
	}

	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	resMap := map[string]any{
		"runtime":   runtimeUsed,
		"stdout":    outStr,
		"stderr":    errStr,
		"exit_code": exitCode,
		"status":    "completed",
	}
	if outStr != "" {
		resMap["result"] = outStr
	} else if errStr != "" {
		resMap["result"] = errStr
	} else {
		resMap["result"] = "(evaluated with no output)"
	}

	return toolResult(resMap), nil
}
