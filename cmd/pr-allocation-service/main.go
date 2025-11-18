package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/meld/pr-allocation-service/internal/config"
	"github.com/meld/pr-allocation-service/internal/service"
	"github.com/meld/pr-allocation-service/internal/storage/postgres"
	transport "github.com/meld/pr-allocation-service/internal/transport/http"
	"github.com/meld/pr-allocation-service/pkg/logger"
	"go.uber.org/zap"
)

func main() {
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
	time.Sleep(3 * time.Second)
	storage, err := postgres.NewPostgresStorage(
		ctx,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)
	// Waiting for db is ready to receive connections. Set 3 seconds from experience
	if err != nil {
		log.Fatal(ctx, "Failed to connect to database", zap.Error(err))
	}
	defer storage.Close(ctx)
	log.Info(ctx, "storage connection established")
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
	log.Info(ctx, "starting http server")
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(ctx, "Failed to start http server", zap.Error(err))
		os.Exit(1)
	}
	log.Info(ctx, "http server started")
}
