package activity

import (
	"net/http"
	"strconv"

	"crm/internal/respond"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	filters := ActivityFilters{
		UserID:       r.URL.Query().Get("user_id"),
		Action:       r.URL.Query().Get("action"),
		ResourceType: r.URL.Query().Get("resource_type"),
	}

	entries, total, err := h.svc.list(page, perPage, filters)
	if err != nil {
		respond.ServerError(w, err)
		return
	}
	respond.JSON(
		w,
		http.StatusOK,
		entries,
		nil,
		&respond.Meta{Page: page, PerPage: perPage, Total: total},
	)
}
