package wrappers

import (
	"context"
	"net/http"
	"time"

	"github.com/AnuragThePathak/my-go-packages/srv"
	// "github.com/go-chi/cors"
)

type Server interface {
	StartWithGracefulShutdown(
		parentCtx context.Context,
		timeout time.Duration,
		handlers ...srv.CleanupHandler,
	)

	Start() (*http.Server, error)
}

