package auth

import (
	"encoding/json"
	"net/http"

	"crm/internal/ctxutil"
	"crm/internal/respond"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
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
	if req.Email == "" || req.Password == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Email and password are required"},
			nil,
		)
		return
	}
	resp, err := h.svc.login(req.Email, req.Password)
	if err != nil {
		if ae, ok := err.(*AuthError); ok {
			respond.JSON(
				w,
				http.StatusUnauthorized,
				nil,
				&respond.Error{Code: ae.Code, Message: ae.Message},
				nil,
			)
		} else {
			respond.JSON(
				w,
				http.StatusInternalServerError,
				nil,
				&respond.Error{Code: "INTERNAL", Message: "Login failed"},
				nil,
			)
		}
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		resp,
		nil,
		nil,
	)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
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
	if req.RefreshToken == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Refresh token is required"},
			nil,
		)
		return
	}
	resp, err := h.svc.refresh(req.RefreshToken)
	if err != nil {
		if ae, ok := err.(*AuthError); ok {
			respond.JSON(
				w,
				http.StatusUnauthorized,
				nil,
				&respond.Error{Code: ae.Code, Message: ae.Message},
				nil,
			)
		} else {
			respond.JSON(
				w,
				http.StatusInternalServerError,
				nil,
				&respond.Error{Code: "INTERNAL", Message: "Refresh failed"},
				nil,
			)
		}
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		resp,
		nil,
		nil,
	)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
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
	if req.RefreshToken == "" {
		respond.JSON(
			w,
			http.StatusOK,
			map[string]string{"message": "Logged out"},
			nil,
			nil,
		)
		return
	}
	h.svc.logout(req.RefreshToken)
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Logged out"},
		nil,
		nil,
	)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID := ctxutil.GetUserID(r)
	u, err := h.svc.getUser(userID)
	if err != nil {
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "Failed to load user"},
			nil,
		)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		u,
		nil,
		nil,
	)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := ctxutil.GetUserID(r)
	var req UpdateProfileRequest
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
	u, err := h.svc.updateProfile(userID, req)
	if err != nil {
		respond.JSON(
			w,
			http.StatusInternalServerError,
			nil,
			&respond.Error{Code: "INTERNAL", Message: "Failed to update profile"},
			nil,
		)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		u,
		nil,
		nil,
	)
}
