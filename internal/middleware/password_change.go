package middleware

import (
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"net/http"
)

// PasswordChangeChecker resolves whether a user must set a new password
// before using the application. *auth.Service satisfies it; a fake
// implementation avoids a database in middleware unit tests.
type PasswordChangeChecker interface {
	MustChangePassword(userID string) (bool, error)
}

// RequirePasswordChanged blocks requests from accounts flagged with
// must_change_password until they change their password. Self-service routes
// (me, profile, password, permissions) must be registered outside this
// middleware so a flagged account can still clear the flag.
func RequirePasswordChanged(authSvc PasswordChangeChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := ctxutil.GetUserID(r)
			if userID == "" {
				respond.JSON(
					w,
					http.StatusUnauthorized,
					nil,
					&respond.Error{Code: "UNAUTHORIZED", Message: "Not authenticated"},
					nil,
				)
				return
			}
			must, err := authSvc.MustChangePassword(userID)
			if err != nil {
				respond.ServerError(w, err)
				return
			}
			if must {
				respond.JSON(
					w,
					http.StatusForbidden,
					nil,
					&respond.Error{Code: "PASSWORD_CHANGE_REQUIRED", Message: "Password change required"},
					nil,
				)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
