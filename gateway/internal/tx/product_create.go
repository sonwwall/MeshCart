package tx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/dtm-labs/dtm/client/dtmcli"
	"github.com/dtm-labs/dtm/client/workflow"
	"github.com/google/uuid"

	"go.uber.org/zap"
	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/types"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	productrpc "meshcart/gateway/rpc/product"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"
)

const (
	ProductCreateWorkflowName = "gateway.product.create"
	productStatusOnline       = 2
	productStatusOffline      = 1
)

type ProductCreateCoordinator interface {
	CreateProduct(ctx context.Context, req *types.CreateProductRequest, operatorID int64) (*types.CreateProductData, *common.BizError)
}

type DTMProductCreateCoordinator struct {
	dtmServer       string
	productClient   productWorkflowClient
	inventoryClient inventoryWorkflowClient
}

type createProductWorkflowPayload struct {
	Request    *types.CreateProductRequest `json:"request"`
	OperatorID int64                       `json:"operator_id"`
}

type createdProductWorkflowResult struct {
	ProductID int64                   `json:"product_id"`
	SKUs      []*productpb.ProductSku `json:"skus"`
}

var (
	registerWorkflowOnce sync.Once
	workflowExecutorMu   sync.RWMutex
	workflowExecutor     func(wf *workflow.Workflow, data []byte) ([]byte, error)
)

type productWorkflowClient interface {
	CreateProductSaga(ctx context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error)
	CompensateCreateProductSaga(ctx context.Context, req *productpb.CompensateCreateProductSagaRequest) (*productrpc.ChangeProductStatusResponse, error)
	ChangeProductStatus(ctx context.Context, req *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error)
}

type inventoryWorkflowClient interface {
	InitSkuStocksSaga(ctx context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	CompensateInitSkuStocksSaga(ctx context.Context, req *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventoryrpc.CompensateInitSkuStocksResponse, error)
}

func NewProductCreateCoordinator(cfg config.DTMConfig, productClient productWorkflowClient, inventoryClient inventoryWorkflowClient) ProductCreateCoordinator {
	if cfg.Server == "" || cfg.WorkflowCallbackURL == "" {
		return nil
	}
	workflow.InitHTTP(cfg.Server, cfg.WorkflowCallbackURL)

	coordinator := &DTMProductCreateCoordinator{
		dtmServer:       cfg.Server,
		productClient:   productClient,
		inventoryClient: inventoryClient,
	}

	workflowExecutorMu.Lock()
	workflowExecutor = coordinator.runWorkflow
	workflowExecutorMu.Unlock()

	registerWorkflowOnce.Do(func() {
		_ = workflow.Register2(ProductCreateWorkflowName, func(wf *workflow.Workflow, data []byte) ([]byte, error) {
			workflowExecutorMu.RLock()
			executor := workflowExecutor
			workflowExecutorMu.RUnlock()
			if executor == nil {
				return nil, common.ErrInternalError
			}
			return executor(wf, data)
		})
	})

	return coordinator
}

func (c *DTMProductCreateCoordinator) CreateProduct(ctx context.Context, req *types.CreateProductRequest, operatorID int64) (*types.CreateProductData, *common.BizError) {
	payload, err := json.Marshal(createProductWorkflowPayload{
		Request:    req,
		OperatorID: operatorID,
	})
	if err != nil {
		return nil, common.ErrInternalError
	}

	gid := uuid.NewString()
	logx.L(ctx).Info("dtm product create workflow start",
		zap.String("gid", gid),
		zap.Int64("operator_id", operatorID),
		zap.String("title", req.Title),
		zap.Int("sku_count", len(req.SKUs)),
		zap.Int32("target_status", req.Status),
	)
	result, execErr := workflow.ExecuteCtx(ctx, ProductCreateWorkflowName, gid, payload)
	if execErr != nil {
		logx.L(ctx).Error("dtm product create workflow failed",
			zap.String("gid", gid),
			zap.Error(execErr),
		)
		if bizErr, ok := execErr.(*common.BizError); ok {
			return nil, bizErr
		}
		if errors.Is(execErr, dtmcli.ErrFailure) {
			return nil, common.ErrInternalError
		}
		return nil, common.ErrInternalError
	}

	var data types.CreateProductData
	if err := json.Unmarshal(result, &data); err != nil {
		logx.L(ctx).Error("dtm product create workflow decode result failed",
			zap.String("gid", gid),
			zap.Error(err),
		)
		return nil, common.ErrInternalError
	}
	logx.L(ctx).Info("dtm product create workflow succeeded",
		zap.String("gid", gid),
		zap.Int64("product_id", data.ProductID),
		zap.Int("sku_count", len(data.SKUs)),
	)
	return &data, nil
}

func (c *DTMProductCreateCoordinator) runWorkflow(wf *workflow.Workflow, raw []byte) ([]byte, error) {
	var payload createProductWorkflowPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Request == nil || payload.OperatorID <= 0 {
		return nil, rollbackError(common.ErrInvalidParam)
	}

	req := payload.Request
	operatorID := payload.OperatorID

	var created createdProductWorkflowResult
	inventoryInitialized := false

	createBranch := wf.NewBranch()
	createBranch.OnRollback(func(bb *dtmcli.BranchBarrier) error {
		logx.L(wf.Context).Warn("dtm workflow rollback product create start",
			zap.String("gid", bb.Gid),
			zap.String("branch_id", bb.BranchID),
			zap.Int64("product_id", created.ProductID),
		)
		resp, err := c.productClient.CompensateCreateProductSaga(wf.Context, &productpb.CompensateCreateProductSagaRequest{
			GlobalTxId: bb.Gid,
			BranchId:   bb.BranchID,
			ProductId:  created.ProductID,
			OperatorId: operatorID,
		})
		if err != nil {
			logx.L(wf.Context).Error("dtm workflow rollback product create rpc failed",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
				zap.Error(err),
			)
			return err
		}
		if resp.Code != common.CodeOK {
			logx.L(wf.Context).Error("dtm workflow rollback product create business failed",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
				zap.Int32("code", resp.Code),
				zap.String("message", resp.Message),
			)
			return common.NewBizError(resp.Code, resp.Message)
		}
		logx.L(wf.Context).Info("dtm workflow rollback product create succeeded",
			zap.String("gid", bb.Gid),
			zap.String("branch_id", bb.BranchID),
			zap.Int64("product_id", created.ProductID),
		)
		return nil
	})
	createResult, err := createBranch.Do(func(bb *dtmcli.BranchBarrier) ([]byte, error) {
		logx.L(wf.Context).Info("dtm workflow product create branch start",
			zap.String("gid", bb.Gid),
			zap.String("branch_id", bb.BranchID),
		)
		resp, err := c.productClient.CreateProductSaga(wf.Context, &productpb.CreateProductSagaRequest{
			GlobalTxId:   bb.Gid,
			BranchId:     bb.BranchID,
			Title:        req.Title,
			SubTitle:     req.SubTitle,
			CategoryId:   req.CategoryID,
			Brand:        req.Brand,
			Description:  req.Description,
			TargetStatus: req.Status,
			Skus:         buildWorkflowSKUInputs(req.SKUs),
			CreatorId:    operatorID,
		})
		if err != nil {
			logx.L(wf.Context).Error("dtm workflow product create branch rpc failed",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
				zap.Error(err),
			)
			return nil, err
		}
		if resp.Code != common.CodeOK {
			logx.L(wf.Context).Error("dtm workflow product create branch business failed",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int32("code", resp.Code),
				zap.String("message", resp.Message),
			)
			return nil, common.NewBizError(resp.Code, resp.Message)
		}
		logx.L(wf.Context).Info("dtm workflow product create branch succeeded",
			zap.String("gid", bb.Gid),
			zap.String("branch_id", bb.BranchID),
			zap.Int64("product_id", resp.ProductID),
			zap.Int("sku_count", len(resp.Skus)),
		)
		return json.Marshal(createdProductWorkflowResult{
			ProductID: resp.ProductID,
			SKUs:      resp.Skus,
		})
	})
	if err != nil {
		return nil, rollbackError(err)
	}
	if err := json.Unmarshal(createResult, &created); err != nil {
		return nil, rollbackError(common.ErrInternalError)
	}

	initItems := buildWorkflowInitStockItems(req.SKUs, created.SKUs)
	if len(initItems) != len(created.SKUs) {
		return nil, rollbackError(common.ErrInternalError)
	}

	inventoryBranch := wf.NewBranch()
	inventoryBranch.OnRollback(func(bb *dtmcli.BranchBarrier) error {
		if !inventoryInitialized {
			logx.L(wf.Context).Info("dtm workflow rollback inventory init skipped because branch never succeeded",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
			)
			return nil
		}
		skuIDs := skuIDsOf(created.SKUs)
		logx.L(wf.Context).Warn("dtm workflow rollback inventory init start",
			zap.String("gid", bb.Gid),
			zap.String("branch_id", bb.BranchID),
			zap.Int64s("sku_ids", skuIDs),
		)
		resp, err := c.inventoryClient.CompensateInitSkuStocksSaga(wf.Context, &inventorypb.CompensateInitSkuStocksSagaRequest{
			GlobalTxId: bb.Gid,
			BranchId:   bb.BranchID,
			SkuIds:     skuIDs,
		})
		if err != nil {
			logx.L(wf.Context).Error("dtm workflow rollback inventory init rpc failed",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64s("sku_ids", skuIDs),
				zap.Error(err),
			)
			return err
		}
		if resp.Code != common.CodeOK {
			logx.L(wf.Context).Error("dtm workflow rollback inventory init business failed",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64s("sku_ids", skuIDs),
				zap.Int32("code", resp.Code),
				zap.String("message", resp.Message),
			)
			return common.NewBizError(resp.Code, resp.Message)
		}
		logx.L(wf.Context).Info("dtm workflow rollback inventory init succeeded",
			zap.String("gid", bb.Gid),
			zap.String("branch_id", bb.BranchID),
			zap.Int64s("sku_ids", skuIDs),
		)
		return nil
	})
	_, err = inventoryBranch.Do(func(bb *dtmcli.BranchBarrier) ([]byte, error) {
		logx.L(wf.Context).Info("dtm workflow inventory init branch start",
			zap.String("gid", bb.Gid),
			zap.String("branch_id", bb.BranchID),
			zap.Int64("product_id", created.ProductID),
		)
		resp, err := c.inventoryClient.InitSkuStocksSaga(wf.Context, &inventorypb.InitSkuStocksSagaRequest{
			GlobalTxId: bb.Gid,
			BranchId:   bb.BranchID,
			Stocks:     initItems,
		})
		if err != nil {
			logx.L(wf.Context).Error("dtm workflow inventory init branch rpc failed",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
				zap.Error(err),
			)
			return nil, err
		}
		if resp.Code != common.CodeOK {
			logx.L(wf.Context).Error("dtm workflow inventory init branch business failed",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
				zap.Int32("code", resp.Code),
				zap.String("message", resp.Message),
			)
			return nil, common.NewBizError(resp.Code, resp.Message)
		}
		logx.L(wf.Context).Info("dtm workflow inventory init branch succeeded",
			zap.String("gid", bb.Gid),
			zap.String("branch_id", bb.BranchID),
			zap.Int64("product_id", created.ProductID),
			zap.Int("stock_count", len(resp.Stocks)),
		)
		return json.Marshal(map[string]any{"ok": true})
	})
	if err != nil {
		return nil, rollbackError(err)
	}
	inventoryInitialized = true

	if req.Status == productStatusOnline {
		publishBranch := wf.NewBranch()
		publishBranch.OnRollback(func(bb *dtmcli.BranchBarrier) error {
			logx.L(wf.Context).Warn("dtm workflow rollback publish product start",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
			)
			resp, err := c.productClient.ChangeProductStatus(wf.Context, &productpb.ChangeProductStatusRequest{
				ProductId:  created.ProductID,
				Status:     productStatusOffline,
				OperatorId: operatorID,
			})
			if err != nil {
				logx.L(wf.Context).Error("dtm workflow rollback publish product rpc failed",
					zap.String("gid", bb.Gid),
					zap.String("branch_id", bb.BranchID),
					zap.Int64("product_id", created.ProductID),
					zap.Error(err),
				)
				return err
			}
			if resp.Code != common.CodeOK {
				logx.L(wf.Context).Error("dtm workflow rollback publish product business failed",
					zap.String("gid", bb.Gid),
					zap.String("branch_id", bb.BranchID),
					zap.Int64("product_id", created.ProductID),
					zap.Int32("code", resp.Code),
					zap.String("message", resp.Message),
				)
				return common.NewBizError(resp.Code, resp.Message)
			}
			logx.L(wf.Context).Info("dtm workflow rollback publish product succeeded",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
			)
			return nil
		})
		_, err = publishBranch.Do(func(bb *dtmcli.BranchBarrier) ([]byte, error) {
			logx.L(wf.Context).Info("dtm workflow publish product branch start",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
			)
			resp, err := c.productClient.ChangeProductStatus(wf.Context, &productpb.ChangeProductStatusRequest{
				ProductId:  created.ProductID,
				Status:     productStatusOnline,
				OperatorId: operatorID,
			})
			if err != nil {
				logx.L(wf.Context).Error("dtm workflow publish product branch rpc failed",
					zap.String("gid", bb.Gid),
					zap.String("branch_id", bb.BranchID),
					zap.Int64("product_id", created.ProductID),
					zap.Error(err),
				)
				return nil, err
			}
			if resp.Code != common.CodeOK {
				logx.L(wf.Context).Error("dtm workflow publish product branch business failed",
					zap.String("gid", bb.Gid),
					zap.String("branch_id", bb.BranchID),
					zap.Int64("product_id", created.ProductID),
					zap.Int32("code", resp.Code),
					zap.String("message", resp.Message),
				)
				return nil, common.NewBizError(resp.Code, resp.Message)
			}
			logx.L(wf.Context).Info("dtm workflow publish product branch succeeded",
				zap.String("gid", bb.Gid),
				zap.String("branch_id", bb.BranchID),
				zap.Int64("product_id", created.ProductID),
			)
			return json.Marshal(map[string]any{"ok": true})
		})
		if err != nil {
			return nil, rollbackError(err)
		}
	}

	result, err := json.Marshal(types.CreateProductData{
		ProductID: created.ProductID,
		SKUs:      toCreatedProductSKUs(created.SKUs),
	})
	if err != nil {
		return nil, rollbackError(common.ErrInternalError)
	}
	return result, nil
}

func buildWorkflowSKUInputs(items []types.ProductSkuInput) []*productpb.ProductSkuInput {
	inputs := make([]*productpb.ProductSkuInput, 0, len(items))
	for _, item := range items {
		attrs := make([]*productpb.ProductSkuAttrInput, 0, len(item.Attrs))
		for _, attr := range item.Attrs {
			attrs = append(attrs, &productpb.ProductSkuAttrInput{
				AttrName:  attr.AttrName,
				AttrValue: attr.AttrValue,
				Sort:      attr.Sort,
			})
		}
		inputs = append(inputs, &productpb.ProductSkuInput{
			Id:          item.ID,
			SkuCode:     item.SKUCode,
			Title:       item.Title,
			SalePrice:   item.SalePrice,
			MarketPrice: item.MarketPrice,
			Status:      item.Status,
			CoverUrl:    item.CoverURL,
			Attrs:       attrs,
		})
	}
	return inputs
}

func buildWorkflowInitStockItems(requestSKUs []types.ProductSkuInput, createdSKUs []*productpb.ProductSku) []*inventorypb.InitSkuStockItem {
	if len(requestSKUs) == 0 || len(createdSKUs) == 0 || len(requestSKUs) != len(createdSKUs) {
		return nil
	}
	result := make([]*inventorypb.InitSkuStockItem, 0, len(createdSKUs))
	for idx, sku := range requestSKUs {
		stock := int64(0)
		if sku.InitialStock != nil {
			stock = *sku.InitialStock
		}
		result = append(result, &inventorypb.InitSkuStockItem{
			SkuId:      createdSKUs[idx].GetId(),
			TotalStock: stock,
		})
	}
	return result
}

func skuIDsOf(skus []*productpb.ProductSku) []int64 {
	result := make([]int64, 0, len(skus))
	for _, sku := range skus {
		if sku == nil {
			continue
		}
		result = append(result, sku.GetId())
	}
	return result
}

func toCreatedProductSKUs(skus []*productpb.ProductSku) []types.CreatedProductSKUData {
	result := make([]types.CreatedProductSKUData, 0, len(skus))
	for _, sku := range skus {
		result = append(result, types.CreatedProductSKUData{
			ID:      sku.GetId(),
			SKUCode: sku.GetSkuCode(),
		})
	}
	return result
}

func rollbackError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, dtmcli.ErrFailure) {
		return err
	}
	if bizErr, ok := err.(*common.BizError); ok {
		return fmt.Errorf("%s: %w", bizErr.Msg, dtmcli.ErrFailure)
	}
	return fmt.Errorf("%v: %w", err, dtmcli.ErrFailure)
}
