package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func startMockLSP(t *testing.T, onNotify func(method string, params json.RawMessage)) (*Client, func(method string, params any)) {
	t.Helper()
	clientToSrvR, clientToSrvW := io.Pipe()
	srvToClientR, srvToClientW := io.Pipe()

	var srvWriteMu sync.Mutex
	sendSrv := func(payload any) {
		body, _ := json.Marshal(payload)
		srvWriteMu.Lock()
		defer srvWriteMu.Unlock()
		_, _ = fmt.Fprintf(srvToClientW, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}

	go func() {
		br := bufio.NewReader(clientToSrvR)
		for {
			contentLen := -1
			for {
				line, err := br.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimRight(line, "\r\n")
				if line == "" {
					break
				}
				if idx := strings.IndexByte(line, ':'); idx > 0 {
					k := strings.TrimSpace(line[:idx])
					v := strings.TrimSpace(line[idx+1:])
					if strings.EqualFold(k, "Content-Length") {
						contentLen, _ = strconv.Atoi(v)
					}
				}
			}
			if contentLen <= 0 {
				continue
			}
			buf := make([]byte, contentLen)
			if _, err := io.ReadFull(br, buf); err != nil {
				return
			}
			var msg rpcMessage
			if err := json.Unmarshal(buf, &msg); err != nil {
				continue
			}
			if msg.ID == nil {
				if msg.Method == "exit" {
					_ = srvToClientW.Close()
					return
				}
				continue
			}
			switch msg.Method {
			case "textDocument/completion":
				sendSrv(map[string]any{
					"jsonrpc": "2.0",
					"id":      *msg.ID,
					"result": CompletionList{
						Items: []CompletionItem{
							{Label: "Println", Kind: 3, Detail: "func(a ...any)"},
						},
					},
				})
			case "textDocument/hover":
				sendSrv(map[string]any{
					"jsonrpc": "2.0",
					"id":      *msg.ID,
					"result": map[string]any{
						"contents": MarkupContent{Kind: "markdown", Value: "func Println(a ...any)"},
					},
				})
			case "textDocument/definition":
				sendSrv(map[string]any{
					"jsonrpc": "2.0",
					"id":      *msg.ID,
					"result": Location{
						URI:   PathToURI("/tmp/main.go"),
						Range: Range{Start: Position{Line: 9, Character: 4}},
					},
				})
			case "textDocument/rename":
				sendSrv(map[string]any{
					"jsonrpc": "2.0",
					"id":      *msg.ID,
					"result": WorkspaceEdit{
						Changes: map[string][]TextEdit{
							PathToURI("/tmp/main.go"): {
								{Range: Range{Start: Position{Line: 1, Character: 5}, End: Position{Line: 1, Character: 8}}, NewText: "bar"},
							},
						},
					},
				})
			case "textDocument/formatting":
				sendSrv(map[string]any{
					"jsonrpc": "2.0",
					"id":      *msg.ID,
					"result": []TextEdit{
						{Range: Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 5}}, NewText: "package main"},
					},
				})
			case "shutdown":
				sendSrv(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": nil})
			default:
				sendSrv(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": nil})
			}
		}
	}()

	c := NewStreamClient(clientToSrvW, srvToClientR, onNotify)
	notifyClient := func(method string, params any) {
		sendSrv(map[string]any{
			"jsonrpc": "2.0",
			"method":  method,
			"params":  params,
		})
	}
	return c, notifyClient
}

func TestLSPManagerRequestsAndDiagnostics(t *testing.T) {
	mgr := NewManager()
	defer mgr.ShutdownAll()

	root := t.TempDir()
	file := filepath.Join(root, "main.go")

	diagCh := make(chan []Diagnostic, 1)
	mgr.OnDiagnostics = func(path string, diags []Diagnostic) {
		if path == file {
			diagCh <- diags
		}
	}

	c, notifyClient := startMockLSP(t, mgr.handleNotification)
	mgr.RegisterClient(root, "gopls", c)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	items, ok := mgr.Completion(ctx, root, file, 1, 4)
	if !ok || len(items) != 1 || items[0].Label != "Println" {
		t.Fatalf("unexpected completion: ok=%v items=%+v", ok, items)
	}

	hoverTxt, ok := mgr.Hover(ctx, root, file, 1, 4)
	if !ok || hoverTxt != "func Println(a ...any)" {
		t.Fatalf("unexpected hover: ok=%v txt=%q", ok, hoverTxt)
	}

	locs, ok := mgr.Definition(ctx, root, file, 1, 4)
	if !ok || len(locs) != 1 || locs[0].Range.Start.Line != 9 {
		t.Fatalf("unexpected definition: ok=%v locs=%+v", ok, locs)
	}

	we, ok := mgr.Rename(ctx, root, file, 1, 5, "bar")
	if !ok || len(we.Changes) != 1 {
		t.Fatalf("unexpected rename: ok=%v we=%+v", ok, we)
	}

	edits, ok := mgr.Formatting(ctx, root, file, 4, false)
	if !ok || len(edits) != 1 {
		t.Fatalf("unexpected formatting: ok=%v edits=%+v", ok, edits)
	}

	notifyClient("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         PathToURI(file),
		Diagnostics: []Diagnostic{{Range: Range{Start: Position{Line: 2, Character: 1}}, Severity: SeverityError, Message: "undefined: x"}},
	})

	select {
	case ds := <-diagCh:
		if len(ds) != 1 || ds[0].Message != "undefined: x" {
			t.Fatalf("unexpected diagnostics: %+v", ds)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for diagnostics")
	}
}
