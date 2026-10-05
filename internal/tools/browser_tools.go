package tools

import (
	"context"

	"github.com/hasdev/forge-ade/internal/browseruse"
)

// browser_* tools — the agent's hands on the in-app browser (forge-ade's port
// of the browser-use plugin capability). Workflow mirrors ZCode's
// control-browser skill: snapshot first, act on refs, re-snapshot after
// actions; the page renders live in the Browser viewer panel.
func RegisterBrowserTools(r *Registry, bm *browseruse.Manager) {
	if bm == nil {
		return
	}
	r.Register(browserNavigateTool(bm))
	r.Register(browserSnapshotTool(bm))
	r.Register(browserClickTool(bm))
	r.Register(browserTypeTool(bm))
	r.Register(browserKeyTool(bm))
	r.Register(browserScrollTool(bm))
	r.Register(browserScreenshotTool(bm))
	if bm.HasTabs() {
		r.Register(browserTabsTool(bm))
	}
	r.Register(browserCloseTool(bm))
}

func browserNavigateTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "browser_navigate",
		Cost: "cheap",
		Description: "Open a URL in the managed browser and return the page outline with element refs. " +
			"The browser viewer in the right sidebar shows the page live while you work.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "URL to open (https:// prefix optional)"},
			},
			"required": []string{"url"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			url := argString(args, "url")
			if url == "" {
				return nil, errRequired("url")
			}
			res, err := bm.Navigate(ctx, url)
			if err != nil {
				return nil, err
			}
			if bm.ViewerVisible() {
				res["note"] = "The page is open in the app's Browser viewer panel; the user can see and interact with it."
			} else {
				res["note"] = "The page loaded in the app's browser. The viewer panel was closed — it opens automatically; tell the user they can also open the Browser tab to watch."
			}
			return res, nil
		},
	}
}

func browserSnapshotTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "browser_snapshot",
		Cost: "cheap",
		Description: "Capture an accessibility-style outline of the current page: headings, text, and interactive elements " +
			"each tagged with a ref (e.g. ref=e12). Always take a fresh snapshot before clicking or typing — refs change as the page changes.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			snap, err := bm.Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			return map[string]any{"snapshot": snap}, nil
		},
	}
}

func browserClickTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "browser_click",
		Cost: "medium",
		Description: "Click an element from the latest browser_snapshot by its ref (e.g. ref=e12). " +
			"Returns the updated page outline. Never guess refs — snapshot first.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ref":     map[string]any{"type": "string", "description": "Element ref from the latest snapshot (e.g. e12)"},
				"element": map[string]any{"type": "string", "description": "Optional human-readable element description (logged for the user)"},
			},
			"required": []string{"ref"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			ref := argString(args, "ref")
			if ref == "" {
				return nil, errRequired("ref")
			}
			return bm.Click(ctx, ref)
		},
	}
}

func browserTypeTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "browser_type",
		Cost: "medium",
		Description: "Type text into an editable element from the latest browser_snapshot by its ref. " +
			"Set submit: true to press Enter after typing (search boxes, forms).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ref":    map[string]any{"type": "string", "description": "Element ref from the latest snapshot"},
				"text":   map[string]any{"type": "string", "description": "Text to insert"},
				"submit": map[string]any{"type": "boolean", "description": "Press Enter after typing (default false)"},
			},
			"required": []string{"ref", "text"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			ref := argString(args, "ref")
			text := argString(args, "text")
			if ref == "" {
				return nil, errRequired("ref")
			}
			return bm.Type(ctx, ref, text, argBool(args, "submit", false))
		},
	}
}

func browserKeyTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name:        "browser_key",
		Cost:        "cheap",
		Description: "Press a key in the browser: enter, tab, escape, backspace, delete, arrowup/down/left/right, home, end, pageup, pagedown.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "Key name, e.g. Enter or ArrowDown"},
			},
			"required": []string{"key"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			key := argString(args, "key")
			if key == "" {
				return nil, errRequired("key")
			}
			return bm.Key(ctx, key)
		},
	}
}

func browserScrollTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name:        "browser_scroll",
		Cost:        "cheap",
		Description: "Scroll the page. Returns the visible content after scrolling.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"direction": map[string]any{"type": "string", "description": "up or down (default down)"},
				"amount":    map[string]any{"type": "integer", "description": "Wheel notches, ~100px each (default 5)"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			direction := argString(args, "direction")
			if direction == "" {
				direction = "down"
			}
			return bm.Scroll(ctx, direction, argInt(args, "amount", 5))
		},
	}
}

func browserScreenshotTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name: "browser_screenshot",
		Cost: "cheap",
		Description: "Capture a PNG of the current page (saved to disk, path returned; also shown live in the Browser viewer). " +
			"Prefer browser_snapshot for understanding the page — screenshots are for visual verification only.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			path, err := bm.Screenshot(ctx)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"screenshot_path": path,
				"note":            "The image is shown in the Browser viewer panel; use the snapshot tool to read page content.",
			}, nil
		},
	}
}

func browserTabsTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name:        "browser_tabs",
		Cost:        "cheap",
		Description: "Manage browser tabs: list, new (with url), select (by index), close (current tab).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "description": "list | new | select | close"},
				"url":    map[string]any{"type": "string", "description": "URL for action=new"},
				"index":  map[string]any{"type": "integer", "description": "Tab index for action=select"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			action := argString(args, "action")
			if action == "" {
				action = "list"
			}
			return bm.Tabs(ctx, action, argInt(args, "index", 0), argString(args, "url"))
		},
	}
}

func browserCloseTool(bm *browseruse.Manager) ToolSpec {
	return ToolSpec{
		Name:        "browser_close",
		Cost:        "cheap",
		Description: "Shut down the managed browser (closes all tabs and frees resources). It relaunches automatically on next use.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			bm.Close()
			return map[string]any{"closed": true}, nil
		},
	}
}

func errRequired(field string) error {
	return &fieldRequired{field: field}
}

type fieldRequired struct{ field string }

func (e *fieldRequired) Error() string { return e.field + " is required" }
