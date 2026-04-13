package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/lib"
	"github.com/anuragthepathak/subscription-management/internal/observability"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	// ReminderTask is the task name for subscription reminders.
	ReminderTask = "subscription:reminder"
	// RenewalTask is the task name for automatic subscription renewals.
	RenewalTask = "subscription:renewal"
	// ExpirationTask is the task name for subscription expiration.
	ExpirationTask = "subscription:expiration"
	// RenewalHoursBeforeDay is how many hours before the renewal date to process renewals
	RenewalHoursBeforeDay = 8
)

// ReminderPayload represents the data needed to process a reminder.
type ReminderPayload struct {
	SubscriptionID string `json:"subscription_id"`
	DaysBefore     int    `json:"days_before"`
	RenewalDate    string `json:"renewal_date"`
}

// RenewalPayload represents the data needed to process an automatic renewal.
type RenewalPayload struct {
	SubscriptionID string `json:"subscription_id"`
	RenewalDate    string `json:"renewal_date"`
}

// ExpirationPayload represents the data needed to process a subscription expiration.
type ExpirationPayload struct {
	SubscriptionID string `json:"subscription_id"`
	ValidTill      string `json:"valid_till"`
}

// SubscriptionScheduler handles scheduling of subscription-related tasks.
type SubscriptionScheduler struct {
	subscriptionService services.SubscriptionServiceInternal
	redisClient         *redis.Client
	client              *asynq.Client
	interval            time.Duration
	reminderDays        []int
	tracer              trace.Tracer
}

// NewSubscriptionScheduler creates a new subscription scheduler.
func NewSubscriptionScheduler(
	subscriptionService services.SubscriptionServiceInternal,
	redisClient *redis.Client,
	redisConfig *asynq.RedisClientOpt,
	interval time.Duration,
	reminderDays []int,
	name string,
) *SubscriptionScheduler {
	client := asynq.NewClient(redisConfig)
	return &SubscriptionScheduler{
		subscriptionService: subscriptionService,
		redisClient:         redisClient,
		client:              client,
		interval:            interval,
		reminderDays:        reminderDays,
		tracer:              otel.Tracer(name),
	}
}

// Start begins the scheduler loop.
func (s *SubscriptionScheduler) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run once immediately.
	if err := s.pollSubscriptions(ctx); err != nil {
		slog.Warn("Initial subscription poll failed (will retry on next tick)",
			slog.Any("error", err),
		)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.pollSubscriptions(ctx); err != nil {
				slog.Error("Subscription poll failed",
					slog.Any("error", err),
				)
			}
		}
	}
}

// pollSubscriptions checks for subscriptions needing reminders and renewals, then schedules tasks.
func (s *SubscriptionScheduler) pollSubscriptions(ctx context.Context) error {
	// Start a trace span for this entire scheduler tick execution
	ctx, span := s.tracer.Start(ctx, "Scheduler Tick: Poll Subscriptions",
		trace.WithSpanKind(trace.SpanKindProducer), // Marks this span as a job producer
		trace.WithAttributes(
			attribute.String("scheduler.interval", s.interval.String()),
			attribute.IntSlice("scheduler.reminder_days", s.reminderDays),
		),
	)
	defer span.End()

	slog.InfoContext(ctx, "Polling subscriptions")

	var errs []error

	// Handle reminder tasks
	if err := s.handleReminderTasks(ctx); err != nil {
		slog.ErrorContext(ctx, "Reminder tasks failed",
			slog.Any("error", err),
		)
		errs = append(errs, err)
	}

	// Handle renewal tasks
	if err := s.handleRenewalTasks(ctx); err != nil {
		slog.ErrorContext(ctx, "Renewal tasks failed",
			slog.Any("error", err),
		)
		errs = append(errs, err)
	}

	// Handle expiration tasks
	if err := s.handleExpirationTasks(ctx); err != nil {
		slog.ErrorContext(ctx, "Expiration tasks failed",
			slog.Any("error", err),
		)
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// handleReminderTasks checks for subscriptions needing reminders and schedules tasks.
func (s *SubscriptionScheduler) handleReminderTasks(ctx context.Context) error {
	activeSubscriptions, err := s.getSubscriptionsDueForReminder(ctx)
	if err != nil {
		return err
	}

	scheduled := 0
	// Check each subscription for upcoming renewal dates.
	for _, subscription := range activeSubscriptions {
		daysBefore := lib.DaysBetween(time.Now(), subscription.ValidTill, nil)
		redisKey := fmt.Sprintf("reminder_sent:%s:%d", subscription.ID.Hex(), daysBefore)
		exists, err := s.redisClient.Exists(ctx, redisKey).Result()
		if err != nil {
			slog.ErrorContext(ctx, "Failed to check Redis for sent reminder",
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.Any("error", err),
			)
			continue
		}

		if exists == 0 { // Key does not exist, reminder not sent recently.
			if err := s.scheduleReminderTask(ctx, subscription, daysBefore); err != nil {
				slog.ErrorContext(ctx, "Failed to schedule reminder task",
					slog.String("subscription_id", subscription.ID.Hex()),
					slog.Any("error", err),
				)
			} else {
				scheduled++
			}
		}
	}

	if scheduled > 0 {
		slog.InfoContext(ctx, "Reminder tasks scheduled",
			slog.Int("count", scheduled),
		)
	}

	return nil
}

// handleRenewalTasks checks for subscriptions needing automatic renewal and schedules tasks.
func (s *SubscriptionScheduler) handleRenewalTasks(ctx context.Context) error {
	renewalSubscriptions, err := s.getSubscriptionsDueForRenewal(ctx)
	if err != nil {
		return err
	}

	scheduled := 0
	for _, subscription := range renewalSubscriptions {
		if err := s.scheduleRenewalTask(ctx, subscription); err != nil {
			slog.ErrorContext(ctx, "Failed to schedule renewal task",
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.Any("error", err),
			)
		} else {
			scheduled++
		}
	}

	if scheduled > 0 {
		slog.InfoContext(ctx, "Renewal tasks scheduled",
			slog.Int("count", scheduled),
		)
	}

	return nil
}

// handleExpirationTasks checks for subscriptions that are expired and schedules tasks.
func (s *SubscriptionScheduler) handleExpirationTasks(ctx context.Context) error {
	expiringSubscriptions, err := s.getSubscriptionsDueForExpiration(ctx)
	if err != nil {
		return err
	}

	scheduled := 0
	for _, subscription := range expiringSubscriptions {
		if err := s.scheduleExpirationTask(ctx, subscription); err != nil {
			slog.ErrorContext(ctx, "Failed to schedule expiration task",
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.Any("error", err),
			)
		} else {
			scheduled++
		}
	}

	if scheduled > 0 {
		slog.InfoContext(ctx, "Expiration tasks scheduled",
			slog.Int("count", scheduled),
		)
	}

	return nil
}

// getSubscriptionsDueForReminder retrieves subscriptions that are due for reminders.
func (s *SubscriptionScheduler) getSubscriptionsDueForReminder(ctx context.Context) ([]*models.Subscription, error) {
	return s.subscriptionService.FetchUpcomingRenewalsInternal(ctx, s.reminderDays)
}

// getSubscriptionsDueForRenewal retrieves subscriptions that are due for automatic renewal.
func (s *SubscriptionScheduler) getSubscriptionsDueForRenewal(ctx context.Context) ([]*models.Subscription, error) {
	// Calculate time range: now to RenewalHoursBeforeDay hours ahead
	now := time.Now()
	renewalWindowStart := now.Add(time.Hour)
	renewalWindowEnd := now.Add(time.Hour * RenewalHoursBeforeDay)

	return s.subscriptionService.FetchSubscriptionsDueForRenewalInternal(ctx, renewalWindowStart, renewalWindowEnd)
}

// New method to get subscriptions that need to be marked as expired
func (s *SubscriptionScheduler) getSubscriptionsDueForExpiration(ctx context.Context) ([]*models.Subscription, error) {
	// Get canceled subscriptions that are past their validity period but not marked as expired yet
	return s.subscriptionService.FetchCanceledExpiredSubscriptionsInternal(ctx)
}

// scheduleReminderTask creates and enqueues a reminder task.
func (s *SubscriptionScheduler) scheduleReminderTask(ctx context.Context, subscription *models.Subscription, daysBefore int) error {
	payload := ReminderPayload{
		SubscriptionID: subscription.ID.Hex(),
		DaysBefore:     daysBefore,
		RenewalDate:    subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	headers := observability.InjectIntoTaskHeaders(ctx)
	task := asynq.NewTaskWithHeaders(ReminderTask, payloadBytes, headers)

	info, err := s.client.Enqueue(
		task,
		asynq.Unique(24*time.Hour),    // Prevent duplicate pending tasks.
		asynq.Retention(24*time.Hour), // Keep task for 24h after processing.
		asynq.Timeout(45*time.Second), // Handler must finish in 45s.
		asynq.MaxRetry(3),             // Retry up to 3 times if failed.
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.DebugContext(ctx, "Reminder task enqueued",
		slog.String("task_id", info.ID),
		slog.String("subscription_id", subscription.ID.Hex()),
		slog.Int("days_before", daysBefore),
	)

	return nil
}

// scheduleRenewalTask creates and enqueues a renewal task.
func (s *SubscriptionScheduler) scheduleRenewalTask(ctx context.Context, subscription *models.Subscription) error {
	payload := RenewalPayload{
		SubscriptionID: subscription.ID.Hex(),
		RenewalDate:    subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	headers := observability.InjectIntoTaskHeaders(ctx)
	task := asynq.NewTaskWithHeaders(RenewalTask, payloadBytes, headers)

	// Calculate when the task should be processed - 4 hours before the renewal date
	processAt := subscription.ValidTill.Add(-time.Hour * RenewalHoursBeforeDay)

	// If the process time is in the past (very close to renewal), process immediately
	if processAt.Before(time.Now()) {
		processAt = time.Now()
	}

	info, err := s.client.Enqueue(
		task,
		asynq.Unique(24*time.Hour),    // Prevent duplicate pending tasks.
		asynq.Retention(24*time.Hour), // Keep task for 24h after processing.
		asynq.Timeout(45*time.Second), // Handler must finish in 45s.
		asynq.MaxRetry(5),             // Retry up to 5 times if failed.
		asynq.ProcessAt(processAt),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.DebugContext(ctx, "Renewal task enqueued",
		slog.String("task_id", info.ID),
		slog.String("subscription_id", subscription.ID.Hex()),
		slog.String("process_at", processAt.Format(time.RFC3339)),
	)

	return nil
}

// New method to schedule expiration task
func (s *SubscriptionScheduler) scheduleExpirationTask(ctx context.Context, subscription *models.Subscription) error {
	payload := ExpirationPayload{
		SubscriptionID: subscription.ID.Hex(),
		ValidTill:      subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	headers := observability.InjectIntoTaskHeaders(ctx)
	task := asynq.NewTaskWithHeaders(ExpirationTask, payloadBytes, headers)

	// Schedule task for immediate processing
	info, err := s.client.Enqueue(
		task,
		asynq.Unique(24*time.Hour),    // Prevent duplicate pending tasks
		asynq.Retention(24*time.Hour), // Keep task for 24h after processing
		asynq.Timeout(30*time.Second), // Handler must finish in 30s
		asynq.MaxRetry(3),             // Retry up to 3 times if failed
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.DebugContext(ctx, "Expiration task enqueued",
		slog.String("task_id", info.ID),
		slog.String("subscription_id", subscription.ID.Hex()),
	)

	return nil
}

// Close cleanly shuts down the scheduler.
func (s *SubscriptionScheduler) Close() error {
	return s.client.Close()
}
