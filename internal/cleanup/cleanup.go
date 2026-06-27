package cleanup

import (
	"context"
	"log"
	"time"

	"uploadBroker/internal/config"
	"uploadBroker/internal/metadata"
	"uploadBroker/internal/storage"
)

type Task struct {
	cfg     *config.Config
	store   *metadata.Store
	drivers map[string]storage.Storage
}

func New(cfg *config.Config, store *metadata.Store, drivers map[string]storage.Storage) *Task {
	return &Task{cfg: cfg, store: store, drivers: drivers}
}

func (t *Task) Start(ctx context.Context) {
	ticker := time.NewTicker(t.cfg.CleanupInterval)
	defer ticker.Stop()

	t.runOnce()

	for {
		select {
		case <-ticker.C:
			t.runOnce()
		case <-ctx.Done():
			return
		}
	}
}

func (t *Task) runOnce() {
	now := time.Now().Unix()

	expired, err := t.store.ListExpired(now)
	if err != nil {
		log.Printf("cleanup: list expired failed: %v", err)
		return
	}

	var deleted, failed int
	for _, rec := range expired {
		drv, ok := t.drivers[rec.Backend]
		if !ok {
			log.Printf("cleanup: driver %s not available for %s", rec.Backend, rec.StorageKey)
			failed++
			continue
		}
		if err := drv.Delete(context.Background(), rec.StorageKey); err != nil {
			log.Printf("cleanup: delete storage %s failed: %v", rec.StorageKey, err)
			failed++
			continue
		}
		if err := t.store.Delete(rec.Key); err != nil {
			log.Printf("cleanup: delete metadata %s failed: %v", rec.Key, err)
			failed++
			continue
		}
		deleted++
	}

	if deleted > 0 || failed > 0 {
		log.Printf("cleanup: deleted=%d failed=%d total=%d", deleted, failed, len(expired))
	}
}
