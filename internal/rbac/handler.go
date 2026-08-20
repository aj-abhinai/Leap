package rbac

import (
	"crm/internal/auth"
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// respondError maps service errors onto the HTTP contract: protection
// violations answer 403, missing resources 404, and name collisions 409.
func (h *Handler) respondError(w http.ResponseWriter, err error) {
	if h.writeProtected(w, err) {
		return
	}
	switch {
	case errors.Is(err, ErrNotFound), respond.IsNotFound(err):
		respond.JSON(
			w,
			http.StatusNotFound,
			nil,
			&respond.Error{Code: "NOT_FOUND", Message: "Role or user not found"},
			nil,
		)
	case errors.Is(err, ErrDuplicate), errors.Is(err, ErrRoleInUse):
		respond.JSON(
			w,
			http.StatusConflict,
			nil,
			&respond.Error{Code: "CONFLICT", Message: err.Error()},
			nil,
		)
	case errors.Is(err, ErrInvalidPermission):
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
			nil,
		)
	default:
		respond.ServerError(w, err)
	}
}
func (h *Handler) writeProtected(w http.ResponseWriter, err error) bool {
	if !errors.Is(err, ErrSelfDelete) &&
		!errors.Is(err, ErrLastSuperadminProtected) &&
		!errors.Is(err, ErrSelfRoleChange) &&
		!errors.Is(err, ErrWildcardRestricted) &&
		!errors.Is(err, ErrSuperadminAssignmentRestricted) &&
		!errors.Is(err, ErrSuperadminRoleProtected) &&
		!errors.Is(err, ErrLastManagerProtected) {
		return false
	}
	respond.JSON(
		w,
		http.StatusForbidden,
		nil,
		&respond.Error{Code: "FORBIDDEN", Message: err.Error()},
		nil,
	)
	return true
}

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.svc.listRoles()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		roles,
		nil,
		nil,
	)
}

func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"},
			nil,
		)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Name is required"},
			nil,
		)
		return
	}
	role, err := h.svc.createRole(req, ctxutil.GetUserID(r))
	if err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusCreated,
		role,
		nil,
		nil,
	)
}

func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"},
			nil,
		)
		return
	}
	if req.Name != nil {
		*req.Name = strings.TrimSpace(*req.Name)
		if *req.Name == "" {
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: "BAD_REQUEST", Message: "Name cannot be blank"},
				nil,
			)
			return
		}
	}
	role, err := h.svc.updateRole(id, req, ctxutil.GetUserID(r))
	if err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		role,
		nil,
		nil,
	)
}

func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.deleteRole(id, ctxutil.GetUserID(r)); err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Role deleted"},
		nil,
		nil,
	)
}

func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.svc.listPermissions()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		perms,
		nil,
		nil,
	)
}

func (h *Handler) AssignPermission(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "id")
	var body struct {
		PermissionID string `json:"permission_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"},
			nil,
		)
		return
	}
	if err := h.svc.assignPermission(roleID, body.PermissionID, ctxutil.GetUserID(r)); err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Permission assigned"},
		nil,
		nil,
	)
}

func (h *Handler) RemovePermission(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "id")
	permID := chi.URLParam(r, "permission_id")
	if err := h.svc.removePermission(roleID, permID, ctxutil.GetUserID(r)); err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Permission removed"},
		nil,
		nil,
	)
}

func (h *Handler) GetRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "id")
	perms, err := h.svc.getRolePermissions(roleID)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		perms,
		nil,
		nil,
	)
}

func (h *Handler) SetRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "id")
	var req SetRolePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"},
			nil,
		)
		return
	}
	if req.PermissionIDs == nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "permission_ids is required"},
			nil,
		)
		return
	}
	role, err := h.svc.setRolePermissions(roleID, req.PermissionIDs, ctxutil.GetUserID(r))
	if err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		role,
		nil,
		nil,
	)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.listUsers()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		users,
		nil,
		nil,
	)
}

// ListAssigneeOptions serves the minimal id+name user list for lead assignee
// dropdowns. Gated on lead:read (not settings:manage) so sales users can
// assign without seeing admin user details.
func (h *Handler) ListAssigneeOptions(w http.ResponseWriter, r *http.Request) {
	options, err := h.svc.listAssigneeOptions()
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		options,
		nil,
		nil,
	)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"},
			nil,
		)
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Name, email and password are required"},
			nil,
		)
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: err.Error()},
			nil,
		)
		return
	}
	user, err := h.svc.createUser(req.Name, req.Email, req.Password, ctxutil.GetUserID(r))
	if err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusCreated,
		user,
		nil,
		nil,
	)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.deleteUser(id, ctxutil.GetUserID(r)); err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "User deleted"},
		nil,
		nil,
	)
}

func (h *Handler) SetUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req SetUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"},
			nil,
		)
		return
	}
	if err := h.svc.setUserRole(userID, req.RoleID, ctxutil.GetUserID(r)); err != nil {
		h.respondError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Role updated"},
		nil,
		nil,
	)
}

func (h *Handler) MePermissions(w http.ResponseWriter, r *http.Request) {
	userID := ctxutil.GetUserID(r)
	perms, err := h.svc.GetUserPermissions(userID)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		perms,
		nil,
		nil,
	)
}
