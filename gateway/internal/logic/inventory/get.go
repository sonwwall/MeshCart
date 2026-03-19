package inventory

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"

	"go.uber.org/zap"
)

type GetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogic {
	return &GetLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *GetLogic) Get(skuID int64, identity *middleware.AuthIdentity) (*types.InventoryStockData, *common.BizError) {
	if skuID <= 0 {
		return nil, common.ErrInvalidParam
	}
	if bizErr := requireInventoryRead(identity, l.svcCtx.AccessControl); bizErr != nil {
		return nil, bizErr
	}
	if bizErr := ensureInventoryOwnership(l.ctx, l.svcCtx, []int64{skuID}, identity, false); bizErr != nil {
		return nil, bizErr
	}

	resp, err := l.svcCtx.InventoryClient.GetSkuStock(l.ctx, &inventorypb.GetSkuStockRequest{SkuId: skuID})
	if err != nil {
		logx.L(l.ctx).Error("inventory rpc get sku stock failed", zap.Error(err), zap.Int64("sku_id", skuID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(l.ctx).Warn("inventory rpc get sku stock returned business error",
			zap.Int64("sku_id", skuID),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Stock == nil {
		logx.L(l.ctx).Error("inventory rpc get sku stock returned nil stock", zap.Int64("sku_id", skuID))
		return nil, common.ErrInternalError
	}
	return toStockData(resp.Stock), nil
}
