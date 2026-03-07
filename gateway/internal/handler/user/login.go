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

func Login(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			// hertz-contrib tracing middleware 已经为入站请求创建 span。
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		var req types.UserLoginRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("user login request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}

		loginLogic := userlogic.NewLoginLogic(ctx, svcCtx)
		data, bizErr := loginLogic.Login(&req)
		if bizErr != nil {
			logx.L(ctx).Warn("user login failed",
				zap.String("username", req.Username),
				zap.Int32("code", bizErr.Code),
				zap.String("message", bizErr.Msg),
			)
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		logx.L(ctx).Info("user login success", zap.String("username", req.Username))
		c.JSON(200, common.Success(data, traceID))
	}
}
