package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/clock"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/core/otelattr"
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
	emailSender         notifications.EmailSender
	redisClient         redis.UniversalClient
	server              *asynq.Server
	queueName           string
	concurrency         int
	name                string
	getTime             clock.NowFn
}

// NewQueueWorker creates a new queue worker.
func NewQueueWorker(
	subscriptionService services.SubscriptionServiceInternal,
	userService services.UserServiceInternal,
	emailSender notifications.EmailSender,
	redisClient redis.UniversalClient,
	redisConfig asynq.RedisConnOpt,
	concurrency int,
	queueName string,
	name string,
	nowFn clock.NowFn,
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
		queueName,
		concurrency,
		name,
		nowFn,
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

	if err := w.server.Start(mux); err != nil {
		return fmt.Errorf("failed to start queue worker: %w", err)
	}
	slog.Info("Queue worker event loop started",
		logattr.WorkerName(w.name),
		logattr.Queue(w.queueName),
		logattr.Concurrency(w.concurrency),
	)
	return nil
}

// handleSubscriptionReminder processes a subscription reminder task.
func (w *QueueWorker) handleSubscriptionReminder(ctx context.Context, task *asynq.Task) error {
	var payload ReminderPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal payload",
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to unmarshal task payload: %w", err)
	}

	ctx = observability.EnrichContext(ctx, payload.UserID, payload.SubscriptionID)
	observability.EnrichSpan(ctx)
	trace.SpanFromContext(ctx).SetAttributes(
		otelattr.DaysBefore(payload.DaysBefore),
	)

	slog.DebugContext(ctx, "Processing subscription reminder",
		logattr.DaysBefore(payload.DaysBefore),
		logattr.Queue(w.queueName),
	)

	// Parse the subscription ID.
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Invalid subscription ID",
			logattr.DaysBefore(payload.DaysBefore),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Fetch the subscription from the database.
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch subscription",
			logattr.DaysBefore(payload.DaysBefore),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	// Ensure the subscription is still active.
	if subscription.Status != models.Active {
		slog.DebugContext(ctx, "Skipping reminder for non-active subscription",
			logattr.Status(string(subscription.Status)),
			logattr.Queue(w.queueName),
		)
		return nil
	}

	// Get the user information.
	user, err := w.userService.FetchUserByIDInternal(ctx, subscription.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch user",
			logattr.DaysBefore(payload.DaysBefore),
			logattr.ValidTill(subscription.ValidTill),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to fetch user: %w", err)
	}

	// Send the email notification.
	if err = w.emailSender.SendReminderEmail(
		ctx,
		user.Email,
		user.Name,
		subscription,
		payload.DaysBefore,
	); err != nil {
		slog.ErrorContext(ctx, "Failed to send reminder email",
			logattr.DaysBefore(payload.DaysBefore),
			logattr.ValidTill(subscription.ValidTill),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to send reminder email: %w", err)
	}
	slog.InfoContext(ctx, "Reminder email sent",
		logattr.DaysBefore(payload.DaysBefore),
		logattr.ValidTill(subscription.ValidTill),
		logattr.Queue(w.queueName),
	)

	// Store in Redis that the reminder was sent.
	key := fmt.Sprintf("reminder_sent:%s:%d",
		subscription.ID.Hex(),
		payload.DaysBefore,
	)
	if err = w.redisClient.Set(ctx, key, "", 24*time.Hour).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to set reminder sent key in Redis",
			logattr.DaysBefore(payload.DaysBefore),
			logattr.ValidTill(subscription.ValidTill),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
	}

	return nil
}

// handleSubscriptionRenewal processes an automatic subscription renewal task.
func (w *QueueWorker) handleSubscriptionRenewal(ctx context.Context, task *asynq.Task) error {
	var payload RenewalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal renewal task payload",
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to unmarshal renewal task payload: %w", err)
	}

	ctx = observability.EnrichContext(ctx, payload.UserID, payload.SubscriptionID)
	observability.EnrichSpan(ctx)

	slog.DebugContext(ctx, "Processing subscription renewal",
		logattr.Queue(w.queueName),
	)

	// Parse the subscription ID
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Invalid subscription ID",
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Fetch the subscription from the database
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch subscription",
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	// Ensure the subscription is still active
	if subscription.Status != models.Active {
		slog.DebugContext(ctx, "Skipping renewal for non-active subscription",
			logattr.Status(string(subscription.Status)),
			logattr.Queue(w.queueName),
		)
		return nil
	}

	// Check if the renewal date is within our window (now to next 4 hours)
	now := w.getTime()
	renewalWindow := now.Add(time.Hour * RenewalHoursBeforeDay)
	if subscription.ValidTill.After(renewalWindow) {
		slog.DebugContext(ctx, "Skipping renewal: outside valid window",
			logattr.ValidTill(subscription.ValidTill),
			logattr.Queue(w.queueName),
		)
		return nil
	}

	// Process the automatic renewal
	renewedSubscription, err := w.subscriptionService.RenewSubscriptionInternal(ctx, subscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to renew subscription",
			logattr.ValidTill(subscription.ValidTill),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to renew subscription: %w", err)
	}

	// Send a confirmation email to the user
	user, err := w.userService.FetchUserByIDInternal(ctx, subscription.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch user for renewal notification",
			logattr.ValidTill(renewedSubscription.ValidTill),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		// Continue without sending email
		return nil
	}

	// Send email notification of the successful renewal
	if err = w.emailSender.SendRenewalConfirmationEmail(
		ctx,
		user.Email,
		user.Name,
		renewedSubscription,
	); err != nil {
		slog.ErrorContext(ctx, "Failed to send renewal confirmation email",
			logattr.ValidTill(renewedSubscription.ValidTill),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		// Continue execution even if email fails
	} else {
		slog.InfoContext(ctx, "Renewal confirmation email sent",
			logattr.ValidTill(renewedSubscription.ValidTill),
			logattr.Queue(w.queueName),
		)
	}

	return nil
}

func (w *QueueWorker) handleSubscriptionExpiration(ctx context.Context, task *asynq.Task) error {
	var payload ExpirationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal expiration task payload",
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to unmarshal expiration task payload: %w", err)
	}

	ctx = observability.EnrichContext(ctx, payload.UserID, payload.SubscriptionID)
	observability.EnrichSpan(ctx)

	slog.DebugContext(ctx, "Processing subscription expiration",
		logattr.Queue(w.queueName),
	)

	// Parse the subscription ID
	subscriptionID, err := bson.ObjectIDFromHex(payload.SubscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Invalid subscription ID",
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Fetch the subscription from the database
	subscription, err := w.subscriptionService.FetchSubscriptionByIDInternal(ctx, subscriptionID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch subscription",
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	// Ensure the subscription is canceled and past validity period
	if subscription.Status != models.Canceled {
		slog.DebugContext(ctx, "Skipping expiration for non-canceled subscription",
			logattr.Status(string(subscription.Status)),
			logattr.Queue(w.queueName),
		)
		return nil
	}

	// Double-check that the subscription is past its validity date
	now := w.getTime()
	if subscription.ValidTill.After(now) {
		slog.DebugContext(ctx, "Skipping expiration: subscription still valid",
			logattr.ValidTill(subscription.ValidTill),
			logattr.Queue(w.queueName),
		)
		return nil
	}

	// Update the subscription status to Expired
	if err := w.subscriptionService.MarkCanceledSubscriptionAsExpiredInternal(ctx, subscriptionID); err != nil {
		slog.ErrorContext(ctx, "Failed to mark subscription as expired",
			logattr.ValidTill(subscription.ValidTill),
			logattr.Queue(w.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to mark subscription as expired: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the worker.
func (w *QueueWorker) Stop() {
	w.server.Shutdown()
}
