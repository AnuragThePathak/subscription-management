package observability

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	subscriptionIDKey = attribute.Key("subscription.id")
	schedulerIntervalKey = attribute.Key("scheduler.interval")
	schedulerReminderDaysKey = attribute.Key("scheduler.reminder_days")
	taskTypeKey = attribute.Key("scheduler.task_type")
	processAtKey = attribute.Key("scheduler.process_at")
	tasksScheduledKey = attribute.Key("scheduler.tasks_scheduled")
)

func SubscriptionID(val string) attribute.KeyValue {
	return subscriptionIDKey.String(val)
}

func SchedulerInterval(val string) attribute.KeyValue {
	return schedulerIntervalKey.String(val)
}

func SchedulerReminderDays(val []int) attribute.KeyValue {
	return schedulerReminderDaysKey.IntSlice(val)
}

func TaskType(val string) attribute.KeyValue {
	return taskTypeKey.String(val)
}

func ProcessAt(val time.Time) attribute.KeyValue {
	return processAtKey.String(val.Format(time.RFC3339))
}

func TasksScheduled(val int) attribute.KeyValue {
	return tasksScheduledKey.Int(val)
}
