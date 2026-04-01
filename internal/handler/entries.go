package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/eyop23/ledger_service/internal/db"
	"github.com/eyop23/ledger_service/internal/dto"
	"github.com/eyop23/ledger_service/internal/pagination"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type EntriesHandler struct {
	queries *db.Queries
}

func NewEntriesHandler(queries *db.Queries) *EntriesHandler {
	return &EntriesHandler{queries: queries}
}

// GET /accounts/{id}/entries
func (h *EntriesHandler) GetEntries(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	// Parse limit
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "limit must be a positive integer")
			return
		}
		if n > 100 {
			writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "limit must not exceed 100")
			return
		}
		limit = n
	}

	// Parse cursor — use far-future sentinel when absent (returns first page)
	cursorTime := pgtype.Timestamptz{Time: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), Valid: true}
	cursorID := pgtype.UUID{Bytes: maxUUID(), Valid: true}

	if raw := r.URL.Query().Get("cursor"); raw != "" {
		c, err := pagination.Decode(raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "invalid cursor")
			return
		}
		cursorTime = pgtype.Timestamptz{Time: c.CreatedAt, Valid: true}
		cursorID = pgtype.UUID{Bytes: c.ID, Valid: true}
	}

	entries, err := h.queries.GetEntriesByAccount(r.Context(), &db.GetEntriesByAccountParams{
		AccountID: id,
		Column2:   cursorTime,
		Column3:   cursorID,
		Limit:     int32(limit),
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, dto.ErrInternal, "an internal error occurred")
		return
	}

	// Build next_cursor from last entry if a full page was returned
	var nextCursor *string
	if len(entries) == limit {
		last := entries[len(entries)-1]
		encoded := pagination.Encode(pagination.Cursor{
			CreatedAt: last.CreatedAt.Time,
			ID:        uuid.UUID(last.ID.Bytes),
		})
		nextCursor = &encoded
	}

	resp := dto.EntriesResponse{
		Entries:    make([]dto.EntryResponse, len(entries)),
		NextCursor: nextCursor,
	}
	for i, e := range entries {
		resp.Entries[i] = mapEntry(e)
	}

	writeJSON(w, http.StatusOK, resp)
}

func mapEntry(e *db.Entry) dto.EntryResponse {
	return dto.EntryResponse{
		ID:            uuid.UUID(e.ID.Bytes).String(),
		TransactionID: uuid.UUID(e.TransactionID.Bytes).String(),
		Direction:     e.Direction,
		Amount:        e.Amount,
		CreatedAt:     e.CreatedAt.Time.Format(time.RFC3339),
	}
}

// maxUUID returns the maximum UUID value used as a sentinel for the first page cursor.
func maxUUID() [16]byte {
	return [16]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
}
