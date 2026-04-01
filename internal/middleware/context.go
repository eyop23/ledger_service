package middleware

import "context"

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	ActorIDKey   contextKey = "actor_id"
)

func contextWithValue(ctx context.Context, key contextKey, val string) context.Context {
	return context.WithValue(ctx, key, val)
}

// GetRequestID retrieves the request_id from context.
func GetRequestID(ctx context.Context) string {
	v, _ := ctx.Value(RequestIDKey).(string)
	return v
}

// GetActorID retrieves the actor_id from context.
func GetActorID(ctx context.Context) string {
	v, _ := ctx.Value(ActorIDKey).(string)
	return v
}
