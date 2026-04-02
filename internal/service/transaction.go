package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/eyop23/ledger_service/internal/db"
	"github.com/eyop23/ledger_service/internal/dto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxRetries = 3

// TransactionResult holds everything the handler needs to build a response —
// no additional DB calls required after PostTransaction or GetTransaction returns.
type TransactionResult struct {
	Transaction *db.Transaction
	Entries     []*db.Entry
	Replayed    bool // true when returned from idempotent replay (23505) — handler writes 200 not 201
}

type TransactionService struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewTransactionService(queries *db.Queries, pool *pgxpool.Pool) *TransactionService {
	return &TransactionService{queries: queries, pool: pool}
}

func (s *TransactionService) PostTransaction(ctx context.Context, req *dto.CreateTransactionRequest, actor string) (*TransactionResult, error) {
	// --- Step 1: App-level validation (before any DB transaction) ---

	if len(req.Entries) < 2 {
		return s.rejectWithAudit(ctx, req, actor, dto.ErrInvalidRequest, "at least 2 entries required")
	}

	var totalDebits, totalCredits int64
	for _, e := range req.Entries {
		if e.Direction != "DEBIT" && e.Direction != "CREDIT" {
			return s.rejectWithAudit(ctx, req, actor, dto.ErrInvalidRequest,
				fmt.Sprintf("invalid direction %q: must be DEBIT or CREDIT", e.Direction))
		}
		if e.Direction == "DEBIT" {
			totalDebits += e.Amount
		} else {
			totalCredits += e.Amount
		}
	}

	if totalDebits != totalCredits {
		return s.rejectWithAudit(ctx, req, actor, dto.ErrDebitCreditMismatch,
			fmt.Sprintf("debits (%d) do not equal credits (%d)", totalDebits, totalCredits))
	}

	// Validate accounts — deduplicate to avoid redundant lookups
	accountIDs := make(map[string]pgtype.UUID)
	for _, e := range req.Entries {
		if _, seen := accountIDs[e.AccountID]; seen {
			continue
		}
		uid, err := uuid.Parse(e.AccountID)
		if err != nil {
			return s.rejectWithAudit(ctx, req, actor, dto.ErrInvalidRequest,
				fmt.Sprintf("invalid account_id %q", e.AccountID))
		}
		pgID := pgtype.UUID{Bytes: uid, Valid: true}
		acc, err := s.queries.GetAccount(ctx, pgID)
		if err != nil {
			return s.rejectWithAudit(ctx, req, actor, dto.ErrUnknownAccount,
				fmt.Sprintf("account %s not found", e.AccountID))
		}
		if acc.Currency != req.Currency {
			return s.rejectWithAudit(ctx, req, actor, dto.ErrCurrencyMismatch,
				fmt.Sprintf("account %s currency %s does not match transaction currency %s",
					e.AccountID, acc.Currency, req.Currency))
		}
		accountIDs[e.AccountID] = pgID
	}

	// --- Step 2: REPEATABLE READ transaction with retry ---
	for attempt := 0; attempt < maxRetries; attempt++ {
		tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		if err != nil {
			return nil, fmt.Errorf("begin tx: %w", err)
		}

		result, err := s.runTransaction(ctx, tx, req, actor, accountIDs)
		if err == nil {
			return result, nil
		}

		tx.Rollback(ctx)

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "40001": // serialization failure — retry
				continue
			case "23505": // unique violation on idempotency_key — idempotent replay
				return s.fetchByIdempotencyKey(ctx, req.IdempotencyKey)
			}
		}

		return nil, err
	}

	return nil, errors.New("transaction failed after max retries")
}

// runTransaction executes the INSERT statements inside the REPEATABLE READ tx.
// Audit is written inside the same tx — if it fails everything rolls back.
func (s *TransactionService) runTransaction(
	ctx context.Context,
	tx pgx.Tx,
	req *dto.CreateTransactionRequest,
	actor string,
	accountIDs map[string]pgtype.UUID,
) (*TransactionResult, error) {
	qtx := s.queries.WithTx(tx)
	now := time.Now().UTC()

	// a. Insert transaction row
	txn, err := qtx.CreateTransaction(ctx, &db.CreateTransactionParams{
		ID:             pgtype.UUID{Bytes: uuid.New(), Valid: true},
		IdempotencyKey: req.IdempotencyKey,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         "posted",
		CreatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	// b. Insert entry rows
	entries := make([]*db.Entry, 0, len(req.Entries))
	for _, e := range req.Entries {
		entry, err := qtx.CreateEntry(ctx, &db.CreateEntryParams{
			ID:            pgtype.UUID{Bytes: uuid.New(), Valid: true},
			TransactionID: txn.ID,
			AccountID:     accountIDs[e.AccountID],
			Direction:     e.Direction,
			Amount:        e.Amount,
			CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		})
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	// c. Insert audit log inside the same tx —
	// if this fails the whole tx rolls back: money never moves without a record.
	auditPayload, _ := json.Marshal(buildPostedPayload(txn, entries))
	_, err = qtx.CreateAuditLog(ctx, &db.CreateAuditLogParams{
		ID:         pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Actor:      actor,
		Action:     "transaction.posted",
		EntityType: "transaction",
		EntityID:   txn.ID,
		Payload:    auditPayload,
		CreatedAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &TransactionResult{Transaction: txn, Entries: entries}, nil
}

// rejectWithAudit writes an audit record for the rejected transaction then returns a ServiceError.
// Per PDF completeness requirement, if the audit write fails we return 500 — not 422.
func (s *TransactionService) rejectWithAudit(ctx context.Context, req *dto.CreateTransactionRequest, actor, code, message string) (*TransactionResult, error) {
	payload, _ := json.Marshal(map[string]any{
		"reason":  code,
		"request": req,
	})
	_, err := s.queries.CreateAuditLog(ctx, &db.CreateAuditLogParams{
		ID:         pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Actor:      actor,
		Action:     "transaction.rejected",
		EntityType: "transaction",
		EntityID:   pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Payload:    payload,
		CreatedAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("write rejected audit log: %w", err)
	}
	return nil, &ServiceError{Code: code, Message: message}
}

func (s *TransactionService) GetTransaction(ctx context.Context, id pgtype.UUID) (*TransactionResult, error) {
	txn, err := s.queries.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}
	entries, err := s.queries.GetEntriesByTransaction(ctx, txn.ID)
	if err != nil {
		return nil, fmt.Errorf("get entries: %w", err)
	}
	return &TransactionResult{Transaction: txn, Entries: entries, Replayed: true}, nil
}

// fetchByIdempotencyKey is called when a 23505 unique violation fires on idempotency_key.
// Returns the original transaction + entries so the handler can respond with 200.
func (s *TransactionService) fetchByIdempotencyKey(ctx context.Context, key string) (*TransactionResult, error) {
	txn, err := s.queries.GetTransactionByIdempotencyKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("fetch original transaction: %w", err)
	}
	entries, err := s.queries.GetEntriesByTransaction(ctx, txn.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch original entries: %w", err)
	}
	return &TransactionResult{Transaction: txn, Entries: entries, Replayed: true}, nil
}

func buildPostedPayload(txn *db.Transaction, entries []*db.Entry) map[string]any {
	entryList := make([]map[string]any, len(entries))
	for i, e := range entries {
		entryList[i] = map[string]any{
			"account_id": uuid.UUID(e.AccountID.Bytes).String(),
			"direction":  e.Direction,
			"amount":     e.Amount,
		}
	}
	return map[string]any{
		"transaction": map[string]any{
			"id":              uuid.UUID(txn.ID.Bytes).String(),
			"amount":          txn.Amount,
			"currency":        txn.Currency,
			"status":          txn.Status,
			"idempotency_key": txn.IdempotencyKey,
		},
		"entries": entryList,
	}
}
