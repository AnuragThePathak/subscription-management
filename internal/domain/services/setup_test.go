package services_test

import (
	"time"

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
