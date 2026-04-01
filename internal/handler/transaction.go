package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/eyop23/ledger_service/internal/db"
	"github.com/eyop23/ledger_service/internal/dto"
	"github.com/eyop23/ledger_service/internal/middleware"
	"github.com/eyop23/ledger_service/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TransactionHandler struct {
	svc *service.TransactionService
}

func NewTransactionHandler(svc *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

// POST /transactions
func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, dto.ErrInvalidRequest, "invalid request body")
		return
	}
	if req.IdempotencyKey == "" {
		writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "idempotency_key is required")
		return
	}
	if len(req.Entries) == 0 {
		writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "entries are required")
		return
	}

	actor := middleware.GetActorID(r.Context())

	result, err := h.svc.PostTransaction(r.Context(), &req, actor)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}

	writeJSON(w, status, mapTransactionResult(result))
}

// GET /transactions/{id}
func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	result, err := h.svc.GetTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, dto.ErrInvalidRequest, "transaction not found")
			return
		}
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapTransactionResult(result))
}

func mapTransactionResult(r *service.TransactionResult) dto.TransactionResponse {
	entries := make([]dto.TransactionEntryResponse, len(r.Entries))
	for i, e := range r.Entries {
		entries[i] = mapTransactionEntry(e)
	}
	return dto.TransactionResponse{
		ID:             uuid.UUID(r.Transaction.ID.Bytes).String(),
		IdempotencyKey: r.Transaction.IdempotencyKey,
		Amount:         r.Transaction.Amount,
		Currency:       r.Transaction.Currency,
		Status:         r.Transaction.Status,
		CreatedAt:      r.Transaction.CreatedAt.Time.Format(time.RFC3339),
		Entries:        entries,
	}
}

func mapTransactionEntry(e *db.Entry) dto.TransactionEntryResponse {
	return dto.TransactionEntryResponse{
		ID:        uuid.UUID(e.ID.Bytes).String(),
		AccountID: uuid.UUID(e.AccountID.Bytes).String(),
		Direction: e.Direction,
		Amount:    e.Amount,
		CreatedAt: e.CreatedAt.Time.Format(time.RFC3339),
	}
}
