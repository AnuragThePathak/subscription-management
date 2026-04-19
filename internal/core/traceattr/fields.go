package traceattr

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	// Business attributes
	subscriptionIDKey = attribute.Key("subscription.id")
	daysBeforeKey     = attribute.Key("subscription.days_before")

	// Queue related attributes
	taskIDKey    = attribute.Key("job.id")
	taskTypeKey  = attribute.Key("job.type")
	processAtKey = attribute.Key("job.process_at")
)

// SubscriptionID returns an attribute.KeyValue for the subscription ID.
func SubscriptionID(id string) attribute.KeyValue {
	return subscriptionIDKey.String(id)
}

// DaysBefore returns an attribute.KeyValue for the days before reminder.
func DaysBefore(days int) attribute.KeyValue {
	return daysBeforeKey.Int(days)
}

// TaskID returns an attribute.KeyValue for the task ID.
func TaskID(id string) attribute.KeyValue {
	return taskIDKey.String(id)
}

// TaskType returns an attribute.KeyValue for the task type.
func TaskType(t string) attribute.KeyValue {
	return taskTypeKey.String(t)
}

// ProcessAt returns an attribute.KeyValue for the scheduler process at time.
func ProcessAt(t time.Time) attribute.KeyValue {
	return processAtKey.String(t.Format(time.RFC3339))
}
