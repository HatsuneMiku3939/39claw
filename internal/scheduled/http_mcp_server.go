package scheduled

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const (
	httpMCPBasePath          = "/mcp/scheduled-tasks"
	httpMCPReadHeaderTimeout = 5 * time.Second
	httpMCPSessionIdleTTL    = 15 * time.Minute
)

type HTTPServerDependencies struct {
	Store                  app.ScheduledTaskStore
	Timezone               *time.Location
	DefaultReportChannelID string
	Logger                 *slog.Logger
}

type HTTPServer struct {
	logger    *slog.Logger
	transport *mcpserver.StreamableHTTPServer
	http      *http.Server
}

func NewHTTPServer(deps HTTPServerDependencies) (*HTTPServer, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	mcpTools := &MCPServer{
		Store:                  deps.Store,
		Timezone:               deps.Timezone,
		DefaultReportChannelID: deps.DefaultReportChannelID,
	}
	mcpHandler, err := mcpTools.BuildServer()
	if err != nil {
		return nil, err
	}

	httpServer := &http.Server{
		ReadHeaderTimeout: httpMCPReadHeaderTimeout,
	}
	transport := mcpserver.NewStreamableHTTPServer(
		mcpHandler,
		mcpserver.WithStreamableHTTPServer(httpServer),
		mcpserver.WithStateful(true),
		mcpserver.WithSessionIdleTTL(httpMCPSessionIdleTTL),
	)
	mux := http.NewServeMux()
	mux.Handle(httpMCPBasePath, transport)
	httpServer.Handler = mux

	return &HTTPServer{
		logger:    logger,
		transport: transport,
		http:      httpServer,
	}, nil
}

func (s *HTTPServer) Start(ctx context.Context) (string, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen for scheduled MCP HTTP server: %w", err)
	}

	s.http.BaseContext = func(net.Listener) context.Context {
		return ctx
	}

	go func() {
		if err := s.http.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("scheduled MCP HTTP server stopped unexpectedly", "error", err)
		}
	}()

	return "http://" + listener.Addr().String() + httpMCPBasePath, nil
}

func (s *HTTPServer) Close(ctx context.Context) error {
	if s.http == nil {
		return nil
	}

	return s.http.Shutdown(ctx)
}
