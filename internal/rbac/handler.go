package rbac

import (
	"crm/internal/respond"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.svc.ListRoles()
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, roles, nil, nil)
}

func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.Name == "" {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Name is required"}, nil)
		return
	}
	role, err := h.svc.CreateRole(req)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusCreated, role, nil, nil)
}

func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	role, err := h.svc.UpdateRole(id, req)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, role, nil, nil)
}

func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteRole(id); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Role deleted"}, nil, nil)
}

func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.svc.ListPermissions()
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, perms, nil, nil)
}

func (h *Handler) AssignPermission(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "id")
	var body struct {
		PermissionID string `json:"permission_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if err := h.svc.AssignPermission(roleID, body.PermissionID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Permission assigned"}, nil, nil)
}

func (h *Handler) RemovePermission(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "id")
	permID := chi.URLParam(r, "permission_id")
	if err := h.svc.RemovePermission(roleID, permID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Permission removed"}, nil, nil)
}

func (h *Handler) GetRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "id")
	perms, err := h.svc.GetRolePermissions(roleID)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, perms, nil, nil)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers()
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, users, nil, nil)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Name, email and password are required"}, nil)
		return
	}
	user, err := h.svc.CreateUser(req.Name, req.Email, req.Password)
	if err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusCreated, user, nil, nil)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteUser(id); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "User deleted"}, nil, nil)
}

func (h *Handler) AssignUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var body struct {
		RoleID string `json:"role_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if err := h.svc.AssignUserRole(userID, body.RoleID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Role assigned"}, nil, nil)
}

func (h *Handler) RemoveUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	roleID := chi.URLParam(r, "role_id")
	if err := h.svc.RemoveUserRole(userID, roleID); err != nil {
		respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: err.Error()}, nil)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Role removed"}, nil, nil)
}
