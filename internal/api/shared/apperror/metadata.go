package apperror

// metaKey defines a strict type for structured logging keys
type metaKey string

const (
	KeyUserEmail   metaKey = "user_email"
	KeyUserID      metaKey = "user_id"
	KeyAttemptedID metaKey = "attempted_id"
)
