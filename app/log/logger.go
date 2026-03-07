package log

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Service string
	Env     string
	Level   string
	LogDir  string
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

	encoderCfg := zapcore.EncoderConfig{
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
	}

	cores := []zapcore.Core{
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			level,
		),
	}
	if cfg.LogDir != "" && cfg.Service != "" {
		if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
			return err
		}
		logFile := filepath.Join(cfg.LogDir, cfg.Service+".log")
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(file),
			level,
		))
	}

	logger := zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(1))

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
