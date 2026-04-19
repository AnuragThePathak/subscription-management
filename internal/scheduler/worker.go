package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/core/traceattr"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/notifications"
	"github.com/anuragthepathak/subscription-management/internal/observability"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/otel/trace"
)

// QueueWorker handles processing of background tasks from various queues.
type QueueWorker struct {
	subscriptionService services.SubscriptionServiceInternal
	userService         services.UserServiceInternal
	emailSender         *notifications.EmailSender
	redisClient         *redis.Client
	server              *asynq.Server
	name                string
}

// NewQueueWorker creates a new queue worker.
func NewQueueWorker(
	subscriptionService services.SubscriptionServiceInternal,
	userService services.UserServiceInternal,
	emailSender *notifications.EmailSender,
	redisClient *redis.Client,
	redisConfig *asynq.RedisClientOpt,
	concurrency int,
	queueName string,
	name string,
) *QueueWorker {
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

	return &QueueWorker{
		subscriptionService,
		userService,
		emailSender,
		redisClient,
		server,
		name,
	}
}

// Start begins processing tasks from the queue.
func (w *QueueWorker) Start() error {
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
func (w *QueueWorker) handleSubscriptionReminder(ctx context.Context, task *asynq.Task) error {
	var payload ReminderPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal payload",
			logattr.Error(err),
		)
		return fmt.Errorf("failed to unmarshal task payload: %w", err)
	}

	ctx = observability.EnrichContext(ctx, payload.UserID, payload.SubscriptionID)
	observability.EnrichSpan(ctx)
	trace.SpanFromContext(ctx).SetAttributes(
		traceattr.SchedulerDaysBefore(payload.DaysBefore),
	)

	slog.DebugContext(ctx, "Processing subscription reminder",
		logattr.DaysBefore(payload.DaysBefore),
	)

	// Parse the subscription ID.
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Invalid subscription ID",
		logattr.DaysBefore(payload.DaysBefore),
		logattr.Error(err),
	)
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Fetch the subscription from the database.
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch subscription",
			logattr.DaysBefore(payload.DaysBefore),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	// Ensure the subscription is still active.
	if subscription.Status != models.Active {
		slog.DebugContext(ctx, "Skipping reminder for non-active subscription",
			logattr.Status(string(subscription.Status)),
		)
		return nil
	}

	// Process the reminder (send an email).
	if err := w.sendReminderNotification(ctx, subscription, payload.DaysBefore); err != nil {
		slog.ErrorContext(ctx, "Failed to send reminder notification",
			logattr.DaysBefore(payload.DaysBefore),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to send reminder notification: %w", err)
	}

	return nil
}

// handleSubscriptionRenewal processes an automatic subscription renewal task.
func (w *QueueWorker) handleSubscriptionRenewal(ctx context.Context, task *asynq.Task) error {
	var payload RenewalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal renewal task payload: %w", err)
	}

	ctx = observability.EnrichContext(ctx, payload.UserID, payload.SubscriptionID)
	observability.EnrichSpan(ctx)

	slog.DebugContext(ctx, "Processing subscription renewal")

	// Parse the subscription ID
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Fetch the subscription from the database
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	// Ensure the subscription is still active
	if subscription.Status != models.Active {
		slog.DebugContext(ctx, "Skipping renewal for non-active subscription",
			logattr.Status(string(subscription.Status)),
		)
		return nil
	}

	// Check if the renewal date is within our window (now to next 4 hours)
	now := time.Now()
	renewalWindow := now.Add(time.Hour * RenewalHoursBeforeDay)
	if subscription.ValidTill.After(renewalWindow) {
		slog.DebugContext(ctx, "Skipping renewal: outside valid window",
			logattr.ValidTill(subscription.ValidTill),
		)
		return nil
	}

	// Process the automatic renewal
	renewedSubscription, err := w.subscriptionService.RenewSubscriptionInternal(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to renew subscription: %w", err)
	}

	slog.InfoContext(ctx, "Subscription renewed",
		logattr.ValidTill(renewedSubscription.ValidTill),
	)

	// Send a confirmation email to the user
	user, err := w.userService.FetchUserByIDInternal(ctx, subscription.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch user for renewal notification",
			logattr.Error(err),
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
				logattr.Error(err),
			)
			// Continue execution even if email fails
		}
	}

	return nil
}

func (w *QueueWorker) handleSubscriptionExpiration(ctx context.Context, task *asynq.Task) error {
	var payload ExpirationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal expiration task payload: %w", err)
	}

	ctx = observability.EnrichContext(ctx, payload.UserID, payload.SubscriptionID)
	observability.EnrichSpan(ctx)

	slog.DebugContext(ctx, "Processing subscription expiration")

	// Parse the subscription ID
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Fetch the subscription from the database
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	// Ensure the subscription is canceled and past validity period
	if subscription.Status != models.Canceled {
		slog.DebugContext(ctx, "Skipping expiration for non-canceled subscription",
			logattr.Status(string(subscription.Status)),
		)
		return nil
	}

	// Double-check that the subscription is past its validity date
	now := time.Now()
	if subscription.ValidTill.After(now) {
		slog.DebugContext(ctx, "Skipping expiration: subscription still valid",
			logattr.ValidTill(subscription.ValidTill),
		)
		return nil
	}

	// Update the subscription status to Expired
	if err := w.subscriptionService.MarkCanceledSubscriptionAsExpiredInternal(ctx, subscriptionID); err != nil {
		return fmt.Errorf("failed to mark subscription as expired: %w", err)
	}

	slog.InfoContext(ctx, "Subscription expired")

	return nil
}

// sendReminderNotification handles sending the actual reminder notification.
func (w *QueueWorker) sendReminderNotification(ctx context.Context, subscription *models.Subscription, daysBefore int) error {
	// Get the user information.
	user, err := w.userService.FetchUserByIDInternal(ctx, subscription.UserID)
	if err != nil {
		return fmt.Errorf("failed to fetch user: %w", err)
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
			logattr.Error(err),
		)
		return fmt.Errorf("failed to send reminder email: %w", err)
	}

	slog.InfoContext(ctx, "Reminder notification sent",
		logattr.DaysBefore(daysBefore),
	)

	// Store in Redis that the reminder was sent.
	key := fmt.Sprintf("reminder_sent:%s:%d", subscription.ID.Hex(), daysBefore)
	err = w.redisClient.SetEx(ctx, key, "", 24*time.Hour).Err()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to set reminder sent key in Redis",
			logattr.Error(err),
		)
	}

	return nil
}

// Stop gracefully shuts down the worker.
func (w *QueueWorker) Stop() {
	w.server.Shutdown()
}
