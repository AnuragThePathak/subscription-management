package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"

	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RequestHandler holds shared dependencies for processing HTTP requests.
type RequestHandler struct {
	validate *validator.Validate
}

// NewRequestHandler creates a new RequestHandler with the provided validator.
func NewRequestHandler(validate *validator.Validate) *RequestHandler {
	return &RequestHandler{validate: validate}
}

// readRequestBody decodes and validates the JSON request body.
func (h *RequestHandler) readRequestBody(w http.ResponseWriter, r *http.Request, bodyObj any) bool {
	if bodyObj == nil {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(bodyObj); err != nil {
		slog.WarnContext(r.Context(), "Failed to decode request body",
			logattr.Method(r.Method),
			logattr.Path(r.URL.Path),
			logattr.Error(err),
		)
		WriteAPIResponse(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "Invalid JSON"},
		)
		return false
	}
	if err := h.validate.Struct(bodyObj); err != nil {
		slog.WarnContext(r.Context(), "Request validation failed",
			logattr.Method(r.Method),
			logattr.Path(r.URL.Path),
			logattr.Error(err),
		)
		WriteAPIResponse(
			w,
			http.StatusBadRequest,
			map[string]string{"error": err.Error()},
		)
		return false
	}
	return true
}

// ServeRequest processes an HTTP request using the provided InternalRequest configuration.
func (h *RequestHandler) ServeRequest(req InternalRequest) {
	if !h.readRequestBody(req.W, req.R, req.ReqBodyObj) {
		return
	}
	if req.SuccessCode == 0 {
		slog.WarnContext(req.R.Context(), "SuccessCode not set, defaulting to 200",
			logattr.Method(req.R.Method),
			logattr.Path(req.R.URL.Path),
		)
		req.SuccessCode = http.StatusOK
	}

	respBodyObj, err := req.EndpointLogic()
	if err != nil {
		span := trace.SpanFromContext(req.R.Context())

		if appErr, ok := errors.AsType[apperror.AppError](err); ok {
			status := appErr.Status()

			logAttrs := []any{
				logattr.Method(req.R.Method),
				logattr.Path(req.R.URL.Path),
				logattr.HTTPStatus(status),
				logattr.ErrorCode(string(appErr.Code())),
				logattr.Message(appErr.Message()),
				logattr.Error(err),
			}
			for _, attr := range appErr.LogAttributes() {
				logAttrs = append(logAttrs, attr)
			}

			if status >= 500 {
				span.RecordError(err)
				span.SetStatus(codes.Error, appErr.Message())

				slog.ErrorContext(req.R.Context(), "Request failed",
					logAttrs...,
				)
			} else {
				slog.WarnContext(req.R.Context(), "Request rejected",
					logAttrs...,
				)
			}
			WriteAPIResponse(
				req.W,
				status,
				map[string]string{"error": appErr.Message()},
			)
		} else {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Unhandled error")

			slog.ErrorContext(req.R.Context(), "Unhandled request error",
				logattr.Method(req.R.Method),
				logattr.Path(req.R.URL.Path),
				logattr.Error(err),
			)
			WriteAPIResponse(
				req.W,
				http.StatusInternalServerError,
				map[string]string{
					"error": "An unexpected internal error occurred.",
				},
			)
		}
		return
	}

	WriteAPIResponse(req.W, req.SuccessCode, respBodyObj)
}

// WriteAPIResponse writes the response in JSON format.
func WriteAPIResponse(w http.ResponseWriter, statusCode int, res any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if res != nil {
		json.NewEncoder(w).Encode(res)
	}
}
