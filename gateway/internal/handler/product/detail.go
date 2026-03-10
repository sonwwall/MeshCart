package product

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	productlogic "meshcart/gateway/internal/logic/product"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func GetProductDetail(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		productID, bizErr := parseProductID(c)
		if bizErr != nil {
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		logic := productlogic.NewDetailLogic(ctx, svcCtx)
		data, bizErr := logic.Get(productID)
		if bizErr != nil {
			logx.L(ctx).Warn("get product detail failed", zap.Int64("product_id", productID), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(data, traceID))
	}
}
