package forgemcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
)

// ToolHandler processes a tool invocation with arguments.
type ToolHandler func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error)

// Server implements an MCP server instance.
type Server struct {
	mu        sync.RWMutex
	tools     map[string]Tool
	handlers  map[string]ToolHandler
	workspace string
}

// NewServer creates a new MCP Server instance.
func NewServer(workspace string) *Server {
	return &Server{
		tools:     make(map[string]Tool),
		handlers:  make(map[string]ToolHandler),
		workspace: workspace,
	}
}

// SetWorkspace updates the server's working directory.
func (s *Server) SetWorkspace(ws string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspace = ws
}

// GetWorkspace returns the server's active workspace path.
func (s *Server) GetWorkspace() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workspace
}

// RegisterTool registers an available tool with its handler.
func (s *Server) RegisterTool(t Tool, h ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[t.Name] = t
	s.handlers[t.Name] = h
}

// ListTools returns all registered tools.
func (s *Server) ListTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]Tool, 0, len(s.tools))
	for _, t := range s.tools {
		list = append(list, t)
	}
	return list
}

// HandleRequest processes an incoming JSON-RPC 2.0 message.
func (s *Server) HandleRequest(ctx context.Context, req *JSONRPCMessage) *JSONRPCMessage {
	if req.JSONRPC != JSONRPCVersion {
		return &JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    CodeInvalidRequest,
				Message: "Invalid JSON-RPC version, expected 2.0",
			},
		}
	}

	switch req.Method {
	case "initialize":
		res := InitializeResult{
			ProtocolVersion: MCPProtocolVersion,
			Capabilities: ServerCapabilities{
				Tools: map[string]interface{}{
					"listChanged": true,
				},
				Resources: map[string]interface{}{},
				Prompts:   map[string]interface{}{},
			},
			ServerInfo: ServerInfo{
				Name:    ServerName,
				Version: ServerVersion,
			},
		}
		return &JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  res,
		}

	case "notifications/initialized", "initialized":
		// Client confirmation; no response for notifications
		return nil

	case "ping":
		return &JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  map[string]interface{}{},
		}

	case "tools/list":
		return &JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": s.ListTools(),
			},
		}

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    CodeInvalidParams,
					Message: fmt.Sprintf("invalid tool call params: %v", err),
				},
			}
		}

		s.mu.RLock()
		handler, ok := s.handlers[params.Name]
		s.mu.RUnlock()

		if !ok {
			return &JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    CodeMethodNotFound,
					Message: fmt.Sprintf("tool not found: %s", params.Name),
				},
			}
		}

		result, err := handler(ctx, params.Arguments)
		if err != nil {
			return &JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Result:  ErrorResult(err),
			}
		}

		return &JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  result,
		}

	case "resources/list":
		return &JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result: map[string]interface{}{
				"resources": []interface{}{},
			},
		}

	case "prompts/list":
		return &JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result: map[string]interface{}{
				"prompts": []interface{}{},
			},
		}

	default:
		// If ID is nil, it's an unhandled notification (do not reply)
		if req.ID == nil {
			return nil
		}
		return &JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    CodeMethodNotFound,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			},
		}
	}
}

// ServeStdio starts the MCP JSON-RPC 2.0 loop reading from `in` and writing to `out`.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Set scanner buffer up to 10MB to accommodate base64 image snapshots
	const maxBuffer = 10 * 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxBuffer)

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
			writeMessage(out, resp)
			continue
		}

		resp := s.HandleRequest(ctx, &req)
		if resp != nil {
			if err := writeMessage(out, resp); err != nil {
				log.Printf("[forge-mcp] write error: %v", err)
			}
		}
	}

	return scanner.Err()
}

func writeMessage(out io.Writer, msg *JSONRPCMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = out.Write(b)
	return err
}

// TextResult creates a success ToolCallResult containing plain text.
func TextResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: text,
			},
		},
	}
}

// ErrorResult creates an error ToolCallResult with formatted message.
func ErrorResult(err error) *ToolCallResult {
	return &ToolCallResult{
		IsError: true,
		Content: []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Error: %v", err),
			},
		},
	}
}

// ImageResult creates a ToolCallResult containing a base64 encoded image.
func ImageResult(dataBase64 string, mimeType string) *ToolCallResult {
	if mimeType == "" {
		mimeType = "image/png"
	}
	return &ToolCallResult{
		Content: []ContentItem{
			{
				Type:     "image",
				Data:     dataBase64,
				MimeType: mimeType,
			},
		},
	}
}
