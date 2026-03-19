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

type AdjustLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdjustLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdjustLogic {
	return &AdjustLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *AdjustLogic) Adjust(skuID int64, req *types.AdjustInventoryStockRequest, identity *middleware.AuthIdentity) (*types.InventoryStockData, *common.BizError) {
	if skuID <= 0 || req == nil || req.TotalStock < 0 {
		return nil, common.ErrInvalidParam
	}
	if bizErr := requireInventoryWrite(identity, l.svcCtx.AccessControl); bizErr != nil {
		return nil, bizErr
	}
	if bizErr := ensureInventoryOwnership(l.ctx, l.svcCtx, []int64{skuID}, identity, true); bizErr != nil {
		return nil, bizErr
	}

	resp, err := l.svcCtx.InventoryClient.AdjustStock(l.ctx, &inventorypb.AdjustStockRequest{
		SkuId:      skuID,
		TotalStock: req.TotalStock,
		Reason:     &req.Reason,
	})
	if err != nil {
		logx.L(l.ctx).Error("inventory rpc adjust stock failed", zap.Error(err), zap.Int64("sku_id", skuID))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		logx.L(l.ctx).Warn("inventory rpc adjust stock returned business error",
			zap.Int64("sku_id", skuID),
			zap.Int64("total_stock", req.TotalStock),
			zap.Int32("code", resp.Code),
			zap.String("message", resp.Message),
		)
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Stock == nil {
		logx.L(l.ctx).Error("inventory rpc adjust stock returned nil stock", zap.Int64("sku_id", skuID))
		return nil, common.ErrInternalError
	}
	return toStockData(resp.Stock), nil
}
