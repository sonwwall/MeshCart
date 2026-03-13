package cart

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	cartlogic "meshcart/gateway/internal/logic/cart"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func RemoveCartItem(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		itemID, bizErr := parseItemID(c)
		if bizErr != nil {
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}
		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logic := cartlogic.NewRemoveLogic(ctx, svcCtx)
		bizErr = logic.Remove(identity.UserID, itemID)
		if bizErr != nil {
			logx.L(ctx).Warn("remove cart item failed", zap.Int64("user_id", identity.UserID), zap.Int64("item_id", itemID), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(nil, traceID))
	}
}
