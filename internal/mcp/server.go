// Package mcp implements a Model Context Protocol (MCP) server for Launcher4j.
// It exposes Java project management operations as AI-callable tools via stdio JSON-RPC.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	mcpVersion    = "2024-11-05"
	serverName    = "launcher4j-mcp"
	serverVersion = "0.2.0"
)

// JSON-RPC message types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type jsonRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server is the MCP server.
type Server struct {
	handler  *Handler
	mu       sync.Mutex
	writer   io.Writer
}

// NewServer creates a new MCP server.
func NewServer(handler *Handler) *Server {
	return &Server{
		handler: handler,
		writer:  os.Stdout,
	}
}

// Run starts the MCP server loop, reading JSON-RPC messages from stdin.
func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Try to parse as request or notification
		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.log("MCP parse error: %v, raw: %s", err, string(line[:min(len(line), 200)]))
			continue
		}

		if req.ID == nil {
			// Notification (no response expected)
			s.handleNotification(req)
		} else {
			// Request (expects response)
			result, err := s.handleRequest(req)
			s.mu.Lock()
			s.writeResponse(req.ID, result, err)
			s.mu.Unlock()
		}
	}

	return scanner.Err()
}

// handleRequest routes a JSON-RPC request to the appropriate handler.
func (s *Server) handleRequest(req jsonRPCRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.Params)
	case "tools/list":
		return s.handler.listTools()
	case "tools/call":
		return s.handler.callTool(req.Params)
	default:
		return nil, &rpcError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
	}
}

// handleNotification processes notifications (no response sent).
func (s *Server) handleNotification(req jsonRPCRequest) {
	switch req.Method {
	case "notifications/initialized":
		s.log("Client initialized, MCP server ready")
	default:
		s.log("Unhandled notification: %s", req.Method)
	}
}

// handleInitialize responds to the standard MCP initialize request.
func (s *Server) handleInitialize(params json.RawMessage) (any, *rpcError) {
	s.log("Initialize received")
	return map[string]any{
		"protocolVersion": mcpVersion,
		"capabilities": map[string]any{
			"tools": map[string]bool{},
		},
		"serverInfo": map[string]string{
			"name":    serverName,
			"version": serverVersion,
		},
	}, nil
}

// writeResponse writes a JSON-RPC response to stdout.
func (s *Server) writeResponse(id any, result any, rpcErr *rpcError) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rpcErr,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		s.log("Failed to marshal response: %v", err)
		return
	}
	fmt.Fprintf(s.writer, "%s\n", data)
}

// log writes a debug message to stderr.
func (s *Server) log(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[launcher4j-mcp] "+format+"\n", args...)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
