package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/doc-war/uploadbroker/internal/metadata"
	"github.com/doc-war/uploadbroker/internal/storage"
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

	activeCount := -1
	if n, err := h.store.CountActive(time.Now().Unix()); err == nil {
		activeCount = n
	}

	// time 返回二进制（可执行文件）的最终修改时间，即部署落盘时间；
	// 取不到时返回空字符串（不回退为请求时间，避免误导）。
	binTime := binaryModTime()
	timeStr := ""
	if !binTime.IsZero() {
		timeStr = binTime.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":     h.cfgVersion,
		"status":      status,
		"storage":     allDriverStatus,
		"sqlite":      storeStatus,
		"activeCount": activeCount,
		"time":        timeStr,
	})
}

// binaryModTime 返回当前运行二进制的修改时间（部署落盘时间）。
func binaryModTime() time.Time {
	exe, err := os.Executable()
	if err != nil {
		return time.Time{}
	}
	info, err := os.Stat(exe)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime().UTC()
}
