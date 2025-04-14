package wrappers

import (
	"context"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/queue"
)

// QueueWorker wraps the ReminderWorker to provide graceful shutdown capabilities.
type QueueWorker struct {
	Worker *queue.ReminderWorker
}

// Shutdown gracefully shuts down the worker, respecting the provided context.
func (w *QueueWorker) Shutdown(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	closeChan := make(chan error, 1)

	go func() {
		slog.Info("Stopping queue worker", slog.String("component", "queue_worker"))
		w.Worker.Stop()
		close(closeChan)
	}()

	select {
	case <-closeChan:
		slog.Info("Queue worker stopped successfully", slog.String("component", "queue_worker"))
		return nil
	case <-ctx.Done():
		slog.Warn("Context expired while stopping queue worker", slog.String("component", "queue_worker"))
		return ctx.Err()
	}
}
