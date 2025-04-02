package endpoint

import (
	"net/http"
)

type InternalRequest struct {
    W             http.ResponseWriter
    R             *http.Request
    EndpointLogic func() (any, error) // ✅ Now supports both *T and []T
    SuccessCode   int
    ReqBodyObj    any
}

type InternalModel[T any] interface {
	ToResponse() T
}
