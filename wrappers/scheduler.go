package wrappers

import (
	"context"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/queue"
)

// Scheduler wraps the SubscriptionScheduler to provide graceful shutdown capabilities.
type Scheduler struct {
	Scheduler *queue.SubscriptionScheduler
}

// Shutdown gracefully shuts down the scheduler, respecting the provided context.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	closeChan := make(chan error, 1)

	go func() {
		slog.Info("Stopping subscription scheduler", slog.String("component", "scheduler"))
		closeChan <- s.Scheduler.Close()
	}()

	select {
	case err := <-closeChan:
		if err != nil {
			slog.Error("Failed to stop subscription scheduler", slog.String("component", "scheduler"), slog.Any("error", err))
		} else {
			slog.Info("Subscription scheduler stopped successfully", slog.String("component", "scheduler"))
		}
		return err
	case <-ctx.Done():
		slog.Warn("Context expired while stopping subscription scheduler", slog.String("component", "scheduler"))
		return ctx.Err()
	}
}
