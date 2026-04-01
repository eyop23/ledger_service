package middleware

import "net/http"

// ActorID extracts the X-Actor-ID header and injects it into the request context.
// Empty string is stored as-is if the header is absent — requests are not rejected.
func ActorID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := r.Header.Get("X-Actor-ID")
		ctx := contextWithValue(r.Context(), ActorIDKey, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
