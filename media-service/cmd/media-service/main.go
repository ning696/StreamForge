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
	livekittoken "streamforge/media-service/internal/livekit"
	"streamforge/media-service/internal/room"
	"streamforge/media-service/internal/state"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	redisClient := state.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	store := state.NewRedisStore(redisClient)
	issuer := livekittoken.NewIssuer(livekittoken.FromConfig(cfg))
	server := gateway.NewServer(cfg, store, issuer, room.NewHub())
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

	log.Printf("media-service room/token service listening on :%d", cfg.ServerPort)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
