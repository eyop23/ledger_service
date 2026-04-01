package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/eyop23/ledger_service/internal/db"
	"github.com/eyop23/ledger_service/internal/dto"
	"github.com/eyop23/ledger_service/internal/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AuditHandler struct {
	queries *db.Queries
}

func NewAuditHandler(queries *db.Queries) *AuditHandler {
	return &AuditHandler{queries: queries}
}

// GET /audit
func (h *AuditHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse limit
	limit := 20
	if raw := q.Get("limit"); raw != "" {
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

	// Optional filters
	entityType := q.Get("entity_type")

	var entityID pgtype.UUID
	if raw := q.Get("entity_id"); raw != "" {
		uid, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "invalid entity_id UUID")
			return
		}
		entityID = pgtype.UUID{Bytes: uid, Valid: true}
	}

	var fromTime pgtype.Timestamptz
	if raw := q.Get("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "from must be RFC3339 timestamp")
			return
		}
		fromTime = pgtype.Timestamptz{Time: t, Valid: true}
	}

	var toTime pgtype.Timestamptz
	if raw := q.Get("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "to must be RFC3339 timestamp")
			return
		}
		toTime = pgtype.Timestamptz{Time: t, Valid: true}
	}

	// Cursor — use far-future sentinel when absent
	cursorTime := pgtype.Timestamptz{Time: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), Valid: true}
	cursorID := pgtype.UUID{Bytes: maxUUID(), Valid: true}

	if raw := q.Get("cursor"); raw != "" {
		c, err := pagination.Decode(raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, dto.ErrInvalidRequest, "invalid cursor")
			return
		}
		cursorTime = pgtype.Timestamptz{Time: c.CreatedAt, Valid: true}
		cursorID = pgtype.UUID{Bytes: c.ID, Valid: true}
	}

	logs, err := h.queries.GetAuditLogs(r.Context(), &db.GetAuditLogsParams{
		Column1: entityType,
		Column2: entityID,
		Column3: fromTime,
		Column4: toTime,
		Column5: cursorTime,
		Column6: cursorID,
		Limit:   int32(limit),
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, dto.ErrInternal, "an internal error occurred")
		return
	}

	var nextCursor *string
	if len(logs) == limit {
		last := logs[len(logs)-1]
		encoded := pagination.Encode(pagination.Cursor{
			CreatedAt: last.CreatedAt.Time,
			ID:        uuid.UUID(last.ID.Bytes),
		})
		nextCursor = &encoded
	}

	resp := dto.AuditLogsResponse{
		AuditLogs:  make([]dto.AuditLogResponse, len(logs)),
		NextCursor: nextCursor,
	}
	for i, l := range logs {
		resp.AuditLogs[i] = mapAuditLog(l)
	}

	writeJSON(w, http.StatusOK, resp)
}

func mapAuditLog(l *db.AuditLog) dto.AuditLogResponse {
	var payload interface{}
	_ = json.Unmarshal(l.Payload, &payload)
	return dto.AuditLogResponse{
		ID:         uuid.UUID(l.ID.Bytes).String(),
		Actor:      l.Actor,
		Action:     l.Action,
		EntityType: l.EntityType,
		EntityID:   uuid.UUID(l.EntityID.Bytes).String(),
		Payload:    payload,
		CreatedAt:  l.CreatedAt.Time.Format(time.RFC3339),
	}
}
