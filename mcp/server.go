package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HandlerFunc is the function signature for a tool handler.
type HandlerFunc func(args map[string]any) (*ToolCallResult, error)

type session struct {
	id      string
	msgCh   chan Response // server → client via SSE
	closeCh chan struct{}
}

type Server struct {
	name    string
	version string

	tools    []Tool
	handlers map[string]HandlerFunc

	sessions map[string]*session
	mu       sync.RWMutex
}

func NewServer(name, version string) *Server {
	return &Server{
		name:     name,
		version:  version,
		handlers: make(map[string]HandlerFunc),
		sessions: make(map[string]*session),
	}
}

// AddTool registers a tool with its schema and handler.
func (s *Server) AddTool(tool Tool, handler HandlerFunc) {
	s.tools = append(s.tools, tool)
	s.handlers[tool.Name] = handler
}

// Start begins the HTTP server.
// Two endpoints:
//
//	GET  /sse     → SSE stream, gives client its sessionId
//	POST /message → JSON-RPC requests from client
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/message", s.handleMessage)
	mux.HandleFunc("/health", s.handleHealth)

	log.Printf("[mcp] server %s v%s listening on %s", s.name, s.version, addr)
	log.Printf("[mcp] %d tools registered", len(s.tools))
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	sess := &session{
		id:      uuid.NewString(),
		msgCh:   make(chan Response, 32),
		closeCh: make(chan struct{}),
	}

	s.mu.Lock()
	s.sessions[sess.id] = sess
	s.mu.Unlock()

	log.Printf("[mcp] client connected session=%s", sess.id[:8])

	// Format: data: /message?sessionId=<id>
	fmt.Fprintf(w, "event: endpoint\ndata: /message?sessionId=%s\n\n", sess.id)
	flusher.Flush()

	// keep the session alive
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	defer func() {
		s.mu.Lock()
		delete(s.sessions, sess.id)
		s.mu.Unlock()
		log.Printf("[mcp] client disconnected session=%s", sess.id[:8])
	}()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			return

		case msg := <-sess.msgCh:
			// Send response as SSE event
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()

		case <-ticker.C:
			// keep the connection alive with a ping
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	sessionID := r.URL.Query().Get("sessionId")
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		resp := ErrResponse(nil, ErrParse, "parse error: "+err.Error())
		sendSSE(sess, resp)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var resp Response
	switch req.Method {
	case "initialize":
		resp = s.handleInitialize(req)
	case "tools/list":
		resp = s.handleToolsList(req)
	case "tools/call":
		resp = s.handleToolCall(req)
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
		return
	default:
		resp = ErrResponse(req.ID, ErrMethodNotFound, "method not found: "+req.Method)
	}

	sendSSE(sess, resp)

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleInitialize(req Request) Response {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCaps{
			Tools: &ToolsCap{ListChanged: false},
		},
		ServerInfo: Info{
			Name:    s.name,
			Version: s.version,
		},
	}
	return OKResponse(req.ID, result)
}

func (s *Server) handleToolsList(req Request) Response {
	return OKResponse(req.ID, ToolsListResult{Tools: s.tools})
}

func (s *Server) handleToolCall(req Request) Response {
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return ErrResponse(req.ID, ErrInvalidParams, "invalid params")
	}

	var params ToolCallParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return ErrResponse(req.ID, ErrInvalidParams, "parse params: "+err.Error())
	}

	handler, ok := s.handlers[params.Name]
	if !ok {
		return OKResponse(req.ID, ErrorResult("tool not found: "+params.Name))
	}

	log.Printf("[mcp] tool call: %s args=%v", params.Name, params.Arguments)

	result, err := handler(params.Arguments)
	if err != nil {
		return OKResponse(req.ID, ErrorResult(err.Error()))
	}

	return OKResponse(req.ID, result)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mu.RLock()
	sessions := len(s.sessions)
	s.mu.RUnlock()
	fmt.Fprintf(w, `{"status":"ok","tools":%d,"sessions":%d}`, len(s.tools), sessions)
}

func sendSSE(sess *session, resp Response) {
	select {
	case sess.msgCh <- resp:
	default:
		log.Printf("[mcp] session %s message buffer full", sess.id[:8])
	}
}

// getString safely extracts a string from tool arguments.
// func getString(args map[string]interface{}, key, fallback string) string {
// 	if v, ok := args[key]; ok {
// 		if s, ok := v.(string); ok && s != "" {
// 			return s
// 		}
// 	}
// 	return fallback
// }

// getFloat64 safely extracts a float64 from tool arguments.
// func getFloat64(args map[string]interface{}, key string, fallback float64) float64 {
// 	if v, ok := args[key]; ok {
// 		switch n := v.(type) {
// 		case float64:
// 			return n
// 		case int:
// 			return float64(n)
// 		}
// 	}
// 	return fallback
// }

// getInt safely extracts an int from tool arguments.
// func getInt(args map[string]interface{}, key string, fallback int) int {
// 	return int(getFloat64(args, key, float64(fallback)))
// }

// ─────────────────────────────────────────────
//  Tool builder helpers (replaces mcp-go DSL)
// ─────────────────────────────────────────────

// NewTool creates a tool definition.
func NewTool(name, description string, props map[string]Property, required []string) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: InputSchema{
			Type:       "object",
			Properties: props,
			Required:   required,
		},
	}
}

// Str creates a string property.
func Str(description string) Property {
	return Property{Type: "string", Description: description}
}

// Num creates a number property.
func Num(description string) Property {
	return Property{Type: "number", Description: description}
}

// stripPrefix removes scheme+host from a URL leaving just the path+query.
// func stripPrefix(url string) string {
// 	// http://host/path → /path
// 	if idx := strings.Index(url, "://"); idx >= 0 {
// 		rest := url[idx+3:]
// 		if idx2 := strings.Index(rest, "/"); idx2 >= 0 {
// 			return rest[idx2:]
// 		}
// 	}
// 	return url
// }
