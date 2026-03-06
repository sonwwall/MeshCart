package trace

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Config struct {
	ServiceName string
	Environment string
	Endpoint    string
	Insecure    bool
}

func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	// 1) 创建 OTLP Trace exporter，把 span 发给 OTel Collector。
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// 2) 给当前进程打上资源标签，便于在 Jaeger/Grafana 中按服务筛选。
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// 3) 创建全局 TracerProvider，应用里所有 span 都从这里生产。
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// 4) 注册全局 Provider 与 Propagator，确保跨服务可传播 trace 上下文。
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func Tracer(name string) oteltrace.Tracer {
	return otel.Tracer(name)
}

func StartSpan(ctx context.Context, tracerName, spanName string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	// 统一的 span 创建入口：返回“携带新 span 的 context”。
	return Tracer(tracerName).Start(ctx, spanName, opts...)
}

func TraceID(ctx context.Context) string {
	spanCtx := oteltrace.SpanContextFromContext(ctx)
	if !spanCtx.HasTraceID() {
		return ""
	}
	return spanCtx.TraceID().String()
}

func SpanID(ctx context.Context) string {
	spanCtx := oteltrace.SpanContextFromContext(ctx)
	if !spanCtx.HasSpanID() {
		return ""
	}
	return spanCtx.SpanID().String()
}
