package dto

// GET /audit
type AuditLogResponse struct {
	ID         string      `json:"id"`
	Actor      string      `json:"actor"`
	Action     string      `json:"action"`
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
	Payload    interface{} `json:"payload"`
	CreatedAt  string      `json:"created_at"`
}

type AuditLogsResponse struct {
	AuditLogs  []AuditLogResponse `json:"audit_logs"`
	NextCursor *string            `json:"next_cursor"`
}
