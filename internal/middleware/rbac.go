package middleware

import (
	"crm/internal/rbac"
	"crm/internal/respond"
	"net/http"
)

func RequirePermission(rbacSvc *rbac.Service, permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r)
		if userID == "" {
			respond.JSON(w, http.StatusUnauthorized, nil, &respond.Error{Code: "UNAUTHORIZED", Message: "Not authenticated"}, nil)
			return
		}
		perms, err := rbacSvc.GetUserPermissions(userID)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "Failed to check permissions"}, nil)
			return
		}
		for _, p := range perms {
			if p == "*" || p == permission {
				next(w, r)
				return
			}
		}
		respond.JSON(w, http.StatusForbidden, nil, &respond.Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}, nil)
	}
}
