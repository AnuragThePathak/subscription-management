package otelattr

var (
	// StatusSuccess is the attribute value for a successful operation.
	StatusSuccess = statusKey.String("success")
	// StatusError is the attribute value for a failed operation.
	StatusError = statusKey.String("error")
)
