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

func ListOwnedProducts(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := middleware.TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}
		ctx = logx.WithTraceID(ctx, traceID)

		var req types.ListProductsRequest
		if err := c.BindAndValidate(&req); err != nil {
			logx.L(ctx).Warn("admin product list request bind failed", zap.Error(err))
			c.JSON(200, common.Fail(common.ErrInvalidParam, traceID))
			return
		}
		identity, ok := middleware.IdentityFromRequest(ctx, c)
		if !ok {
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
			return
		}

		logic := productlogic.NewListLogic(ctx, svcCtx)
		data, bizErr := logic.ListOwned(&req, identity)
		if bizErr != nil {
			logx.L(ctx).Warn("list owned products failed", zap.Int32("code", bizErr.Code), zap.String("message", bizErr.Msg))
			c.JSON(200, common.Fail(bizErr, traceID))
			return
		}

		c.JSON(200, common.Success(data, traceID))
	}
}
