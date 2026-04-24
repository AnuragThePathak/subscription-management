package otelattr

var (
	// StatusSuccess is the attribute value for a successful operation.
	StatusSuccess = statusKey.String("success")
	// StatusError is the attribute value for a failed operation.
	StatusError = statusKey.String("error")
)

var (
	StateCompleted = stateKey.String("completed")
	StateActive    = stateKey.String("active")
	StatePending   = stateKey.String("pending")
	StateScheduled = stateKey.String("scheduled")
	StateRetry     = stateKey.String("retry")
	StateArchived  = stateKey.String("archived")
)
