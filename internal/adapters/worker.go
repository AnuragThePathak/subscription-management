package adapters

import (
	"context"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/scheduler"
)

// SchedulerWorker wraps the ReminderWorker to provide graceful shutdown capabilities.
type SchedulerWorker struct {
	Worker *scheduler.ReminderWorker
}

// Shutdown gracefully shuts down the worker, respecting the provided context.
func (w *SchedulerWorker) Shutdown(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	closeChan := make(chan error, 1)

	go func() {
		slog.Info("Stopping queue worker")
		w.Worker.Stop()
		close(closeChan)
	}()

	select {
	case <-closeChan:
		slog.Info("Queue worker stopped successfully")
		return nil
	case <-ctx.Done():
		slog.Warn("Context expired while stopping queue worker")
		return ctx.Err()
	}
}
