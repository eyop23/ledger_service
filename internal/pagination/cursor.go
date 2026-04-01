package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Cursor encodes the position of the last row on a page.
// The next page query uses (created_at, id) < (cursor.CreatedAt, cursor.ID)
// to return only rows older than this position.
type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// Encode converts a Cursor into an opaque base64 string safe to send in HTTP responses.
func Encode(c Cursor) string {
	b, _ := json.Marshal(c)
	return base64.StdEncoding.EncodeToString(b)
}

// Decode parses a base64 cursor string back into a Cursor.
// Returns an error if the string is malformed.
func Decode(s string) (Cursor, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, fmt.Errorf("invalid cursor payload: %w", err)
	}
	return c, nil
}
