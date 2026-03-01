package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"

	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/go-playground/validator/v10"
)

// readRequestBody decodes and validates the JSON request body.
func readRequestBody(w http.ResponseWriter, r *http.Request, bodyObj any) bool {
	if bodyObj == nil {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(bodyObj); err != nil {
		WriteAPIResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return false
	}
	if err := validator.New(validator.WithRequiredStructEnabled()).Struct(bodyObj); err != nil {
		WriteAPIResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return false
	}
	return true
}

// ServeRequest processes an HTTP request using the provided InternalRequest configuration.
func ServeRequest(req InternalRequest) {
	if req.ReqBodyObj != nil && !readRequestBody(req.W, req.R, req.ReqBodyObj) {
		return
	}
	if req.SuccessCode == 0 {
		slog.Warn("SuccessCode not set, defaulting to 200")
		req.SuccessCode = http.StatusOK
	}

	respBodyObj, err := req.EndpointLogic()
	if err != nil {
		slog.Debug("Request failed", slog.String("error", err.Error()))
		if appErr, ok := errors.AsType[apperror.AppError](err); ok {
			WriteAPIResponse(req.W, appErr.Status(), map[string]string{"error": appErr.Message()})
		} else {
			WriteAPIResponse(req.W, http.StatusInternalServerError, nil)
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
		if err := json.NewEncoder(w).Encode(res); err != nil {
			// Headers and status code are already sent; the response cannot be modified at this point.
			slog.Error("Failed to encode response", slog.Any("error", err))
		}
	}
}
