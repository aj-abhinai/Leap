package export

import (
	"bytes"
	"net/http"
	"time"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// CSV streams the requested entity as a CSV file download.
// GET /api/export/csv?entity=contacts|leads
func (h *Handler) CSV(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("entity")

	var buf bytes.Buffer
	var err error
	switch entity {
	case "contacts":
		err = h.svc.ExportContactsCSV(&buf)
	case "leads":
		err = h.svc.ExportLeadsCSV(&buf)
	default:
		http.Error(w, `entity must be one of: contacts, leads`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}

	filename := entity + "-" + time.Now().Format("20060102") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	// UTF-8 BOM so Excel opens multi-byte characters correctly.
	w.Write(append([]byte{0xEF, 0xBB, 0xBF}, buf.Bytes()...))
}
