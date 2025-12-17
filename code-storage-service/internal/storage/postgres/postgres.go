package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Meldy183/shared/pkg/logger"
	"go.uber.org/zap"
)

type Storage struct {
	db *sql.DB
}

func NewPostgresStorage(ctx context.Context, host, port, user, password, dbname, sslmode string) (*Storage, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)
	log := logger.FromContext(ctx)
	log.Debug(ctx, "destination:",
		zap.String("dsn", dsn),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Info(ctx, "postgres storage initialized successfully",
		zap.String("host", host),
		zap.String("port", port),
		zap.String("dbname", dbname),
	)
	return &Storage{db: db}, nil
}

func (s *Storage) Close(ctx context.Context) error {
	log := logger.FromContext(ctx)
	if err := s.db.Close(); err != nil {
		log.Error(ctx, "failed to close database connection", zap.Error(err))
		return err
	}
	log.Info(ctx, "database connection closed successfully")
	return nil
}
