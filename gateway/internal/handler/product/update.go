package product

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	productlogic "meshcart/gateway/internal/logic/product"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

func UpdateProduct(svcCtx *svc.ServiceContext) app.HandlerFunc {
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

		var req types.UpdateProductRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("update product request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}

		logic := productlogic.NewUpdateLogic(ctx, svcCtx)
		bizErr = logic.Update(productID, &req)
		if bizErr != nil {
			logx.L(ctx).Warn("update product failed", zap.Int64("product_id", productID), zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(nil, traceID))
	}
}
