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

// decodeJSON decodes the request body into dst, writing a 400 on failure. It
// reports whether the caller may proceed.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		respond.JSON(
			w,
			http.StatusBadRequest,
			nil,
			&respond.Error{Code: "BAD_REQUEST", Message: "Invalid JSON"},
			nil,
		)
		return false
	}
	return true
}

// writeAuthError writes an AuthError with the given status; other errors go
// through respond.ServerError.
func writeAuthError(w http.ResponseWriter, status int, err error) {
	var ae *AuthError
	if !errors.As(err, &ae) {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		status,
		nil,
		&respond.Error{Code: ae.Code, Message: ae.Message},
		nil,
	)
}

// establishSession writes the refresh and CSRF cookies for a fresh session.
// If the CSRF token cannot be generated no usable session exists, so both
// cookies are cleared again and a 500 is sent instead.
func (h *Handler) establishSession(w http.ResponseWriter, refreshToken string) bool {
	h.svc.setRefreshCookie(w, refreshToken)
	if err := h.svc.setCSRFCookie(w); err != nil {
		h.svc.clearRefreshCookie(w)
		h.svc.clearCSRFCookie(w)
		respond.ServerError(w, err)
		return false
	}
	return true
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decodeJSON(w, r, &req) {
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
		writeAuthError(w, http.StatusUnauthorized, err)
		return
	}
	if h.actLog != nil {
		h.actLog.LogLogin(u.ID, u.Name, u.Email)
	}
	if !h.establishSession(w, resp.RefreshToken) {
		return
	}
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
		writeAuthError(w, http.StatusUnauthorized, err)
		return
	}
	if !h.establishSession(w, resp.RefreshToken) {
		return
	}
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
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := h.svc.updateProfile(userID, req)
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, err)
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
	if !decodeJSON(w, r, &req) {
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
		writeAuthError(w, http.StatusBadRequest, err)
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
