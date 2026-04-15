package apperror

// MetaKey defines a strict type for structured logging keys
type MetaKey string

const (
	KeyUserEmail    MetaKey = "user_email"
	KeyUserID       MetaKey = "user_id"
	KeyAttemptedID  MetaKey = "attempted_id"
)