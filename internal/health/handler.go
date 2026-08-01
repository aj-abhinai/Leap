package health

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"crm/internal/respond"
)

type Pinger interface {
	PingContext(context.Context) error
}

func Live(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"}, nil, nil)
}

func Ready(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			respond.JSON(w, http.StatusServiceUnavailable, nil, &respond.Error{
				Code:    "not_ready",
				Message: "database is unavailable",
			}, nil)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]string{"status": "ready"}, nil, nil)
	}
}

var _ Pinger = (*sql.DB)(nil)
