package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/burakcan/ipburack/internal/config"
	"github.com/burakcan/ipburack/internal/geodb"
	"github.com/burakcan/ipburack/internal/handlers"
	"github.com/burakcan/ipburack/internal/logger"
	"github.com/burakcan/ipburack/internal/middleware"
)

func main() {
	log := logger.New()
	cfg := config.Load()

	log.Info("starting server", map[string]any{
		"host":                  cfg.Host,
		"port":                  cfg.Port,
		"mmdb_path":             cfg.MMDBPath,
		"mmdb_url":              cfg.MMDBURL,
		"update_interval_hours": cfg.UpdateIntervalHours,
		"api_key_enabled":       cfg.APIKey != "",
	})

	// Initialize the geo database
	updateInterval := time.Duration(cfg.UpdateIntervalHours) * time.Hour
	geo := geodb.New(cfg.MMDBPath, cfg.MMDBURL, updateInterval, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := geo.Start(ctx); err != nil {
		log.Error("failed to start geo database", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// Initialize handlers and auth middleware
	h := handlers.New(geo)
	auth := middleware.NewAuth(cfg.APIKey)

	// Set up routes (health is public, lookup requires auth)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /lookup", auth.Wrap(h.LookupSelf))
	mux.HandleFunc("GET /lookup/{ip}", auth.Wrap(h.LookupIP))

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("server listening", map[string]any{
			"addr": cfg.Addr(),
		})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", map[string]any{
				"error": err.Error(),
			})
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server", nil)

	// Give outstanding requests time to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", map[string]any{
			"error": err.Error(),
		})
	}

	// Stop the geo database (stops background updates)
	geo.Stop()

	log.Info("server stopped", nil)
}
