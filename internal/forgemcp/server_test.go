package forgemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/internal/board"
)

func TestMCPServerHandshakeAndTools(t *testing.T) {
	tempDir := t.TempDir()
	server := NewServer(tempDir)
	bm := board.NewManager(tempDir)
	server.RegisterBoardTools(bm)
	server.RegisterWorkspaceTools(bm)

	ctx := context.Background()

	// 1. Initialize
	initReq := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	initResp := server.HandleRequest(ctx, initReq)
	if initResp == nil || initResp.Error != nil {
		t.Fatalf("expected initialize success, got: %+v", initResp)
	}

	res, ok := initResp.Result.(InitializeResult)
	if !ok || res.ProtocolVersion != MCPProtocolVersion {
		t.Fatalf("unexpected initialize result: %+v", initResp.Result)
	}

	// 2. Ping
	pingResp := server.HandleRequest(ctx, &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "ping",
	})
	if pingResp == nil || pingResp.Error != nil {
		t.Fatalf("expected ping success, got: %+v", pingResp)
	}

	// 3. Tools List
	listResp := server.HandleRequest(ctx, &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/list",
	})
	if listResp == nil || listResp.Error != nil {
		t.Fatalf("expected tools/list success, got: %+v", listResp)
	}

	toolsMap, ok := listResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("tools/list result not a map")
	}
	tools, ok := toolsMap["tools"].([]Tool)
	if !ok || len(tools) == 0 {
		t.Fatalf("expected tools list, got: %+v", toolsMap)
	}

	hasCreate := false
	for _, tool := range tools {
		if tool.Name == "forge_board_create_task" {
			hasCreate = true
			break
		}
	}
	if !hasCreate {
		t.Errorf("expected forge_board_create_task in tools list")
	}

	// 4. Create Task via Tool Call
	createArgs, _ := json.Marshal(map[string]interface{}{
		"name": "forge_board_create_task",
		"arguments": map[string]interface{}{
			"title":    "Build MCP feature",
			"priority": "high",
			"status":   "in_progress",
		},
	})

	callResp := server.HandleRequest(ctx, &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  createArgs,
	})
	if callResp == nil || callResp.Error != nil {
		t.Fatalf("expected tool call success, got: %+v", callResp)
	}

	callResult, ok := callResp.Result.(*ToolCallResult)
	if !ok || callResult.IsError || len(callResult.Content) == 0 {
		t.Fatalf("invalid tool call result: %+v", callResp.Result)
	}

	if len(callResult.Content[0].Text) == 0 {
		t.Errorf("expected text content in tool call result")
	}

	// 5. Verify task file exists on disk in .forge/board_tasks.json
	boardPath := filepath.Join(tempDir, ".forge", "board_tasks.json")
	tasks, err := bm.ListTasks(tempDir)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("expected 1 task in board (%s), got err: %v, count: %d", boardPath, err, len(tasks))
	}
	if tasks[0].Title != "Build MCP feature" || tasks[0].Status != "in_progress" {
		t.Errorf("task data mismatch: %+v", tasks[0])
	}
}

func TestIPCDaemon(t *testing.T) {
	tempDir := t.TempDir()
	server := NewServer(tempDir)
	bm := board.NewManager(tempDir)
	server.RegisterBoardTools(bm)
	server.RegisterWorkspaceTools(bm)

	daemon, err := StartIPCDaemon(server, 0)
	if err != nil {
		t.Fatalf("failed to start IPC daemon: %v", err)
	}
	defer daemon.Stop()

	port := daemon.Port()
	if port <= 0 {
		t.Fatalf("invalid daemon port: %d", port)
	}

	// Test health endpoint
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to call /health: err=%v, status=%v", err, resp)
	}
	_ = resp.Body.Close()

	// Test RPC forward
	rpcResp, err := ForwardToLiveDaemon(port, &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "ping",
	})
	if err != nil || rpcResp == nil || rpcResp.Error != nil {
		t.Fatalf("failed to forward RPC: err=%v, resp=%+v", err, rpcResp)
	}
}
