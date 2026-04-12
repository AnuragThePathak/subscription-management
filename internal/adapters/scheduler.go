package adapters

import (
	"context"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/scheduler"
)

// Scheduler wraps the SubscriptionScheduler to provide graceful shutdown capabilities.
type Scheduler struct {
	Scheduler *scheduler.SubscriptionScheduler
}

// Shutdown gracefully shuts down the scheduler, respecting the provided context.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	closeChan := make(chan error, 1)

	go func() {
		slog.InfoContext(ctx, "Stopping subscription scheduler")
		closeChan <- s.Scheduler.Close()
	}()

	select {
	case err := <-closeChan:
		if err != nil {
			slog.ErrorContext(ctx, "Failed to stop subscription scheduler", slog.Any("error", err))
		} else {
			slog.InfoContext(ctx, "Subscription scheduler stopped successfully")
		}
		return err
	case <-ctx.Done():
		slog.WarnContext(ctx, "Context expired while stopping subscription scheduler")
		return ctx.Err()
	}
}
