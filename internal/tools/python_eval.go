package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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

func pythonEvalTool() ToolSpec {
	return ToolSpec{
		Name: "python_eval",
		Description: "Execute Python code in interactive REPL mode. Trailing expressions automatically evaluate and display their representation (like IPython/Jupyter). " +
			"Use for mathematical computations, data transformations, string operations, regex testing, AST processing, JSON/API parsing, and algorithm verification. " +
			"Runs in the workspace directory with standard libraries available.",
		Cost: "cheap",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code":    map[string]any{"type": "string", "description": "Python code or expression to evaluate"},
				"timeout": map[string]any{"type": "integer", "description": "Execution timeout in seconds (default 20, max 60)"},
			},
			"required": []string{"code"},
		},
		Handler: handlePythonEval,
	}
}

func pythonToolAlias() ToolSpec {
	spec := pythonEvalTool()
	spec.Name = "python"
	spec.Description = "Alias for python_eval. Execute Python code in REPL mode."
	return spec
}

func handlePythonEval(ctx context.Context, args map[string]any) (any, error) {
	code := argString(args, "code")
	if code == "" {
		code = argString(args, "script")
	}
	if code == "" {
		return nil, fmt.Errorf("code parameter is required")
	}

	pyBin := findPythonBinary()
	if pyBin == "" {
		return nil, fmt.Errorf("python is not installed or not found in system PATH")
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

	cmd := exec.CommandContext(execCtx, pyBin, "-c", pythonRunnerScript)
	cmd.Dir = defaultCwd(ctx)
	cmd.Stdin = strings.NewReader(code)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("python execution timed out after %ds", timeoutSec)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to run python: %w", err)
		}
	}

	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	resMap := map[string]any{
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
		resMap["result"] = "(evaluated to None)"
	}

	return toolResult(resMap), nil
}
