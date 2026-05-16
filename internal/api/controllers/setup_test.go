package controllers_test

import (
	"net/http"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/appctx"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mockTime is a stable timestamp used across tests that need deterministic time.
var mockTime = time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

// defaultUserID is a stable, deterministic ObjectID used across all tests.
var defaultUserID = bson.NewObjectID()
var defaultUserHex = defaultUserID.Hex()
var defaultUserEmail = "alice@example.com"

// injectUserID injects a userID into the request context, simulating what the
// Authentication middleware does for protected routes.
func injectUserID(req *http.Request, userID string) *http.Request {
	return req.WithContext(appctx.WithUserID(req.Context(), userID))
}