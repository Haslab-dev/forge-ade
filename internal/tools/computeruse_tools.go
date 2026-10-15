package tools

// Computer-use agent tools backed by internal/computeruse (native macOS
// desktop control). Registration mirrors RegisterBrowserTools.

import (
	"context"
	"fmt"
	"strings"

	"github.com/hasdev/forge-ade/internal/computeruse"
)

// RegisterComputerUseTools registers the native desktop-control tools.
// Screenshots resolve their sandbox folder from the calling session's
// workspace (SessionBridge), falling back to the app default folder.
func RegisterComputerUseTools(r *Registry, mgr *computeruse.Manager) {
	r.Register(computerScreenshotTool(mgr))
	r.Register(computerClickTool(mgr))
	r.Register(computerTypeTool(mgr))
	r.Register(computerKeyTool(mgr))
	r.Register(computerScrollTool(mgr))
	r.Register(computerWindowListTool(mgr))
}

func computerScreenshotTool(mgr *computeruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "computer_screenshot",
		Cost: "medium",
		Description: "Capture the macOS screen (optionally a region: x,y,w,h). The image is " +
			"injected into your context so you can see the screen. ALWAYS take a screenshot " +
			"before and after acting, and before clicking — coordinates come from what you see.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"x": map[string]any{"type": "integer", "description": "Region left (omit for full screen)"},
				"y": map[string]any{"type": "integer", "description": "Region top (omit for full screen)"},
				"w": map[string]any{"type": "integer", "description": "Region width (omit for full screen)"},
				"h": map[string]any{"type": "integer", "description": "Region height (omit for full screen)"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			region, err := computeruse.ValidateRegion(
				argInt(args, "x", 0), argInt(args, "y", 0), argInt(args, "w", 0), argInt(args, "h", 0))
			if err != nil {
				return nil, err
			}
			// Scope the capture to the calling session's workspace sandbox.
			projectPath := ""
			if sb := SessionBridgeFrom(ctx); sb != nil {
				projectPath = sb.GetWorkspaceFolder()
			}
			path, w, h, err := mgr.Screenshot(projectPath, region)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"screenshot_path": path,
				"width":           w,
				"height":          h,
				"note":            "The screenshot image is attached to this result — read it visually before acting.",
			}, nil
		},
	}
}

func computerClickTool(mgr *computeruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "computer_click",
		Cost: "cheap",
		Description: "Click the mouse at screen coordinates (x, y). button: left (default), right, " +
			"middle; double: true for double-click. Take a computer_screenshot first to choose coordinates.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"x":      map[string]any{"type": "integer", "description": "X coordinate (required)"},
				"y":      map[string]any{"type": "integer", "description": "Y coordinate (required)"},
				"button": map[string]any{"type": "string", "enum": []string{"left", "right", "middle"}},
				"double": map[string]any{"type": "boolean", "description": "Double-click"},
			},
			"required": []string{"x", "y"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			spec, err := computeruse.ValidateClick(argInt(args, "x", 0), argInt(args, "y", 0),
				argString(args, "button"), argBool(args, "double", false))
			if err != nil {
				return nil, err
			}
			if err := mgr.Click(spec); err != nil {
				return nil, err
			}
			return map[string]any{
				"clicked": fmt.Sprintf("%d,%d (%s%s)", spec.X, spec.Y, spec.Button, doubleSuffix(spec.Double)),
				"note":    "Take a computer_screenshot to verify the effect.",
			}, nil
		},
	}
}

func doubleSuffix(d bool) string {
	if d {
		return ", double)"
	}
	return ")"
}

func computerTypeTool(mgr *computeruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "computer_type",
		Cost: "cheap",
		Description: "Type text into the currently focused control (as if on the keyboard). " +
			"Newlines are sent as Return. Click the target field first.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "Text to type (required)"},
			},
			"required": []string{"text"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			text := argString(args, "text")
			if strings.TrimSpace(text) == "" {
				return nil, errRequired("text")
			}
			if err := mgr.TypeText(text); err != nil {
				return nil, err
			}
			return map[string]any{"typed_chars": len([]rune(text))}, nil
		},
	}
}

func computerKeyTool(mgr *computeruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "computer_key",
		Cost: "cheap",
		Description: "Press a key or chord: \"Return\", \"Escape\", \"Tab\", \"cmd+t\", " +
			"\"ctrl+shift+esc\", \"up\", \"f5\", single characters, etc. Modifiers: cmd, ctrl, alt, shift.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "Key or + separated chord (required)"},
			},
			"required": []string{"key"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			chord := argString(args, "key")
			if strings.TrimSpace(chord) == "" {
				return nil, errRequired("key")
			}
			if err := mgr.Key(chord); err != nil {
				return nil, err
			}
			return map[string]any{"pressed": chord}, nil
		},
	}
}

func computerScrollTool(mgr *computeruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "computer_scroll",
		Cost: "cheap",
		Description: "Scroll at screen coordinates (x, y). direction: up|down|left|right " +
			"(default down); amount: lines 1-50 (default 3).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"x":         map[string]any{"type": "integer", "description": "X coordinate (required)"},
				"y":         map[string]any{"type": "integer", "description": "Y coordinate (required)"},
				"direction": map[string]any{"type": "string", "enum": []string{"up", "down", "left", "right"}},
				"amount":    map[string]any{"type": "integer", "description": "Lines to scroll (1-50)"},
			},
			"required": []string{"x", "y"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			spec, err := computeruse.ValidateScroll(argInt(args, "x", 0), argInt(args, "y", 0),
				argString(args, "direction"), argInt(args, "amount", 0))
			if err != nil {
				return nil, err
			}
			if err := mgr.Scroll(spec); err != nil {
				return nil, err
			}
			return map[string]any{"scrolled": fmt.Sprintf("%s %d at %d,%d", spec.Direction, spec.Amount, spec.X, spec.Y)}, nil
		},
	}
}

func computerWindowListTool(mgr *computeruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "computer_window_list",
		Cost: "cheap",
		Description: "List on-screen windows with title, app, position and size — useful to " +
			"orient before clicking. Requires Screen Recording permission for titles.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			windows, err := mgr.WindowList()
			if err != nil {
				return nil, err
			}
			return map[string]any{"windows": windows, "count": len(windows)}, nil
		},
	}
}
