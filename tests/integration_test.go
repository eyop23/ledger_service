package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/eyop23/ledger_service/internal/db"
	"github.com/eyop23/ledger_service/internal/dto"
	"github.com/eyop23/ledger_service/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDB spins up a fresh Postgres container, runs migrations, and returns
// a pool and queries ready for use. Container is torn down when the test ends.
func setupTestDB(t *testing.T) (*pgxpool.Pool, *db.Queries) {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase("ledger_test"),
		postgres.WithUsername("ledger"),
		postgres.WithPassword("ledger"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	sqlDB := stdlib.OpenDBFromPool(pool)
	err = goose.Up(sqlDB, "../migrations")
	require.NoError(t, err)

	return pool, db.New(pool)
}

// ── Account tests ────────────────────────────────────────────────────────────

func TestCreateAccount(t *testing.T) {
	pool, queries := setupTestDB(t)
	svc := service.NewAccountService(queries, pool)

	account, err := svc.CreateAccount(context.Background(), "USD", "test-actor")
	require.NoError(t, err)
	assert.NotEmpty(t, uuid.UUID(account.ID.Bytes).String())
	assert.Equal(t, "USD", account.Currency)
	assert.False(t, account.CreatedAt.Time.IsZero())
}

func TestGetAccount(t *testing.T) {
	pool, queries := setupTestDB(t)
	svc := service.NewAccountService(queries, pool)

	created, err := svc.CreateAccount(context.Background(), "USD", "test-actor")
	require.NoError(t, err)

	fetched, err := svc.GetAccount(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, created.Currency, fetched.Currency)
}

func TestGetBalance_Empty(t *testing.T) {
	pool, queries := setupTestDB(t)
	svc := service.NewAccountService(queries, pool)

	account, err := svc.CreateAccount(context.Background(), "USD", "test-actor")
	require.NoError(t, err)

	balance, err := svc.GetBalance(context.Background(), account.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), balance)
}

// ── Transaction tests ────────────────────────────────────────────────────────

func TestPostTransaction_Valid(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, err := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	require.NoError(t, err)
	dst, err := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	require.NoError(t, err)

	result, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "transfer-1",
		Amount:         5000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "DEBIT", Amount: 5000},
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "CREDIT", Amount: 5000},
		},
	}, "test-actor")
	require.NoError(t, err)
	assert.Equal(t, "posted", result.Transaction.Status)
	assert.Len(t, result.Entries, 2)

	balance, err := accountSvc.GetBalance(ctx, src.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(-5000), balance)
}

func TestPostTransaction_DebitCreditMismatch(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	dst, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")

	_, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "mismatch-1",
		Amount:         5000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "DEBIT", Amount: 5000},
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "CREDIT", Amount: 3000},
		},
	}, "test-actor")

	require.Error(t, err)
	var svcErr *service.ServiceError
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, dto.ErrDebitCreditMismatch, svcErr.Code)
}

func TestPostTransaction_NegativeBalance(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	dst, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")

	// Non-negative balance enforcement is optional per spec.
	// Accounts start at 0 — posting a debit posts successfully and results in negative balance.
	result, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "neg-balance-1",
		Amount:         5000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "DEBIT", Amount: 5000},
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "CREDIT", Amount: 5000},
		},
	}, "test-actor")

	require.NoError(t, err)
	assert.Equal(t, "posted", result.Transaction.Status)

	balance, err := accountSvc.GetBalance(ctx, src.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(-5000), balance)
}

func TestPostTransaction_UnknownAccount(t *testing.T) {
	pool, queries := setupTestDB(t)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	_, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "unknown-1",
		Amount:         1000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.New().String(), Direction: "DEBIT", Amount: 1000},
			{AccountID: uuid.New().String(), Direction: "CREDIT", Amount: 1000},
		},
	}, "test-actor")

	require.Error(t, err)
	var svcErr *service.ServiceError
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, dto.ErrUnknownAccount, svcErr.Code)
}

func TestPostTransaction_CurrencyMismatch(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, _ := accountSvc.CreateAccount(ctx, "ETB", "test-actor")
	dst, _ := accountSvc.CreateAccount(ctx, "ETB", "test-actor")

	_, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "currency-1",
		Amount:         1000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "DEBIT", Amount: 1000},
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "CREDIT", Amount: 1000},
		},
	}, "test-actor")

	require.Error(t, err)
	var svcErr *service.ServiceError
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, dto.ErrCurrencyMismatch, svcErr.Code)
}

// ── Idempotency tests ────────────────────────────────────────────────────────

func TestIdempotency_SameKey(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	dst, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")

	// Fund src
	_, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "fund-idem",
		Amount:         5000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "DEBIT", Amount: 5000},
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "CREDIT", Amount: 5000},
		},
	}, "test-actor")
	require.NoError(t, err)

	req := &dto.CreateTransactionRequest{
		IdempotencyKey: "idem-key-1",
		Amount:         1000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "DEBIT", Amount: 1000},
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "CREDIT", Amount: 1000},
		},
	}

	first, err := txnSvc.PostTransaction(ctx, req, "test-actor")
	require.NoError(t, err)

	second, err := txnSvc.PostTransaction(ctx, req, "test-actor")
	require.NoError(t, err)

	// Same transaction ID returned — no duplicate created
	assert.Equal(t, first.Transaction.ID, second.Transaction.ID)
	assert.True(t, second.Replayed)
}

func TestIdempotency_Concurrent(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	dst, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")

	// Fund src with enough for one transaction
	_, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "fund-concurrent",
		Amount:         1000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "DEBIT", Amount: 1000},
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "CREDIT", Amount: 1000},
		},
	}, "test-actor")
	require.NoError(t, err)

	const workers = 10
	errors := make([]error, workers)
	results := make([]*service.TransactionResult, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errors[i] = txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
				IdempotencyKey: "concurrent-key",
				Amount:         1000,
				Currency:       "USD",
				Entries: []dto.EntryRequest{
					{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "DEBIT", Amount: 1000},
					{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "CREDIT", Amount: 1000},
				},
			}, "test-actor")
		}(i)
	}
	wg.Wait()

	// All calls must succeed (either created or replayed)
	for i, err := range errors {
		require.NoError(t, err, "worker %d failed", i)
	}

	// All results must reference the same transaction ID
	firstID := results[0].Transaction.ID
	for _, r := range results {
		assert.Equal(t, firstID, r.Transaction.ID)
	}

	// Verify only 2 entries exist in DB (not 10 × 2 = 20)
	entries, err := queries.GetEntriesByTransaction(ctx, firstID)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

// ── Audit tests ──────────────────────────────────────────────────────────────

func TestAuditLog_AccountCreated(t *testing.T) {
	pool, queries := setupTestDB(t)
	svc := service.NewAccountService(queries, pool)
	ctx := context.Background()

	_, err := svc.CreateAccount(ctx, "USD", "test-actor")
	require.NoError(t, err)

	logs, err := queries.GetAuditLogs(ctx, &db.GetAuditLogsParams{
		Column1: "account",
		Limit:   10,
		Column5: farFuture(),
		Column6: maxPgUUID(),
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "account.created", logs[0].Action)
	assert.Equal(t, "test-actor", logs[0].Actor)
}

func TestAuditLog_TransactionPosted(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	dst, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")

	_, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "audit-posted",
		Amount:         1000,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "DEBIT", Amount: 1000},
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "CREDIT", Amount: 1000},
		},
	}, "test-actor")
	require.NoError(t, err)

	logs, err := queries.GetAuditLogs(ctx, &db.GetAuditLogsParams{
		Column1: "transaction",
		Limit:   10,
		Column5: farFuture(),
		Column6: maxPgUUID(),
	})
	require.NoError(t, err)

	var found bool
	for _, l := range logs {
		if l.Action == "transaction.posted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected transaction.posted audit record")
}

func TestAuditLog_RejectedTransaction(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	dst, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")

	// This will fail — debits do not equal credits
	_, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
		IdempotencyKey: "audit-rejected",
		Amount:         9999,
		Currency:       "USD",
		Entries: []dto.EntryRequest{
			{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "DEBIT", Amount: 9999},
			{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "CREDIT", Amount: 1},
		},
	}, "test-actor")
	require.Error(t, err)

	// Audit record must still exist even though transaction was rejected
	logs, err := queries.GetAuditLogs(ctx, &db.GetAuditLogsParams{
		Column1: "transaction",
		Limit:   10,
		Column5: farFuture(),
		Column6: maxPgUUID(),
	})
	require.NoError(t, err)

	var found bool
	for _, l := range logs {
		if l.Action == "transaction.rejected" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected transaction.rejected audit record even for failed transaction")
}

// ── Pagination test ──────────────────────────────────────────────────────────

func TestGetEntries_Pagination(t *testing.T) {
	pool, queries := setupTestDB(t)
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)
	ctx := context.Background()

	src, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")
	dst, _ := accountSvc.CreateAccount(ctx, "USD", "test-actor")

	// Post 13 transactions (26 entries total — 13 debits on src, 13 credits on dst)
	for i := 0; i < 13; i++ {
		// Fund dst first so src can debit
		_, err := txnSvc.PostTransaction(ctx, &dto.CreateTransactionRequest{
			IdempotencyKey: "fund-page-" + uuid.New().String(),
			Amount:         100,
			Currency:       "USD",
			Entries: []dto.EntryRequest{
				{AccountID: uuid.UUID(dst.ID.Bytes).String(), Direction: "DEBIT", Amount: 100},
				{AccountID: uuid.UUID(src.ID.Bytes).String(), Direction: "CREDIT", Amount: 100},
			},
		}, "test-actor")
		require.NoError(t, err)
	}

	// First page — limit 10
	page1, err := queries.GetEntriesByAccount(ctx, &db.GetEntriesByAccountParams{
		AccountID: src.ID,
		Column2:   farFuture(),
		Column3:   maxPgUUID(),
		Limit:     10,
	})
	require.NoError(t, err)
	assert.Len(t, page1, 10)

	// Second page using cursor from last entry of page 1
	last := page1[len(page1)-1]
	page2, err := queries.GetEntriesByAccount(ctx, &db.GetEntriesByAccountParams{
		AccountID: src.ID,
		Column2:   last.CreatedAt,
		Column3:   last.ID,
		Limit:     10,
	})
	require.NoError(t, err)
	assert.Len(t, page2, 3) // 13 total, 10 on page 1, 3 on page 2
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func farFuture() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), Valid: true}
}

func maxPgUUID() pgtype.UUID {
	return pgtype.UUID{
		Bytes: [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		Valid: true,
	}
}
