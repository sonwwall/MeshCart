package user

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func Me(_ *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			logx.L(ctx).Warn("jwt identity missing on protected endpoint")
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logx.L(ctx).Info("jwt identity resolved", zap.String("username", identity.Username), zap.Int64("user_id", identity.UserID))
		c.JSON(200, common.Success(identity, traceID))
	}
}
