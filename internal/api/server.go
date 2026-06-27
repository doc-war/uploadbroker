package api

import (
	"fmt"
	"net"
	"net/http"

	"uploadBroker/internal/config"
	"uploadBroker/internal/metadata"
	"uploadBroker/internal/storage"
)

func StartServer(cfg *config.Config, store *metadata.Store, drivers map[string]storage.Storage) (net.Listener, http.Handler, error) {
	mux := http.NewServeMux()

	mux.Handle("/v1/upload", NewUploadHandler(cfg, store, drivers))
	mux.Handle("/v1/health", NewHealthHandler(cfg.Version, store, drivers))
	mux.Handle("/tmp/", NewReadHandler(cfg, store, drivers))

	handler := LoggingMiddleware(mux)

	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return nil, nil, fmt.Errorf("listen: %w", err)
	}

	return listener, handler, nil
}
