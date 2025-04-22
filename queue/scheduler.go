package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/lib"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
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
}

// NewSubscriptionScheduler creates a new subscription scheduler.
func NewSubscriptionScheduler(
	subscriptionService services.SubscriptionServiceInternal,
	redisClient *redis.Client,
	redisConfig *asynq.RedisClientOpt,
	interval time.Duration,
	reminderDays []int,
) *SubscriptionScheduler {
	client := asynq.NewClient(redisConfig)
	return &SubscriptionScheduler{
		subscriptionService: subscriptionService,
		redisClient:         redisClient,
		client:              client,
		interval:            interval,
		reminderDays:        reminderDays,
	}
}

// Start begins the scheduler loop.
func (s *SubscriptionScheduler) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run once immediately.
	if err := s.pollSubscriptions(ctx); err != nil {
		slog.Error("Failed initial subscription polling",
			slog.String("component", "scheduler"),
			slog.Any("error", err),
		)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.pollSubscriptions(ctx); err != nil {
				slog.Error("Failed to poll subscriptions",
					slog.String("component", "scheduler"),
					slog.Any("error", err),
				)
			}
		}
	}
}

// pollSubscriptions checks for subscriptions needing reminders and renewals, then schedules tasks.
func (s *SubscriptionScheduler) pollSubscriptions(ctx context.Context) error {
	slog.Info("Polling for subscriptions requiring reminders and renewals",
		slog.String("component", "scheduler"))

	// Handle reminder tasks
	if err := s.handleReminderTasks(ctx); err != nil {
		slog.Error("Failed to handle reminder tasks",
			slog.String("component", "scheduler"),
			slog.Any("error", err),
		)
	}

	// Handle renewal tasks
	if err := s.handleRenewalTasks(ctx); err != nil {
		slog.Error("Failed to handle renewal tasks",
			slog.String("component", "scheduler"),
			slog.Any("error", err),
		)
	}

	// Handle expiration tasks
	if err := s.handleExpirationTasks(ctx); err != nil {
		slog.Error("Failed to handle expiration tasks",
			slog.String("component", "scheduler"),
			slog.Any("error", err),
		)
	}

	return nil
}

// handleReminderTasks checks for subscriptions needing reminders and schedules tasks.
func (s *SubscriptionScheduler) handleReminderTasks(ctx context.Context) error {
	activeSubscriptions, err := s.getSubscriptionsDueForReminder(ctx)
	if err != nil {
		return err
	}

	// Check each subscription for upcoming renewal dates.
	for _, subscription := range activeSubscriptions {
		daysBefore := lib.DaysBetween(time.Now(), subscription.ValidTill, nil)
		redisKey := fmt.Sprintf("reminder_sent:%s:%d", subscription.ID.Hex(), daysBefore)
		exists, err := s.redisClient.Exists(ctx, redisKey).Result()
		if err != nil {
			slog.Error("Failed to check Redis for sent reminder",
				slog.String("component", "scheduler"),
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.Int("days_before", daysBefore),
				slog.Any("error", err),
			)
			continue
		}

		if exists == 0 { // Key does not exist, reminder not sent recently.
			if err := s.scheduleReminderTask(subscription, daysBefore); err != nil {
				slog.Error("Failed to schedule reminder task",
					slog.String("component", "scheduler"),
					slog.String("subscription_id", subscription.ID.Hex()),
					slog.Int("days_before", daysBefore),
					slog.Any("error", err),
				)
			} else {
				slog.Info("Scheduled reminder task",
					slog.String("component", "scheduler"),
					slog.String("subscription_id", subscription.ID.Hex()),
					slog.Int("days_before", daysBefore),
				)
			}
		} else {
			slog.Info("Reminder already sent recently (Redis)",
				slog.String("component", "scheduler"),
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.Int("days_before", daysBefore),
			)
		}
	}

	return nil
}

// handleRenewalTasks checks for subscriptions needing automatic renewal and schedules tasks.
func (s *SubscriptionScheduler) handleRenewalTasks(ctx context.Context) error {
	renewalSubscriptions, err := s.getSubscriptionsDueForRenewal(ctx)
	if err != nil {
		return err
	}

	slog.Info("Found subscriptions due for renewal",
		slog.String("component", "scheduler"),
		slog.Int("count", len(renewalSubscriptions)))

	// Schedule renewal tasks for each subscription approaching renewal
	for _, subscription := range renewalSubscriptions {
		if err := s.scheduleRenewalTask(subscription); err != nil {
			slog.Error("Failed to schedule renewal task",
				slog.String("component", "scheduler"),
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.Any("error", err),
			)
		} else {
			slog.Info("Scheduled renewal task",
				slog.String("component", "scheduler"),
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.String("renewal_date", subscription.ValidTill.Format(time.RFC3339)),
			)
		}
	}

	return nil
}

// handleExpirationTasks checks for subscriptions that are expired and schedules tasks.
func (s *SubscriptionScheduler) handleExpirationTasks(ctx context.Context) error {
	expiringSubscriptions, err := s.getSubscriptionsDueForExpiration(ctx)
	if err != nil {
		return err
	}

	slog.Info("Found subscriptions due for expiration check",
		slog.String("component", "scheduler"),
		slog.Int("count", len(expiringSubscriptions)))

	// Schedule expiration tasks for each subscription
	for _, subscription := range expiringSubscriptions {
		if err := s.scheduleExpirationTask(subscription); err != nil {
			slog.Error("Failed to schedule expiration task",
				slog.String("component", "scheduler"),
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.Any("error", err),
			)
		} else {
			slog.Info("Scheduled expiration task",
				slog.String("component", "scheduler"),
				slog.String("subscription_id", subscription.ID.Hex()),
				slog.String("valid_till", subscription.ValidTill.Format(time.RFC3339)),
			)
		}
	}

	return nil
}

// getSubscriptionsDueForReminder retrieves subscriptions that are due for reminders.
func (s *SubscriptionScheduler) getSubscriptionsDueForReminder(ctx context.Context) ([]*models.Subscription, error) {
	return s.subscriptionService.GetUpcomingRenewalsInternal(ctx, s.reminderDays)
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
	// Get cancelled subscriptions that are past their validity period but not marked as expired yet
	return s.subscriptionService.FetchCancelledExpiredSubscriptionsInternal(ctx)
}

// scheduleReminderTask creates and enqueues a reminder task.
func (s *SubscriptionScheduler) scheduleReminderTask(subscription *models.Subscription, daysBefore int) error {
	payload := ReminderPayload{
		SubscriptionID: subscription.ID.Hex(),
		DaysBefore:     daysBefore,
		RenewalDate:    subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(ReminderTask, payloadBytes)

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

	slog.Info("Reminder task scheduled",
		slog.String("component", "scheduler"),
		slog.String("task_id", info.ID),
		slog.String("subscription_id", subscription.ID.Hex()),
		slog.Int("days_before", daysBefore),
	)

	return nil
}

// scheduleRenewalTask creates and enqueues a renewal task.
func (s *SubscriptionScheduler) scheduleRenewalTask(subscription *models.Subscription) error {
	payload := RenewalPayload{
		SubscriptionID: subscription.ID.Hex(),
		RenewalDate:    subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(RenewalTask, payloadBytes)

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
		asynq.Timeout(45*time.Second), // Handler must finish in 60s.
		asynq.MaxRetry(5),             // Retry up to 5 times if failed.
		asynq.ProcessAt(processAt),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.Info("Renewal task scheduled",
		slog.String("component", "scheduler"),
		slog.String("task_id", info.ID),
		slog.String("subscription_id", subscription.ID.Hex()),
		slog.String("process_at", processAt.Format(time.RFC3339)),
	)

	return nil
}

// New method to schedule expiration task
func (s *SubscriptionScheduler) scheduleExpirationTask(subscription *models.Subscription) error {
	payload := ExpirationPayload{
		SubscriptionID: subscription.ID.Hex(),
		ValidTill:      subscription.ValidTill.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(ExpirationTask, payloadBytes)

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

	slog.Info("Expiration task scheduled",
		slog.String("component", "scheduler"),
		slog.String("task_id", info.ID),
		slog.String("subscription_id", subscription.ID.Hex()),
	)

	return nil
}

// Close cleanly shuts down the scheduler.
func (s *SubscriptionScheduler) Close() error {
	return s.client.Close()
}
