package wrappers

import (
	"context"

	"github.com/anuragthepathak/subscription-management/queue"
)

type Scheduler struct {
	Scheduler *queue.SubscriptionScheduler
}

func (s *Scheduler) Shutdown(ctx context.Context) error {
	// Check if context is already done before we even start
	if ctx.Err() != nil {
		return ctx.Err()
	}

	closeChan := make(chan error, 1)

	go func() {
		closeChan <- s.Scheduler.Close()
	}()

	// Wait for either Close() to complete or context to expire
	select {
	case err := <-closeChan:
		return err
	case <-ctx.Done():
		// Context expired, but we can't really abort Close() once started
		// Let the goroutine complete in the background
		return ctx.Err()
	}
}
