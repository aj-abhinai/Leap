package ctxutil

import (
	"context"
	"net/http"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	ClientIPKey contextKey = "client_ip"
)

func GetUserID(r *http.Request) string {
	id, _ := r.Context().Value(UserIDKey).(string)
	return id
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetClientIP returns the client IP resolved by the middleware.ClientIP
// middleware, or "" when none was resolved.
func GetClientIP(r *http.Request) string {
	ip, _ := r.Context().Value(ClientIPKey).(string)
	return ip
}

func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ClientIPKey, ip)
}
