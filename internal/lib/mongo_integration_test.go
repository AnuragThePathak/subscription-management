//go:build integration

package lib_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

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

func newTestCollection(t *testing.T) *mongo.Collection {
	t.Helper()

	dbName := "sub_test_" + bson.NewObjectID().Hex()
	db := mongoClient.Database(dbName)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
	})

	return db.Collection("lib_test")
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

type dummyDoc struct {
	ID   bson.ObjectID `bson:"_id,omitempty"`
	Name string        `bson:"name"`
}

func newDummyDoc(name string) *dummyDoc {
	return &dummyDoc{
		ID:   bson.NewObjectID(),
		Name: name,
	}
}

func TestFindOne(t *testing.T) {
	// Happy path
	t.Run("successfully finds a document", func(t *testing.T) {
		collection := newTestCollection(t)
		doc := newDummyDoc("Test")
		_, err := collection.InsertOne(t.Context(), doc)
		require.NoError(t, err)

		got, err := lib.FindOne[dummyDoc](t.Context(), collection, bson.M{"_id": doc.ID})
		require.NoError(t, err)
		assert.Equal(t, doc, got)
	})

	// Multiple matches
	t.Run("successfully returns first match without erroring on multiple matches", func(t *testing.T) {
		collection := newTestCollection(t)
		
		// Insert two targets
		doc1 := newDummyDoc("Target")
		doc2 := newDummyDoc("Target")
		_, err := collection.InsertMany(t.Context(), []any{doc1, doc2})
		require.NoError(t, err)

		// Prove it gracefully grabs one and ignores the other without panicking
		got, err := lib.FindOne[dummyDoc](t.Context(), collection, bson.M{"name": "Target"})
		
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "Target", got.Name)
	})

	// Not found
	t.Run("translates mongo.ErrNoDocuments to apperror.ErrNotFound", func(t *testing.T) {
		collection := newTestCollection(t)
		_, err := collection.InsertOne(t.Context(), newDummyDoc("Test"))
		require.NoError(t, err)

		got, err := lib.FindOne[dummyDoc](t.Context(), collection, bson.M{"_id": bson.NewObjectID()})

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})

	// Deadline exceeded
	t.Run("translates context.DeadlineExceeded to apperror", func(t *testing.T) {
		collection := newTestCollection(t)
		
		// Create a context that is already expired
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := lib.FindOne[dummyDoc](ctx, collection, bson.M{"_id": bson.NewObjectID()})

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

func TestFindMany(t *testing.T) {
	// Happy path
	t.Run("successfully finds multiple matching documents", func(t *testing.T) {
		collection := newTestCollection(t)
		// 2 Targets, 1 Noise
		doc1 := newDummyDoc("Target")
		doc2 := newDummyDoc("Target")
		noise := newDummyDoc("Noise")
		expectedDocs := []*dummyDoc{doc1, doc2}
		_, err := collection.InsertMany(t.Context(), []any{doc1, doc2, noise})
		require.NoError(t, err)
		
		got, err := lib.FindMany[dummyDoc](t.Context(), collection, bson.M{"name": "Target"})
		
		require.NoError(t, err)
		require.Len(t, got, 2, "expected exactly 2 documents, cursor iteration or filter failed")
		assert.ElementsMatch(t, expectedDocs, got, "expected docs didn't match")
	})

	// Not found
	t.Run("doesn't return error when no documents are found", func(t *testing.T) {
		collection := newTestCollection(t)
		_, err := collection.InsertOne(t.Context(), newDummyDoc("Test"))
		require.NoError(t, err)

		got, err := lib.FindMany[dummyDoc](t.Context(), collection, bson.M{"name": "NonExistent"})

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	// Deadline exceeded
	t.Run("translates context.DeadlineExceeded to apperror", func(t *testing.T) {
		collection := newTestCollection(t)
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := lib.FindMany[dummyDoc](ctx, collection, bson.M{})

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

func TestCount(t *testing.T) {
	// Happy path
	t.Run("successfully counts matching documents", func(t *testing.T) {
		collection := newTestCollection(t)
		doc1 := newDummyDoc("Target")
		doc2 := newDummyDoc("Target")
		noise := newDummyDoc("Noise")
		_, err := collection.InsertMany(t.Context(), []any{doc1, doc2, noise})
		require.NoError(t, err)

		count, err := lib.Count(t.Context(), collection, bson.M{"name": "Target"})

		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	// Not found
	t.Run("returns 0 when no documents are found", func(t *testing.T) {
		collection := newTestCollection(t)
		_, err := collection.InsertOne(t.Context(), newDummyDoc("Test"))
		require.NoError(t, err)

		count, err := lib.Count(t.Context(), collection, bson.M{"name": "NonExistent"})

		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	// Deadline exceeded
	t.Run("translates context.DeadlineExceeded to apperror", func(t *testing.T) {
		collection := newTestCollection(t)
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		count, err := lib.Count(ctx, collection, bson.M{})

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Equal(t, int64(0), count)
	})
}
