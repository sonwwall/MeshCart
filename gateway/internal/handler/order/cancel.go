package order

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	orderlogic "meshcart/gateway/internal/logic/order"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func CancelOrder(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		orderID, bizErr := parseOrderID(c)
		if bizErr != nil {
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}
		var req types.CancelOrderRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("cancel order request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}
		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logic := orderlogic.NewCancelLogic(ctx, svcCtx)
		data, bizErr := logic.Cancel(identity.UserID, orderID, &req)
		if bizErr != nil {
			logx.L(ctx).Warn("cancel order failed", zap.Int64("user_id", identity.UserID), zap.Int64("order_id", orderID), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(data, traceID))
	}
}
