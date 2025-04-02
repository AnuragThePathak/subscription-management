package endpoint

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/go-playground/validator/v10"
)

func readRequestBody(
	w http.ResponseWriter,
	r *http.Request,
	bodyObj any,
) bool {
	if bodyObj == nil {
		return true
	}

	// Decode JSON request body
	if err := json.NewDecoder(r.Body).Decode(bodyObj); err != nil {
		writeAPIResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return false
	}

	// Perform struct validation
	if err := validator.New(validator.WithRequiredStructEnabled()).Struct(bodyObj); err != nil {
		writeAPIResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return false
	}

	return true
}

func ServeRequest[T InternalModel[R], R any](req InternalRequest) {
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
			writeAPIResponse(req.W, appErr.Status(), appErr.Message())
		} else {
			writeAPIResponse(req.W, http.StatusInternalServerError, nil)
		}
		return
	}

	// Check if the response is a slice
	switch v := respBodyObj.(type) {
	case []T: // ✅ Handle slice case
		writeAPIResponse(req.W, req.SuccessCode, toResponseSlice(v))
	case T: // ✅ Handle single object case
		writeAPIResponse(req.W, req.SuccessCode, v.ToResponse())
	default:
		writeAPIResponse(req.W, http.StatusInternalServerError, nil)
	}
}


func writeAPIResponse(
	w http.ResponseWriter,
	statusCode int,
	res any,
) {
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if res != nil {
		if err := json.NewEncoder(w).Encode(res); err != nil {
			slog.Error(
				"Failed to encode response",
				slog.String("component", "endpoint"),
				slog.Any("error", err),
			)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
