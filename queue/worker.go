package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/email"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ReminderWorker handles processing of reminder tasks.
type ReminderWorker struct {
	subscriptionService services.SubscriptionServiceInternal
	userService         services.UserServiceInternal
	emailSender         *email.EmailSender
	redisClient         *redis.Client
	server              *asynq.Server
}

// NewReminderWorker creates a new reminder worker.
func NewReminderWorker(
	subscriptionService services.SubscriptionServiceInternal,
	userService services.UserServiceInternal,
	emailSender *email.EmailSender,
	redisClient *redis.Client,
	redisConfig *asynq.RedisClientOpt,
	concurrency int,
) *ReminderWorker {
	// Configure the server with appropriate concurrency.
	server := asynq.NewServer(
		redisConfig,
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"default": 10, // Process reminder tasks with higher priority.
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
	}
}

// Start begins processing tasks from the queue.
func (w *ReminderWorker) Start(ctx context.Context) error {
	// Register task handlers.
	mux := asynq.NewServeMux()
	mux.HandleFunc(ReminderTask, w.handleSubscriptionReminder)
	mux.HandleFunc(RenewalTask, w.handleSubscriptionRenewal)
	mux.HandleFunc(ExpirationTask, w.handleSubscriptionExpiration)

	// Start the worker server.
	slog.Info("Starting reminder worker",
		slog.String("component", "worker"))

	return w.server.Start(mux)
}

// handleSubscriptionReminder processes a subscription reminder task.
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
		slog.Info("Skipping reminder for non-active subscription",
			slog.String("component", "worker"),
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
	
	slog.Info("Processing subscription renewal",
		slog.String("component", "worker"),
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
		slog.Info("Skipping renewal for non-active subscription",
			slog.String("component", "worker"),
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("status", string(subscription.Status)),
		)
		return nil
	}
	
	// Check if the renewal date is within our window (now to next 4 hours)
	now := time.Now()
	renewalWindow := now.Add(time.Hour * RenewalHoursBeforeDay)
	if subscription.ValidTill.After(renewalWindow) {
		slog.Info("Skipping renewal: outside valid window",
			slog.String("component", "worker"),
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("renewal_date", subscription.ValidTill.Format(time.RFC3339)),
		)
		return nil
	}
	
	// Process the automatic renewal
	renewedSubscription, err := w.subscriptionService.RenewSubscriptionInternal(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to renew subscription: %v", err)
	}
	
	slog.Info("Successfully renewed subscription",
		slog.String("component", "worker"),
		slog.String("subscription_id", payload.SubscriptionID),
		slog.String("new_subscription_id", renewedSubscription.ID.Hex()),
		slog.String("new_renewal_date", renewedSubscription.ValidTill.Format(time.RFC3339)),
	)
	
	// Send a confirmation email to the user
	user, err := w.userService.FetchUserByIDInternal(ctx, subscription.UserID)
	if err != nil {
		slog.Error("Failed to fetch user for renewal notification",
			slog.String("component", "worker"),
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
			slog.Error("Failed to send renewal confirmation email",
				slog.String("component", "worker"),
				slog.String("subscription_id", payload.SubscriptionID),
				slog.String("user_email", user.Email),
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
	
	slog.Info("Processing subscription expiration",
		slog.String("component", "worker"),
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
	
	// Ensure the subscription is cancelled and past validity period
	if subscription.Status != models.Cancelled {
		slog.Info("Skipping expiration for non-cancelled subscription",
			slog.String("component", "worker"),
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("status", string(subscription.Status)),
		)
		return nil
	}
	
	// Double-check that the subscription is past its validity date
	now := time.Now()
	if subscription.ValidTill.After(now) {
		slog.Info("Skipping expiration: subscription still valid",
			slog.String("component", "worker"),
			slog.String("subscription_id", payload.SubscriptionID),
			slog.String("valid_till", subscription.ValidTill.Format(time.RFC3339)),
		)
		return nil
	}
	
	// Update the subscription status to Expired
	if err := w.subscriptionService.MarkCancelledSubscriptionAsExpiredInternal(ctx, subscriptionID); err != nil {
		return fmt.Errorf("failed to mark subscription as expired: %v", err)
	}
	
	slog.Info("Successfully marked subscription as expired",
		slog.String("component", "worker"),
		slog.String("subscription_id", payload.SubscriptionID),
		slog.String("previous_status", string(subscription.Status)),
		slog.String("new_status", string(models.Expired)),
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
		slog.Error("Failed to send reminder email",
			slog.String("component", "worker"),
			slog.String("subscription_id", subscription.ID.Hex()),
			slog.String("user_email", user.Email),
			slog.Any("error", err),
		)
		return fmt.Errorf("failed to send reminder email: %v", err)
	}

	slog.Info("Reminder notification sent successfully",
		slog.String("component", "worker"),
		slog.String("subscription_id", subscription.ID.Hex()),
		slog.String("subscription_name", subscription.Name),
		slog.Int("days_before", daysBefore),
		slog.String("user_email", user.Email),
	)

	// Store in Redis that the reminder was sent.
	key := fmt.Sprintf("reminder_sent:%s:%d", subscription.ID.Hex(), daysBefore)
	err = w.redisClient.SetEx(ctx, key, "", 24*time.Hour).Err()
	if err != nil {
		slog.Error("Failed to set reminder sent key in Redis",
			slog.String("component", "worker"),
			slog.String("subscription_id", subscription.ID.Hex()),
			slog.Int("days_before", daysBefore),
			slog.Any("error", err),
		)
	}

	return nil
}

// Stop gracefully shuts down the worker.
func (w *ReminderWorker) Stop() {
	w.server.Shutdown()
}
