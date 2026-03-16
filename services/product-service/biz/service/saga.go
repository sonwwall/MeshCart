package service

import (
	"context"
	"encoding/json"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"
	dalmodel "meshcart/services/product-service/dal/model"

	"go.uber.org/zap"
)

const (
	productTxActionCreateProductSaga           = "create_product_saga"
	productTxActionCompensateCreateProductSaga = "compensate_create_product_saga"
	txStatusPrepared                           = "prepared"
	txStatusSucceeded                          = "succeeded"
	txStatusCompensated                        = "compensated"
)

type productSagaPayload struct {
	ProductID    int64   `json:"product_id"`
	SKUIDs       []int64 `json:"sku_ids"`
	TargetStatus int32   `json:"target_status"`
}

func (s *ProductService) CreateProductSaga(ctx context.Context, req *productpb.CreateProductSagaRequest) (int64, []*productpb.ProductSku, *common.BizError) {
	if req == nil || strings.TrimSpace(req.GetGlobalTxId()) == "" || strings.TrimSpace(req.GetBranchId()) == "" {
		return 0, nil, common.ErrInvalidParam
	}
	logx.L(ctx).Info("product create saga start",
		zap.String("global_tx_id", req.GetGlobalTxId()),
		zap.String("branch_id", req.GetBranchId()),
		zap.Int64("creator_id", req.GetCreatorId()),
		zap.String("title", req.GetTitle()),
		zap.Int("sku_count", len(req.GetSkus())),
		zap.Int32("target_status", req.GetTargetStatus()),
	)

	compensated, err := s.repo.GetTxBranch(ctx, repository.TxBranchFilter{
		GlobalTxID: req.GetGlobalTxId(),
		BranchID:   req.GetBranchId(),
		Action:     productTxActionCompensateCreateProductSaga,
	})
	if err != nil {
		return 0, nil, common.ErrInternalError
	}
	if compensated != nil && compensated.Status == txStatusCompensated {
		return 0, nil, common.ErrInternalError
	}

	existing, err := s.repo.GetTxBranch(ctx, repository.TxBranchFilter{
		GlobalTxID: req.GetGlobalTxId(),
		BranchID:   req.GetBranchId(),
		Action:     productTxActionCreateProductSaga,
	})
	if err != nil {
		return 0, nil, common.ErrInternalError
	}
	if existing != nil && existing.Status == txStatusSucceeded {
		product, repoErr := s.repo.GetByID(ctx, existing.BizID)
		if repoErr != nil {
			return 0, nil, mapRepositoryError(repoErr)
		}
		return product.ID, toProductSkusForCreate(sliceProductSKUPtrs(product.Skus)), nil
	}

	productModel, skuModels, bizErr := s.buildModelsForWrite(
		0,
		req.Title,
		req.SubTitle,
		req.CategoryId,
		req.Brand,
		req.Description,
		initialSagaProductStatus(req.GetTargetStatus()),
		req.Skus,
		req.CreatorId,
		req.CreatorId,
	)
	if bizErr != nil {
		return 0, nil, bizErr
	}

	payload, marshalErr := json.Marshal(productSagaPayload{
		ProductID:    productModel.ID,
		SKUIDs:       skuIDsOf(skuModels),
		TargetStatus: req.GetTargetStatus(),
	})
	if marshalErr != nil {
		return 0, nil, common.ErrInternalError
	}

	branch := &dalmodel.ProductTxBranch{
		GlobalTxID:      req.GetGlobalTxId(),
		BranchID:        req.GetBranchId(),
		Action:          productTxActionCreateProductSaga,
		Status:          txStatusSucceeded,
		PayloadSnapshot: string(payload),
	}
	if err := s.repo.CreateWithTxBranch(ctx, branch, productModel, skuModels); err != nil {
		return 0, nil, mapRepositoryError(err)
	}
	logx.L(ctx).Info("product create saga succeeded",
		zap.String("global_tx_id", req.GetGlobalTxId()),
		zap.String("branch_id", req.GetBranchId()),
		zap.Int64("product_id", productModel.ID),
		zap.Int64s("sku_ids", skuIDsOf(skuModels)),
	)
	return productModel.ID, toProductSkusForCreate(skuModels), nil
}

func (s *ProductService) CompensateCreateProductSaga(ctx context.Context, req *productpb.CompensateCreateProductSagaRequest) *common.BizError {
	if req == nil || strings.TrimSpace(req.GetGlobalTxId()) == "" || strings.TrimSpace(req.GetBranchId()) == "" {
		return common.ErrInvalidParam
	}
	logx.L(ctx).Info("product compensate create saga start",
		zap.String("global_tx_id", req.GetGlobalTxId()),
		zap.String("branch_id", req.GetBranchId()),
		zap.Int64("product_id", req.GetProductId()),
		zap.Int64("operator_id", req.GetOperatorId()),
	)

	existingComp, err := s.repo.GetTxBranch(ctx, repository.TxBranchFilter{
		GlobalTxID: req.GetGlobalTxId(),
		BranchID:   req.GetBranchId(),
		Action:     productTxActionCompensateCreateProductSaga,
	})
	if err != nil {
		return common.ErrInternalError
	}
	if existingComp != nil && existingComp.Status == txStatusCompensated {
		logx.L(ctx).Info("product compensate create saga skipped because already compensated",
			zap.String("global_tx_id", req.GetGlobalTxId()),
			zap.String("branch_id", req.GetBranchId()),
			zap.Int64("product_id", req.GetProductId()),
		)
		return nil
	}

	forward, err := s.repo.GetTxBranch(ctx, repository.TxBranchFilter{
		GlobalTxID: req.GetGlobalTxId(),
		BranchID:   req.GetBranchId(),
		Action:     productTxActionCreateProductSaga,
	})
	if err != nil {
		return common.ErrInternalError
	}

	productID := req.GetProductId()
	if forward == nil {
		branch := &dalmodel.ProductTxBranch{
			GlobalTxID:      req.GetGlobalTxId(),
			BranchID:        req.GetBranchId(),
			Action:          productTxActionCompensateCreateProductSaga,
			Status:          txStatusCompensated,
			BizID:           productID,
			PayloadSnapshot: "{}",
		}
		if err := s.repo.CompensateCreateWithTxBranch(ctx, branch, "", "", 0); err != nil {
			return common.ErrInternalError
		}
		logx.L(ctx).Info("product compensate create saga empty compensation succeeded",
			zap.String("global_tx_id", req.GetGlobalTxId()),
			zap.String("branch_id", req.GetBranchId()),
			zap.Int64("product_id", productID),
		)
		return nil
	}

	if productID <= 0 {
		productID = forward.BizID
	}
	branch := &dalmodel.ProductTxBranch{
		GlobalTxID:      req.GetGlobalTxId(),
		BranchID:        req.GetBranchId(),
		Action:          productTxActionCompensateCreateProductSaga,
		Status:          txStatusCompensated,
		BizID:           productID,
		PayloadSnapshot: forward.PayloadSnapshot,
	}
	if err := s.repo.CompensateCreateWithTxBranch(ctx, branch, req.GetGlobalTxId(), req.GetBranchId(), productID); err != nil {
		return common.ErrInternalError
	}
	logx.L(ctx).Info("product compensate create saga succeeded",
		zap.String("global_tx_id", req.GetGlobalTxId()),
		zap.String("branch_id", req.GetBranchId()),
		zap.Int64("product_id", productID),
	)
	return nil
}

func skuIDsOf(skus []*dalmodel.ProductSKU) []int64 {
	result := make([]int64, 0, len(skus))
	for _, sku := range skus {
		if sku == nil {
			continue
		}
		result = append(result, sku.ID)
	}
	return result
}

func initialSagaProductStatus(targetStatus int32) int32 {
	if targetStatus == ProductStatusOnline {
		return ProductStatusOffline
	}
	return targetStatus
}

func sliceProductSKUPtrs(items []dalmodel.ProductSKU) []*dalmodel.ProductSKU {
	result := make([]*dalmodel.ProductSKU, 0, len(items))
	for idx := range items {
		result = append(result, &items[idx])
	}
	return result
}
