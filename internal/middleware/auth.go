package middleware

import (
	"context"
	"crm/internal/auth"
	"crm/internal/respond"
	"net/http"
	"strings"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func Auth(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				respond.JSON(w, http.StatusUnauthorized, nil, &respond.Error{Code: "UNAUTHORIZED", Message: "Missing or invalid token"}, nil)
				return
			}
			token := strings.TrimPrefix(header, "Bearer ")
			userID, err := authSvc.ValidateJWT(token)
			if err != nil {
				respond.JSON(w, http.StatusUnauthorized, nil, &respond.Error{Code: "UNAUTHORIZED", Message: "Invalid or expired token"}, nil)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(r *http.Request) string {
	id, _ := r.Context().Value(UserIDKey).(string)
	return id
}
