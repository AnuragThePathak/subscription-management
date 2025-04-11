package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/repositories"
	"github.com/hibiken/asynq"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ReminderWorker handles processing of reminder tasks
type ReminderWorker struct {
	subscriptionRepo repositories.SubscriptionRepository
	server           *asynq.Server
}

// NewReminderWorker creates a new reminder worker
func NewReminderWorker(
	subscriptionRepo repositories.SubscriptionRepository,
	redisConfig *asynq.RedisClientOpt,
	concurrency int,
) *ReminderWorker {
	// Configure the server with appropriate concurrency
	server := asynq.NewServer(
		redisConfig,
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"default": 10, // Process reminder tasks with higher priority
				"low":     5,
			},
		},
	)

	return &ReminderWorker{
		subscriptionRepo: subscriptionRepo,
		server:           server,
	}
}

// Start begins processing tasks from the queue
func (w *ReminderWorker) Start(ctx context.Context) error {
	// Register task handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc(ReminderTask, w.handleSubscriptionReminder)

	// Start the worker server
	slog.Info("Starting reminder worker",
		slog.String("component", "worker"))

	return w.server.Start(mux)
}

// handleSubscriptionReminder processes a subscription reminder task
func (w *ReminderWorker) handleSubscriptionReminder(ctx context.Context, task *asynq.Task) error {
	var payload ReminderPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal task payload: %v", err)
	}

	slog.Info("Processing subscription reminder",
		slog.String("component", "worker"),
		slog.String("subscription_id", payload.SubscriptionID),
		slog.Int("days_before", payload.DaysBefore),
	)

	// Parse the subscription ID
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID: %v", err)
	}

	// Fetch the subscription from the database
	subscription, err := w.subscriptionRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %v", err)
	}

	// Ensure the subscription is still active
	if subscription.Status != models.Active {
		slog.Info("Skipping reminder for non-active subscription",
			slog.String("component", "worker"),
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("status", string(subscription.Status)),
		)
		return nil
	}

	// Process the reminder (in this case, we'd send an email)
	return w.sendReminderNotification(ctx, subscription, payload.DaysBefore)
}

// sendReminderNotification handles sending the actual reminder notification
func (w *ReminderWorker) sendReminderNotification(_ context.Context, subscription *models.Subscription, daysBefore int) error {
	// In a real implementation, this would send an email or push notification
	// For now, we'll just log the notification
	slog.Info("Would send reminder notification",
		slog.String("component", "worker"),
		slog.String("subscription_id", subscription.ID.Hex()),
		slog.String("subscription_name", subscription.Name),
		slog.Int("days_before", daysBefore),
		slog.Time("renewal_date", subscription.RenewalDate),
	)

	// TODO: Add actual email sending logic here
	// return sendReminderEmail({
	//     to: subscription.user.email,
	//     type: `${daysBefore} days before reminder`,
	//     subscription: subscription,
	// })

	return nil
}

// Stop gracefully shuts down the worker
func (w *ReminderWorker) Stop() {
	w.server.Shutdown()
}
