package service

import (
	"context"
	"encoding/json"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	dalmodel "meshcart/services/inventory-service/dal/model"

	"go.uber.org/zap"
)

const (
	inventoryTxActionInitSkuStocksSaga           = "init_sku_stocks_saga"
	inventoryTxActionCompensateInitSkuStocksSaga = "compensate_init_sku_stocks_saga"
	inventoryTxStatusSucceeded                   = "succeeded"
	inventoryTxStatusCompensated                 = "compensated"
)

type inventorySagaPayload struct {
	SKUIDs []int64 `json:"sku_ids"`
}

func (s *InventoryService) InitSkuStocksSaga(ctx context.Context, req *inventorypb.InitSkuStocksSagaRequest) ([]*inventorypb.SkuStock, *common.BizError) {
	if req == nil || strings.TrimSpace(req.GetGlobalTxId()) == "" || strings.TrimSpace(req.GetBranchId()) == "" || len(req.GetStocks()) == 0 {
		return nil, common.ErrInvalidParam
	}
	logx.L(ctx).Info("inventory init saga start",
		zap.String("global_tx_id", req.GetGlobalTxId()),
		zap.String("branch_id", req.GetBranchId()),
		zap.Int("stock_count", len(req.GetStocks())),
	)

	compensated, err := s.repo.GetTxBranch(ctx, req.GetGlobalTxId(), req.GetBranchId(), inventoryTxActionCompensateInitSkuStocksSaga)
	if err != nil {
		return nil, common.ErrInternalError
	}
	if compensated != nil && compensated.Status == inventoryTxStatusCompensated {
		return nil, common.ErrInternalError
	}

	existing, err := s.repo.GetTxBranch(ctx, req.GetGlobalTxId(), req.GetBranchId(), inventoryTxActionInitSkuStocksSaga)
	if err != nil {
		return nil, common.ErrInternalError
	}
	if existing != nil && existing.Status == inventoryTxStatusSucceeded {
		var payload inventorySagaPayload
		if existing.PayloadSnapshot != "" {
			_ = json.Unmarshal([]byte(existing.PayloadSnapshot), &payload)
		}
		stocks, repoErr := s.repo.ListBySKUIDs(ctx, payload.SKUIDs)
		if repoErr != nil {
			return nil, mapRepositoryError(repoErr)
		}
		return toRPCSkuStocks(stocks), nil
	}

	models := make([]*dalmodel.InventoryStock, 0, len(req.GetStocks()))
	skuIDs := make([]int64, 0, len(req.GetStocks()))
	for _, item := range req.GetStocks() {
		if item == nil || item.GetSkuId() <= 0 || item.GetTotalStock() < 0 {
			return nil, common.ErrInvalidParam
		}
		skuIDs = append(skuIDs, item.GetSkuId())
		models = append(models, &dalmodel.InventoryStock{
			ID:             item.GetSkuId(),
			SKUID:          item.GetSkuId(),
			TotalStock:     item.GetTotalStock(),
			ReservedStock:  0,
			AvailableStock: item.GetTotalStock(),
			Status:         StockStatusActive,
			Version:        1,
		})
	}

	payload, marshalErr := json.Marshal(inventorySagaPayload{SKUIDs: skuIDs})
	if marshalErr != nil {
		return nil, common.ErrInternalError
	}
	branch := &dalmodel.InventoryTxBranch{
		GlobalTxID:      req.GetGlobalTxId(),
		BranchID:        req.GetBranchId(),
		Action:          inventoryTxActionInitSkuStocksSaga,
		Status:          inventoryTxStatusSucceeded,
		PayloadSnapshot: string(payload),
	}
	stocks, repoErr := s.repo.CreateBatchWithTxBranch(ctx, branch, models)
	if repoErr != nil {
		return nil, mapRepositoryError(repoErr)
	}
	logx.L(ctx).Info("inventory init saga succeeded",
		zap.String("global_tx_id", req.GetGlobalTxId()),
		zap.String("branch_id", req.GetBranchId()),
		zap.Int64s("sku_ids", skuIDs),
	)
	return toRPCSkuStocks(stocks), nil
}

func (s *InventoryService) CompensateInitSkuStocksSaga(ctx context.Context, req *inventorypb.CompensateInitSkuStocksSagaRequest) *common.BizError {
	if req == nil || strings.TrimSpace(req.GetGlobalTxId()) == "" || strings.TrimSpace(req.GetBranchId()) == "" {
		return common.ErrInvalidParam
	}
	logx.L(ctx).Info("inventory compensate init saga start",
		zap.String("global_tx_id", req.GetGlobalTxId()),
		zap.String("branch_id", req.GetBranchId()),
		zap.Int64s("sku_ids", req.GetSkuIds()),
	)

	existingComp, err := s.repo.GetTxBranch(ctx, req.GetGlobalTxId(), req.GetBranchId(), inventoryTxActionCompensateInitSkuStocksSaga)
	if err != nil {
		return common.ErrInternalError
	}
	if existingComp != nil && existingComp.Status == inventoryTxStatusCompensated {
		logx.L(ctx).Info("inventory compensate init saga skipped because already compensated",
			zap.String("global_tx_id", req.GetGlobalTxId()),
			zap.String("branch_id", req.GetBranchId()),
			zap.Int64s("sku_ids", req.GetSkuIds()),
		)
		return nil
	}

	forward, err := s.repo.GetTxBranch(ctx, req.GetGlobalTxId(), req.GetBranchId(), inventoryTxActionInitSkuStocksSaga)
	if err != nil {
		return common.ErrInternalError
	}

	skuIDs := req.GetSkuIds()
	payload := inventorySagaPayload{SKUIDs: skuIDs}
	if forward != nil && forward.PayloadSnapshot != "" {
		_ = json.Unmarshal([]byte(forward.PayloadSnapshot), &payload)
	}
	rawPayload, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return common.ErrInternalError
	}

	branch := &dalmodel.InventoryTxBranch{
		GlobalTxID:      req.GetGlobalTxId(),
		BranchID:        req.GetBranchId(),
		Action:          inventoryTxActionCompensateInitSkuStocksSaga,
		Status:          inventoryTxStatusCompensated,
		PayloadSnapshot: string(rawPayload),
	}
	if err := s.repo.CompensateInitWithTxBranch(ctx, branch, payload.SKUIDs); err != nil {
		return common.ErrInternalError
	}
	logx.L(ctx).Info("inventory compensate init saga succeeded",
		zap.String("global_tx_id", req.GetGlobalTxId()),
		zap.String("branch_id", req.GetBranchId()),
		zap.Int64s("sku_ids", payload.SKUIDs),
	)
	return nil
}
