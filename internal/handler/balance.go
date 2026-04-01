package handler

import (
	"errors"
	"net/http"

	"github.com/eyop23/ledger_service/internal/dto"
	"github.com/eyop23/ledger_service/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BalanceHandler struct {
	svc *service.AccountService
}

func NewBalanceHandler(svc *service.AccountService) *BalanceHandler {
	return &BalanceHandler{svc: svc}
}

// GET /accounts/{id}/balance
func (h *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	// Verify account exists and get currency
	account, err := h.svc.GetAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, dto.ErrInvalidRequest, "account not found")
			return
		}
		handleServiceError(w, err)
		return
	}

	balance, err := h.svc.GetBalance(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, dto.BalanceResponse{
		AccountID: uuid.UUID(account.ID.Bytes).String(),
		Balance:   balance,
		Currency:  account.Currency,
	})
}
