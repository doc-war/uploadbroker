package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"uploadBroker/internal/api"
	"uploadBroker/internal/cleanup"
	"uploadBroker/internal/config"
	"uploadBroker/internal/metadata"
	"uploadBroker/internal/storage"
)

const (
	version = "1.0.0"
	logDir  = "./data/logs"
)

func main() {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("create log dir: %v", err)
	}

	logFile, err := os.OpenFile(logDir+"/broker.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("panic: %v\n\n%s", r, debug.Stack())
			os.WriteFile(logDir+"/panic.log", []byte(msg), 0644)
			log.Printf("panic recovered, see panic.log")
			os.Exit(1)
		}
	}()

	cfgPath := flag.String("config", "./uploadBroker.yaml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg.Version = version

	log.Printf("loaded config from %s", *cfgPath)
	log.Printf("  version: %s", version)
	log.Printf("  listen: %s", cfg.Listen)
	log.Printf("  base_url: %s", cfg.BaseURL)
	log.Printf("  url_prefix: %s", cfg.URLPrefix)
	log.Printf("  default_ttl: %v", cfg.DefaultTTL)
	log.Printf("  cleanup_interval: %v", cfg.CleanupInterval)
	log.Printf("  metadata_db: %s", cfg.MetadataDB)
	log.Printf("  upload_driver: %s", cfg.Storage.UploadDriver)
	log.Printf("  limits: image=%v audio=%v video=%v", cfg.Limits.Image, cfg.Limits.Audio, cfg.Limits.Video)
	if cfg.HMACSecret != "" {
		log.Printf("  hmac_secret: <configured>")
	}
	log.Printf("  url_blake2b_salts: %d salt(s) configured", len(cfg.URLBlake2bSalts))

	store, err := metadata.Open(cfg.MetadataDB)
	if err != nil {
		log.Fatalf("metadata: %v", err)
	}
	defer store.Close()

	if _, ok := cfg.Storage.Drivers[cfg.Storage.UploadDriver]; !ok {
		log.Fatalf("upload_driver %s not configured", cfg.Storage.UploadDriver)
	}

	drivers := make(map[string]storage.Storage)
	for name, dc := range cfg.Storage.Drivers {
		switch name {
		case "local":
			drivers[name] = storage.NewLocalDriver(dc.Root)
		default:
			log.Fatalf("unsupported driver: %s", name)
		}
	}
	log.Printf("  drivers: %d configured (%s)", len(drivers), cfg.Storage.UploadDriver)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cleanup.New(cfg, store, drivers).Start(ctx)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	listener, handler, err := api.StartServer(cfg, store, drivers)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	addr := listener.Addr().String()
	os.WriteFile(".port", []byte(addr+"\n"), 0644)
	log.Printf("listening on %s", addr)
	if err := http.Serve(listener, handler); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
