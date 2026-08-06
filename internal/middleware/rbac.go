package middleware

import (
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"net/http"
)

// PermissionChecker resolves the permission names granted to a user.
// *rbac.Service satisfies it; a fake implementation avoids a database in
// middleware unit tests.
type PermissionChecker interface {
	GetUserPermissions(userID string) ([]string, error)
}

func RequirePermission(rbacSvc PermissionChecker, permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		perms, err := rbacSvc.GetUserPermissions(userID)
		if err != nil {
			respond.ServerError(w, err)
			return
		}
		for _, p := range perms {
			if p == "*" || p == permission {
				next(w, r)
				return
			}
		}
		respond.JSON(
			w,
			http.StatusForbidden,
			nil,
			&respond.Error{Code: "FORBIDDEN", Message: "Insufficient permissions"},
			nil,
		)
	}
}
