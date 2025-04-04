package endpoint

import (
	"net/http"
)

// InternalRequest carries the necessary data to process a request.
type InternalRequest struct {
	W             http.ResponseWriter
	R             *http.Request
	EndpointLogic func() (any, error) // Should return the internal model (or slice of it) or an error.
	SuccessCode   int
	ReqBodyObj    any // Optional request body
}
