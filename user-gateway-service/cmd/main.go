package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meldy183/shared/pkg/logger"
	"github.com/Meldy183/user-gateway-service/internal/client"
	"github.com/Meldy183/user-gateway-service/internal/config"
	"github.com/Meldy183/user-gateway-service/internal/service"
	"github.com/Meldy183/user-gateway-service/internal/transport"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func main() {
	pathConfig := os.Getenv("CONFIG_PATH")
	err := config.MustLoadConfig(pathConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	env := cfg.ENV
	log := logger.NewLogger(env)
	ctx := context.Background()
	ctx = logger.WithLogger(ctx, log)
	ctx = logger.WithRequestID(ctx, uuid.NewString())
	log.Debug(ctx, "Enabled debug logging")
	log.Info(ctx, "Enabled info logging")
	log.Info(ctx, "starting user-gateway-service",
		zap.String("env", env),
	)

	// Initialize clients
	prClient := client.NewPRAllocationClient(cfg.GetPRAllocationURL())
	codeClient := client.NewCodeStorageClient(cfg.GetCodeStorageURL())

	log.Info(ctx, "clients initialized",
		zap.String("pr_allocation_url", cfg.GetPRAllocationURL()),
		zap.String("code_storage_url", cfg.GetCodeStorageURL()),
	)

	// Initialize service and handler
	svc := service.NewService(prClient, codeClient)
	handler := transport.NewHandler(svc)
	router := mux.NewRouter()
	handler.RegisterRoutes(router, log)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // Longer for file uploads
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

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error(ctx, "graceful shutdown failed, forcing shutdown", zap.Error(err))
			if closeErr := server.Close(); closeErr != nil {
				log.Error(ctx, "server close error", zap.Error(closeErr))
			}
		}
		log.Info(ctx, "server shutdown completed gracefully")
	}
}
