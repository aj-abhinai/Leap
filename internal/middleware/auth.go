package middleware

import (
	"crm/internal/auth"
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"net/http"
	"strings"
)

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
			ctx := ctxutil.WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
