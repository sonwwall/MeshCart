package service

import (
	"context"
	"strings"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	metricsx "meshcart/app/metrics"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	"meshcart/services/inventory-service/biz/errno"
	"meshcart/services/inventory-service/biz/repository"

	"go.uber.org/zap"
)

func (s *InventoryService) ReserveSkuStocks(ctx context.Context, req *inventorypb.ReserveSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	start := time.Now()
	items, bizErr := parseReservationItems(req.GetBizType(), req.GetBizId(), req.GetItems())
	if bizErr != nil {
		metricsx.ObserveInventoryReservation("inventory-service", "reserve", "biz_failed", reserveReasonOf(bizErr), time.Since(start))
		logx.L(ctx).Warn("inventory reserve rejected by invalid request",
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(req.GetItems())),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return nil, bizErr
	}
	skuIDs := reservationSKUIds(items)
	releaseGuard, err := s.reserveGuard.Acquire(ctx, skuIDs)
	if err != nil {
		metricsx.ObserveInventoryReservation("inventory-service", "reserve", "timeout", "hotspot_guard_timeout", time.Since(start))
		logx.L(ctx).Warn("inventory reserve blocked by hotspot guard",
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int64s("sku_ids", skuIDs),
			zap.Error(err),
		)
		return nil, errno.ErrReservationTimeout
	}
	defer releaseGuard()
	logx.L(ctx).Info("inventory reserve start",
		zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
		zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
		zap.Int("item_count", len(items)),
	)

	stocks, err := s.repo.Reserve(ctx, strings.TrimSpace(req.GetBizType()), strings.TrimSpace(req.GetBizId()), items)
	if err != nil {
		mapped := mapRepositoryError(err)
		outcome := "biz_failed"
		if mapped == errno.ErrReservationTimeout {
			outcome = "timeout"
		}
		metricsx.ObserveInventoryReservation("inventory-service", "reserve", outcome, reserveReasonOf(mapped), time.Since(start))
		metricsx.ObserveBizError("inventory-service", "inventory", "reserve", outcome, mapped.Code)
		logx.L(ctx).Error("inventory reserve repository failed",
			zap.Error(err),
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(items)),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return nil, mapped
	}
	metricsx.ObserveInventoryReservation("inventory-service", "reserve", "success", "ok", time.Since(start))
	logx.L(ctx).Info("inventory reserve completed",
		zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
		zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
		zap.Int("stock_count", len(stocks)),
	)
	return toRPCSkuStocks(stocks), nil
}

func (s *InventoryService) ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	items, bizErr := parseReservationItems(req.GetBizType(), req.GetBizId(), req.GetItems())
	if bizErr != nil {
		logx.L(ctx).Warn("inventory release rejected by invalid request",
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(req.GetItems())),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return nil, bizErr
	}
	logx.L(ctx).Info("inventory release start",
		zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
		zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
		zap.Int("item_count", len(items)),
	)

	stocks, err := s.repo.Release(ctx, strings.TrimSpace(req.GetBizType()), strings.TrimSpace(req.GetBizId()), items)
	if err != nil {
		mapped := mapRepositoryError(err)
		logx.L(ctx).Error("inventory release repository failed",
			zap.Error(err),
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(items)),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return nil, mapped
	}
	logx.L(ctx).Info("inventory release completed",
		zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
		zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
		zap.Int("stock_count", len(stocks)),
	)
	return toRPCSkuStocks(stocks), nil
}

func (s *InventoryService) ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	items, bizErr := parseReservationItems(req.GetBizType(), req.GetBizId(), req.GetItems())
	if bizErr != nil {
		logx.L(ctx).Warn("inventory confirm deduct rejected by invalid request",
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(req.GetItems())),
			zap.Int32("code", bizErr.Code),
			zap.String("message", bizErr.Msg),
		)
		return nil, bizErr
	}
	logx.L(ctx).Info("inventory confirm deduct start",
		zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
		zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
		zap.Int("item_count", len(items)),
	)

	stocks, err := s.repo.ConfirmDeduct(ctx, strings.TrimSpace(req.GetBizType()), strings.TrimSpace(req.GetBizId()), items)
	if err != nil {
		mapped := mapRepositoryError(err)
		logx.L(ctx).Error("inventory confirm deduct repository failed",
			zap.Error(err),
			zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
			zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
			zap.Int("item_count", len(items)),
			zap.Int32("mapped_code", mapped.Code),
			zap.String("mapped_message", mapped.Msg),
		)
		return nil, mapped
	}
	logx.L(ctx).Info("inventory confirm deduct completed",
		zap.String("biz_type", strings.TrimSpace(req.GetBizType())),
		zap.String("biz_id", strings.TrimSpace(req.GetBizId())),
		zap.Int("stock_count", len(stocks)),
	)
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

func reservationSKUIds(items []repository.ReservationItem) []int64 {
	result := make([]int64, 0, len(items))
	for _, item := range items {
		result = append(result, item.SKUID)
	}
	return result
}

func reserveReasonOf(bizErr *common.BizError) string {
	if bizErr == nil {
		return "ok"
	}
	switch bizErr {
	case common.ErrInvalidParam:
		return "invalid_param"
	case errno.ErrInsufficientStock:
		return "insufficient_stock"
	case errno.ErrStockFrozen:
		return "stock_frozen"
	case errno.ErrReservationConflict:
		return "reservation_conflict"
	case errno.ErrReservationNotFound:
		return "reservation_not_found"
	case errno.ErrReservationTimeout:
		return "reservation_timeout"
	default:
		return "internal_error"
	}
}
