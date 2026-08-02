package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeMockServer writes a tiny JSON-RPC stdio server script (node or sh) that
// answers initialize / tools/list / tools/call like a real MCP server.
func writeMockServer(t *testing.T, dir string) string {
	t.Helper()
	script := filepath.Join(dir, "mock-server.sh")
	// A sh-based newline-JSON MCP server. Reads one request per line, replies
	// with the appropriate response. `sh` line reading is awkward, so use node
	// when available; fall back to a bash read loop otherwise.
	content := `#!/bin/sh
while IFS= read -r line; do
  method=$(printf '%s' "$line" | sed -n 's/.*"method":"\([^"]*\)".*/\1/p')
  id=$(printf '%s' "$line" | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
  case "$method" in
    initialize)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"protocolVersion\":\"2025-03-26\",\"capabilities\":{\"tools\":{}},\"serverInfo\":{\"name\":\"mock\",\"version\":\"1.0.0\"}}}"
      ;;
    tools/list)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"tools\":[{\"name\":\"echo_tool\",\"description\":\"Echo args back\",\"inputSchema\":{\"type\":\"object\",\"properties\":{\"text\":{\"type\":\"string\"}}}}]}}"
      ;;
    tools/call)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"mock result\"}],\"isError\":false}}"
      ;;
    *)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{}}"
      ;;
  esac
done
`
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return script
}

func TestConnectListAndCallTool(t *testing.T) {
	dir := t.TempDir()
	script := writeMockServer(t, dir)

	mgr := NewManager(dir)
	mgr.SaveServer(ServerConfig{
		Name:    "mock",
		Command: script,
		Args:    []string{},
		Enabled: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := mgr.ConnectAll(ctx); err != nil {
		t.Fatalf("ConnectAll: %v", err)
	}
	defer mgr.DisconnectAll()

	tools := mgr.ListConnectedTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 connected tool, got %d", len(tools))
	}
	if tools[0].Name != "mock/echo_tool" {
		t.Errorf("tool name = %q, want mock/echo_tool", tools[0].Name)
	}

	result, err := mgr.CallTool(ctx, "mock/echo_tool", map[string]any{"text": "hi"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result != "mock result" {
		t.Errorf("result = %q, want 'mock result'", result)
	}

	// Calling an unknown server must error cleanly.
	if _, err := mgr.CallTool(ctx, "nope/tool", nil); err == nil {
		t.Error("expected error calling tool on unconnected server")
	}
}

func TestServerConnectionErrorOnBrokenServer(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.SaveServer(ServerConfig{
		Name:    "broken",
		Command: "/nonexistent/bin/does-not-exist",
		Enabled: true,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = mgr.ConnectAll(ctx)
	if mgr.ServerConnectionError("broken") == nil {
		t.Error("expected a connection error for the broken server")
	}
	// A broken server must not produce any tools.
	if len(mgr.ListConnectedTools()) != 0 {
		t.Error("expected no tools from a broken server")
	}
	_ = json.Marshal
	_ = fmt.Sprintf
}
