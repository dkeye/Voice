package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/dkeye/Voice/internal/config"
	handlers "github.com/dkeye/Voice/internal/transport/http"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("⚠️ Failed to load config:", err)
	}

	r := handlers.SetupRouter(ctx, cfg)
	addr := fmt.Sprintf(":%d", cfg.Port)

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		fmt.Println("Voice server started at :8080")
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
