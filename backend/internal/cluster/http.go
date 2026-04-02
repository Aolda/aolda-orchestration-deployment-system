package cluster

import (
	"net/http"

	"github.com/aolda/aods-backend/internal/core"
)

type Handler struct {
	Service *Service
}

func (h Handler) ListClusters(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.List(r.Context())
	if err != nil {
		core.WriteError(
			w,
			r,
			http.StatusInternalServerError,
			"CLUSTER_CATALOG_READ_FAILED",
			"Could not read the cluster catalog.",
			map[string]any{"error": err.Error()},
			true,
		)
		return
	}

	core.WriteJSON(w, http.StatusOK, struct {
		Items []Summary `json:"items"`
	}{Items: items})
}
