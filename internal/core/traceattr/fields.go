package traceattr

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	subscriptionIDKey        = attribute.Key("subscription.id")
	taskIDKey                = attribute.Key("asynq.task_id")
	emailDaysBeforeKey       = attribute.Key("email.days_before")
	schedulerDaysBeforeKey   = attribute.Key("scheduler.days_before")
	schedulerIntervalKey     = attribute.Key("scheduler.interval")
	schedulerReminderDaysKey = attribute.Key("scheduler.reminder_days")
	taskTypeKey              = attribute.Key("scheduler.task_type")
	processAtKey             = attribute.Key("scheduler.process_at")
	tasksScheduledKey        = attribute.Key("scheduler.tasks_scheduled")
)

// SubscriptionID returns an attribute.KeyValue for the subscription ID.
func SubscriptionID(id string) attribute.KeyValue {
	return subscriptionIDKey.String(id)
}

// TaskID returns an attribute.KeyValue for the task ID.
func TaskID(id string) attribute.KeyValue {
	return taskIDKey.String(id)
}

// EmailDaysBefore returns an attribute.KeyValue for the email days before reminder.
func EmailDaysBefore(days int) attribute.KeyValue {
	return emailDaysBeforeKey.Int(days)
}

// SchedulerDaysBefore returns an attribute.KeyValue for the scheduler days before reminder.
func SchedulerDaysBefore(days int) attribute.KeyValue {
	return schedulerDaysBeforeKey.Int(days)
}

// SchedulerInterval returns an attribute.KeyValue for the scheduler interval.
func SchedulerInterval(interval string) attribute.KeyValue {
	return schedulerIntervalKey.String(interval)
}

// SchedulerReminderDays returns an attribute.KeyValue for the scheduler reminder days.
func SchedulerReminderDays(days []int) attribute.KeyValue {
	return schedulerReminderDaysKey.IntSlice(days)
}

// TaskType returns an attribute.KeyValue for the scheduler task type.
func TaskType(t string) attribute.KeyValue {
	return taskTypeKey.String(t)
}

// ProcessAt returns an attribute.KeyValue for the scheduler process at time.
func ProcessAt(t time.Time) attribute.KeyValue {
	return processAtKey.String(t.Format(time.RFC3339))
}

// TasksScheduled returns an attribute.KeyValue for the number of tasks scheduled.
func TasksScheduled(count int) attribute.KeyValue {
	return tasksScheduledKey.Int(count)
}
