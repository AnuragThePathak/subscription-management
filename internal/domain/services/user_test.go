package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	repomocks "github.com/anuragthepathak/subscription-management/internal/domain/repositories/mocks"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	svcmocks "github.com/anuragthepathak/subscription-management/internal/domain/services/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

// validCreateInput returns a pristine user struct meant for CreateUser input.
// It lacks an ID and timestamps, and has a plain-text password.
func validCreateInput() *models.User {
	return &models.User{
		Name:     "Alice",
		Email:    defaultUserEmail,
		Password: "password123",
	}
}

// validUser returns a fully hydrated user struct as it would appear in the DB.
// It has a populated ID, timestamps, and a dummy hashed password.
func validUser() *models.User {
	return &models.User{
		ID:        defaultUserID,
		Name:      "Alice",
		Email:     defaultUserEmail,
		Password:  "hashed-password",
		CreatedAt: mockTime,
		UpdatedAt: mockTime,
	}
}

// newService is a convenience constructor that wires up a userService with the
// provided mocks so individual tests don't need to repeat the wiring.
func newService(
	repo *repomocks.MockUserRepository,
	subSvc *svcmocks.MockSubscriptionServiceInternal,
) services.UserService {
	return services.NewUserService(repo, subSvc, func() time.Time { return mockTime })
}

// ---------------------------------------------------------------------------
// CreateUser
// ---------------------------------------------------------------------------

func Test_userService_CreateUser(t *testing.T) {
	buildMatcher := func(input models.User) any {
		return mock.MatchedBy(func(u *models.User) bool {
			// Check exact matches for known data
			isStaticValid := u.Name == input.Name &&
				u.Email == input.Email &&
				u.CreatedAt.Equal(mockTime) &&
				u.UpdatedAt.Equal(mockTime)

			// Check that dynamic data was generated
			isDynamicValid := u.ID != bson.NilObjectID &&
				u.Password != input.Password &&
				u.Password != ""

			return isStaticValid && isDynamicValid
		})
	}
	tests := []struct {
		name            string
		input           *models.User
		setupMocks      func(repo *repomocks.MockUserRepository, input models.User)
		wantErr         bool
		wantErrCode     apperror.ErrorCode
		wantEnrichedErr bool
		// assertResult is called only on the success path.
		assertResult func(t *testing.T, input models.User, got *models.User)
	}{
		{
			// Happy path: Proves that when the database confirms the email is free
			// by returning ErrNotFound, the Service correctly bypasses the error
			// block and proceeds to create the user.
			name:  "success - new user created (bypasses ErrNotFound)",
			input: validCreateInput(),
			setupMocks: func(repo *repomocks.MockUserRepository, input models.User) {
				repo.EXPECT().
					FindByEmail(mock.Anything, input.Email).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()

				repo.EXPECT().
					Create(mock.Anything, buildMatcher(input)).
					RunAndReturn(func(_ context.Context, u *models.User) (*models.User, error) {
						return u, nil // echo the user back, as the real repo does
					}).
					Once()
			},
			assertResult: func(t *testing.T, input models.User, got *models.User) {
				t.Helper()
				assert.Equal(t, input.Email, got.Email)
				assert.Equal(t, input.Name, got.Name)
				// Password must have been hashed; plain text must not be stored.
				assert.NotEqual(t, input.Password, got.Password, "password must be hashed")
				// Timestamps must match the fixed clock.
				assert.Equal(t, mockTime, got.CreatedAt)
				assert.Equal(t, mockTime, got.UpdatedAt)
				// ID must have been assigned.
				assert.NotEqual(t, bson.NilObjectID, got.ID)
			},
		},
		{
			// Email is already registered → conflict error.
			name:  "error - email already in use",
			input: validCreateInput(),
			setupMocks: func(repo *repomocks.MockUserRepository, input models.User) {
				repo.EXPECT().
					FindByEmail(mock.Anything, input.Email).
					Return(&models.User{Email: input.Email}, nil). // user found → conflict
					Once()
			},
			wantErr:         true,
			wantErrCode:     apperror.ErrConflict,
			wantEnrichedErr: true,
		},
		{
			// FindByEmail fails with a non-NotFound app error (e.g. DB error).
			name:  "error - repository FindByEmail returns db error (non-NotFound AppError)",
			input: validCreateInput(),
			setupMocks: func(repo *repomocks.MockUserRepository, input models.User) {
				repo.EXPECT().
					FindByEmail(mock.Anything, input.Email).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:         true,
			wantErrCode:     apperror.ErrDB,
			wantEnrichedErr: true,
		},
		{
			// FindByEmail fails with an unknown error (not an AppError)
			name:  "error - repository FindByEmail returns unknown raw error",
			input: validCreateInput(),
			setupMocks: func(repo *repomocks.MockUserRepository, input models.User) {
				repo.EXPECT().
					FindByEmail(mock.Anything, input.Email).
					Return(nil, errors.New("complete system meltdown")).
					Once()
			},
			wantErr: true,
		},
		{
			// Password hashing fails (password too long)
			name: "error - password hashing fails (password too long)",
			input: func() *models.User {
				u := validCreateInput()
				// bcrypt fails if the password is strictly > 72 bytes
				u.Password = string(make([]byte, 73))
				return u
			}(),
			setupMocks: func(repo *repomocks.MockUserRepository, input models.User) {
				// The email check passes
				repo.EXPECT().
					FindByEmail(mock.Anything, input.Email).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()

				// We do NOT EXPECT Create() to be called, because the hashing will fail first!
			},
			wantErr:         true,
			wantErrCode:     apperror.ErrInternal,
			wantEnrichedErr: true,
		},
		{
			// repo.Create fails after the email check passes due to AppError.
			name:  "error - repository Create fails due to error of type AppError",
			input: validCreateInput(),
			setupMocks: func(repo *repomocks.MockUserRepository, input models.User) {
				repo.EXPECT().
					FindByEmail(mock.Anything, input.Email).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()

				repo.EXPECT().
					Create(mock.Anything, buildMatcher(input)).
					Return(nil, apperror.NewDBError(errors.New("insert failed"))).
					Once()
			},
			wantErr:         true,
			wantErrCode:     apperror.ErrDB,
			wantEnrichedErr: true,
		},
		{
			// repo.Create fails after the email check passes due to unknown error type.
			name:  "error - repository Create fails due to error of unknown type",
			input: validCreateInput(),
			setupMocks: func(repo *repomocks.MockUserRepository, input models.User) {
				repo.EXPECT().
					FindByEmail(mock.Anything, input.Email).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()

				repo.EXPECT().
					Create(mock.Anything, buildMatcher(input)).
					Return(nil, errors.New("connection-lost")).
					Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repomocks.NewMockUserRepository(t)
			subSvc := svcmocks.NewMockSubscriptionServiceInternal(t)

			var inputSnapshot models.User
			if tt.input != nil {
				inputSnapshot = *tt.input
			}
			tt.setupMocks(repo, inputSnapshot)

			svc := newService(repo, subSvc)
			got, err := svc.CreateUser(t.Context(), tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
					if tt.wantEnrichedErr {
						assert.NotEmpty(t, appErr.LogAttributes(),
							"expected error to be enriched with log attributes",
						)
					}
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			if tt.assertResult != nil {
				tt.assertResult(t, inputSnapshot, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetAllUsers
// ---------------------------------------------------------------------------

func Test_userService_GetAllUsers(t *testing.T) {
	user2ID := bson.NewObjectID()
	validUsers := func() []*models.User {
		user1 := validUser()
		user2 := validUser()
		user2.ID = user2ID
		user2.Email = "new-email@example.com"
		return []*models.User{user1, user2}
	}

	tests := []struct {
		name        string
		setupMocks  func(repo *repomocks.MockUserRepository)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantUsers   []*models.User
	}{
		// Success
		{
			name: "success - repository GetAll returns the data",
			setupMocks: func(repo *repomocks.MockUserRepository) {
				repo.EXPECT().
					GetAll(mock.Anything).
					Return(validUsers(), nil).
					Once()
			},
			wantErr:   false,
			wantUsers: validUsers(),
		},
		// Repo returns a DB error
		{
			name: "error - repository GetAll returns db error",
			setupMocks: func(repo *repomocks.MockUserRepository) {
				repo.EXPECT().
					GetAll(mock.Anything).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := repomocks.NewMockUserRepository(t)
			subSvc := svcmocks.NewMockSubscriptionServiceInternal(t)
			tt.setupMocks(userRepo)

			svc := newService(userRepo, subSvc)
			got, err := svc.GetAllUsers(t.Context())

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(),
						tt.wantErrCode,
					)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantUsers, got)
		})
	}
}

// ---------------------------------------------------------------------------
// GetUserByID
// ---------------------------------------------------------------------------

func Test_userService_GetUserByID(t *testing.T) {
	tests := []struct {
		name          string
		id            string
		claimedUserID string
		parsedID      bson.ObjectID
		setupMocks    func(repo *repomocks.MockUserRepository, id bson.ObjectID)
		wantErr       bool
		wantErrCode   apperror.ErrorCode
		wantUser      *models.User
	}{
		{
			// Caller owns the resource and ID is valid.
			name:          "success - owner retrieves own profile",
			id:            defaultUserHex,
			claimedUserID: defaultUserHex,
			parsedID:      defaultUserID,
			setupMocks: func(repo *repomocks.MockUserRepository, id bson.ObjectID) {
				repo.EXPECT().
					FindByID(mock.Anything, id).
					Return(validUser(), nil).
					Once()
			},
			wantUser: validUser(),
		},
		{
			// id != claimedUserID → forbidden before any repo call.
			name:          "error - caller does not own resource",
			id:            defaultUserHex,
			claimedUserID: bson.NewObjectID().Hex(), // different user
			setupMocks:    func(_ *repomocks.MockUserRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrForbidden,
		},
		{
			// id is not a valid hex ObjectID → authorization check first, then bad format.
			name:          "error - malformed id string",
			id:            "not-a-valid-objectid",
			claimedUserID: "not-a-valid-objectid", // same value so ownership passes
			setupMocks:    func(_ *repomocks.MockUserRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrUnauthorized,
		},
		{
			// Repo returns a not-found error.
			name:          "error - user not found in repository",
			id:            defaultUserHex,
			claimedUserID: defaultUserHex,
			parsedID:      defaultUserID,
			setupMocks: func(repo *repomocks.MockUserRepository, id bson.ObjectID) {
				repo.EXPECT().
					FindByID(mock.Anything, id).
					Return(nil, apperror.NewNotFoundError("user not found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repomocks.NewMockUserRepository(t)
			subSvc := svcmocks.NewMockSubscriptionServiceInternal(t)
			tt.setupMocks(repo, tt.parsedID)

			svc := newService(repo, subSvc)
			got, err := svc.GetUserByID(t.Context(), tt.id, tt.claimedUserID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s", appErr.Code(), tt.wantErrCode)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantUser, got)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteUser
// ---------------------------------------------------------------------------

func Test_userService_DeleteUser(t *testing.T) {
	tests := []struct {
		name          string
		id            string
		claimedUserID string
		parsedID      bson.ObjectID
		setupSubSvc   func(subSvc *svcmocks.MockSubscriptionServiceInternal, id bson.ObjectID)
		setupRepo     func(repo *repomocks.MockUserRepository, id bson.ObjectID)
		wantErr       bool
		wantErrCode   apperror.ErrorCode
	}{
		{
			// Happy path: caller owns the account, no active subs, repo.Delete succeeds.
			name:          "success - user with no active subscriptions deleted",
			id:            defaultUserHex,
			claimedUserID: defaultUserHex,
			parsedID:      defaultUserID,
			setupSubSvc: func(subSvc *svcmocks.MockSubscriptionServiceInternal, id bson.ObjectID) {
				subSvc.EXPECT().
					HasActiveSubscriptionsInternal(mock.Anything, id).
					Return(false, nil).
					Once()
			},
			setupRepo: func(repo *repomocks.MockUserRepository, id bson.ObjectID) {
				repo.EXPECT().
					Delete(mock.Anything, id).
					Return(nil).
					Once()
			},
		},
		{
			// Caller tries to delete another user's account.
			name:          "error - caller does not own the account",
			id:            defaultUserHex,
			claimedUserID: bson.NewObjectID().Hex(),
			setupSubSvc:   func(_ *svcmocks.MockSubscriptionServiceInternal, _ bson.ObjectID) {},
			setupRepo:     func(_ *repomocks.MockUserRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrForbidden,
		},
		{
			// id is not a valid hex string (ownership passes because both are the same bad hex).
			name:          "error - malformed id string",
			id:            "bad-hex",
			claimedUserID: "bad-hex",
			setupSubSvc:   func(_ *svcmocks.MockSubscriptionServiceInternal, _ bson.ObjectID) {},
			setupRepo:     func(_ *repomocks.MockUserRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrUnauthorized,
		},
		{
			// User has at least one active subscription → deletion blocked.
			name:          "error - user has active subscriptions",
			id:            defaultUserHex,
			claimedUserID: defaultUserHex,
			parsedID:      defaultUserID,
			setupSubSvc: func(subSvc *svcmocks.MockSubscriptionServiceInternal, id bson.ObjectID) {
				subSvc.EXPECT().
					HasActiveSubscriptionsInternal(mock.Anything, id).
					Return(true, nil).
					Once()
			},
			setupRepo:   func(_ *repomocks.MockUserRepository, _ bson.ObjectID) {},
			wantErr:     true,
			wantErrCode: apperror.ErrConflict,
		},
		{
			// HasActiveSubscriptionsInternal itself returns an error.
			name:          "error - subscription service returns error",
			id:            defaultUserHex,
			claimedUserID: defaultUserHex,
			parsedID:      defaultUserID,
			setupSubSvc: func(subSvc *svcmocks.MockSubscriptionServiceInternal, id bson.ObjectID) {
				subSvc.EXPECT().
					HasActiveSubscriptionsInternal(mock.Anything, id).
					Return(false, apperror.NewDBError(errors.New("subscription lookup failed"))).
					Once()
			},
			setupRepo:   func(_ *repomocks.MockUserRepository, _ bson.ObjectID) {},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
		{
			// repo.Delete fails (e.g. user was already deleted concurrently).
			name:          "error - repository Delete returns not found",
			id:            defaultUserHex,
			claimedUserID: defaultUserHex,
			parsedID:      defaultUserID,
			setupSubSvc: func(subSvc *svcmocks.MockSubscriptionServiceInternal, id bson.ObjectID) {
				subSvc.EXPECT().
					HasActiveSubscriptionsInternal(mock.Anything, id).
					Return(false, nil).
					Once()
			},
			setupRepo: func(repo *repomocks.MockUserRepository, id bson.ObjectID) {
				repo.EXPECT().
					Delete(mock.Anything, id).
					Return(apperror.NewNotFoundError("user not found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repomocks.NewMockUserRepository(t)
			subSvc := svcmocks.NewMockSubscriptionServiceInternal(t)
			tt.setupSubSvc(subSvc, tt.parsedID)
			tt.setupRepo(repo, tt.parsedID)

			svc := newService(repo, subSvc)
			err := svc.DeleteUser(t.Context(), tt.id, tt.claimedUserID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(),
						tt.wantErrCode,
					)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// FetchUserByIDInternal
// ---------------------------------------------------------------------------

func TestUserService_FetchUserByIDInternal(t *testing.T) {
	tests := []struct {
		name        string
		id          bson.ObjectID
		setupMocks  func(repo *repomocks.MockUserRepository, id bson.ObjectID)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantUser    *models.User
	}{
		{
			// Success - repository FindByID returns the data
			name: "success - repository FindByID returns the data",
			id:   defaultUserID,
			setupMocks: func(repo *repomocks.MockUserRepository, id bson.ObjectID) {
				repo.EXPECT().
					FindByID(mock.Anything, id).
					Return(validUser(), nil).
					Once()
			},
			wantErr:  false,
			wantUser: validUser(),
		},
		{
			// Repo returns a DB error
			name: "error - repository FindByID returns db error",
			id:   defaultUserID,
			setupMocks: func(repo *repomocks.MockUserRepository, id bson.ObjectID) {
				repo.EXPECT().
					FindByID(mock.Anything, id).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repomocks.NewMockUserRepository(t)
			subSvc := svcmocks.NewMockSubscriptionServiceInternal(t)
			tt.setupMocks(repo, tt.id)

			svc := newService(repo, subSvc)
			got, err := svc.FetchUserByIDInternal(t.Context(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s", appErr.Code(), tt.wantErrCode)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantUser, got)
		})
	}
}

// ---------------------------------------------------------------------------
// FindUserByEmailInternal
// ---------------------------------------------------------------------------

func TestUserService_FindUserByEmailInternal(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		setupMocks  func(repo *repomocks.MockUserRepository, email string)
		wantUser    *models.User
		wantErr     bool
		wantErrCode apperror.ErrorCode
	}{
		{
			// Success - repo.FindByEmail returns the data
			name:  "success - repository FindByEmail returns the data",
			email: defaultUserEmail,
			setupMocks: func(repo *repomocks.MockUserRepository, email string) {
				repo.EXPECT().
					FindByEmail(mock.Anything, email).
					Return(validUser(), nil).
					Once()
			},
			wantUser: validUser(),
		},
		{
			// Repo returns a DB error
			name:  "error - repository FindByEmail returns db error",
			email: defaultUserEmail,
			setupMocks: func(repo *repomocks.MockUserRepository, email string) {
				repo.EXPECT().
					FindByEmail(mock.Anything, email).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repomocks.NewMockUserRepository(t)
			subSvc := svcmocks.NewMockSubscriptionServiceInternal(t)
			tt.setupMocks(repo, tt.email)

			svc := newService(repo, subSvc)
			got, err := svc.FetchUserByEmailInternal(t.Context(), tt.email)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(),
						tt.wantErrCode,
					)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantUser, got)
		})
	}
}
