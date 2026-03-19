package payment

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	paymentlogic "meshcart/gateway/internal/logic/payment"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func ClosePayment(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		paymentID, bizErr := parsePaymentID(c)
		if bizErr != nil {
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}
		var req types.ClosePaymentRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("close payment request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}
		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logic := paymentlogic.NewCloseLogic(ctx, svcCtx)
		data, bizErr := logic.Close(identity.UserID, paymentID, &req)
		if bizErr != nil {
			logx.L(ctx).Warn("close payment failed", zap.Int64("user_id", identity.UserID), zap.Int64("payment_id", paymentID), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(data, traceID))
	}
}
