package trace

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type hertzHeaderCarrier struct {
	ctx *app.RequestContext
}

func (c hertzHeaderCarrier) Get(key string) string {
	return string(c.ctx.GetHeader(key))
}

func (c hertzHeaderCarrier) Set(key, value string) {
	c.ctx.Request.Header.Set(key, value)
}

func (c hertzHeaderCarrier) Keys() []string {
	keys := make([]string, 0, 16)
	c.ctx.Request.Header.VisitAll(func(k, _ []byte) {
		keys = append(keys, string(k))
	})
	return keys
}

func ExtractFromHertz(ctx context.Context, c *app.RequestContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	carrier := hertzHeaderCarrier{ctx: c}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func InjectToHertz(ctx context.Context, c *app.RequestContext) {
	carrier := hertzHeaderCarrier{ctx: c}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

var _ propagation.TextMapCarrier = (*hertzHeaderCarrier)(nil)
