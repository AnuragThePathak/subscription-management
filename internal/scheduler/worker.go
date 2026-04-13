package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/notifications"
	"github.com/anuragthepathak/subscription-management/internal/observability"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ReminderWorker handles processing of reminder tasks.
type ReminderWorker struct {
	subscriptionService services.SubscriptionServiceInternal
	userService         services.UserServiceInternal
	emailSender         *notifications.EmailSender
	redisClient         *redis.Client
	server              *asynq.Server
	name                string
}

// NewReminderWorker creates a new reminder worker.
func NewReminderWorker(
	subscriptionService services.SubscriptionServiceInternal,
	userService services.UserServiceInternal,
	emailSender *notifications.EmailSender,
	redisClient *redis.Client,
	redisConfig *asynq.RedisClientOpt,
	concurrency int,
	queueName string,
	name string,
) *ReminderWorker {
	// Configure the server with appropriate concurrency.
	server := asynq.NewServer(
		redisConfig,
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				queueName: 10, // Process reminder tasks with higher priority.
				"low":     5,
			},
		},
	)

	return &ReminderWorker{
		subscriptionService,
		userService,
		emailSender,
		redisClient,
		server,
		name,
	}
}

// Start begins processing tasks from the queue.
func (w *ReminderWorker) Start() error {
	// Register task handlers.
	mux := asynq.NewServeMux()
	
	// Inject OpenTelemetry middleware to extract trace IDs from the queue payload headers
	mux.Use(observability.AsynqTracingMiddleware(w.name))

	mux.HandleFunc(ReminderTask, w.handleSubscriptionReminder)
	mux.HandleFunc(RenewalTask, w.handleSubscriptionRenewal)
	mux.HandleFunc(ExpirationTask, w.handleSubscriptionExpiration)

	return w.server.Start(mux)
}

// handleSubscriptionReminder processes a subscription reminder task.
func (w *ReminderWorker) handleSubscriptionReminder(ctx context.Context, task *asynq.Task) error {
	var payload ReminderPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal task payload: %v", err)
	}

	slog.DebugContext(ctx, "Processing subscription reminder",
		slog.String("subscription_id", payload.SubscriptionID),
		slog.Int("days_before", payload.DaysBefore),
	)

	// Parse the subscription ID.
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID: %v", err)
	}

	// Fetch the subscription from the database.
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %v", err)
	}

	// Ensure the subscription is still active.
	if subscription.Status != models.Active {
		slog.DebugContext(ctx, "Skipping reminder for non-active subscription",
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("status", string(subscription.Status)),
		)
		return nil
	}

	// Process the reminder (send an email).
	return w.sendReminderNotification(ctx, subscription, payload.DaysBefore)
}

// handleSubscriptionRenewal processes an automatic subscription renewal task.
func (w *ReminderWorker) handleSubscriptionRenewal(ctx context.Context, task *asynq.Task) error {
	var payload RenewalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal renewal task payload: %v", err)
	}

	slog.DebugContext(ctx, "Processing subscription renewal",
		slog.String("subscription_id", payload.SubscriptionID),
	)

	// Parse the subscription ID
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID: %v", err)
	}

	// Fetch the subscription from the database
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %v", err)
	}

	// Ensure the subscription is still active
	if subscription.Status != models.Active {
		slog.DebugContext(ctx, "Skipping renewal for non-active subscription",
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("status", string(subscription.Status)),
		)
		return nil
	}

	// Check if the renewal date is within our window (now to next 4 hours)
	now := time.Now()
	renewalWindow := now.Add(time.Hour * RenewalHoursBeforeDay)
	if subscription.ValidTill.After(renewalWindow) {
		slog.DebugContext(ctx, "Skipping renewal: outside valid window",
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("valid_till", subscription.ValidTill.Format(time.RFC3339)),
		)
		return nil
	}

	// Process the automatic renewal
	renewedSubscription, err := w.subscriptionService.RenewSubscriptionInternal(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to renew subscription: %v", err)
	}

	slog.InfoContext(ctx, "Subscription renewed",
		slog.String("subscription_id", payload.SubscriptionID),
		slog.String("user_id", subscription.UserID.Hex()),
		slog.String("new_valid_till", renewedSubscription.ValidTill.Format(time.RFC3339)),
	)

	// Send a confirmation email to the user
	user, err := w.userService.FetchUserByIDInternal(ctx, subscription.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch user for renewal notification",
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("user_id", subscription.UserID.Hex()),
			slog.Any("error", err),
		)
		// Continue without sending email
	} else {
		// Send email notification of the successful renewal
		if err = w.emailSender.SendRenewalConfirmationEmail(
			ctx,
			user.Email,
			user.Name,
			renewedSubscription,
		); err != nil {
			slog.ErrorContext(ctx, "Failed to send renewal confirmation email",
				slog.String("subscription_id", payload.SubscriptionID),
				slog.String("user_id", user.ID.Hex()),
				slog.Any("error", err),
			)
			// Continue execution even if email fails
		}
	}

	return nil
}

func (w *ReminderWorker) handleSubscriptionExpiration(ctx context.Context, task *asynq.Task) error {
	var payload ExpirationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal expiration task payload: %v", err)
	}

	slog.DebugContext(ctx, "Processing subscription expiration",
		slog.String("subscription_id", payload.SubscriptionID),
	)

	// Parse the subscription ID
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID: %v", err)
	}

	// Fetch the subscription from the database
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %v", err)
	}

	// Ensure the subscription is canceled and past validity period
	if subscription.Status != models.Canceled {
		slog.DebugContext(ctx, "Skipping expiration for non-canceled subscription",
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("status", string(subscription.Status)),
		)
		return nil
	}

	// Double-check that the subscription is past its validity date
	now := time.Now()
	if subscription.ValidTill.After(now) {
		slog.DebugContext(ctx, "Skipping expiration: subscription still valid",
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("valid_till", subscription.ValidTill.Format(time.RFC3339)),
		)
		return nil
	}

	// Update the subscription status to Expired
	if err := w.subscriptionService.MarkCanceledSubscriptionAsExpiredInternal(ctx, subscriptionID); err != nil {
		return fmt.Errorf("failed to mark subscription as expired: %v", err)
	}

	slog.InfoContext(ctx, "Subscription expired",
		slog.String("subscription_id", payload.SubscriptionID),
		slog.String("user_id", subscription.UserID.Hex()),
	)

	return nil
}

// sendReminderNotification handles sending the actual reminder notification.
func (w *ReminderWorker) sendReminderNotification(ctx context.Context, subscription *models.Subscription, daysBefore int) error {
	// Get the user information.
	user, err := w.userService.FetchUserByIDInternal(ctx, subscription.UserID)
	if err != nil {
		return fmt.Errorf("failed to fetch user: %v", err)
	}

	// Send the email notification.
	if err = w.emailSender.SendReminderEmail(
		ctx,
		user.Email,
		user.Name,
		subscription,
		daysBefore,
	); err != nil {
		slog.ErrorContext(ctx, "Failed to send reminder email",
			slog.String("subscription_id", subscription.ID.Hex()),
			slog.String("user_id", subscription.UserID.Hex()),
			slog.Any("error", err),
		)
		return fmt.Errorf("failed to send reminder email: %v", err)
	}

	slog.InfoContext(ctx, "Reminder notification sent",
		slog.String("subscription_id", subscription.ID.Hex()),
		slog.String("user_id", subscription.UserID.Hex()),
		slog.Int("days_before", daysBefore),
	)

	// Store in Redis that the reminder was sent.
	key := fmt.Sprintf("reminder_sent:%s:%d", subscription.ID.Hex(), daysBefore)
	err = w.redisClient.SetEx(ctx, key, "", 24*time.Hour).Err()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to set reminder sent key in Redis",
			slog.String("subscription_id", subscription.ID.Hex()),
			slog.Any("error", err),
		)
	}

	return nil
}

// Stop gracefully shuts down the worker.
func (w *ReminderWorker) Stop() {
	w.server.Shutdown()
}
