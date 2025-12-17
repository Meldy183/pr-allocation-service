package logger

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	loggerRequestIDKey = "x-request-id"
	loggerKey          = "logger"
)

type Logger interface {
	Info(ctx context.Context, mgs string, fields ...zap.Field)
	Debug(ctx context.Context, mgs string, fields ...zap.Field)
	Warn(ctx context.Context, mgs string, fields ...zap.Field)
	Error(ctx context.Context, mgs string, fields ...zap.Field)
	Fatal(ctx context.Context, mgs string, fields ...zap.Field)
}

type L struct {
	z *zap.Logger
}

func NewLogger(env string) Logger {
	loggerCfg := zap.NewProductionConfig()
	loggerCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	if env == "dev" {
		loggerCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	logger, err := loggerCfg.Build()
	if err != nil {
		return nil
	}
	l := L{z: logger}
	return &l
}

func (l *L) Info(ctx context.Context, mgs string, fields ...zap.Field) {
	id, ok := ctx.Value(loggerRequestIDKey).(string)
	if !ok {
		id = uuid.NewString()
	}
	fields = append(fields, zap.String(loggerRequestIDKey, id))
	l.z.Info(mgs, fields...)
}
func (l *L) Debug(ctx context.Context, mgs string, fields ...zap.Field) {
	id, ok := ctx.Value(loggerRequestIDKey).(string)
	if !ok {
		id = uuid.NewString()
	}
	fields = append(fields, zap.String(loggerRequestIDKey, id))
	l.z.Debug(mgs, fields...)
}
func (l *L) Warn(ctx context.Context, mgs string, fields ...zap.Field) {
	id, ok := ctx.Value(loggerRequestIDKey).(string)
	if !ok {
		id = uuid.NewString()
	}
	fields = append(fields, zap.String(loggerRequestIDKey, id))
	l.z.Warn(mgs, fields...)
}
func (l *L) Error(ctx context.Context, mgs string, fields ...zap.Field) {
	id, ok := ctx.Value(loggerRequestIDKey).(string)
	if !ok {
		id = uuid.NewString()
	}
	fields = append(fields, zap.String(loggerRequestIDKey, id))
	l.z.Error(mgs, fields...)
}

func (l *L) Fatal(ctx context.Context, mgs string, fields ...zap.Field) {
	id, ok := ctx.Value(loggerRequestIDKey).(string)
	if !ok {
		id = uuid.NewString()
	}
	fields = append(fields, zap.String(loggerRequestIDKey, id))
	l.z.Fatal(mgs, fields...)
}
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, loggerRequestIDKey, requestID)
}

func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func FromContext(ctx context.Context) Logger {
	logger, ok := ctx.Value(loggerKey).(Logger)
	if !ok || logger == nil {
		// Return a basic logger as fallback to avoid panics
		return NewLogger("prod")
	}
	return logger
}

//
