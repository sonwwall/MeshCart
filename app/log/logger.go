package log

import (
	"context"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Service string
	Env     string
	Level   string
}

var (
	globalLogger = zap.NewNop()
	loggerMu     sync.RWMutex
)

func Init(cfg Config) error {
	level := zapcore.InfoLevel
	if err := level.Set(strings.ToLower(cfg.Level)); err != nil {
		return err
	}

	zapCfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: cfg.Env != "prod",
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    "func",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := zapCfg.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	if err != nil {
		return err
	}

	baseFields := make([]zap.Field, 0, 2)
	if cfg.Service != "" {
		baseFields = append(baseFields, zap.String(fieldService, cfg.Service))
	}
	if cfg.Env != "" {
		baseFields = append(baseFields, zap.String(fieldEnv, cfg.Env))
	}
	logger = logger.With(baseFields...)

	loggerMu.Lock()
	globalLogger = logger
	loggerMu.Unlock()
	return nil
}

func L(ctx context.Context) *zap.Logger {
	loggerMu.RLock()
	logger := globalLogger
	loggerMu.RUnlock()

	fields := ContextFields(ctx)
	if len(fields) == 0 {
		return logger
	}
	return logger.With(fields...)
}

func Sync() {
	loggerMu.RLock()
	logger := globalLogger
	loggerMu.RUnlock()
	_ = logger.Sync()
}
