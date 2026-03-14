package inventory

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	inventorylogic "meshcart/gateway/internal/logic/inventory"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func AdjustStock(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		skuID, bizErr := parseSKUID(c)
		if bizErr != nil {
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		var req types.AdjustInventoryStockRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("adjust stock request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}
		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logic := inventorylogic.NewAdjustLogic(ctx, svcCtx)
		data, bizErr := logic.Adjust(skuID, &req, identity)
		if bizErr != nil {
			logx.L(ctx).Warn("adjust stock failed", zap.Int64("sku_id", skuID), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}
		c.JSON(200, common.Success(data, traceID))
	}
}
