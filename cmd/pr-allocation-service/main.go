package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/meld/pr-allocation-service/config"
	httpHandler "github.com/meld/pr-allocation-service/internal/http"
	"github.com/meld/pr-allocation-service/internal/service"
	"github.com/meld/pr-allocation-service/internal/storage/postgres"
	"github.com/meld/pr-allocation-service/pkg/logger"
)

func main() {
	cfg := config.Load()

	log := logger.NewLogger(cfg.Env)
	if log == nil {
		fmt.Println("Failed to initialize logger")
		os.Exit(1)
	}

	// Create context with logger
	ctx := context.Background()
	ctx = logger.WithLogger(ctx, log)

	log.Info(ctx, "starting pr-allocation-service",
		zap.String("env", cfg.Env),
		zap.String("port", cfg.ServerPort))

	storage, err := postgres.NewPostgresStorage(
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)
	if err != nil {
		log.Error(ctx, "failed to initialize storage", zap.Error(err))
		os.Exit(1)
	}
	defer storage.Close()

	log.Info(ctx, "database connection established")

	svc := service.NewService(storage)
	handler := httpHandler.NewHandler(svc)

	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info(ctx, "server starting", zap.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx, "server failed to start", zap.Error(err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info(ctx, "server shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error(ctx, "server forced to shutdown", zap.Error(err))
		os.Exit(1)
	}

	log.Info(ctx, "server stopped gracefully")
}
