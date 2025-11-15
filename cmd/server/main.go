package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	router "github.com/dkeye/Voice/internal/adapters/http"
	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/app/orch"
	"github.com/dkeye/Voice/internal/app/sfu"
	"github.com/dkeye/Voice/internal/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize zerolog global logger early so config.Load can use it.
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	// Human-friendly output for terminal; in production you may want JSON only.
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cfg, err := config.Load()
	if err != nil {
		log.Error().Err(err).Msg("failed to load config")
	}

	// Properly wire orchestrator with room manager and policy.
	manager := app.NewRoomManager()
	policy := app.SimplePolicy{}
	reg := app.NewRegistry()
	relays := sfu.NewRelayManager()

	orch := &orch.Orchestrator{
		Registry: reg,
		Rooms:    manager,
		Policy:   policy,
		Relays:   relays,
	}

	r := router.SetupRouter(ctx, cfg, orch)
	addr := fmt.Sprintf(":%d", cfg.Port)

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("Voice server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("Shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}
	log.Info().Msg("Server exited gracefully")
}
