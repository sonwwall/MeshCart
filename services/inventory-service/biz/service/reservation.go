package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	"meshcart/services/inventory-service/biz/repository"

	"go.uber.org/zap"
)

func (s *InventoryService) ReserveSkuStocks(ctx context.Context, req *inventorypb.ReserveSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	items, bizErr := parseReservationItems(req.GetBizType(), req.GetBizId(), req.GetItems())
	if bizErr != nil {
		return nil, bizErr
	}

	stocks, err := s.repo.Reserve(ctx, strings.TrimSpace(req.GetBizType()), strings.TrimSpace(req.GetBizId()), items)
	if err != nil {
		logx.L(ctx).Error("inventory reserve repository failed",
			zap.Error(err),
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(items)),
		)
		return nil, mapRepositoryError(err)
	}
	return toRPCSkuStocks(stocks), nil
}

func (s *InventoryService) ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	items, bizErr := parseReservationItems(req.GetBizType(), req.GetBizId(), req.GetItems())
	if bizErr != nil {
		return nil, bizErr
	}

	stocks, err := s.repo.Release(ctx, strings.TrimSpace(req.GetBizType()), strings.TrimSpace(req.GetBizId()), items)
	if err != nil {
		logx.L(ctx).Error("inventory release repository failed",
			zap.Error(err),
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(items)),
		)
		return nil, mapRepositoryError(err)
	}
	return toRPCSkuStocks(stocks), nil
}

func (s *InventoryService) ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	items, bizErr := parseReservationItems(req.GetBizType(), req.GetBizId(), req.GetItems())
	if bizErr != nil {
		return nil, bizErr
	}

	stocks, err := s.repo.ConfirmDeduct(ctx, strings.TrimSpace(req.GetBizType()), strings.TrimSpace(req.GetBizId()), items)
	if err != nil {
		logx.L(ctx).Error("inventory confirm deduct repository failed",
			zap.Error(err),
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(items)),
		)
		return nil, mapRepositoryError(err)
	}
	return toRPCSkuStocks(stocks), nil
}

func parseReservationItems(bizType, bizID string, rawItems []*inventorypb.StockReservationItem) ([]repository.ReservationItem, *common.BizError) {
	if strings.TrimSpace(bizType) == "" || strings.TrimSpace(bizID) == "" || len(rawItems) == 0 {
		return nil, common.ErrInvalidParam
	}

	seen := make(map[int64]struct{}, len(rawItems))
	items := make([]repository.ReservationItem, 0, len(rawItems))
	for _, item := range rawItems {
		if item == nil || item.GetSkuId() <= 0 || item.GetQuantity() <= 0 {
			return nil, common.ErrInvalidParam
		}
		if _, ok := seen[item.GetSkuId()]; ok {
			return nil, common.ErrInvalidParam
		}
		seen[item.GetSkuId()] = struct{}{}
		items = append(items, repository.ReservationItem{
			SKUID:    item.GetSkuId(),
			Quantity: item.GetQuantity(),
		})
	}
	return items, nil
}
