package scheduled

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/oklog/ulid/v2"
)

const (
	httpMCPServerPathSSE      = "/mcp/scheduled-tasks/sse"
	httpMCPServerPathMessages = "/mcp/scheduled-tasks/messages"
	httpMCPKeepAliveInterval  = 15 * time.Second
	httpMCPReadHeaderTimeout  = 5 * time.Second
	httpMCPSessionBufferSize  = 16
)

type HTTPServerDependencies struct {
	Store                  app.ScheduledTaskStore
	Timezone               *time.Location
	DefaultReportChannelID string
	Logger                 *slog.Logger
}

type HTTPServer struct {
	logger *slog.Logger
	server *MCPServer

	mu       sync.Mutex
	sessions map[string]chan jsonRPCResponse
	listener net.Listener
	http     *http.Server
}

func NewHTTPServer(deps HTTPServerDependencies) (*HTTPServer, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("scheduled task store must not be nil")
	}
	if deps.Timezone == nil {
		return nil, fmt.Errorf("timezone must not be nil")
	}

	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &HTTPServer{
		logger: logger,
		server: &MCPServer{
			Store:                  deps.Store,
			Timezone:               deps.Timezone,
			DefaultReportChannelID: deps.DefaultReportChannelID,
		},
		sessions: make(map[string]chan jsonRPCResponse),
	}, nil
}

func (s *HTTPServer) Start(ctx context.Context) (string, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen for scheduled MCP HTTP server: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(httpMCPServerPathSSE, s.handleSSE)
	mux.HandleFunc(httpMCPServerPathMessages, s.handleMessagePost)

	server := &http.Server{
		ReadHeaderTimeout: httpMCPReadHeaderTimeout,
		Handler:           mux,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	s.mu.Lock()
	s.listener = listener
	s.http = server
	s.mu.Unlock()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("scheduled MCP HTTP server stopped unexpectedly", "error", err)
		}
	}()

	return "http://" + listener.Addr().String() + httpMCPServerPathSSE, nil
}

func (s *HTTPServer) Close(ctx context.Context) error {
	s.mu.Lock()
	server := s.http
	s.http = nil
	s.listener = nil
	s.mu.Unlock()

	if server == nil {
		return nil
	}

	return server.Shutdown(ctx)
}

func (s *HTTPServer) handleSSE(responseWriter http.ResponseWriter, request *http.Request) {
	flusher, ok := responseWriter.(http.Flusher)
	if !ok {
		http.Error(responseWriter, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sessionID := ulid.Make().String()
	sessionCh := make(chan jsonRPCResponse, httpMCPSessionBufferSize)
	s.registerSession(sessionID, sessionCh)
	defer s.unregisterSession(sessionID)

	responseWriter.Header().Set("Content-Type", "text/event-stream")
	responseWriter.Header().Set("Cache-Control", "no-cache")
	responseWriter.Header().Set("Connection", "keep-alive")
	responseWriter.WriteHeader(http.StatusOK)

	endpointURL := s.messageEndpointURL(request, sessionID)
	if _, err := fmt.Fprintf(responseWriter, "event: endpoint\ndata: %s\n\n", endpointURL); err != nil {
		return
	}
	flusher.Flush()

	keepAliveTicker := time.NewTicker(httpMCPKeepAliveInterval)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-request.Context().Done():
			return
		case <-keepAliveTicker.C:
			if _, err := io.WriteString(responseWriter, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case response := <-sessionCh:
			payload, err := json.Marshal(response)
			if err != nil {
				s.logger.Error("marshal scheduled MCP HTTP response", "error", err)
				return
			}

			if _, err := fmt.Fprintf(responseWriter, "event: message\ndata: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *HTTPServer) handleMessagePost(responseWriter http.ResponseWriter, request *http.Request) {
	sessionID := strings.TrimSpace(request.URL.Query().Get("sessionId"))
	if sessionID == "" {
		http.Error(responseWriter, "missing sessionId", http.StatusBadRequest)
		return
	}

	sessionCh, ok := s.session(sessionID)
	if !ok {
		http.Error(responseWriter, "unknown sessionId", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(responseWriter, err.Error(), http.StatusBadRequest)
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(responseWriter, err.Error(), http.StatusBadRequest)
		return
	}

	response := s.server.handleRequest(request.Context(), req)
	if len(req.ID) > 0 && response.JSONRPC != "" {
		select {
		case sessionCh <- response:
		case <-request.Context().Done():
			http.Error(responseWriter, request.Context().Err().Error(), http.StatusRequestTimeout)
			return
		}
	}

	responseWriter.WriteHeader(http.StatusAccepted)
}

func (s *HTTPServer) messageEndpointURL(request *http.Request, sessionID string) string {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}

	return fmt.Sprintf(
		"%s://%s%s?sessionId=%s",
		scheme,
		request.Host,
		httpMCPServerPathMessages,
		url.QueryEscape(sessionID),
	)
}

func (s *HTTPServer) registerSession(sessionID string, sessionCh chan jsonRPCResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = sessionCh
}

func (s *HTTPServer) unregisterSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *HTTPServer) session(sessionID string) (chan jsonRPCResponse, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionCh, ok := s.sessions[sessionID]
	return sessionCh, ok
}
