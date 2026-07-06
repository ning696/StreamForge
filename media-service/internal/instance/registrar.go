package instance

import (
	"context"
	"time"

	"streamforge/media-service/internal/config"
	"streamforge/media-service/internal/router"
)

type StatsFunc func() (rooms int, peers int)

type Registrar struct {
	cfg       config.Config
	store     router.Store
	startedAt time.Time
	stats     StatsFunc
}

func NewRegistrar(cfg config.Config, store router.Store, stats StatsFunc) *Registrar {
	return &Registrar{
		cfg:       cfg,
		store:     store,
		startedAt: time.Now(),
		stats:     stats,
	}
}

func (r *Registrar) RegisterOnce(ctx context.Context) error {
	rooms, peers := 0, 0
	if r.stats != nil {
		rooms, peers = r.stats()
	}
	return r.store.UpsertInstance(ctx, router.InstanceInfo{
		ID:              r.cfg.MediaInstanceID,
		Host:            r.cfg.PublicHTTPHost,
		HTTPPort:        r.cfg.ServerPort,
		WSPort:          r.cfg.ServerPort,
		RTCPortRange:    r.cfg.RTCPortRange,
		StartedAt:       r.startedAt,
		LastHeartbeatAt: time.Now(),
		Status:          "healthy",
		RoomCount:       rooms,
		PeerCount:       peers,
	})
}

func (r *Registrar) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		_ = r.RegisterOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = r.RegisterOnce(ctx)
			}
		}
	}()
}
