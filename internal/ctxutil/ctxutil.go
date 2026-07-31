package ctxutil

import (
	"context"
	"net/http"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func GetUserID(r *http.Request) string {
	id, _ := r.Context().Value(UserIDKey).(string)
	return id
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}
