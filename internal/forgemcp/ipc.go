package forgemcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/board"
	"github.com/hasdev/forge-ade/internal/browseruse"
)

const (
	DefaultIPCPort = 42110
	IPCHealthPath  = "/health"
	IPCRPCPath     = "/rpc"
	IPCSSEPath     = "/sse"
)

// IPCDaemon manages the local HTTP IPC server inside the ForgeADE desktop app.
type IPCDaemon struct {
	server   *Server
	listener net.Listener
	httpSrv  *http.Server
	port     int
	mu       sync.Mutex
}

// StartIPCDaemon starts the local IPC server embedded in the GUI app.
func StartIPCDaemon(server *Server, preferredPort int) (*IPCDaemon, error) {
	if preferredPort <= 0 {
		preferredPort = DefaultIPCPort
	}

	addr := fmt.Sprintf("127.0.0.1:%d", preferredPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// If preferred port is busy, fallback to any free port
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("failed to bind IPC listener: %w", err)
		}
	}

	tcpAddr := listener.Addr().(*net.TCPAddr)
	port := tcpAddr.Port

	// Record the port into ~/.forge/ipc.port for CLI discovery
	recordIPCPort(port)

	mux := http.NewServeMux()
	daemon := &IPCDaemon{
		server:   server,
		listener: listener,
		port:     port,
	}

	mux.HandleFunc(IPCHealthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"app":       "ForgeADE",
			"version":   ServerVersion,
			"workspace": server.GetWorkspace(),
			"port":      port,
		})
	})

	mux.HandleFunc(IPCRPCPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var req JSONRPCMessage
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
			return
		}
		resp := server.HandleRequest(r.Context(), &req)
		w.Header().Set("Content-Type", "application/json")
		if resp != nil {
			_ = json.NewEncoder(w).Encode(resp)
		}
	})

	// Simple SSE handler for external agents connecting via HTTP SSE
	mux.HandleFunc(IPCSSEPath, func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send endpoint info
		fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", IPCRPCPath)
		flusher.Flush()

		// Keep connection open until context cancellation
		<-r.Context().Done()
	})

	daemon.httpSrv = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		if err := daemon.httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[forge-mcp] IPC server stopped: %v", err)
		}
	}()

	log.Printf("[forge-mcp] Live IPC daemon listening on http://127.0.0.1:%d", port)
	return daemon, nil
}

// Stop terminates the IPC daemon.
func (d *IPCDaemon) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = d.httpSrv.Shutdown(ctx)
	}
	clearIPCPort()
}

// Port returns the active daemon port.
func (d *IPCDaemon) Port() int {
	return d.port
}

// CheckLiveDaemon checks if a live ForgeADE desktop GUI is running.
func CheckLiveDaemon() (int, bool) {
	port := readIPCPort()
	if port <= 0 {
		port = DefaultIPCPort
	}

	client := &http.Client{Timeout: 350 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, IPCHealthPath))
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		return port, true
	}
	return 0, false
}

// ForwardToLiveDaemon sends a JSON-RPC message to the running ForgeADE app.
func ForwardToLiveDaemon(port int, req *JSONRPCMessage) (*JSONRPCMessage, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d%s", port, IPCRPCPath), "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return nil, err
	}

	var rpcResp JSONRPCMessage
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, err
	}
	return &rpcResp, nil
}

// RunCLI is the main entry point for the CLI / stdio execution mode.
func RunCLI() {
	ctx := context.Background()
	cwd, _ := os.Getwd()

	// 1. Check if live ForgeADE desktop app is currently running
	if port, live := CheckLiveDaemon(); live {
		// Run in Relay mode to the live GUI
		runRelay(ctx, port)
		return
	}

	// 2. Fallback: Run in standalone headless mode
	server := NewServer(cwd)
	bm := board.NewManager(cwd)
	server.RegisterBoardTools(bm)
	server.RegisterWorkspaceTools(bm)

	// Note: Browser tools in standalone mode can be enabled if CDP is available
	bu := browseruse.NewManager(cwd)
	server.RegisterBrowserTools(bu)

	if err := server.ServeStdio(ctx, os.Stdin, os.Stdout); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "forge-mcp error: %v\n", err)
	}
}

func runRelay(ctx context.Context, port int) {
	scanner := bufioNewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCMessage
		if err := json.Unmarshal(line, &req); err != nil {
			resp := &JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				Error: &JSONRPCError{
					Code:    CodeParseError,
					Message: fmt.Sprintf("parse error: %v", err),
				},
			}
			writeMessage(os.Stdout, resp)
			continue
		}

		resp, err := ForwardToLiveDaemon(port, &req)
		if err != nil {
			resp = &JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    CodeInternalError,
					Message: fmt.Sprintf("IPC forward error: %v", err),
				},
			}
		}

		if resp != nil {
			_ = writeMessage(os.Stdout, resp)
		}
	}
}

func bufioNewScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)
	return scanner
}

func recordIPCPort(port int) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".forge")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "ipc.port"), []byte(strconv.Itoa(port)), 0644)
}

func readIPCPort() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	data, err := os.ReadFile(filepath.Join(home, ".forge", "ipc.port"))
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(string(bytes.TrimSpace(data)))
	if err != nil {
		return 0
	}
	return port
}

func clearIPCPort() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	_ = os.Remove(filepath.Join(home, ".forge", "ipc.port"))
}
