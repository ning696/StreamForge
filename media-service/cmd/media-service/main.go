package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"streamforge/media-service/internal/config"
	"streamforge/media-service/internal/gateway"
	"streamforge/media-service/internal/instance"
	"streamforge/media-service/internal/room"
	"streamforge/media-service/internal/router"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	redisClient := router.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	store := router.NewRedisStore(redisClient)
	hub := room.NewHub(cfg.MediaInstanceID)
	registrar := instance.NewRegistrar(cfg, store, hub.Stats)
	registrar.Start(ctx, 10*time.Second)

	server := gateway.NewServer(cfg, store, hub)
	httpServer := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.ServerPort),
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("media-service %s listening on :%d", cfg.MediaInstanceID, cfg.ServerPort)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
