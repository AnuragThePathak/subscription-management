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

var defaultSubID = bson.NewObjectID()

// validBill returns a valid Bill with a new ObjectID ready for insertion.
func validBill() *models.Bill {
	return &models.Bill{
		ID:             bson.NewObjectID(),
		Amount:         999,
		Currency:       models.USD,
		SubscriptionID: defaultSubID,
		StartDate:      mockToday,
		EndDate:        mockOneMonthLater,
		Status:         models.Paid,
		CreatedAt:      mockTime,
		UpdatedAt:      mockTime,
	}
}

// newBillRepo creates a fresh BillRepository backed by a uniquely named
// database so tests never share state. Dropped at the end of the test.
func newBillRepo(t *testing.T) (repositories.BillRepository, *mongo.Collection) {
	t.Helper()

	dbName := "bill_test_" + bson.NewObjectID().Hex()
	db := mongoClient.Database(dbName)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	repo, err := repositories.NewBillRepository(ctx, db)
	require.NoError(t, err, "NewBillRepository should not error")

	return repo, db.Collection("bills")
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestBillRepository_Create(t *testing.T) {
	t.Run("success - bill inserted and verified in db", func(t *testing.T) {
		repo, collection := newBillRepo(t)
		bill := validBill()

		_, err := repo.Create(t.Context(), bill)
		require.NoError(t, err)

		// Read-Back Verification
		savedBill := &models.Bill{}
		err = collection.FindOne(t.Context(), bson.M{"_id": bill.ID}).Decode(savedBill)

		require.NoError(t, err)
		assert.Equal(t, bill, savedBill)
	})

	t.Run("error - duplicate key returns conflict", func(t *testing.T) {
		repo, _ := newBillRepo(t)
		bill := validBill()

		_, err := repo.Create(t.Context(), bill)
		require.NoError(t, err)

		// Insert exactly the same ID again
		got, err := repo.Create(t.Context(), bill)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrConflict)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func TestBillRepository_GetByID(t *testing.T) {
	t.Run("success - found exact bill and ignores decoy", func(t *testing.T) {
		repo, collection := newBillRepo(t)

		target := validBill()
		decoy := validBill()
		_, err := collection.InsertMany(t.Context(), []*models.Bill{decoy, target})
		require.NoError(t, err)

		got, err := repo.GetByID(t.Context(), target.ID)

		require.NoError(t, err)
		assert.Equal(t, target, got)
	})

	t.Run("error - not found returns not-found error", func(t *testing.T) {
		repo, collection := newBillRepo(t)
		noise := validBill()
		_, err := collection.InsertOne(t.Context(), noise)
		require.NoError(t, err)

		got, err := repo.GetByID(t.Context(), bson.NewObjectID())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// GetRecentBill
// ---------------------------------------------------------------------------

func TestBillRepository_GetRecentBill(t *testing.T) {
	t.Run("success - retrieves most recent PAID bill for specific subscription", func(t *testing.T) {
		repo, collection := newBillRepo(t)
		targetSubID := defaultSubID

		// 1. Decoy: Wrong Subscription ID, but is Paid and very recent.
		decoyWrongSub := validBill()
		decoyWrongSub.SubscriptionID = bson.NewObjectID() // Break collision
		decoyWrongSub.StartDate = mockTomorrow

		// 2. Decoy: Right Sub, but REFUNDED
		// (even though it's the newest, it should be ignored).
		decoyUnpaid := validBill()
		decoyUnpaid.Status = models.Refunded
		decoyUnpaid.StartDate = mockTomorrow

		// 3. Decoy: Right Sub, Paid, but older. (Tests the Sort order!)
		decoyOlderPaid := validBill()
		decoyOlderPaid.StartDate = mockYesterday // Use a time before mockToday

		// 4. Target: Right Sub, Paid, Most Recent valid one.
		targetBill := validBill()

		// POISON THE WELL: Insert all 3 decoys before the target
		_, err := collection.InsertMany(
			t.Context(),
			[]*models.Bill{decoyWrongSub, decoyUnpaid, decoyOlderPaid, targetBill},
		)
		require.NoError(t, err)

		got, err := repo.GetRecentBill(t.Context(), targetSubID)

		require.NoError(t, err)
		assert.Equal(t, targetBill, got, "Failed to get the most recent paid bill. A decoy breached the filter/sort.")
	})

	t.Run("error - no paid bills exist for sub returns not-found", func(t *testing.T) {
		repo, collection := newBillRepo(t)

		// Insert an REFUNDED bill to prove it doesn't accidentally return it
		decoyUnpaid := validBill()
		decoyUnpaid.Status = models.Refunded
		_, err := collection.InsertOne(t.Context(), decoyUnpaid)
		require.NoError(t, err)

		got, err := repo.GetRecentBill(t.Context(), defaultSubID)

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestBillRepository_Update(t *testing.T) {
	t.Run("success - updates target bill and ignores decoy", func(t *testing.T) {
		repo, collection := newBillRepo(t)

		target := validBill()
		decoy := validBill()
		
		// Poison the well
		_, err := collection.InsertMany(t.Context(), []*models.Bill{decoy, target})
		require.NoError(t, err)

		// Mutate target
		target.Status = models.Refunded
		target.Amount = 0

		_, err = repo.Update(t.Context(), target)
		require.NoError(t, err)

		// Read-Back Target Verification
		updatedTarget := &models.Bill{}
		err = collection.FindOne(t.Context(), bson.M{"_id": target.ID}).Decode(updatedTarget)
		
		require.NoError(t, err)
		assert.Equal(t, target, updatedTarget)

		// Vault Lock: Prove Decoy was completely untouched
		untouchedDecoy := &models.Bill{}
		err = collection.FindOne(t.Context(), bson.M{"_id": decoy.ID}).Decode(untouchedDecoy)
		
		require.NoError(t, err)
		assert.Equal(t, decoy, untouchedDecoy, "Decoy was corrupted! Update filter is broken.")
	})

	t.Run("error - updating non-existent id returns not-found", func(t *testing.T) {
		repo, collection := newBillRepo(t)
		
		noise := validBill()
		_, err := collection.InsertOne(t.Context(), noise)
		require.NoError(t, err)

		got, err := repo.Update(t.Context(), validBill())

		require.Error(t, err)
		assertAppErrorCode(t, err, apperror.ErrNotFound)
		assert.Nil(t, got)
	})
}
