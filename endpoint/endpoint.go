package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"

	"log/slog"

	"github.com/anuragthepathak/subscription-management/apperror"
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

	respBodyObj, err := req.EndpointLogic()
	if err != nil {
		slog.Debug("Request failed", slog.String("error", err.Error()))
		var appErr apperror.AppError
		if errors.As(err, &appErr) {
			WriteAPIResponse(req.W, appErr.Status(), appErr.Message())
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
			slog.Error("Failed to encode response", slog.Any("error", err))
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
