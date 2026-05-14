//go:build integration

// Package txntest contains integration tests for the TxnExecutor.
// It lives in its own directory so it can have an independent TestMain
// that spins up a MongoDB replica-set container — required for
// multi-document transactions — without forcing the bill/subscription
// integration tests (which need only a standalone node) to carry that
// extra complexity.
package txntest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/domain/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ---------------------------------------------------------------------------
// Package-level container & client — replica-set so transactions work.
// ---------------------------------------------------------------------------

var mongoClient *mongo.Client

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:8", mongodb.WithReplicaSet("rs0"))
	if err != nil {
		panic("failed to start MongoDB container: " + err.Error())
	}
	defer func() { _ = container.Terminate(ctx) }()

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		panic("failed to get MongoDB connection string: " + err.Error())
	}

	// On macOS with Docker Desktop, rs.initiate() registers the container's
	// internal bridge IP as the RS member address. The host Go process cannot
	// reach that IP (Docker runs in a VM). directConnection=true bypasses RS
	// topology discovery and talks directly to the mapped localhost port.
	// WithReplicaSet always produces a URI with ?replicaSet=..., so & is correct.
	uri += "&directConnection=true"

	// serverSelectionTimeout=60s gives the RS election time to complete.
	// testcontainers marks the container "ready" after rs.initiate() fires,
	// but the primary election itself takes a few more seconds.
	mongoClient, err = mongo.Connect(
		options.Client().
			ApplyURI(uri).
			SetServerSelectionTimeout(60 * time.Second),
	)
	if err != nil {
		panic("failed to connect to MongoDB: " + err.Error())
	}
	defer func() { _ = mongoClient.Disconnect(ctx) }()

	// Ping blocks until the driver selects a server (primary elected).
	pingCtx, pingCancel := context.WithTimeout(ctx, 60*time.Second)
	defer pingCancel()
	if err = mongoClient.Ping(pingCtx, nil); err != nil {
		panic("MongoDB replica set primary not elected in time: " + err.Error())
	}

	m.Run()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTxnCol returns a fresh collection in a uniquely named database.
// The database is dropped at the end of the test.
func newTxnCol(t *testing.T) *mongo.Collection {
	t.Helper()
	dbName := "txn_test_" + bson.NewObjectID().Hex()
	db := mongoClient.Database(dbName)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
	})
	return db.Collection("sentinel")
}

// exists reports whether the given id is present in col, queried outside
// any transaction so it only sees committed data.
func exists(t *testing.T, col *mongo.Collection, id bson.ObjectID) bool {
	t.Helper()
	res := col.FindOne(context.Background(), bson.M{"_id": id})
	return res.Err() == nil
}

// txnSentinel is a minimal document used to prove a write inside a
// transaction either persisted (commit) or was rolled back (abort).
type txnSentinel struct {
	ID   bson.ObjectID `bson:"_id"`
	Name string        `bson:"name"`
}

func newTxnSentinel() *txnSentinel {
	return &txnSentinel{
		ID:   bson.NewObjectID(),
		Name: "txn-test",
	}
}

// ---------------------------------------------------------------------------
// TxnExecutor.WithTransaction
// ---------------------------------------------------------------------------

func TestTxnExecutor_WithTransaction(t *testing.T) {
	t.Run("commits on success - write is visible after fn returns nil", func(t *testing.T) {
		col := newTxnCol(t)
		executor := repositories.NewTxnExecutor(mongoClient)
		doc := newTxnSentinel()

		err := executor.WithTransaction(context.Background(), func(ctx context.Context) error {
			_, insertErr := col.InsertOne(ctx, doc)
			return insertErr
		})

		require.NoError(t, err)
		// Read outside any transaction — committed data must be visible.
		assert.True(t, exists(t, col, doc.ID), "document should be present after successful commit")
	})

	t.Run("rolls back on error - write is absent after fn returns an error", func(t *testing.T) {
		col := newTxnCol(t)
		executor := repositories.NewTxnExecutor(mongoClient)

		doc := newTxnSentinel()

		expectedError := errors.New("something went wrong, abort transaction")

		err := executor.WithTransaction(context.Background(), func(ctx context.Context) error {
			// Perform a write inside the transaction…
			if _, insertErr := col.InsertOne(ctx, doc); insertErr != nil {
				return insertErr
			}
			// …then signal failure so the driver aborts and rolls back.
			return expectedError
		})

		// The executor must propagate the fn error to the caller.
		require.ErrorIs(t, err, expectedError)
		// The write must not be visible — it was rolled back.
		assert.False(t, exists(t, col, doc.ID), "document should be absent after rollback")
	})
}
