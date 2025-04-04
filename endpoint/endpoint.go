package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/go-playground/validator/v10"
	"log/slog"
)

// readRequestBody decodes JSON into bodyObj and validates it.
func readRequestBody(w http.ResponseWriter, r *http.Request, bodyObj any) bool {
	if bodyObj == nil {
		return true
	}
	// Decode JSON request body.
	if err := json.NewDecoder(r.Body).Decode(bodyObj); err != nil {
		WriteAPIResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return false
	}
	// Perform struct validation.
	if err := validator.New(validator.WithRequiredStructEnabled()).Struct(bodyObj); err != nil {
		WriteAPIResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return false
	}
	return true
}

// ServeRequest is a generic handler that does not perform any conversion.
// It delegates conversion to controller code if needed.
func ServeRequest(req InternalRequest) {
	if req.ReqBodyObj != nil {
		if !readRequestBody(req.W, req.R, req.ReqBodyObj) {
			return
		}
	}

	respBodyObj, err := req.EndpointLogic()
	if err != nil {
		slog.Debug(err.Error())
		var appErr apperror.AppError
		if ok := errors.As(err, &appErr); ok {
			WriteAPIResponse(req.W, appErr.Status(), appErr.Message())
		} else {
			WriteAPIResponse(req.W, http.StatusInternalServerError, nil)
		}
		return
	}

	WriteAPIResponse(req.W, req.SuccessCode, respBodyObj)
}

// WriteAPIResponse encodes and writes the response in JSON format.
func WriteAPIResponse(w http.ResponseWriter, statusCode int, res any) {
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if res != nil {
		if err := json.NewEncoder(w).Encode(res); err != nil {
			slog.Error("Failed to encode response", slog.Any("error", err))
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}