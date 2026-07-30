package auth

import (
	"crm/internal/respond"
	"encoding/json"
	"net/http"
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
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.Email == "" || req.Password == "" {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Email and password are required"}, nil)
		return
	}
	resp, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if ae, ok := err.(*AuthError); ok {
			respond.JSON(w, http.StatusUnauthorized, nil, &respond.Error{Code: ae.Code, Message: ae.Message}, nil)
		} else {
			respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "Login failed"}, nil)
		}
		return
	}
	respond.JSON(w, http.StatusOK, resp, nil, nil)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.RefreshToken == "" {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Refresh token is required"}, nil)
		return
	}
	resp, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		if ae, ok := err.(*AuthError); ok {
			status := http.StatusUnauthorized
			if ae.Code == "TOKEN_REVOKED" {
				status = http.StatusUnauthorized
			}
			respond.JSON(w, status, nil, &respond.Error{Code: ae.Code, Message: ae.Message}, nil)
		} else {
			respond.JSON(w, http.StatusInternalServerError, nil, &respond.Error{Code: "INTERNAL", Message: "Refresh failed"}, nil)
		}
		return
	}
	respond.JSON(w, http.StatusOK, resp, nil, nil)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.JSON(w, http.StatusBadRequest, nil, &respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"}, nil)
		return
	}
	if req.RefreshToken == "" {
		respond.JSON(w, http.StatusOK, map[string]string{"message": "Logged out"}, nil, nil)
		return
	}
	h.svc.Logout(req.RefreshToken)
	respond.JSON(w, http.StatusOK, map[string]string{"message": "Logged out"}, nil, nil)
}
