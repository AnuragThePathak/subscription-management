package otelattr

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	// Business attributes
	subscriptionIDKey = attribute.Key("subscription.id")
	daysBeforeKey     = attribute.Key("subscription.days_before")

	// Queue related attributes
	taskTypeKey  = attribute.Key("job.type")
	processAtKey = attribute.Key("job.process_at")
	queueKey     = attribute.Key("job.queue")
	statusKey    = attribute.Key("job.status")
	stateKey     = attribute.Key("queue.state")
)

// SubscriptionID returns an attribute.KeyValue for the subscription ID.
func SubscriptionID(id string) attribute.KeyValue {
	return subscriptionIDKey.String(id)
}

// DaysBefore returns an attribute.KeyValue for the days before reminder.
func DaysBefore(days int) attribute.KeyValue {
	return daysBeforeKey.Int(days)
}

// TaskType returns an attribute.KeyValue for the task type.
func TaskType(t string) attribute.KeyValue {
	return taskTypeKey.String(t)
}

// ProcessAt returns an attribute.KeyValue for the scheduler process at time.
func ProcessAt(t time.Time) attribute.KeyValue {
	return processAtKey.String(t.Format(time.RFC3339))
}

// Queue returns an attribute.KeyValue for the queue name.
func Queue(q string) attribute.KeyValue {
	return queueKey.String(q)
}
