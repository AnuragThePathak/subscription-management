# Testing Philosophy

This document explains the testing strategy, the specific patterns used, and *why* the tests are structured the way they are. The goal is not just coverage numbers — it's proving correctness under adversarial conditions while keeping the test suite fast and maintainable.

---

## Two-Tier Strategy

| Tier | Layer | What It Proves | Infrastructure | Speed |
|---|---|---|---|---|
| **Unit** | Services, Controllers, Middleware | Business logic correctness with mocked dependencies | None (pure Go) | ~3s |
| **Integration** | Repositories | Query filters, sort orders, boundary conditions, and error mapping against a real database | Testcontainers (Docker) | ~15s |

The split is deliberate: services are tested with mocks because they orchestrate behavior, and the interesting bugs are in the branching logic. Repositories are tested against real MongoDB because the interesting bugs are in the query filters — `$gt` vs `$gte`, sort direction, multi-field filters — and mocking those would just be testing your assumptions about the driver.

```bash
make test          # Unit tests only (~3s)
make integration   # Integration tests only (requires Docker)
make test-all      # Both with race detector
```

---

## Unit Test Patterns — Service Layer

### Table-Driven Tests with Closure-Based Mock Setup

Every service test follows a consistent structure: a `tests` slice of structs, each containing a `setupMocks` closure. This keeps mock configuration **co-located with the test scenario** rather than scattered across helper functions:

```go
tests := []struct {
    name          string
    input         *models.Subscription
    claimedUserID string
    setupMocks    func(
        subRepo  *repomocks.MockSubscriptionRepository,
        billRepo *repomocks.MockBillRepository,
        metrics  *svcmocks.MockSubscriptionMetrics,
        input    models.Subscription,
        userID   bson.ObjectID,
    )
    wantErr     bool
    wantErrCode apperror.ErrorCode
    assertResult func(t *testing.T, input models.Subscription, got *models.Subscription, userID bson.ObjectID)
}{
    {
        name:  "success - subscription and bill created",
        setupMocks: func(...) {
            billRepo.EXPECT().Create(mock.Anything, buildBillMatcher(input)).RunAndReturn(...)
            subRepo.EXPECT().Create(mock.Anything, buildMatcher(input, userID)).RunAndReturn(...)
            metrics.EXPECT().IncSubscriptionsCreated(mock.Anything).Once()
        },
        assertResult: func(t *testing.T, input models.Subscription, got *models.Subscription, ...) {
            assert.Equal(t, input.Name, got.Name)
            assert.Equal(t, models.Active, got.Status)
            // ...
        },
    },
    {
        name:  "error - bill repository Create fails",
        setupMocks: func(...) {
            billRepo.EXPECT().Create(mock.Anything, ...).
                Return(nil, apperror.NewDBError(errors.New("insert failed"))).Once()
        },
        wantErr:     true,
        wantErrCode: apperror.ErrDB,
    },
}
```

**Why closures?** Each test case has different mock expectations. Closures let you express "in *this* scenario, the bill repo returns an error" right next to the test case name, without a separate setup function per scenario.

### Mutation Prevention via Input Snapshot

Before passing `tt.input` to `setupMocks`, the test runner takes a **value copy**:

```go
var inputSnapshot models.Subscription
if tt.input != nil {
    inputSnapshot = *tt.input  // copy BEFORE the service can mutate it
}
tt.setupMocks(subRepo, billRepo, metrics, inputSnapshot, tt.parsedUserID)
```

This protects against a subtle class of bugs: if the service mutates the input struct (e.g., setting `CreatedAt`, `Status`), the mock matchers still compare against the **original** values. Without the snapshot, a mutation in the service could accidentally satisfy a matcher that should have failed.

The same pattern appears in `CancelSubscription` tests, where `wantSub` is snapshot-copied before being passed to mock setup:

```go
var expectedSub models.Subscription
if tt.wantSub != nil {
    expectedSub = *tt.wantSub  // snapshot before mock wiring
}
tt.setupMocks(subRepo, billRepo, metrics, tt.parsedSubID, expectedSub)
```

### Typed Error Code Assertions

Every error test case asserts both the error's existence *and* its specific `ErrorCode`:

```go
if tt.wantErr {
    require.Error(t, err)
    if appErr, ok := errors.AsType[apperror.AppError](err); ok {
        assert.Equal(t, tt.wantErrCode, appErr.Code())
    } else {
        // Safety net: if we expected an AppError code but got a raw error, fail explicitly
        assert.Empty(t, tt.wantErrCode,
            "test case defined a wantErrCode (%s), but received raw error: %v",
            tt.wantErrCode, err)
    }
    assert.Nil(t, got)
    return
}
```

This catches two classes of bugs:
1. **Wrong error type**: The service returns a raw `error` where it should return an `AppError` (the safety net clause).
2. **Wrong error code**: The service returns `ErrBadRequest` where it should return `ErrUnauthorized` — these map to different HTTP status codes, so getting the code wrong is a real bug.

### Structural Matchers (Not `mock.Anything`)

Happy-path tests use `mock.MatchedBy` closures to verify the **shape** of objects passed to mocks, not just that *something* was passed:

```go
buildBillMatcher := func(input models.Subscription) any {
    return mock.MatchedBy(func(b *models.Bill) bool {
        isStaticValid := b.Amount == input.Price &&
            b.Currency == input.Currency &&
            b.StartDate.Equal(mockToday) &&
            b.EndDate.Equal(mockOneMonthLater) &&
            b.Status == models.Paid

        isDynamicValid := b.ID != bson.NilObjectID &&
            b.SubscriptionID != bson.NilObjectID
        return isStaticValid && isDynamicValid
    })
}
```

The matcher separates **static** fields (must match exact values) from **dynamic** fields (must be non-zero, but the exact value is generated at runtime). This catches bugs like "the service forgot to set the bill's `Currency`" without being brittle about auto-generated IDs.

Error-path tests use `mock.Anything` because the specific arguments don't matter — the test is about what happens when the dependency *fails*, not what was passed to it.

### Transaction Bypass with `noopTxnFn`

Service unit tests don't need a real MongoDB transaction. The `noopTxnFn` executes the callback directly:

```go
func noopTxnFn(ctx context.Context, fn func(context.Context) error) error {
    return fn(ctx)  // no transaction wrapping
}
```

This means service tests run in ~3 seconds with zero infrastructure. The actual transaction behavior is verified separately in the [integration test for `TxnFn`](../internal/domain/repositories/txntest/txn_integration_test.go).

### Deterministic Clock

All services receive a fixed `clock.NowFn`:

```go
func newSubService(...) services.SubscriptionService {
    return services.NewSubscriptionService(
        noopTxnFn, subRepo, billRepo, metrics,
        func() time.Time { return mockTime },  // always January 15, 2025 12:00 UTC
    )
}
```

This eliminates test flakiness from wall-clock drift. Date arithmetic (`ValidTill = now + 1 month`) is predictable and assertions can use exact timestamps.

### Implicit Mock Verification

Mocks are created with `repomocks.NewMockSubscriptionRepository(t)` — passing `t` means mockery automatically calls `AssertExpectations` at test cleanup. If a test sets up `subRepo.EXPECT().GetByID(...)` but the service never calls `GetByID`, the test fails. This catches missing code paths without explicit "assert was called" lines.

---

## Integration Test Patterns — Repository Layer

### Container Sharing with Database-Level Isolation

A single MongoDB container is started once per package via `TestMain`:

```go
func TestMain(m *testing.M) {
    container, _ := mongodb.Run(ctx, "mongo:8")
    defer container.Terminate(ctx)

    uri, _ := container.ConnectionString(ctx)
    mongoClient, _ = mongo.Connect(options.Client().ApplyURI(uri))
    m.Run()
}
```

Each test gets a **unique database name** (e.g., `sub_test_682a1f...`), dropped at cleanup:

```go
func newSubRepo(t *testing.T) (repositories.SubscriptionRepository, *mongo.Collection) {
    dbName := "sub_test_" + bson.NewObjectID().Hex()
    db := mongoClient.Database(dbName)
    t.Cleanup(func() { _ = db.Drop(ctx) })
    // ...
}
```

**Why this works**: One container start (~2s), zero data interference between tests, no manual cleanup, tests can run in parallel.

### Pre-Poisoning / Maximum Collision

Integration tests deliberately **poison the database** with documents designed to confuse a broken query filter. The test names say it all:

```go
// POISON THE WELL: Insert all 3 decoys before the target
_, err := collection.InsertMany(t.Context(),
    []*models.Bill{decoyWrongSub, decoyUnpaid, decoyOlderPaid, targetBill},
)
```

The [`GetRecentBill`](../internal/domain/repositories/bill_integration_test.go#L137) test is a textbook example of this pattern — it inserts **four** documents that a buggy query could return:

| Document | Subscription | Status | Recency | Should Match? | What It Tests |
|---|---|---|---|---|---|
| `decoyWrongSub` | ❌ Different | Paid | Newest | No | `subscription_id` filter |
| `decoyUnpaid` | ✅ Correct | Refunded | Newest | No | `status: paid` filter |
| `decoyOlderPaid` | ✅ Correct | Paid | Oldest | No | Sort order (`-start_date`) |
| `targetBill` | ✅ Correct | Paid | Middle | **Yes** | The actual target |

If the query is missing **any** filter clause or has the wrong sort direction, a decoy will be returned instead of the target. This is much more powerful than "insert one document, read it back."

The same collision pattern appears in `GetByID` (insert a decoy with similar fields, verify only the target is returned), `Update` (verify the decoy is untouched), and `Delete` (verify the decoy survives).

### Vault Lock Verification

After mutation operations (`Update`, `Delete`), tests verify that **non-targeted documents are completely untouched**:

```go
// Vault Lock: Prove Decoy was completely untouched
untouchedDecoy := &models.Subscription{}
err = collection.FindOne(t.Context(), bson.M{"_id": decoy.ID}).Decode(untouchedDecoy)
require.NoError(t, err)
assert.Equal(t, decoy, untouchedDecoy, "Decoy was corrupted! Update filter is broken.")
```

This catches filter bugs like `bson.M{"status": "active"}` (matches multiple documents) instead of `bson.M{"_id": sub.ID}` (matches exactly one).

### Ghost Subscription Detection

Several tests check for "ghost subscriptions" — documents that are `status: active` but have `valid_till` in the past. A correct query must check **both** fields:

```go
t.Run("excludes subscriptions that are marked active but chronologically expired", func(t *testing.T) {
    sub := validSub()        // status: Active
    sub.ValidTill = mockToday // but expired
    _, err := collection.InsertOne(t.Context(), sub)

    got, err := repo.GetActiveSubscriptions(t.Context(), mockTime)
    assert.Empty(t, got, "expected empty slice because valid_till is in the past, even though status is active")
})
```

This verifies the compound filter `{status: active, valid_till: {$gt: now}}` — if either clause is missing, the ghost will leak through.

### Boundary Condition Tests

Query boundaries (`$gt` vs `$gte`, `$lt` vs `$lte`) are tested explicitly:

```go
t.Run("boundary - excludes if valid_till is exactly the cutoff time", func(t *testing.T) {
    sub := validSub()
    sub.ValidTill = mockTime // Exactly AT the cutoff
    _, err := collection.InsertOne(t.Context(), sub)

    got, err := repo.GetActiveSubscriptions(t.Context(), mockTime)
    assert.Empty(t, got, "expected exact cutoff to be excluded (query should use $gt)")
})
```

The assertion message documents the **design intent** — the test proves the query uses `$gt` (strictly greater than), not `$gte` (greater or equal). If someone changes the query operator, this test catches it with a clear explanation.

### Read-Back Verification

After `Create`, tests don't just check the return value — they **read the document back from the database** through the raw collection (bypassing the repository) to verify the insert actually persisted:

```go
_, err := repo.Create(t.Context(), sub)
require.NoError(t, err)

// Read-Back Verification (bypass repository, use raw collection)
savedSub := &models.Subscription{}
err = collection.FindOne(t.Context(), bson.M{"_id": sub.ID}).Decode(savedSub)
require.NoError(t, err)
assert.Equal(t, sub, savedSub)
```

This catches bugs where the repository returns a stale object without actually writing to the database.

### Forced Infrastructure Failures

Timeout and infrastructure errors are tested by **injecting an already-expired context**:

```go
t.Run("returns error when database operation fails", func(t *testing.T) {
    ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Second))
    defer cancel()

    got, err := repo.GetAll(ctx)

    require.Error(t, err)
    assertAppErrorCode(t, err, apperror.ErrTimeout)
    assert.Nil(t, got)
})
```

This verifies that the repository correctly wraps driver-level timeout errors into the `AppError` system with the appropriate error code.

---

## Controller Tests

Controller tests use `httptest.NewRecorder()` with mocked services to verify the HTTP layer:
- Correct status codes for success and error cases
- Proper JSON response bodies
- Service methods called with expected arguments
- Context values (user ID, subscription ID) correctly extracted from middleware

---

## Notable Test Patterns — Other Packages

### Adversarial IP Spoofing Tests — [`TestClientIP`](../internal/lib/net_test.go)

The `ClientIP` tests are organized by **proxy topology**, not by input/output — each section simulates a real network scenario:

| Section | What It Proves |
|---|---|
| **Direct connections** | Public `RemoteAddr` → headers must be **ignored** even if present (spoofing guard) |
| **XFF traversal** | Right-to-left walk finds the rightmost public IP; attacker-prepended IPs on the left are skipped |
| **All-private XFF** | Falls through to `X-Real-IP`, then to `RemoteAddr` (ultimate fallback) |
| **Malformed input** | Missing port in `RemoteAddr` → clean error, no panic |

The test `"Direct public IP — XFF present but must be ignored (spoofing guard)"` is particularly important: an attacker can set `X-Forwarded-For: 1.2.3.4` on a direct connection, and a naive implementation would trust it.

### Calendar Math Edge Cases — [`TestCalcRenewalDate`](../internal/lib/time_test.go)

Subscription renewal dates must handle every edge case in the Gregorian calendar. The test suite covers **20+ cases** across these categories:

| Category | Example | Why It Matters |
|---|---|---|
| Month-end clamping | Jan 31 → Feb 28 (non-leap) | Naive `AddDate(0,1,0)` overflows to March 3 |
| Leap year | Jan 31 → Feb 29 (leap), Feb 29 → Feb 28 (next year) | A billing bug here affects real revenue |
| Year boundary | Dec 31 → Jan 31 | Year rollover |
| Day preservation | Jan 29 → Feb 28 (clamp), Feb 28 → Mar 28 (no clamp) | Must not over-clamp |

`TestDaysBetween` adds midnight normalization tests — `23:59 → 00:01` across a day boundary should count as 1 day, not 0 — and a leap-day-to-leap-day span across 4 years asserting exactly 1461 days.

### OTel Telemetry Trap — [`TestOTel_Middleware`](../internal/api/middlewares/otel_test.go)

The OTel middleware test uses an **in-memory exporter** (`tracetest.NewInMemoryExporter()`) as a "telemetry trap" to capture spans without running Jaeger:

```go
exporter := tracetest.NewInMemoryExporter()
tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
otel.SetTracerProvider(tp)
```

It then proves two things:
1. **Span name** is the chi route **pattern** (`/users/{userID}/subscriptions`), not the raw URL (`/users/alice_123/subscriptions`). Using raw URLs as span names creates high-cardinality metric explosion.
2. **`http.route` attribute** is explicitly set on the span — this is what Prometheus uses for route-level metrics.

The test sends a request to a parameterized route specifically to verify the pattern resolution.

### Independent Mathematical Verification — [`Test_jwtService_GenerateTokens`](../internal/domain/services/jwt_test.go)

The `GenerateTokens` test avoids circular testing. Instead of using the service's own `ValidateToken` to verify the generated token (which would prove nothing if both are broken), it parses the token directly with the raw `jwt.Parse` function:

```go
parsedToken, err := jwt.Parse(got.AccessToken,
    func(token *jwt.Token) (any, error) {
        // Verify the signing algorithm
        assert.Equal(t, jwt.SigningMethodHS256, token.Method)
        return []byte(jwtCfg.AccessSecret), nil
    },
    jwt.WithTimeFunc(func() time.Time { return mockTime }),
)
```

This independently proves: correct signing algorithm, correct signing key, correct claims (userId, email, type, issuer), and correct expiry. The `ValidateToken` tests then use a separate `buildToken` helper (also independent of the service) to construct adversarial tokens — wrong secret, wrong issuer, expired, wrong type — proving rejection logic.

---

## What the Test Suite Catches

The combination of these patterns creates a defense-in-depth approach:

| Bug Class | Caught By |
|---|---|
| Wrong HTTP status code for an error | Typed `ErrorCode` assertions in unit tests |
| Service mutates input before validation | Input snapshot copy |
| Query filter missing a field | Pre-poisoned decoys in integration tests |
| `$gt` vs `$gte` boundary error | Explicit boundary condition tests |
| Update/Delete affects wrong documents | Vault lock verification |
| Ghost subscriptions leak through queries | Ghost subscription detection tests |
| Transaction callback not wired correctly | Separate `TxnFn` integration test |
| Mock never called (dead code) | Implicit `AssertExpectations` via `t` parameter |
| Clock-dependent flakiness | Deterministic `clock.NowFn` injection |
| Noise document confuses sort order | Maximum collision with decoys inserted in adversarial order |
| XFF header spoofing | Adversarial IP tests with spoofing guard scenarios |
| Calendar overflow (Jan 31 + 1 month) | Exhaustive leap year and month-end clamping tests |
| High-cardinality OTel span names | In-memory exporter verifying route pattern, not raw URL |
| Circular token test (generate validates itself) | Independent JWT parsing with raw library |
| Token type confusion (access ↔ refresh) | Cross-type swap tests with separate secrets |
