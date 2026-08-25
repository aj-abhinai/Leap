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

// authorize checks the request against any of the allowed permissions and
// writes the 401/403/500 response when the request must be rejected. It
// returns true when the handler should proceed.
func authorize(rbacSvc PermissionChecker, w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	userID := ctxutil.GetUserID(r)
	if userID == "" {
		respond.JSON(
			w,
			http.StatusUnauthorized,
			nil,
			&respond.Error{Code: "UNAUTHORIZED", Message: "Not authenticated"},
			nil,
		)
		return false
	}
	perms, err := rbacSvc.GetUserPermissions(userID)
	if err != nil {
		respond.ServerError(w, err)
		return false
	}
	for _, p := range perms {
		if p == "*" {
			return true
		}
		for _, want := range allowed {
			if p == want {
				return true
			}
		}
	}
	respond.JSON(
		w,
		http.StatusForbidden,
		nil,
		&respond.Error{Code: "FORBIDDEN", Message: "Insufficient permissions"},
		nil,
	)
	return false
}

// RequirePermission wraps next so it runs only when the user holds the
// permission; unauthorized requests get a 403.
func RequirePermission(rbacSvc PermissionChecker, permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorize(rbacSvc, w, r, permission) {
			return
		}
		next(w, r)
	}
}

// RequireAny allows the request when the user holds any one of the given
// permissions — the sanctioned pattern for reference reads consumed by more
// than one module.
func RequireAny(rbacSvc PermissionChecker, permissions []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authorize(rbacSvc, w, r, permissions...) {
			return
		}
		next(w, r)
	}
}
