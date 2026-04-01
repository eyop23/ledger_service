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

type AccountHandler struct {
	svc *service.AccountService
}

func NewAccountHandler(svc *service.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

// POST /accounts
func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, dto.ErrInvalidRequest, "invalid request body")
		return
	}
	if req.Currency == "" {
		writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "currency is required")
		return
	}

	actor := middleware.GetActorID(r.Context())

	account, err := h.svc.CreateAccount(r.Context(), req.Currency, actor)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mapAccount(account))
}

// GET /accounts/{id}
func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	account, err := h.svc.GetAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, dto.ErrInvalidRequest, "account not found")
			return
		}
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapAccount(account))
}

func mapAccount(a *db.Account) dto.AccountResponse {
	return dto.AccountResponse{
		ID:        uuid.UUID(a.ID.Bytes).String(),
		Currency:  a.Currency,
		CreatedAt: a.CreatedAt.Time.Format(time.RFC3339),
	}
}
