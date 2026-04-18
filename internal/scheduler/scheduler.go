package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/core/traceattr"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/lib"
	"github.com/anuragthepathak/subscription-management/internal/observability"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
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
	UserID         string `json:"user_id"`
	DaysBefore     int    `json:"days_before"`
	RenewalDate    string `json:"renewal_date"`
}

// RenewalPayload represents the data needed to process an automatic renewal.
type RenewalPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	RenewalDate    string `json:"renewal_date"`
}

// ExpirationPayload represents the data needed to process a subscription expiration.
type ExpirationPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	ValidTill      string `json:"valid_till"`
}

// SubscriptionScheduler handles scheduling of subscription-related tasks.
type SubscriptionScheduler struct {
	subscriptionService services.SubscriptionServiceInternal
	redisClient         *redis.Client
	client              *asynq.Client
	interval            time.Duration
	reminderDays        []int
	startupDelay        time.Duration
	queueName           string
	tracer              trace.Tracer
}

// NewSubscriptionScheduler creates a new subscription scheduler.
func NewSubscriptionScheduler(
	subscriptionService services.SubscriptionServiceInternal,
	redisClient *redis.Client,
	redisConfig *asynq.RedisClientOpt,
	interval time.Duration,
	reminderDays []int,
	startupDelay time.Duration,
	queueName string,
	name string,
) *SubscriptionScheduler {
	client := asynq.NewClient(redisConfig)
	return &SubscriptionScheduler{
		subscriptionService: subscriptionService,
		redisClient:         redisClient,
		client:              client,
		interval:            interval,
		reminderDays:        reminderDays,
		startupDelay:        startupDelay,
		queueName:           queueName,
		tracer:              otel.Tracer(name),
	}
}

// Start begins the scheduler loop.
func (s *SubscriptionScheduler) Start(ctx context.Context) error {
	delayTimer := time.NewTimer(s.startupDelay)
	select {
	case <-ctx.Done():
		delayTimer.Stop() // Clean up the timer to prevent memory leaks
		return ctx.Err()
	case <-delayTimer.C:
		_ = s.pollSubscriptions(ctx)
	}
	delayTimer.Stop()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.pollSubscriptions(ctx); err != nil {
				slog.Error("Failed to fetch active subscriptions", logattr.Error(err))
			}
		}
	}
}

// pollSubscriptions checks for subscriptions needing reminders and renewals, then schedules tasks.
func (s *SubscriptionScheduler) pollSubscriptions(ctx context.Context) error {
	// Start a trace span for this entire scheduler tick execution
	ctx, span := s.tracer.Start(ctx, "Scheduler Tick: Poll Subscriptions")
	defer span.End()

	slog.InfoContext(ctx, "Polling subscriptions")

	var errs []error

	// Handle reminder tasks
	if err := s.handleReminderTasks(ctx); err != nil {
		slog.ErrorContext(ctx, "Reminder tasks failed",
			logattr.Error(err),
		)
		errs = append(errs, err)
	}

	// Handle renewal tasks
	if err := s.handleRenewalTasks(ctx); err != nil {
		slog.ErrorContext(ctx, "Renewal tasks failed",
			logattr.Error(err),
		)
		errs = append(errs, err)
	}

	// Handle expiration tasks
	if err := s.handleExpirationTasks(ctx); err != nil {
		slog.ErrorContext(ctx, "Expiration tasks failed",
			logattr.Error(err),
		)
		errs = append(errs, err)
	}

	finalErr := errors.Join(errs...)
	if finalErr != nil {
		span.RecordError(finalErr)
		span.SetStatus(codes.Error, finalErr.Error())

		slog.ErrorContext(ctx, "Poll subscriptions completed with partial failures",
			slog.Int("failure_count", len(errs)),
			logattr.Error(finalErr),
		)
	}
	return finalErr
}

// handleReminderTasks checks for subscriptions needing reminders and schedules tasks.
func (s *SubscriptionScheduler) handleReminderTasks(ctx context.Context) error {
	ctx = appctx.WithTaskType(ctx, ReminderTask)
	ctx, span := s.tracer.Start(ctx, "Phase: Reminder Tasks",
		trace.WithAttributes(
			traceattr.TaskType(ReminderTask),
		),
	)
	defer span.End()

	activeSubscriptions, err := s.getSubscriptionsDueForReminder(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	scheduled := 0
	// Check each subscription for upcoming renewal dates.
	for _, subscription := range activeSubscriptions {
		sCtx := observability.EnrichContext(
			ctx, subscription.UserID.Hex(),
			subscription.ID.Hex(),
		)

		daysBefore := lib.DaysBetween(time.Now(), subscription.ValidTill, nil)
		redisKey := fmt.Sprintf("reminder_sent:%s:%d", subscription.ID.Hex(), daysBefore)
		exists, err := s.redisClient.Exists(sCtx, redisKey).Result()
		if err != nil {
			span.RecordError(err)
			slog.ErrorContext(sCtx, "Failed to check Redis for sent reminder",
				logattr.DaysBefore(daysBefore),
				logattr.Error(err),
			)
			continue
		}

		if exists == 0 { // Key does not exist, reminder not sent recently.
			if err := s.scheduleReminderTask(sCtx, subscription, daysBefore); err != nil {
				span.RecordError(err)
				slog.ErrorContext(sCtx, "Failed to schedule reminder task",
					logattr.SubscriptionID(subscription.ID.Hex()),
					logattr.DaysBefore(daysBefore),
					logattr.Error(err),
				)
			} else {
				scheduled++
			}
		}
	}

	span.SetAttributes(traceattr.TasksScheduled(scheduled))
	if scheduled > 0 {
		slog.InfoContext(ctx, "Reminder tasks scheduled",
			logattr.Count(scheduled),
		)
	}

	return nil
}

// handleRenewalTasks checks for subscriptions needing automatic renewal and schedules tasks.
func (s *SubscriptionScheduler) handleRenewalTasks(ctx context.Context) error {
	ctx = appctx.WithTaskType(ctx, RenewalTask)
	ctx, span := s.tracer.Start(ctx, "Phase: Renewal Tasks",
		trace.WithAttributes(
			traceattr.TaskType(RenewalTask),
		),
	)
	defer span.End()

	renewalSubscriptions, err := s.getSubscriptionsDueForRenewal(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	scheduled := 0
	for _, subscription := range renewalSubscriptions {
		sCtx := observability.EnrichContext(
			ctx, subscription.UserID.Hex(),
			subscription.ID.Hex(),
		)

		if err := s.scheduleRenewalTask(sCtx, subscription); err != nil {
			span.RecordError(err)
			slog.ErrorContext(sCtx, "Failed to schedule renewal task",
				logattr.Error(err),
			)
		} else {
			scheduled++
		}
	}

	span.SetAttributes(traceattr.TasksScheduled(scheduled))
	if scheduled > 0 {
		slog.InfoContext(ctx, "Renewal tasks scheduled",
			logattr.Count(scheduled),
		)
	}

	return nil
}

// handleExpirationTasks checks for subscriptions that are expired and schedules tasks.
func (s *SubscriptionScheduler) handleExpirationTasks(ctx context.Context) error {
	ctx = appctx.WithTaskType(ctx, ExpirationTask)
	ctx, span := s.tracer.Start(ctx, "Phase: Expiration Tasks",
		trace.WithAttributes(
			traceattr.TaskType(ExpirationTask),
		),
	)
	defer span.End()

	expiringSubscriptions, err := s.getSubscriptionsDueForExpiration(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	scheduled := 0
	for _, subscription := range expiringSubscriptions {
		sCtx := observability.EnrichContext(
			ctx, subscription.UserID.Hex(),
			subscription.ID.Hex(),
		)
		if err := s.scheduleExpirationTask(sCtx, subscription); err != nil {
			span.RecordError(err)
			slog.ErrorContext(sCtx, "Failed to schedule expiration task",
				logattr.Error(err),
			)
		} else {
			scheduled++
		}
	}

	span.SetAttributes(traceattr.TasksScheduled(scheduled))
	if scheduled > 0 {
		slog.InfoContext(ctx, "Expiration tasks scheduled",
			logattr.Count(scheduled),
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
	// Create a dedicated child span for the network boundary
	ctx, span := s.tracer.Start(ctx, "Enqueue Reminder Task",
		observability.AsynqProducerAttributes(ReminderTask)...,
	)
	defer span.End()
	observability.EnrichSpan(ctx)
	span.SetAttributes(
		traceattr.SchedulerDaysBefore(daysBefore),
	)

	payload := ReminderPayload{
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID.Hex(),
		DaysBefore:     daysBefore,
		RenewalDate:    subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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
		asynq.Queue(s.queueName),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to enqueue task: %w", err)
	}
	span.SetAttributes(traceattr.TaskID(info.ID))

	slog.DebugContext(ctx, "Reminder task enqueued",
		logattr.TaskID(info.ID),
		slog.Int("days_before", daysBefore),
		slog.String("queue", s.queueName),
	)

	return nil
}

// scheduleRenewalTask creates and enqueues a renewal task.
func (s *SubscriptionScheduler) scheduleRenewalTask(ctx context.Context, subscription *models.Subscription) error {
	// Create a dedicated child span for the network boundary
	ctx, span := s.tracer.Start(ctx, "Enqueue Renewal Task",
		observability.AsynqProducerAttributes(RenewalTask)...,
	)
	defer span.End()
	observability.EnrichSpan(ctx)

	payload := RenewalPayload{
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID.Hex(),
		RenewalDate:    subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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
	span.SetAttributes(traceattr.ProcessAt(processAt))

	info, err := s.client.Enqueue(
		task,
		asynq.Unique(24*time.Hour),    // Prevent duplicate pending tasks.
		asynq.Retention(24*time.Hour), // Keep task for 24h after processing.
		asynq.Timeout(45*time.Second), // Handler must finish in 45s.
		asynq.MaxRetry(5),             // Retry up to 5 times if failed.
		asynq.ProcessAt(processAt),
		asynq.Queue(s.queueName),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to enqueue task: %w", err)
	}
	span.SetAttributes(traceattr.TaskID(info.ID))

	slog.DebugContext(ctx, "Renewal task enqueued",
		logattr.TaskID(info.ID),
		logattr.ProcessAt(processAt),
		slog.String("queue", s.queueName),
	)

	return nil
}

// New method to schedule expiration task
func (s *SubscriptionScheduler) scheduleExpirationTask(ctx context.Context, subscription *models.Subscription) error {
	// Create a dedicated child span for the network boundary
	ctx, span := s.tracer.Start(ctx, "Enqueue Expiration Task",
		observability.AsynqProducerAttributes(ExpirationTask)...,
	)
	defer span.End()
	observability.EnrichSpan(ctx)

	payload := ExpirationPayload{
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID.Hex(),
		ValidTill:      subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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
		asynq.Queue(s.queueName),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to enqueue task: %w", err)
	}
	span.SetAttributes(traceattr.TaskID(info.ID))

	slog.DebugContext(ctx, "Expiration task enqueued",
		logattr.TaskID(info.ID),
	)

	return nil
}

// Close cleanly shuts down the scheduler.
func (s *SubscriptionScheduler) Close() error {
	return s.client.Close()
}
