package api

import (
	"encoding/json"
	"net/http"
	"time"

	"uploadBroker/internal/metadata"
	"uploadBroker/internal/storage"
)

type HealthHandler struct {
	cfgVersion string
	store      *metadata.Store
	drivers    map[string]storage.Storage
}

func NewHealthHandler(version string, store *metadata.Store, drivers map[string]storage.Storage) *HealthHandler {
	return &HealthHandler{cfgVersion: version, store: store, drivers: drivers}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	storeErr := h.store.Health()

	status := "ok"
	httpStatus := http.StatusOK
	allDriverStatus := "ok"

	for name, drv := range h.drivers {
		if err := drv.Health(r.Context()); err != nil {
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
			allDriverStatus = name + ": " + err.Error()
			break
		}
	}

	storeStatus := "ok"
	if storeErr != nil {
		storeStatus = storeErr.Error()
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version": h.cfgVersion,
		"status":  status,
		"storage": allDriverStatus,
		"sqlite":  storeStatus,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}
