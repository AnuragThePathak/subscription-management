package wrappers

import (
	"context"

	"github.com/anuragthepathak/subscription-management/queue"
)

type QueueWorker struct {
	Worker *queue.ReminderWorker
}

func (w *QueueWorker) Shutdown(ctx context.Context) error {
	// Check if context is already done before we even start
	if ctx.Err() != nil {
		return ctx.Err()
	}
	
	closeChan := make(chan error, 1)

	go func() {
		w.Worker.Stop()
		close(closeChan)
	}()

	// Wait for either Close() to complete or context to expire
	select {
	case <-closeChan:
		return nil
	case <-ctx.Done():
		// Context expired, but we can't really abort Close() once started
		// Let the goroutine complete in the background
		return ctx.Err()
	}
}
