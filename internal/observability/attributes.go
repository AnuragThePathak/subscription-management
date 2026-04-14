package observability

import (
	"go.opentelemetry.io/otel/attribute"
)

const (
	subscriptionIDKey = attribute.Key("subscription.id")
	schedulerIntervalKey = attribute.Key("scheduler.interval")
	schedulerReminderDaysKey = attribute.Key("scheduler.reminder_days")
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
