package endpoint_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Setup & Dummies
// ---------------------------------------------------------------------------

// dummyRequest is a simple struct with validation tags to test the validator.
type dummyRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type dummyResponse struct {
	Message string `json:"message"`
}

func setupHandler() *endpoint.RequestHandler {
	v := validator.New()
	return endpoint.NewRequestHandler(v)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRequestHandler_ServeRequest(t *testing.T) {
	handler := setupHandler()

	t.Run("success - parses valid JSON, executes logic, returns 200 OK", func(t *testing.T) {
		reqBody := `{"name": "John Doe", "email": "john@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		var parsedBody dummyRequest
		handler.ServeRequest(endpoint.InternalRequest{
			W: rr,
			R: req,
			EndpointLogic: func() (any, error) {
				// Prove the body was parsed correctly
				assert.Equal(t, "John Doe", parsedBody.Name)
				assert.Equal(t, "john@example.com", parsedBody.Email)
				return &dummyResponse{Message: "success"}, nil
			},
			SuccessCode: http.StatusOK,
			ReqBodyObj:  &parsedBody,
		})

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp dummyResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "success", resp.Message)
	})

	t.Run("success - defaults to 200 OK if SuccessCode is 0", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeRequest(endpoint.InternalRequest{
			W: rr,
			R: req,
			EndpointLogic: func() (any, error) {
				return nil, nil // No body, successful execution
			},
			SuccessCode: 0, // Explicitly omitted
		})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("error - translates AppError to correct HTTP status code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		expectedErr := apperror.NewNotFoundError("user not found")

		handler.ServeRequest(endpoint.InternalRequest{
			W: rr,
			R: req,
			EndpointLogic: func() (any, error) {
				return nil, expectedErr
			},
		})

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), expectedErr.Message())
	})

	t.Run("error - translates unhandled error to 500 Internal Server Error safely", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeRequest(endpoint.InternalRequest{
			W: rr,
			R: req,
			EndpointLogic: func() (any, error) {
				return nil, errors.New("database exploded entirely")
			},
		})

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		// Vault Lock: Prove we don't leak the raw error message to the client
		assert.NotContains(t, rr.Body.String(), "database exploded entirely")
		assert.Contains(t, rr.Body.String(), "An unexpected internal error occurred")
	})

	t.Run("error - invalid JSON returns 400 Bad Request", func(t *testing.T) {
		reqBody := `{"name": "John Doe", "email": missing_quotes}` // Malformed JSON
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		var parsedBody dummyRequest
		handler.ServeRequest(endpoint.InternalRequest{
			W: rr,
			R: req,
			EndpointLogic: func() (any, error) {
				t.Fatal("EndpointLogic should NEVER be called if JSON parsing fails")
				return nil, nil
			},
			ReqBodyObj: &parsedBody,
		})

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid JSON")
	})

	t.Run("error - struct validation failure returns 400 Bad Request", func(t *testing.T) {
		// Missing 'email' which is required by the validator
		reqBody := `{"name": "John Doe"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		var parsedBody dummyRequest
		handler.ServeRequest(endpoint.InternalRequest{
			W: rr,
			R: req,
			EndpointLogic: func() (any, error) {
				t.Fatal("EndpointLogic should NEVER be called if validation fails")
				return nil, nil
			},
			ReqBodyObj: &parsedBody,
		})

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Error:Field validation for")
	})

	t.Run("error - payload exceeding max bytes returns 413 Request Entity Too Large", func(t *testing.T) {
		// Create a payload larger than 1MB (the limit set in endpoint.go)
		largeBody := []byte(
			`{"name":"` + strings.Repeat("a", 2*1024*1024) + `","email":"b@c.com"}`,
		)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(largeBody))
		rr := httptest.NewRecorder()

		var parsedBody dummyRequest
		handler.ServeRequest(endpoint.InternalRequest{
			W: rr,
			R: req,
			EndpointLogic: func() (any, error) {
				t.Fatal("EndpointLogic should NEVER be called if payload is too large")
				return nil, nil
			},
			ReqBodyObj: &parsedBody,
		})

		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
		assert.Contains(t, rr.Body.String(), "Request body too large")
	})
}
