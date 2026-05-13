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

// validSub returns a valid Active subscription
func validSub() *models.Subscription {
	return &models.Subscription{
		ID:        bson.NewObjectID(),
		Name:      "Netflix",
		Price:     999,
		Currency:  models.USD,
		Frequency: models.Monthly,
		Category:  models.Entertainment,
		Status:    models.Active,
		ValidTill: mockOneMonthLater,
		UserID:    bson.NewObjectID(),
		CreatedAt: mockTime,
		UpdatedAt: mockTime,
	}
}

// validCanceledSub returns a Canceled subscription with the given valid_till.
func validCanceledSub() *models.Subscription {
	s := validSub()
	s.Status = models.Canceled
	return s
}

func validExpiredSub() *models.Subscription {
	s := validSub()
	s.Status = models.Expired
	s.ValidTill = mockToday
	return s
}

// newSubRepo creates a fresh SubscriptionRepository backed by a uniquely named
// database. The database is dropped when the test ends.
//
// mongoClient is declared in bill_integration_test.go (same package, same TestMain).
func newSubRepo(t *testing.T) (repositories.SubscriptionRepository, *mongo.Collection) {
	t.Helper()

	dbName := "sub_test_" + bson.NewObjectID().Hex()
	db := mongoClient.Database(dbName)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	repo, err := repositories.NewSubscriptionRepository(ctx, db)
	require.NoError(t, err, "NewSubscriptionRepository should not error")

	return repo, db.Collection("subscriptions")
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_Create(t *testing.T) {
	// Successfully inserted subscription
	t.Run("success - subscription inserted and returned", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub := validSub()

		_, err := repo.Create(t.Context(), sub)

		require.NoError(t, err)
		// The Read-Back Verification
		savedSub := &models.Subscription{}
		err = collection.FindOne(t.Context(), bson.M{"_id": sub.ID}).Decode(savedSub)

		require.NoError(t, err)
		assert.Equal(t, sub, savedSub)
	})

	// Duplicate key error when creating subscription with same ID
	t.Run("error - duplicate key returns conflict", func(t *testing.T) {
		repo, _ := newSubRepo(t)
		sub := validSub()

		_, err := repo.Create(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.Create(t.Context(), sub)
		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrConflict)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_GetByID(t *testing.T) {
	// Happy path: Fetch the subscription by ID
	t.Run("success - found by id", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub := validSub()
		_, err := collection.InsertOne(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.GetByID(t.Context(), sub.ID)

		require.NoError(t, err)
		assert.Equal(t, sub, got)
	})

	// Error
	t.Run("not found - unknown id", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub := validSub()
		_, err := collection.InsertOne(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.GetByID(t.Context(), bson.NewObjectID())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// GetAll
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_GetAll(t *testing.T) {
	// Happy path: Fetch all subscriptions
	t.Run("returns all inserted subscriptions", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		subs := []*models.Subscription{
			validSub(),
			validSub(),
		}
		_, err := collection.InsertMany(t.Context(), subs)
		require.NoError(t, err)

		got, err := repo.GetAll(t.Context())

		require.NoError(t, err)
		assert.ElementsMatch(t, subs, got)
	})

	// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newSubRepo(t)
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
// GetByUserID
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_GetByUserID(t *testing.T) {
	// Successfully retrieved subscriptions for a user
	t.Run("returns only subscriptions for the given user", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		userID := bson.NewObjectID()
		sub1 := validSub()
		sub1.UserID = userID
		sub2 := validSub()
		sub2.UserID = userID
		sub3 := validSub()
		expectedSubs := []*models.Subscription{sub1, sub2}

		_, err := collection.InsertMany(
			t.Context(), []*models.Subscription{sub1, sub2, sub3},
		)
		require.NoError(t, err)

		got, err := repo.GetByUserID(t.Context(), userID)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.ElementsMatch(t, expectedSubs, got)
	})

	/// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newSubRepo(t)
		// Force an error by passing an already-expired context
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := repo.GetByUserID(ctx, bson.NewObjectID())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// GetActiveSubscriptions
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_GetActiveSubscriptions(t *testing.T) {
	// Successfully retrieved active subscriptions
	t.Run("returns active subs with valid_till after the cutoff", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		activeSub1 := validSub()
		activeSub2 := validSub()
		canceledSub := validCanceledSub()
		expiredSub := validExpiredSub()
		expectedSubs := []*models.Subscription{activeSub1, activeSub2}
		_, err := collection.InsertMany(
			t.Context(),
			[]*models.Subscription{activeSub1, activeSub2, canceledSub, expiredSub},
		)
		require.NoError(t, err)

		got, err := repo.GetActiveSubscriptions(t.Context(), mockTime)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.ElementsMatch(t, expectedSubs, got)
	})

	// Ghost subscriptions
	t.Run("excludes subscriptions that are marked active but chronologically expired", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub := validSub()
		sub.ValidTill = mockToday
		_, err := collection.InsertOne(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.GetActiveSubscriptions(t.Context(), mockTime)

		require.NoError(t, err)
		assert.Empty(t, got, "expected empty slice because valid_till is in the past, even though status is active")
	})

	// Boundary condition
	t.Run("boundary - excludes if valid_till is exactly the cutoff time", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub := validSub()
		sub.ValidTill = mockTime // Exactly AT the cutoff
		_, err := collection.InsertOne(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.GetActiveSubscriptions(t.Context(), mockTime)

		require.NoError(t, err)
		assert.Empty(t, got, "expected exact cutoff to be excluded (query should use $gt)")
	})

	// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newSubRepo(t)
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := repo.GetActiveSubscriptions(ctx, mockTime)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// CountActiveSubscriptions
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_CountActiveSubscriptions(t *testing.T) {
	// Successfully count active subscriptions
	t.Run("count matches active subscriptions after cutoff", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		activeSub1 := validSub()
		activeSub2 := validSub()
		canceledSub := validCanceledSub()
		_, err := collection.InsertMany(
			t.Context(),
			[]*models.Subscription{activeSub1, activeSub2, canceledSub},
		)
		require.NoError(t, err)

		got, err := repo.CountActiveSubscriptions(t.Context(), mockTime)

		require.NoError(t, err)
		assert.Equal(t, int64(2), got)
	})

	// Ghost subscriptions
	t.Run("excludes subscriptions that are marked active but chronologically expired", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub := validSub()
		sub.ValidTill = mockToday
		_, err := collection.InsertOne(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.CountActiveSubscriptions(t.Context(), mockTime)

		require.NoError(t, err)
		assert.Equal(t, int64(0), got)
	})

	// Boundary condition
	t.Run("boundary - excludes if valid_till is exactly the cutoff time", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub := validSub()
		sub.ValidTill = mockTime // Exactly AT the cutoff
		_, err := collection.InsertOne(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.CountActiveSubscriptions(t.Context(), mockTime)

		require.NoError(t, err)
		assert.Equal(t, int64(0), got)
	})

	// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newSubRepo(t)
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := repo.CountActiveSubscriptions(ctx, mockTime)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Equal(t, int64(0), got)
	})
}

// ---------------------------------------------------------------------------
// GetSubscriptionsDueForReminder
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_GetSubscriptionsDueForReminder(t *testing.T) {
	// Successfully retrieved subscriptions due for reminder
	t.Run("returns subs expiring within the reminder window", func(t *testing.T) {
		repo, collection := newSubRepo(t)

		// Expires exactly 7 days from now — should be in the [7-day] window.
		sub1 := validSub()
		sub1.ValidTill = mockToday.AddDate(0, 0, 7)
		// Expires exactly 3 days from now — also in the [3-day] window.
		sub2 := validSub()
		sub2.ValidTill = mockToday.AddDate(0, 0, 3)
		// Expires in 15 days — outside both windows.
		sub3 := validSub()
		sub3.ValidTill = mockToday.AddDate(0, 0, 15)
		canceledSub := validCanceledSub()
		expiredSub := validExpiredSub()
		expectedSubs := []*models.Subscription{sub1, sub2}

		_, err := collection.InsertMany(
			t.Context(),
			[]*models.Subscription{sub1, sub2, sub3, canceledSub, expiredSub},
		)
		require.NoError(t, err)

		got, err := repo.GetSubscriptionsDueForReminder(t.Context(), []int{3, 7}, mockTime)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.ElementsMatch(t, expectedSubs, got)
	})

	// Ghost subscriptions
	// We can send reminder email for daysBefore = 0
	// But it's not a valid value for daysBefore as per the design
	// We are supposed to renew on daysBefore = 1 or a few hours into daysBefore = 0
	t.Run("excludes subscriptions that are marked active but chronologically expired", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub := validSub()
		sub.ValidTill = mockToday
		_, err := collection.InsertOne(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.GetSubscriptionsDueForReminder(t.Context(), []int{1}, mockTime)

		require.NoError(t, err)
		assert.Empty(t, got, "expected empty slice because valid_till is in the past, even though status is active")
	})

	// Boundary conditions
	t.Run("boundary - inclusive of start of reminder day and exclusive of end of reminder day ", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		sub1 := validSub()
		sub1.ValidTill = mockTomorrow
		sub2 := validSub()
		sub2.ValidTill = mockTwoDaysLater
		_, err := collection.InsertMany(
			t.Context(), []*models.Subscription{sub1, sub2},
		)
		require.NoError(t, err)

		got, err := repo.GetSubscriptionsDueForReminder(t.Context(), []int{1}, mockTime)

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, sub1, got[0])
	})

	// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newSubRepo(t)
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := repo.GetSubscriptionsDueForReminder(ctx, []int{1, 3, 7}, mockTime)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// GetSubscriptionsDueForRenewal
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_GetSubscriptionsDueForRenewal(t *testing.T) {
	// Successfully retrived subscriptions due renewal
	t.Run("returns active subs with valid_till in the renewal window", func(t *testing.T) {
		repo, collection := newSubRepo(t)
		windowStart := mockToday
		windowEnd := mockTomorrow

		sub1 := validSub()
		sub1.ValidTill = mockToday
		sub2 := validSub()
		sub2.ValidTill = mockTomorrow
		sub3 := validSub()
		canceledSub := validCanceledSub()
		canceledSub.ValidTill = mockTomorrow

		_, err := collection.InsertMany(
			t.Context(), []any{sub1, sub2, sub3, canceledSub},
		)
		require.NoError(t, err)

		got, err := repo.GetSubscriptionsDueForRenewal(t.Context(), windowStart, windowEnd)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, sub1, got[0])
		assert.Equal(t, sub2, got[1])
	})

	// Boundary condition
	t.Run("boundary - inclusive of start and end times", func(t *testing.T) {
		repo, collection := newSubRepo(t)

		subStart := validSub()
		subStart.ValidTill = mockToday // Exactly at startTime

		subEnd := validSub()
		subEnd.ValidTill = mockTomorrow // Exactly at endTime

		_, err := collection.InsertMany(t.Context(), []any{subStart, subEnd})
		require.NoError(t, err)

		got, err := repo.GetSubscriptionsDueForRenewal(t.Context(), mockToday, mockTomorrow)

		require.NoError(t, err)
		require.Len(t, got, 2)
	})

	// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newSubRepo(t)
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := repo.GetSubscriptionsDueForRenewal(ctx, mockToday, mockTomorrow)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

// // ---------------------------------------------------------------------------
// // GetCanceledExpiredSubscriptions
// // ---------------------------------------------------------------------------

func TestSubscriptionRepository_GetCanceledExpiredSubscriptions(t *testing.T) {
	// Successfully retrieved subscriptions due for reminder
	t.Run("returns only canceled subs expired before the cutoff", func(t *testing.T) {
		repo, collection := newSubRepo(t)

		// Target: Canceled AND Expired
		targetSub := validCanceledSub()
		targetSub.ValidTill = mockOneMonthAgo // 1 month ago

		// Decoy 1: Canceled but NOT Expired yet
		decoyFuture := validCanceledSub()

		// Decoy 2: Expired but NOT Canceled (Active)
		decoyActive := validSub()
		decoyActive.ValidTill = mockOneMonthAgo // 1 month ago

		_, err := collection.InsertMany(t.Context(), []any{targetSub, decoyFuture, decoyActive})
		require.NoError(t, err)

		got, err := repo.GetCanceledExpiredSubscriptions(t.Context(), mockTime)

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, targetSub.ID, got[0].ID)
	})

	// Boundary condition
	t.Run("boundary - strictly excludes exact cutoff time", func(t *testing.T) {
		repo, collection := newSubRepo(t)

		sub := validCanceledSub()
		sub.ValidTill = mockToday // Exactly AT the cutoff

		_, err := collection.InsertOne(t.Context(), sub)
		require.NoError(t, err)

		got, err := repo.GetCanceledExpiredSubscriptions(t.Context(), mockToday)

		require.NoError(t, err)
		assert.Empty(t, got, "expected empty slice because query uses $lt, not $lte")
	})

	// Error: Infrastructure failure / Timeout
	t.Run("returns error when database operation fails", func(t *testing.T) {
		repo, _ := newSubRepo(t)
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
		defer cancel()

		got, err := repo.GetCanceledExpiredSubscriptions(ctx, mockTime)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrTimeout)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_Update(t *testing.T) {
	t.Run("success - updates target subscription and ignores decoy", func(t *testing.T) {
		repo, collection := newSubRepo(t)

		target := validSub()
		decoy := validSub() // Has same Status, Frequency, etc.
		_, err := collection.InsertMany(t.Context(), []any{target, decoy})
		require.NoError(t, err)

		// Mutate the target
		target.Status = models.Canceled
		target.Price = 0

		_, err = repo.Update(t.Context(), target)

		require.NoError(t, err)
		updatedTarget := &models.Subscription{}
		err = collection.FindOne(t.Context(), bson.M{"_id": target.ID}).Decode(updatedTarget)
		require.NoError(t, err)
		assert.Equal(t, target, updatedTarget)

		// Vault Lock: Prove Decoy was completely untouched
		untouchedDecoy := &models.Subscription{}
		err = collection.FindOne(t.Context(), bson.M{"_id": decoy.ID}).Decode(untouchedDecoy)
		require.NoError(t, err)
		assert.Equal(t, decoy, untouchedDecoy, "Decoy was corrupted! Update filter is broken.")
	})

	t.Run("not found - updating non-existent id returns not-found error", func(t *testing.T) {
		repo, collection := newSubRepo(t)

		noise := validSub()
		_, err := collection.InsertOne(t.Context(), noise)
		require.NoError(t, err)

		got, err := repo.Update(t.Context(), validSub())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestSubscriptionRepository_Delete(t *testing.T) {
	t.Run("success - deletes exact document and leaves others untouched", func(t *testing.T) {
		repo, collection := newSubRepo(t)

		target := validSub()
		decoy := validSub()
		_, err := collection.InsertMany(t.Context(), []any{target, decoy})
		require.NoError(t, err)

		err = repo.Delete(t.Context(), target.ID)
		require.NoError(t, err)

		// Verify target is gone
		count, err := collection.CountDocuments(t.Context(), bson.M{"_id": target.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)

		// Vault Lock: Verify decoy remains
		untouchedDecoy := &models.Subscription{}
		err = collection.FindOne(t.Context(), bson.M{"_id": decoy.ID}).Decode(untouchedDecoy)
		require.NoError(t, err)
		assert.Equal(t, decoy, untouchedDecoy)
	})

	t.Run("error - non-existent id returns not-found error", func(t *testing.T) {
		repo, _ := newSubRepo(t)

		err := repo.Delete(t.Context(), bson.NewObjectID())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
	})
}
