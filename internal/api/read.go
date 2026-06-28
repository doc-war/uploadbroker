package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/doc-war/uploadbroker/internal/config"
	"github.com/doc-war/uploadbroker/internal/hash"
	"github.com/doc-war/uploadbroker/internal/metadata"
	"github.com/doc-war/uploadbroker/internal/mime"
	"github.com/doc-war/uploadbroker/internal/storage"
)

type ReadHandler struct {
	cfg     *config.Config
	store   *metadata.Store
	drivers map[string]storage.Storage
}

func NewReadHandler(cfg *config.Config, store *metadata.Store, drivers map[string]storage.Storage) *ReadHandler {
	return &ReadHandler{cfg: cfg, store: store, drivers: drivers}
}

func (h *ReadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 40500, "method not allowed")
		return
	}

	key, expireUnix, ext, token, err := parseURLPath(r.URL.Path)
	if err != nil {
		writeError(w, 40401, fmt.Sprintf("invalid url: %v", err))
		return
	}

	if !hash.VerifyToken(h.cfg.URLBlake2bSalts, key, expireUnix, token) {
		writeError(w, 40401, "invalid token")
		return
	}

	if time.Unix(expireUnix, 0).Before(time.Now()) {
		writeError(w, 40402, "resource expired")
		return
	}

	rec, err := h.store.Get(key)
	if err != nil {
		writeError(w, 50000, fmt.Sprintf("db: %v", err))
		return
	}
	if rec == nil {
		writeError(w, 40402, "resource not found")
		return
	}

	expectedExt := mime.ExtensionFromMIME(rec.MIMEType)
	if ext != expectedExt {
		writeError(w, 40401, "extension mismatch")
		return
	}

	drv, ok := h.drivers[rec.Backend]
	if !ok {
		writeError(w, 50001, fmt.Sprintf("driver %s not available", rec.Backend))
		return
	}
	result, err := drv.Get(r.Context(), rec.StorageKey)
	if err != nil {
		writeError(w, 50001, fmt.Sprintf("storage: %v", err))
		return
	}
	defer result.Data.Close()

	w.Header().Set("Content-Type", rec.MIMEType)
	w.Header().Set("Content-Length", strconv.FormatInt(result.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, result.Data)
}

func parseURLPath(urlPath string) (key string, expireUnix int64, ext string, token string, err error) {
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	if len(parts) < 5 {
		return "", 0, "", "", fmt.Errorf("path too short")
	}

	expireStr := parts[1]
	shard := parts[2]
	key = parts[3]
	tokenExt := parts[4]

	if len(key) < 2 || shard != key[:2] {
		return "", 0, "", "", fmt.Errorf("shard mismatch")
	}

	expireUnix, e := strconv.ParseInt(expireStr, 10, 64)
	if e != nil {
		return "", 0, "", "", fmt.Errorf("bad expire")
	}

	dotIdx := strings.LastIndex(tokenExt, ".")
	if dotIdx < 0 {
		return "", 0, "", "", fmt.Errorf("missing ext")
	}
	token = tokenExt[:dotIdx]
	ext = tokenExt[dotIdx:]

	return key, expireUnix, ext, token, nil
}
