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
	"go.uber.org/zap"
)

func Register(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		var req types.UserRegisterRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("user register request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}

		registerLogic := userlogic.NewRegisterLogic(ctx, svcCtx)
		bizErr := registerLogic.Register(&req)
		if bizErr != nil {
			logx.L(ctx).Warn("user register failed",
				zap.String("username", req.Username),
				zap.Int32("code", bizErr.Code),
				zap.String("message", bizErr.Msg),
			)
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		logx.L(ctx).Info("user register success", zap.String("username", req.Username))
		c.JSON(200, common.Success(nil, traceID))
	}
}
