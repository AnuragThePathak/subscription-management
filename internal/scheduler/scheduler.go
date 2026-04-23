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
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// ReminderTask is the task name for subscription reminders.
	ReminderTask = "subscription:reminder"
	// RenewalTask is the task name for automatic subscription renewals.
	RenewalTask = "subscription:renewal"
	// ExpirationTask is the task name for subscription expiration.
	ExpirationTask = "subscription:expiration"
	// RenewalHoursBeforeDay is how many hours before the renewal date to process
	// renewals
	RenewalHoursBeforeDay = 8
)

// ReminderPayload represents the data needed to process a reminder.
type ReminderPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	DaysBefore     int    `json:"days_before"`
}

// RenewalPayload represents the data needed to process an automatic renewal.
type RenewalPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
}

// ExpirationPayload represents the data needed to process a subscription expiration.
type ExpirationPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
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
	name                string
	tracer              trace.Tracer
}

// NewSubscriptionScheduler creates and initializes a new SubscriptionScheduler
// with the provided dependencies and configuration.
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
		name:                name,
		tracer:              otel.Tracer(name),
	}
}

// Start begins the scheduler loop.
func (s *SubscriptionScheduler) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Scheduler event loop started",
		logattr.SchedulerName(s.name),
		logattr.Queue(s.queueName),
		logattr.Interval(s.interval),
		logattr.StartupDelay(s.startupDelay),
		logattr.ReminderDays(s.reminderDays),
	)

	delayTimer := time.NewTimer(s.startupDelay)
	select {
	case <-ctx.Done():
		delayTimer.Stop() // Clean up the timer to prevent memory leaks
		return ctx.Err()
	case <-delayTimer.C:
		s.pollSubscriptions(ctx)
	}
	delayTimer.Stop()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.pollSubscriptions(ctx)
		}
	}
}

// pollSubscriptions checks for subscriptions needing reminders, renewals, or
// expirations, and schedules their respective tasks.
func (s *SubscriptionScheduler) pollSubscriptions(ctx context.Context) {
	// Start a trace span for this entire scheduler tick execution
	ctx, span := s.tracer.Start(ctx, "Scheduler Tick: Poll Subscriptions",
		trace.WithAttributes(
			traceattr.Queue(s.queueName),
		),
	)
	defer span.End()

	slog.InfoContext(ctx, "Polling subscriptions",
		logattr.Queue(s.queueName),
		logattr.Interval(s.interval),
	)

	var errs []error

	// Handle reminder tasks
	if err := s.handleReminderTasks(ctx); err != nil {
		errs = append(errs, err)
	}

	// Handle renewal tasks
	if err := s.handleRenewalTasks(ctx); err != nil {
		errs = append(errs, err)
	}

	// Handle expiration tasks
	if err := s.handleExpirationTasks(ctx); err != nil {
		errs = append(errs, err)
	}

	finalErr := errors.Join(errs...)
	if finalErr != nil {
		span.RecordError(finalErr)
		span.SetStatus(codes.Error, "Poll subscriptions completed with partial failures")

		slog.ErrorContext(ctx, "Poll subscriptions completed with partial failures",
			logattr.Failed(len(errs)),
			logattr.Queue(s.queueName),
			logattr.Error(finalErr),
		)
	}
}

// handleReminderTasks checks for subscriptions needing reminders and schedules
// tasks.
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
		span.SetStatus(codes.Error, "Failed to get subscriptions due for reminder")

		slog.ErrorContext(ctx, "Failed to get subscriptions due for reminder",
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to get subscriptions due for reminder: %w", err)
	}

	scheduled := 0
	failed := 0
	// Check each subscription for upcoming renewal dates.
	for _, subscription := range activeSubscriptions {
		if enqued, err := s.processReminderTask(ctx, subscription); err != nil {
			failed++
		} else if enqued {
			scheduled++
		}
	}

	total := scheduled + failed
	if total > 0 && failed == total {
		err := errors.New("100% reminder task enqueue failure rate detected")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Catastrophic reminder task enqueue failure")

		slog.ErrorContext(ctx, "All reminder tasks failed to enqueue",
			logattr.Total(total),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		// Return to pollSubscriptions so the roll-up log knows the Phase died
		return err
	}

	if scheduled > 0 {
		slog.InfoContext(ctx, "Reminder tasks scheduled",
			logattr.Total(total),
			logattr.Success(scheduled),
			logattr.Failed(failed),
			logattr.Queue(s.queueName),
		)
	}

	return nil
}

// getSubscriptionsDueForReminder retrieves subscriptions that are due for reminders.
func (s *SubscriptionScheduler) getSubscriptionsDueForReminder(ctx context.Context) ([]*models.Subscription, error) {
	return s.subscriptionService.FetchUpcomingRenewalsInternal(ctx, s.reminderDays)
}

// processReminderTask evaluates if a reminder should be sent for a subscription
// and enqueues the task if necessary. It returns true if a task was successfully
// enqueued, and false otherwise (e.g., if already sent or an error occurred).
func (s *SubscriptionScheduler) processReminderTask(
	ctx context.Context, subscription *models.Subscription,
) (bool, error) {
	ctx, span := s.tracer.Start(ctx, "Process Reminder Task",
		trace.WithAttributes(
			traceattr.TaskType(ReminderTask),
		),
	)
	defer span.End()
	ctx = observability.EnrichContext(ctx, subscription.UserID.Hex(), subscription.ID.Hex())
	observability.EnrichSpan(ctx)

	daysBefore := lib.DaysBetween(time.Now(), subscription.ValidTill, nil)
	span.SetAttributes(traceattr.DaysBefore(daysBefore))

	redisKey := fmt.Sprintf("reminder_sent:%s:%d", subscription.ID.Hex(), daysBefore)
	exists, err := s.redisClient.Exists(ctx, redisKey).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to check Redis for sent reminder")

		slog.ErrorContext(ctx, "Failed to check Redis for sent reminder",
			logattr.DaysBefore(daysBefore),
			logattr.RenewalDate(subscription.ValidTill),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return false, fmt.Errorf("failed to check Redis for sent reminder: %w", err)
	}
	if exists > 0 {
		span.SetStatus(codes.Ok, "Reminder already sent")

		slog.DebugContext(ctx, "Reminder already sent",
			logattr.DaysBefore(daysBefore),
			logattr.RenewalDate(subscription.ValidTill),
			logattr.Queue(s.queueName),
		)
		return false, nil
	}

	taskID, err := s.scheduleReminderTask(ctx, subscription, daysBefore)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to schedule reminder task")

		slog.ErrorContext(ctx, "Failed to schedule reminder task",
			logattr.DaysBefore(daysBefore),
			logattr.RenewalDate(subscription.ValidTill),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return false, fmt.Errorf("failed to schedule reminder task: %w", err)
	}
	slog.DebugContext(ctx, "Reminder task enqueued successfully",
		logattr.TaskID(taskID),
		logattr.DaysBefore(daysBefore),
		logattr.RenewalDate(subscription.ValidTill),
		logattr.Queue(s.queueName),
	)
	return true, nil
}

// scheduleReminderTask creates and enqueues a reminder task.
func (s *SubscriptionScheduler) scheduleReminderTask(ctx context.Context, subscription *models.Subscription, daysBefore int) (string, error) {
	// Create a dedicated child span for the network boundary
	ctx, span := s.tracer.Start(ctx, "Enqueue Reminder Task",
		observability.AsynqProducerAttributes(ReminderTask, s.queueName)...,
	)
	defer span.End()

	payload := ReminderPayload{
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID.Hex(),
		DaysBefore:     daysBefore,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal reminder task payload")
		return "", fmt.Errorf("failed to marshal reminder payload: %w", err)
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
		span.SetStatus(codes.Error, "Failed to enqueue reminder task")
		return "", fmt.Errorf("failed to enqueue reminder task: %w", err)
	}
	span.SetAttributes(semconv.MessagingMessageID(info.ID))

	return info.ID, nil
}

// handleRenewalTasks checks for subscriptions needing automatic renewal and
// schedules tasks.
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
		span.SetStatus(codes.Error, "Failed to get subscriptions due for renewal")

		slog.ErrorContext(ctx, "Failed to get subscriptions due for renewal",
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to get subscriptions due for renewal: %w", err)
	}

	scheduled := 0
	failed := 0
	for _, subscription := range renewalSubscriptions {
		if _, err := s.scheduleRenewalTask(ctx, subscription); err != nil {
			failed++
		} else {
			scheduled++
		}
	}

	total := scheduled + failed
	if total > 0 && failed == total {
		err := errors.New("100% renewal task enqueue failure rate detected")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Catastrophic renewal task enqueue failure")

		slog.ErrorContext(ctx, "All renewal tasks failed to enqueue",
			logattr.Total(total),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		// Return to pollSubscriptions so the roll-up log knows the Phase died
		return err
	}

	if scheduled > 0 {
		slog.InfoContext(ctx, "Renewal tasks scheduled",
			logattr.Total(total),
			logattr.Success(scheduled),
			logattr.Failed(failed),
			logattr.Queue(s.queueName),
		)
	}

	return nil
}

// getSubscriptionsDueForRenewal retrieves subscriptions that are due for
// automatic renewal.
func (s *SubscriptionScheduler) getSubscriptionsDueForRenewal(ctx context.Context) ([]*models.Subscription, error) {
	// Calculate time range: now to RenewalHoursBeforeDay hours ahead
	now := time.Now()
	renewalWindowStart := now.Add(time.Hour)
	renewalWindowEnd := now.Add(time.Hour * RenewalHoursBeforeDay)

	return s.subscriptionService.FetchSubscriptionsDueForRenewalInternal(ctx, renewalWindowStart, renewalWindowEnd)
}

// scheduleRenewalTask creates and enqueues a renewal task.
func (s *SubscriptionScheduler) scheduleRenewalTask(ctx context.Context, subscription *models.Subscription) (string, error) {
	// Create a dedicated child span for the network boundary
	ctx, span := s.tracer.Start(ctx, "Enqueue Renewal Task",
		observability.AsynqProducerAttributes(RenewalTask, s.queueName)...,
	)
	defer span.End()
	ctx = observability.EnrichContext(ctx, subscription.UserID.Hex(), subscription.ID.Hex())
	observability.EnrichSpan(ctx)

	payload := RenewalPayload{
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID.Hex(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal renewal payload")
		slog.ErrorContext(ctx, "Failed to marshal renewal payload",
			logattr.RenewalDate(subscription.ValidTill),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return "", fmt.Errorf("failed to marshal renewal payload: %w", err)
	}

	headers := observability.InjectIntoTaskHeaders(ctx)
	task := asynq.NewTaskWithHeaders(RenewalTask, payloadBytes, headers)

	// Calculate when the task should be processed - RenewalHoursBeforeDay hours
	// before the renewal date.
	processAt := subscription.ValidTill.Add(-time.Hour * RenewalHoursBeforeDay)
	// If the process time is in the past (very close to renewal), process
	// immediately
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
		span.SetStatus(codes.Error, "Failed to enqueue renewal task")

		slog.ErrorContext(ctx, "Failed to enqueue renewal task",
			logattr.ProcessAt(processAt),
			logattr.RenewalDate(subscription.ValidTill),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return "", fmt.Errorf("failed to enqueue renewal task: %w", err)
	}
	span.SetAttributes(semconv.MessagingMessageID(info.ID))

	slog.DebugContext(ctx, "Renewal task enqueued",
		logattr.TaskID(info.ID),
		logattr.ProcessAt(processAt),
		logattr.Queue(s.queueName),
	)

	return info.ID, nil
}

// handleExpirationTasks checks for subscriptions that are expired and
// schedules tasks.
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
		span.SetStatus(codes.Error, "Failed to get subscriptions due for expiration")

		slog.ErrorContext(ctx, "Failed to get subscriptions due for expiration",
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return fmt.Errorf("failed to get subscriptions due for expiration: %w", err)
	}

	scheduled := 0
	failed := 0
	for _, subscription := range expiringSubscriptions {
		// We receive the error purely for control flow. Telemetry is handled by the child.
		if _, err := s.scheduleExpirationTask(ctx, subscription); err != nil {
			failed++
		} else {
			scheduled++
		}
	}

	// The 100% Failure Catch (Catastrophic Infrastructure Failure)
	totalAttempted := scheduled + failed
	if totalAttempted > 0 && failed == totalAttempted {
		err := errors.New("100% expiration task enqueue failure rate detected")
		span.RecordError(err)
		span.SetStatus(codes.Error, "Catastrophic expiration task enqueue failure")

		slog.ErrorContext(ctx, "All expiration tasks failed to enqueue",
			logattr.Total(totalAttempted),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		// Return to pollSubscriptions so the roll-up log knows the Phase died
		return err
	}

	if scheduled > 0 {
		slog.InfoContext(ctx, "Expiration tasks scheduled",
			logattr.Total(totalAttempted),
			logattr.Success(scheduled),
			logattr.Failed(failed),
			logattr.Queue(s.queueName),
		)
	}

	return nil
}

// getSubscriptionsDueForExpiration retrieves subscriptions that have reached
// their validity end date but are not yet marked as expired.
func (s *SubscriptionScheduler) getSubscriptionsDueForExpiration(ctx context.Context) ([]*models.Subscription, error) {
	// Get canceled subscriptions that are past their validity period but not
	// marked as expired yet
	return s.subscriptionService.FetchCanceledExpiredSubscriptionsInternal(ctx)
}

// scheduleExpirationTask creates and enqueues a subscription expiration task.
// NOTE: This function owns its own telemetry. Any returned error is strictly
// for control flow and has already been logged and attached to the trace.
func (s *SubscriptionScheduler) scheduleExpirationTask(ctx context.Context, subscription *models.Subscription) (string, error) {
	// Create a dedicated child span for the network boundary
	ctx, span := s.tracer.Start(ctx, "Enqueue Expiration Task",
		observability.AsynqProducerAttributes(ExpirationTask, s.queueName)...,
	)
	defer span.End()
	ctx = observability.EnrichContext(ctx, subscription.UserID.Hex(), subscription.ID.Hex())
	observability.EnrichSpan(ctx)

	payload := ExpirationPayload{
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID.Hex(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal expiration payload")

		slog.ErrorContext(ctx, "Failed to marshal expiration payload",
			logattr.RenewalDate(subscription.ValidTill),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return "", fmt.Errorf("failed to marshal expiration payload: %w", err)
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
		span.SetStatus(codes.Error, "Failed to enqueue expiration task")

		slog.ErrorContext(ctx, "Failed to enqueue expiration task",
			logattr.RenewalDate(subscription.ValidTill),
			logattr.Queue(s.queueName),
			logattr.Error(err),
		)
		return "", fmt.Errorf("failed to enqueue expiration task: %w", err)
	}
	span.SetAttributes(semconv.MessagingMessageID(info.ID))

	slog.DebugContext(ctx, "Expiration task enqueued",
		logattr.TaskID(info.ID),
		logattr.Queue(s.queueName),
	)

	return info.ID, nil
}

// Close cleanly shuts down the scheduler.
func (s *SubscriptionScheduler) Close() error {
	return s.client.Close()
}
