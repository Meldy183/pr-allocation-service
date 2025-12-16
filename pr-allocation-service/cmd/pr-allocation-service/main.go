package main

import (
	"context"
	"fmt"
	"github.com/meld/pr-allocation-service/pr-allocation-service/internal/config"
	"github.com/meld/pr-allocation-service/pr-allocation-service/internal/service"
	"github.com/meld/pr-allocation-service/pr-allocation-service/internal/storage/postgres"
	transport "github.com/meld/pr-allocation-service/pr-allocation-service/internal/transport/http"
	"github.com/meld/pr-allocation-service/pr-allocation-service/pkg/logger"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func main() {
	// i will use english for comments because of simplicity and fast switching while typing
	pathConfig := os.Getenv("CONFIG_PATH")
	err := config.MustLoadConfig(pathConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Get config struct from viper
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(cfg.ENV)
	env := cfg.ENV
	log := logger.NewLogger(env)
	ctx := context.Background()
	ctx = logger.WithLogger(ctx, log)
	ctx = logger.WithRequestID(ctx, uuid.NewString())
	log.Debug(ctx, "Enabled debug logging")
	log.Info(ctx, "Enabled info logging")
	log.Info(ctx, "starting pr-allocation-service",
		zap.String("env", env),
	)

	// Connect to database with retry logic (exponential backoff)
	storage, err := connectWithRetry(
		ctx,
		log,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)
	if err != nil {
		log.Fatal(ctx, "Failed to connect to database after retries", zap.Error(err))
	}
	defer func() {
		if closeErr := storage.Close(ctx); closeErr != nil {
			log.Error(ctx, "Failed to close storage connection", zap.Error(closeErr))
		}
	}()
	log.Info(ctx, "storage connection established")

	// Initialize service and HTTP handler
	svc := service.NewService(storage)
	handler := transport.NewHandler(svc)
	router := mux.NewRouter()
	handler.RegisterRoutes(router, log)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for errors from the server
	serverErrors := make(chan error, 1)

	// Start HTTP server in a goroutine
	go func() {
		log.Info(ctx, "http server starting", zap.String("port", cfg.Server.Port))
		serverErrors <- server.ListenAndServe()
	}()

	// Channel to listen for interrupt or terminate signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(ctx, "server error", zap.Error(err))
		}

	case sig := <-shutdown:
		log.Info(ctx, "shutdown signal received", zap.String("signal", sig.String()))

		// Give outstanding requests 30 seconds to complete
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Gracefully shutdown the server
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error(ctx, "graceful shutdown failed, forcing shutdown", zap.Error(err))
			if closeErr := server.Close(); closeErr != nil {
				log.Error(ctx, "server close error", zap.Error(closeErr))
			}
		}
		log.Info(ctx, "server shutdown completed gracefully")
	}
}

// connectWithRetry attempts to connect to the database with exponential backoff retry logic
func connectWithRetry(
	ctx context.Context,
	log logger.Logger,
	host, port, user, password, dbname, sslmode string,
) (*postgres.Storage, error) {
	const (
		maxRetries     = 10
		initialBackoff = 1 * time.Second
		maxBackoff     = 30 * time.Second
	)

	var storage *postgres.Storage
	var err error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Info(ctx, "attempting to connect to database",
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", maxRetries),
		)

		storage, err = postgres.NewPostgresStorage(ctx, host, port, user, password, dbname, sslmode)
		if err == nil {
			log.Info(ctx, "successfully connected to database",
				zap.Int("attempt", attempt),
			)
			return storage, nil
		}

		if attempt == maxRetries {
			log.Error(ctx, "max retry attempts reached, giving up",
				zap.Int("attempts", attempt),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, err)
		}

		log.Warn(ctx, "database connection failed, retrying...",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)

		time.Sleep(backoff)

		// Exponential backoff with jitter
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return nil, fmt.Errorf("failed to connect to database: %w", err)
}
