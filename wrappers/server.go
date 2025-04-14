package wrappers

import (
	"context"
	"net/http"
	"time"

	"github.com/AnuragThePathak/my-go-packages/srv"
)

// Server provides an interface for starting and gracefully shutting down an HTTP server.
type Server interface {
	// StartWithGracefulShutdown starts the server and shuts it down gracefully on context cancellation.
	StartWithGracefulShutdown(
		parentCtx context.Context,
		timeout time.Duration,
		handlers ...srv.CleanupHandler,
	)

	// Start starts the server without graceful shutdown.
	Start() (*http.Server, error)
}
