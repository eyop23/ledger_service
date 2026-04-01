package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/eyop23/ledger_service/internal/dto"
	"github.com/eyop23/ledger_service/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, dto.ErrorResponse{
		Error: dto.ErrorBody{Code: code, Message: message},
	})
}

// parseUUID parses a UUID string into pgtype.UUID.
// Writes a 422 response and returns false on failure.
func parseUUID(w http.ResponseWriter, s string) (pgtype.UUID, bool) {
	uid, err := uuid.Parse(s)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "invalid UUID: "+s)
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{Bytes: uid, Valid: true}, true
}

// httpStatus maps a ServiceError code to the appropriate HTTP status.
func httpStatus(err *service.ServiceError) int {
	switch err.Code {
	case dto.ErrDebitCreditMismatch,
		dto.ErrInsufficientFunds,
		dto.ErrUnknownAccount,
		dto.ErrCurrencyMismatch,
		dto.ErrInvalidRequest:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// handleServiceError writes the correct response for any error returned by a service call.
func handleServiceError(w http.ResponseWriter, err error) {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		writeError(w, httpStatus(svcErr), svcErr.Code, svcErr.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, dto.ErrInternal, "an internal error occurred")
}
