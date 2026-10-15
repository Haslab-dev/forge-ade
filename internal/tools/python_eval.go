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
elif tree.body and isinstance(tree.body[-1], ast.Assign):
    exec(compile(tree, filename='<repl>', mode='exec'), globals())
    for target in tree.body[-1].targets:
        if isinstance(target, ast.Name) and target.id in globals():
            print(f"{target.id} = {repr(globals()[target.id])}")
else:
    exec(compile(tree, filename='<repl>', mode='exec'), globals())
    defs = [node.name for node in tree.body if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef))]
    if defs:
        print(f"(Defined: {', '.join(defs)}. To evaluate return value, add a trailing call like '{defs[0]}(...)')")
`

// bunRunnerScript is executed with bun -e, evaluating JavaScript/TypeScript expressions
// with automatic representation display for trailing expressions and async promise resolution.
const bunRunnerScript = `
let __val;
try {
  __val = eval(process.argv[1]);
} catch(e) {
  try {
    __val = eval('(async () => {\n' + process.argv[1] + '\n})()');
    if (__val instanceof Promise) {
      __val = await __val;
    }
  } catch {
    console.error(e);
    process.exit(1);
  }
}
if (__val !== undefined) {
  if (typeof __val === 'object' && __val !== null) {
    try {
      console.log(JSON.stringify(__val, null, 2));
    } catch {
      console.log(__val);
    }
  } else {
    console.log(__val);
  }
}
`

func detectCodeLanguage(code string) string {
	trimmed := strings.TrimSpace(code)
	jsIndicators := []string{
		"const ", "let ", "var ", "console.log", "function(", "function ",
		"=>", "===", "!==", "JSON.stringify", "JSON.parse", "process.argv",
		"process.env", "require(", "export default", "export const",
		"interface ", "type ", ": string", ": number", ": boolean",
		"Array.", "Object.keys", "Object.values", ".map(", ".filter(", ".reduce(", ".join(",
	}
	for _, ind := range jsIndicators {
		if strings.Contains(trimmed, ind) {
			return "javascript"
		}
	}
	if strings.HasPrefix(trimmed, "#!/bin/") || strings.HasPrefix(trimmed, "echo ") || strings.HasPrefix(trimmed, "curl ") {
		return "bash"
	}
	return "python"
}

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
			"supports Bun for JavaScript/TypeScript, falling back to Bash. " +
			"Always include a trailing expression (e.g. `fib(10)`) or `print(...)` / `console.log(...)` on the last line so the computed result is returned in a single call.",
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
	spec.Description = "Execute programmatic code (Python, Bun, or Bash) to analyze data, inspect structures, or evaluate expressions. Always include a trailing expression or print statement on the last line to return computed output."
	return spec
}

func pythonEvalTool() ToolSpec {
	spec := evalTool()
	spec.Name = "python_eval"
	spec.Description = "Execute Python code in interactive REPL mode using Sandboxed Python runner (monty / python3, with bun / bash fallback). " +
		"Always include a trailing expression (e.g. `fib(10)`) or `print(...)` on the last line to return computed values in a single call."
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
	if lang == "" || lang == "auto" {
		lang = detectCodeLanguage(code)
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

	montyBin := findMontyBinary()
	pyBin := findPythonBinary()
	bunBin := findBunBinary()

	// Execution strategy:
	// If language is JS/TS and Bun is available: use Bun with bun -e
	// Else if language is Bash: use bash -c
	// Else (Python or auto): Monty -> Python3 -> Bun -> Bash fallback
	var cmd *exec.Cmd
	var runtimeUsed string

	isJSLang := lang == "javascript" || lang == "js" || lang == "typescript" || lang == "ts"
	isBashLang := lang == "bash" || lang == "sh" || lang == "shell"

	if isJSLang && bunBin != "" {
		cmd = exec.CommandContext(execCtx, bunBin, "-e", bunRunnerScript, code)
		runtimeUsed = "bun"
	} else if isBashLang {
		cmd = exec.CommandContext(execCtx, "bash", "-c", code)
		runtimeUsed = "bash"
	} else {
		if montyBin != "" {
			cmd = exec.CommandContext(execCtx, montyBin, "eval", code)
			runtimeUsed = "monty"
		} else if pyBin != "" {
			cmd = exec.CommandContext(execCtx, pyBin, "-c", pythonRunnerScript)
			cmd.Stdin = strings.NewReader(code)
			runtimeUsed = "python3"
		} else if bunBin != "" {
			cmd = exec.CommandContext(execCtx, bunBin, "-e", bunRunnerScript, code)
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

		// Self-healing fallback: if Python failed with SyntaxError and Bun is available, try Bun!
		if runtimeUsed == "python3" && bunBin != "" && (strings.Contains(stderr.String(), "SyntaxError") || exitCode != 0) {
			var bunStdout, bunStderr bytes.Buffer
			fallbackCmd := exec.CommandContext(execCtx, bunBin, "-e", bunRunnerScript, code)
			fallbackCmd.Dir = cwd
			fallbackCmd.Stdout = &bunStdout
			fallbackCmd.Stderr = &bunStderr
			if fallbackErr := fallbackCmd.Run(); fallbackErr == nil {
				outStr := strings.TrimSpace(bunStdout.String())
				errStr := strings.TrimSpace(bunStderr.String())
				resMap := map[string]any{
					"runtime":   "bun (auto-fallback)",
					"stdout":    outStr,
					"stderr":    errStr,
					"exit_code": 0,
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
