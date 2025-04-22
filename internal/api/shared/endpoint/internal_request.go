package endpoint

import (
	"net/http"
)

// InternalRequest encapsulates the data required to process a request.
type InternalRequest struct {
	W             http.ResponseWriter
	R             *http.Request
	EndpointLogic func() (any, error) // Logic to execute for the endpoint.
	SuccessCode   int                 // HTTP status code for successful responses.
	ReqBodyObj    any                 // Optional request body object.
}
