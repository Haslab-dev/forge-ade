package forgemcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hasdev/forge-ade/internal/browseruse"
)

// RegisterBrowserTools registers all Browser-Use tools for web interaction.
func (s *Server) RegisterBrowserTools(bm *browseruse.Manager) {
	if bm == nil {
		return
	}

	s.RegisterTool(Tool{
		Name: "forge_browser_open",
		Description: "Open a URL in ForgeADE's browser and return the page outline with interactive element refs. " +
			"The page renders live inside ForgeADE's browser viewer.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Full URL to open (e.g. 'https://github.com' or 'localhost:3000')",
				},
			},
			"required": []string{"url"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		url := getString(args, "url")
		if url == "" {
			return nil, fmt.Errorf("url is required")
		}
		res, err := bm.Navigate(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("browser navigate failed: %w", err)
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		return TextResult(string(data)), nil
	})

	s.RegisterTool(Tool{
		Name: "forge_browser_snapshot",
		Description: "Extract an accessibility & DOM text outline of the current browser page with element references (e.g. @e1, @e2). " +
			"Use these refs to click or type into elements.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		snapshot, err := bm.Snapshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to take browser snapshot: %w", err)
		}
		return TextResult(snapshot), nil
	})

	s.RegisterTool(Tool{
		Name:        "forge_browser_screenshot",
		Description: "Capture a full visual screenshot of the current browser page. Returns visual image data for multimodal vision models.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		b64, err := bm.Screenshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to take browser screenshot: %w", err)
		}
		return ImageResult(b64, "image/png"), nil
	})

	s.RegisterTool(Tool{
		Name: "forge_browser_click",
		Description: "Click an interactive element in the browser by its reference label (e.g. '@e12' from a snapshot) " +
			"or by coordinate (x, y).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Element reference ID (e.g. '@e4') from snapshot",
				},
				"x": map[string]interface{}{
					"type":        "number",
					"description": "Optional X coordinate for direct click",
				},
				"y": map[string]interface{}{
					"type":        "number",
					"description": "Optional Y coordinate for direct click",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		ref := getString(args, "ref")
		if ref != "" {
			res, err := bm.Click(ctx, ref)
			if err != nil {
				return nil, err
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return TextResult(string(data)), nil
		}

		if xVal, okX := args["x"].(float64); okX {
			if yVal, okY := args["y"].(float64); okY {
				res, err := bm.ClickAt(ctx, xVal, yVal)
				if err != nil {
					return nil, err
				}
				data, _ := json.MarshalIndent(res, "", "  ")
				return TextResult(string(data)), nil
			}
		}

		return nil, fmt.Errorf("either 'ref' or ('x' and 'y') coordinates must be provided")
	})

	s.RegisterTool(Tool{
		Name: "forge_browser_type",
		Description: "Type text into a focused or referenced input field in the browser. " +
			"Optionally submit/press Enter after typing.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Target element reference (e.g. '@e3') from snapshot",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Text string to type into the input",
				},
				"submit": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to press Enter after typing (defaults to false)",
				},
			},
			"required": []string{"ref", "text"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		ref := getString(args, "ref")
		text := getString(args, "text")
		submit := false
		if sVal, ok := args["submit"].(bool); ok {
			submit = sVal
		}
		res, err := bm.Type(ctx, ref, text, submit)
		if err != nil {
			return nil, err
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		return TextResult(string(data)), nil
	})

	s.RegisterTool(Tool{
		Name:        "forge_browser_status",
		Description: "Get the current status, active URL, and title of ForgeADE's browser viewer.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		status := bm.Status()
		data, _ := json.MarshalIndent(status, "", "  ")
		return TextResult(string(data)), nil
	})
}
