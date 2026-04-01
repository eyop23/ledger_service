package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

// RequestID injects a unique request_id into every request context.
// If the caller supplies an X-Request-ID header, that value is used;
// otherwise a new UUID is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := contextWithValue(r.Context(), RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
