package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"uploadBroker/internal/config"
	"uploadBroker/internal/hash"
	"uploadBroker/internal/metadata"
	"uploadBroker/internal/mime"
	"uploadBroker/internal/storage"
)

type UploadHandler struct {
	cfg     *config.Config
	store   *metadata.Store
	drivers map[string]storage.Storage
}

func NewUploadHandler(cfg *config.Config, store *metadata.Store, drivers map[string]storage.Storage) *UploadHandler {
	return &UploadHandler{cfg: cfg, store: store, drivers: drivers}
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 40500, "method not allowed")
		return
	}

	if err := h.verifyHMAC(r); err != nil {
		writeError(w, 50000, err.Error())
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, 40001, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, 40001, "missing file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, 50000, fmt.Sprintf("read: %v", err))
		return
	}

	if len(data) == 0 {
		writeError(w, 40002, "empty file")
		return
	}

	mimeInfo, ok := mime.Detect(data)
	if !ok {
		writeError(w, 40003, "unsupported mime")
		return
	}

	maxSize := h.maxSize(mimeInfo.Category)
	if int64(len(data)) > maxSize {
		writeError(w, 40004, fmt.Sprintf("max size %d bytes", maxSize))
		return
	}

	expires := h.cfg.DefaultTTL
	if es := r.FormValue("expires"); es != "" {
		hours, err := strconv.Atoi(es)
		if err != nil || hours < 1 || hours > 24 {
			writeError(w, 40005, "expires 1-24")
			return
		}
		expires = time.Duration(hours) * time.Hour
	}

	key := hash.Sum(data)
	now := time.Now().UTC()
	expireAt := now.Add(expires)

	existing, err := h.store.Get(key)
	if err != nil {
		writeError(w, 50000, fmt.Sprintf("db: %v", err))
		return
	}

	if existing != nil {
		if time.Unix(existing.ExpireAt, 0).Sub(now) >= 10*time.Minute {
			writeSuccess(w, h.buildResponse(key, existing.MIMEType, existing.Size, existing.ExpireAt))
			return
		}
	}

	drv := h.drivers[h.cfg.Storage.UploadDriver]
	result, err := drv.Put(r.Context(), &storage.Object{
		Key:       key,
		Data:      data,
		MIMEType:  mimeInfo.MIME,
		Extension: mimeInfo.Extension,
	})
	if err != nil {
		writeError(w, 50001, fmt.Sprintf("storage: %v", err))
		return
	}

	if err := h.store.Insert(&metadata.Record{
		Key:        key,
		MIMEType:   mimeInfo.MIME,
		Size:       int64(len(data)),
		Backend:    result.Backend,
		StorageKey: result.StorageKey,
		CreatedAt:  now.Unix(),
		ExpireAt:   expireAt.Unix(),
	}); err != nil {
		log.Printf("db insert failed: %v (storage may leak: %s)", err, result.StorageKey)
		writeError(w, 50000, "db error")
		return
	}

	writeSuccess(w, h.buildResponse(key, mimeInfo.MIME, int64(len(data)), expireAt.Unix()))
}

func (h *UploadHandler) buildResponse(key, mimeType string, size, expireAt int64) map[string]interface{} {
	ext := mime.ExtensionFromMIME(mimeType)
	salt := h.cfg.URLBlake2bSalts[0]
	return map[string]interface{}{
		"url":      hash.BuildURL(h.cfg.BaseURL, h.cfg.URLPrefix, key, ext, expireAt, salt),
		"mimeType": mimeType,
		"size":     size,
		"expireAt": time.Unix(expireAt, 0).Format(time.RFC3339),
	}
}

func (h *UploadHandler) maxSize(category string) int64 {
	switch category {
	case "image":
		return int64(h.cfg.Limits.Image)
	case "audio":
		return int64(h.cfg.Limits.Audio)
	case "video":
		return int64(h.cfg.Limits.Video)
	default:
		return int64(h.cfg.Limits.Image)
	}
}

func (h *UploadHandler) verifyHMAC(r *http.Request) error {
	if h.cfg.HMACSecret == "" {
		return nil
	}
	sign := r.FormValue("sign")
	ts := r.FormValue("timestamp")
	if sign == "" || ts == "" {
		return fmt.Errorf("missing sign or timestamp")
	}
	if !hash.VerifyHMAC(h.cfg.HMACSecret, ts, sign) {
		return fmt.Errorf("invalid sign")
	}
	return nil
}
