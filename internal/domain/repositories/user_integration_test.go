//go:build integration

package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var defaultUserEmail = "email@gmail.com"

// Data Collision: Force all users to share the exact same properties by default.
// The only things that should differ are ID and Email to mathematically prove filters.
func validUser() *models.User {
	return &models.User{
		ID:        bson.NewObjectID(),
		Name:      "Default Name",
		Email:     defaultUserEmail,
		Password:  "hashed_password_123",
		CreatedAt: mockTime,
		UpdatedAt: mockTime,
	}
}

func newUserRepo(t *testing.T) (repositories.UserRepository, *mongo.Collection) {
	t.Helper()

	dbName := "user_test_" + bson.NewObjectID().Hex()
	db := mongoClient.Database(dbName)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	repo, err := repositories.NewUserRepository(ctx, db)
	require.NoError(t, err, "NewUserRepository should not error")

	return repo, db.Collection("users")
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestUserRepository_Create(t *testing.T) {
	t.Run("success - user inserted and verified in db", func(t *testing.T) {
		repo, collection := newUserRepo(t)
		user := validUser()

		_, err := repo.Create(t.Context(), user)
		require.NoError(t, err)

		// Read-Back Verification
		savedUser := &models.User{}
		err = collection.FindOne(t.Context(), bson.M{"_id": user.ID}).Decode(savedUser)

		require.NoError(t, err)
		assert.Equal(t, user, savedUser)
	})

	t.Run("error - duplicate email returns conflict", func(t *testing.T) {
		repo, _ := newUserRepo(t)
		user1 := validUser()

		_, err := repo.Create(t.Context(), user1)
		require.NoError(t, err)

		// Attempt to insert a DIFFERENT user with the SAME email
		user2 := validUser()
		got, err := repo.Create(t.Context(), user2)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrConflict)
		assert.Nil(t, got)
	})

	// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newUserRepo(t)
		// Force an error by passing an already-expired context
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := repo.Create(ctx, validUser())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// FindByEmail
// ---------------------------------------------------------------------------

func TestUserRepository_FindByEmail(t *testing.T) {
	t.Run("success - found exact user and ignores decoy", func(t *testing.T) {
		repo, collection := newUserRepo(t)

		target := validUser()
		decoy := validUser()
		decoy.Email = "decoy@abc.com"

		// Poison the well
		_, err := collection.InsertMany(t.Context(), []*models.User{decoy, target})
		require.NoError(t, err)

		got, err := repo.FindByEmail(t.Context(), target.Email)

		require.NoError(t, err)
		assert.Equal(t, target, got)
	})

	t.Run("error - not found returns not-found error", func(t *testing.T) {
		repo, collection := newUserRepo(t)
		noise := validUser()
		_, err := collection.InsertOne(t.Context(), noise)
		require.NoError(t, err)

		got, err := repo.FindByEmail(t.Context(), "unknown@abc.com")

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestUserRepository_FindByID(t *testing.T) {
	t.Run("success - found exact user and ignores decoy", func(t *testing.T) {
		repo, collection := newUserRepo(t)

		target := validUser()
		decoy := validUser()
		decoy.Email = "decoy@abc.com"

		// Poison the well
		_, err := collection.InsertMany(t.Context(), []*models.User{decoy, target})
		require.NoError(t, err)

		got, err := repo.FindByID(t.Context(), target.ID)

		require.NoError(t, err)
		assert.Equal(t, target, got)
	})

	t.Run("error - not found returns not-found error", func(t *testing.T) {
		repo, collection := newUserRepo(t)
		noise := validUser()
		_, err := collection.InsertOne(t.Context(), noise)
		require.NoError(t, err)

		got, err := repo.FindByID(t.Context(), bson.NewObjectID())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// GetAll
// ---------------------------------------------------------------------------

func TestUserRepository_GetAll(t *testing.T) {
	t.Run("returns all inserted users", func(t *testing.T) {
		repo, collection := newUserRepo(t)
		user1 := validUser()
		user2 := validUser()
		user2.Email = "user2@abc.com"

		users := []*models.User{user1, user2}
		_, err := collection.InsertMany(t.Context(), users)
		require.NoError(t, err)

		got, err := repo.GetAll(t.Context())

		require.NoError(t, err)
		assert.ElementsMatch(t, users, got)
	})

	// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newUserRepo(t)
		// Force an error by passing an already-expired context
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := repo.GetAll(ctx)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestUserRepository_Update(t *testing.T) {
	t.Run("success - updates target user and ignores decoy", func(t *testing.T) {
		repo, collection := newUserRepo(t)

		target := validUser()
		decoy := validUser()
		decoy.Email = "decoy@abc.com"

		// Poison the well
		_, err := collection.InsertMany(t.Context(), []*models.User{decoy, target})
		require.NoError(t, err)

		// Mutate the target
		target.Name = "Updated Name"
		target.Email = "updated@abc.com"

		_, err = repo.Update(t.Context(), target)
		require.NoError(t, err)

		// Read-Back Target Verification
		updatedTarget := &models.User{}
		err = collection.FindOne(t.Context(), bson.M{"_id": target.ID}).Decode(updatedTarget)

		require.NoError(t, err)
		assert.Equal(t, target, updatedTarget)

		// Vault Lock: Prove Decoy was completely untouched
		untouchedDecoy := &models.User{}
		err = collection.FindOne(t.Context(), bson.M{"_id": decoy.ID}).Decode(untouchedDecoy)

		require.NoError(t, err)
		assert.Equal(t, decoy, untouchedDecoy, "Decoy was corrupted! Update filter is broken.")
	})

	t.Run("error - updating to an existing email returns conflict", func(t *testing.T) {
		repo, collection := newUserRepo(t)

		target := validUser()
		decoy := validUser() // Target wants this email
		decoy.Email = "decoy@abc.com"

		_, err := collection.InsertMany(t.Context(), []*models.User{decoy, target})
		require.NoError(t, err)

		// Mutate target to steal the decoy's email
		target.Email = decoy.Email

		got, err := repo.Update(t.Context(), target)

		// Verify the specific error mapping
		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrConflict)
		assert.Nil(t, got)
	})

	t.Run("error - updating non-existent id returns not-found", func(t *testing.T) {
		repo, collection := newUserRepo(t)

		noise := validUser()
		_, err := collection.InsertOne(t.Context(), noise)
		require.NoError(t, err)

		got, err := repo.Update(t.Context(), validUser())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestUserRepository_Delete(t *testing.T) {
	t.Run("success - deletes exact document and leaves others untouched", func(t *testing.T) {
		repo, collection := newUserRepo(t)

		target := validUser()
		decoy := validUser()
		decoy.Email = "decoy@abc.com"

		// Poison the well
		_, err := collection.InsertMany(t.Context(), []*models.User{decoy, target})
		require.NoError(t, err)

		err = repo.Delete(t.Context(), target.ID)
		require.NoError(t, err)

		// Verify target is gone
		count, err := collection.CountDocuments(t.Context(), bson.M{"_id": target.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)

		// Vault Lock: Verify decoy remains
		untouchedDecoy := &models.User{}
		err = collection.FindOne(t.Context(), bson.M{"_id": decoy.ID}).Decode(untouchedDecoy)

		require.NoError(t, err)
		assert.Equal(t, decoy, untouchedDecoy)
	})

	t.Run("error - non-existent id returns not-found error", func(t *testing.T) {
		repo, _ := newUserRepo(t)

		err := repo.Delete(t.Context(), bson.NewObjectID())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
	})
}
