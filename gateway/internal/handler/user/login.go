package user

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	userlogic "meshcart/gateway/internal/logic/user"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func Login(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		ctx = tracex.ExtractFromHertz(ctx, c)
		ctx, span := tracex.StartSpan(ctx, "meshcart.gateway", "gateway.user.login", oteltrace.WithSpanKind(oteltrace.SpanKindServer))
		defer span.End()

		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		var req types.UserLoginRequest
		if err := c.BindAndValidate(&req); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "bind request failed")
			logx.L(ctx).Warn("user login request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}
		span.SetAttributes(attribute.String("user.username", req.Username))

		loginLogic := userlogic.NewLoginLogic(ctx, svcCtx)
		data, bizErr := loginLogic.Login(&req)
		if bizErr != nil {
			span.SetStatus(codes.Error, bizErr.Msg)
			logx.L(ctx).Warn("user login failed",
				zap.String("username", req.Username),
				zap.Int32("code", bizErr.Code),
				zap.String("message", bizErr.Msg),
			)
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		span.SetStatus(codes.Ok, "ok")
		logx.L(ctx).Info("user login success", zap.String("username", req.Username))
		c.JSON(200, common.Success(data, traceID))
	}
}
