// Package main is the entry point for the LLM Gateway server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lex1ng/llm-gateway/api"
	"github.com/lex1ng/llm-gateway/config"
	"github.com/lex1ng/llm-gateway/pkg/gateway"
)

var (
	configPath = flag.String("config", "config/config.yaml", "Path to configuration file")
	envPath    = flag.String("env", "", "Path to .env file (optional, auto-detected if not set)")
	version    = "dev"
)

func main() {
	flag.Parse()

	// Set up structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting llm-gateway", "version", version)

	// Load .env file (does NOT override existing env vars)
	if *envPath != "" {
		if err := godotenv.Load(*envPath); err != nil {
			logger.Warn("failed to load specified .env file", "path", *envPath, "error", err)
		}
	} else {
		_ = godotenv.Load() // silently ignore if .env not found
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err, "path", *configPath)
		os.Exit(1)
	}

	// Create gateway client
	client, err := gateway.NewWithConfig(cfg, gateway.WithLogger(logger))
	if err != nil {
		logger.Error("failed to create gateway client", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	// Create HTTP router
	router := api.NewRouter(client)

	// Set up HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server starting", "address", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
