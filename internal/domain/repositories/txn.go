package repositories

import (
	"context"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type TxnFn func(ctx context.Context, fn func(ctx context.Context) error) error

type TxnExecutor interface {
	// WithTransaction runs fn inside a MongoDB session/transaction.
	// If fn returns an error, the transaction is aborted; otherwise it's committed.
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type mongoTxnExecutor struct {
	client *mongo.Client
}

func NewTxnExecutor(client *mongo.Client) TxnExecutor {
	return &mongoTxnExecutor{client: client}
}

func (e *mongoTxnExecutor) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := e.client.StartSession()
	if err != nil {
		return apperror.NewInternalError(err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sCtx context.Context) (any, error) {
		return nil, fn(sCtx)
	})
	return err
}
