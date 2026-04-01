package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/eyop23/ledger_service/internal/db"
	"github.com/eyop23/ledger_service/internal/dto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ServiceError carries a machine-readable code back to the handler layer.
// Handlers check for this type to decide the HTTP status code.
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type AccountService struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewAccountService(queries *db.Queries, pool *pgxpool.Pool) *AccountService {
	return &AccountService{queries: queries, pool: pool}
}

func (s *AccountService) CreateAccount(ctx context.Context, currency, actor string) (*db.Account, error) {
	if len(currency) != 3 {
		return nil, &ServiceError{
			Code:    dto.ErrInvalidRequest,
			Message: "currency must be exactly 3 characters (ISO 4217)",
		}
	}

	now := time.Now().UTC()
	account, err := s.queries.CreateAccount(ctx, &db.CreateAccountParams{
		ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Currency:  currency,
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}

	// Audit is written outside the insert — if this fails the account still exists.
	// Acceptable trade-off: no money moved, account is recoverable.
	// Documented trade-off in README under "Audit failure strategy".
	payload, _ := json.Marshal(map[string]any{
		"id":         uuid.UUID(account.ID.Bytes).String(),
		"currency":   account.Currency,
		"created_at": account.CreatedAt.Time.Format(time.RFC3339),
	})
	_, _ = s.queries.CreateAuditLog(ctx, &db.CreateAuditLogParams{
		ID:         pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Actor:      actor,
		Action:     "account.created",
		EntityType: "account",
		EntityID:   account.ID,
		Payload:    payload,
		CreatedAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})

	return account, nil
}

func (s *AccountService) GetAccount(ctx context.Context, id pgtype.UUID) (*db.Account, error) {
	return s.queries.GetAccount(ctx, id)
}

func (s *AccountService) GetBalance(ctx context.Context, id pgtype.UUID) (int64, error) {
	return s.queries.GetBalance(ctx, id)
}
