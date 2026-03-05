package log

import (
	"context"

	"go.uber.org/zap"
)

type ctxKey string

const (
	fieldTraceID = "trace_id"
	fieldSpanID  = "span_id"
	fieldUserID  = "user_id"
	fieldService = "service"
	fieldEnv     = "env"
)

const (
	traceIDKey ctxKey = "trace_id"
	spanIDKey  ctxKey = "span_id"
	userIDKey  ctxKey = "user_id"
)

func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDKey, traceID)
}

func WithSpanID(ctx context.Context, spanID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if spanID == "" {
		return ctx
	}
	return context.WithValue(ctx, spanIDKey, spanID)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, userIDKey, userID)
}

func ContextFields(ctx context.Context) []zap.Field {
	if ctx == nil {
		return nil
	}

	fields := make([]zap.Field, 0, 3)
	if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
		fields = append(fields, zap.String(fieldTraceID, traceID))
	}
	if spanID, ok := ctx.Value(spanIDKey).(string); ok && spanID != "" {
		fields = append(fields, zap.String(fieldSpanID, spanID))
	}
	if userID, ok := ctx.Value(userIDKey).(string); ok && userID != "" {
		fields = append(fields, zap.String(fieldUserID, userID))
	}
	return fields
}
