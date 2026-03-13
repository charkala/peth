// Package mcp provides a simplified MCP (Model Context Protocol) server
// that exposes peth tools over HTTP using a JSON-RPC style interface.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
)

// ToolHandler is a function that handles a tool invocation.
type ToolHandler func(params map[string]interface{}) (interface{}, error)

// Tool represents a registered MCP tool.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
	Handler     ToolHandler            `json:"-"`
}

// mcpRequest is the incoming JSON-RPC style request.
type mcpRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// mcpCallParams are the parameters for a tools/call request.
type mcpCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// mcpResponse is the outgoing response.
type mcpResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *mcpError   `json:"error,omitempty"`
}

// mcpError represents an error response.
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server is the MCP HTTP server.
type Server struct {
	port     int
	tools    []Tool
	toolMap  map[string]*Tool
	mu       sync.RWMutex
	server   *http.Server
	listener net.Listener
}

// NewServer creates a new MCP Server on the given port.
func NewServer(port int) *Server {
	return &Server{
		port:    port,
		toolMap: make(map[string]*Tool),
	}
}

// RegisterTool adds a tool to the server's registry.
func (s *Server) RegisterTool(tool Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools = append(s.tools, tool)
	s.toolMap[tool.Name] = &s.tools[len(s.tools)-1]
}

// Start begins serving HTTP requests.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)

	s.server = &http.Server{Handler: mux}

	go s.server.Serve(ln)
	return nil
}

// Stop shuts down the HTTP server.
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}
	err := s.server.Shutdown(context.Background())
	s.server = nil
	return err
}

// Addr returns the listener address (useful when port=0).
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// handler returns the HTTP handler for testing with httptest.
func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)
	return mux
}

// handleMCP handles POST /mcp requests.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, -1, "invalid request: "+err.Error())
		return
	}

	switch req.Method {
	case "tools/list":
		s.handleToolsList(w)
	case "tools/call":
		s.handleToolsCall(w, req.Params)
	default:
		writeError(w, -1, "unknown method: "+req.Method)
	}
}

// handleToolsList returns the list of registered tools.
func (s *Server) handleToolsList(w http.ResponseWriter) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type toolInfo struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
	}

	tools := make([]toolInfo, len(s.tools))
	for i, t := range s.tools {
		tools[i] = toolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	writeResult(w, map[string]interface{}{"tools": tools})
}

// handleToolsCall invokes a registered tool.
func (s *Server) handleToolsCall(w http.ResponseWriter, rawParams json.RawMessage) {
	if len(rawParams) == 0 || string(rawParams) == "null" {
		writeError(w, -1, "missing params for tools/call")
		return
	}

	var params mcpCallParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		writeError(w, -1, "invalid params: "+err.Error())
		return
	}

	s.mu.RLock()
	tool, ok := s.toolMap[params.Name]
	s.mu.RUnlock()

	if !ok {
		writeError(w, -1, "tool not found: "+params.Name)
		return
	}

	if tool.Handler == nil {
		writeError(w, -1, "tool has no handler: "+params.Name)
		return
	}

	result, err := tool.Handler(params.Arguments)
	if err != nil {
		writeError(w, -1, err.Error())
		return
	}

	writeResult(w, result)
}

func writeResult(w http.ResponseWriter, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mcpResponse{Result: result})
}

func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mcpResponse{Error: &mcpError{Code: code, Message: message}})
}
