package inventory

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	inventorylogic "meshcart/gateway/internal/logic/inventory"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func GetSkuStock(svcCtx *svc.ServiceContext) app.HandlerFunc {
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
		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logic := inventorylogic.NewGetLogic(ctx, svcCtx)
		data, bizErr := logic.Get(skuID, identity)
		if bizErr != nil {
			logx.L(ctx).Warn("get sku stock failed", zap.Int64("sku_id", skuID), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}
		c.JSON(200, common.Success(data, traceID))
	}
}
