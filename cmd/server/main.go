package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/dkeye/Voice/internal/adapters"
	"github.com/dkeye/Voice/internal/app"
	"github.com/dkeye/Voice/internal/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("⚠️ Failed to load config:", err)
	}

	// Properly wire orchestrator with room manager and policy.
	manager := app.NewRoomManager()
	policy := app.SimplePolicy{}
	reg := app.NewRegistry()
	orch := &app.Orchestrator{
		Registry: reg,
		Rooms:  manager,
		Policy: policy,
	}

	r := adapters.SetupRouter(ctx, cfg, orch)
	addr := fmt.Sprintf(":%d", cfg.Port)

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		fmt.Printf("Voice server started at %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("server error:", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Println("Server forced to shutdown:", err)
	}
	fmt.Println("Server exited gracefully.")
}
