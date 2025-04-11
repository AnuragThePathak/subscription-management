package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/repositories"
	"github.com/hibiken/asynq"
)

const (
	// ReminderTask is the task name for subscription reminders
	ReminderTask = "subscription:reminder"
)

// ReminderPayload represents the data needed to process a reminder
type ReminderPayload struct {
	SubscriptionID string `json:"subscription_id"`
	DaysBefore     int    `json:"days_before"`
	RenewalDate    string `json:"renewal_date"`
}

// SubscriptionScheduler handles scheduling of subscription-related tasks
type SubscriptionScheduler struct {
	subscriptionRepo repositories.SubscriptionRepository
	client           *asynq.Client
	interval         time.Duration
	reminderDays     []int
}

// NewSubscriptionScheduler creates a new subscription scheduler
func NewSubscriptionScheduler(
	subscriptionRepo repositories.SubscriptionRepository,
	redisConfig *asynq.RedisClientOpt,
	interval time.Duration,
	reminderDays []int,
) *SubscriptionScheduler {
	client := asynq.NewClient(redisConfig)
	return &SubscriptionScheduler{
		subscriptionRepo: subscriptionRepo,
		client:           client,
		interval:         interval,
		reminderDays:     reminderDays,
	}
}

// Start begins the scheduler loop
func (s *SubscriptionScheduler) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run once immediately
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

// pollSubscriptions checks for subscriptions needing reminders and schedules tasks
func (s *SubscriptionScheduler) pollSubscriptions(ctx context.Context) error {
	slog.Info("Polling for subscriptions requiring reminders",
		slog.String("component", "scheduler"))

	activeSubscriptions, err := s.getSubscriptionsDueForReminder(ctx)
	if err != nil {
		return err
	}

	// Check each subscription for upcoming renewal dates
	for _, subscription := range activeSubscriptions {
		daysBefore := daysUntil(subscription.RenewalDate, nil)
		// Schedule a reminder task
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
	}

	return nil
}

// getSubscriptionsDueForReminder retrieves subscriptions that are due for reminders
func (s *SubscriptionScheduler) getSubscriptionsDueForReminder(ctx context.Context) ([]*models.Subscription, error) {
	return s.subscriptionRepo.GetSubscriptionsDueForReminder(ctx, s.reminderDays)
}

// scheduleReminderTask creates and enqueues a reminder task
func (s *SubscriptionScheduler) scheduleReminderTask(subscription *models.Subscription, daysBefore int) error {
	payload := ReminderPayload{
		SubscriptionID: subscription.ID.Hex(),
		DaysBefore:     daysBefore,
		RenewalDate:    subscription.RenewalDate.Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Use a deterministic TaskID to prevent duplicates
	taskID := fmt.Sprintf("%s:%d", subscription.ID.Hex(), daysBefore)

	task := asynq.NewTask(ReminderTask, payloadBytes)

	// Enqueue the task with the custom TaskID
	_, err = s.client.Enqueue(
		task,
		asynq.TaskID(taskID),
		asynq.Retention(24*time.Hour),
		asynq.Timeout(45*time.Second), // Optional: Task handler gets 30s to finish
		asynq.MaxRetry(3),             // Retry up to 3 times on failure
	)
	if err != nil {
		if err == asynq.ErrDuplicateTask {
			slog.Debug("Duplicate task ignored",
				slog.String("component", "scheduler"),
				slog.String("task_id", taskID),
			)
			return nil // Not a fatal error
		}
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	return nil
}

// daysUntil returns the number of full calendar days between now and targetDate.
// It truncates both times to midnight in the provided location (defaults to time.Local).
func daysUntil(targetDate time.Time, loc *time.Location) int {
	if loc == nil {
		loc = time.Local
	}

	now := time.Now().In(loc).Truncate(24 * time.Hour)
	target := targetDate.In(loc).Truncate(24 * time.Hour)

	return int(target.Sub(now).Hours() / 24)
}

// Close cleanly shuts down the scheduler
func (s *SubscriptionScheduler) Close() error {
	return s.client.Close()
}
