package cart

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	cartlogic "meshcart/gateway/internal/logic/cart"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func UpdateCartItem(svcCtx *svc.ServiceContext) app.HandlerFunc {
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

		var req types.UpdateCartItemRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("update cart item request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}
		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logic := cartlogic.NewUpdateLogic(ctx, svcCtx)
		item, bizErr := logic.Update(identity.UserID, itemID, &req)
		if bizErr != nil {
			logx.L(ctx).Warn("update cart item failed", zap.Int64("user_id", identity.UserID), zap.Int64("item_id", itemID), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(item, traceID))
	}
}
