package auth

import (
	"crm/internal/activity"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"crm/internal/ctxutil"
	"crm/internal/respond"
)

type Handler struct {
	svc    *Service
	actLog *activity.Service
}

func NewHandler(svc *Service, actLog *activity.Service) *Handler {
	return &Handler{svc: svc, actLog: actLog}
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
	u, resp, mustChange, err := h.svc.login(req.Email, req.Password)
	if err != nil {
		var ae *AuthError
		if errors.As(err, &ae) {
			respond.JSON(
				w,
				http.StatusUnauthorized,
				nil,
				&respond.Error{Code: ae.Code, Message: ae.Message},
				nil,
			)
			return
		}
		respond.ServerError(w, err)
		return
	}
	if h.actLog != nil {
		h.actLog.LogLogin(u.ID, u.Name, u.Email)
	}
	h.svc.setRefreshCookie(w, resp.RefreshToken)
	h.svc.setCSRFCookie(w)
	respond.JSON(
		w,
		http.StatusOK,
		map[string]any{
			"access_token":         resp.AccessToken,
			"expires_at":           resp.ExpiresAt,
			"must_change_password": mustChange,
		},
		nil,
		nil,
	)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(RefreshCookieName)
	if err != nil || cookie.Value == "" {
		h.svc.clearRefreshCookie(w)
		respond.JSON(
			w,
			http.StatusUnauthorized,
			nil,
			&respond.Error{Code: "UNAUTHORIZED", Message: "Refresh token is required"},
			nil,
		)
		return
	}
	resp, err := h.svc.refresh(cookie.Value)
	if err != nil {
		h.svc.clearRefreshCookie(w)
		var ae *AuthError
		if errors.As(err, &ae) {
			respond.JSON(
				w,
				http.StatusUnauthorized,
				nil,
				&respond.Error{Code: ae.Code, Message: ae.Message},
				nil,
			)
			return
		}
		respond.ServerError(w, err)
		return
	}
	h.svc.setRefreshCookie(w, resp.RefreshToken)
	h.svc.setCSRFCookie(w)
	respond.JSON(
		w,
		http.StatusOK,
		map[string]any{
			"access_token": resp.AccessToken,
			"expires_at":   resp.ExpiresAt,
		},
		nil,
		nil,
	)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(RefreshCookieName); err == nil && cookie.Value != "" {
		userID, userName, email, err := h.svc.logout(cookie.Value)
		if err != nil {
			slog.Error("logout revoke failed", "error", err)
		} else if h.actLog != nil && userID != "" {
			h.actLog.LogLogout(userID, userName, email)
		}
	}
	h.svc.clearRefreshCookie(w)
	h.svc.clearCSRFCookie(w)
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
		respond.ServerError(w, err)
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
		respond.ServerError(w, err)
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

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := ctxutil.GetUserID(r)
	var req ChangePasswordRequest
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
	if req.CurrentPassword == "" || req.NewPassword == "" {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Current and new passwords are required"},
			nil,
		)
		return
	}
	if err := h.svc.changePassword(userID, req.CurrentPassword, req.NewPassword); err != nil {
		var ae *AuthError
		if errors.As(err, &ae) {
			respond.JSON(
				w,
				http.StatusBadRequest,
				nil,
				&respond.Error{Code: ae.Code, Message: ae.Message},
				nil,
			)
			return
		}
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		map[string]string{"message": "Password changed"},
		nil,
		nil,
	)
}
