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

type BatchGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGetLogic {
	return &BatchGetLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *BatchGetLogic) BatchGet(req *types.InventoryBatchGetRequest, identity *middleware.AuthIdentity) (*types.InventoryBatchData, *common.BizError) {
	if req == nil || len(req.SKUIDs) == 0 {
		return nil, common.ErrInvalidParam
	}
	if bizErr := requireInventoryRead(identity, l.svcCtx.AccessControl); bizErr != nil {
		return nil, bizErr
	}
	for _, skuID := range req.SKUIDs {
		if skuID <= 0 {
			return nil, common.ErrInvalidParam
		}
	}

	resp, err := l.svcCtx.InventoryClient.BatchGetSkuStock(l.ctx, &inventorypb.BatchGetSkuStockRequest{SkuIds: req.SKUIDs})
	if err != nil {
		logx.L(l.ctx).Error("inventory rpc batch get sku stock failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	return &types.InventoryBatchData{Stocks: toStocksData(resp.Stocks)}, nil
}
