//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mockTime is a stable reference point for all subscription tests.
var mockTime = time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

// mockToday represents the start of the day for mockTime.
var mockToday = time.Date(
	mockTime.Year(),
	mockTime.Month(),
	mockTime.Day(),
	0,
	0,
	0,
	0,
	mockTime.Location(),
)
var mockTomorrow = mockToday.AddDate(0, 0, 1)
var mockTwoDaysLater = mockToday.AddDate(0, 0, 2)
var mockYesterday = mockToday.AddDate(0, 0, -1)

// mockOneMonthLater is a time one month after mockToday.
var mockOneMonthLater = mockToday.AddDate(0, 1, 0)
var mockTwoMonthsLater = mockToday.AddDate(0, 2, 0)
var mockOneMonthAgo = mockToday.AddDate(0, -1, 0)

// ---------------------------------------------------------------------------
// Package-level container & client — standalone MongoDB, shared by bill and
// subscription integration tests. Per-test database isolation (unique dbName
// per test) means sharing this container causes zero data interference.
// ---------------------------------------------------------------------------

var mongoClient *mongo.Client

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:8")
	if err != nil {
		panic("failed to start MongoDB container: " + err.Error())
	}
	defer func() { _ = container.Terminate(ctx) }()

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		panic("failed to get MongoDB connection string: " + err.Error())
	}

	mongoClient, err = mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		panic("failed to connect to MongoDB: " + err.Error())
	}
	defer func() { _ = mongoClient.Disconnect(ctx) }()

	m.Run()
}

// assertAppErrorCode asserts that err is an AppError with the expected code.
func assertAppErrorCode(t *testing.T, err error, code apperror.ErrorCode) {
	t.Helper()
	if appErr, ok := errors.AsType[apperror.AppError](err); ok {
		assert.Equal(t, code, appErr.Code())
	} else {
		assert.Failf(t,
			"Unexpected error type",
			"test case defined a expected error code (%s), but received raw error: %v",
			code, err,
		)
	}
}
