package product

import (
	"context"
	"fmt"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	productrpc "meshcart/gateway/rpc/product"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type CreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

const (
	sagaBranchIDProductCreate = "product-create"
	sagaBranchIDInventoryInit = "inventory-init"
)

func NewCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLogic {
	return &CreateLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateLogic) Create(req *types.CreateProductRequest, identity *middleware.AuthIdentity) (*types.CreateProductData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.create", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "create"))

	if strings.TrimSpace(req.Title) == "" || len(req.SKUs) == 0 {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(common.ErrInvalidParam.Code)), attribute.String("biz.message", common.ErrInvalidParam.Msg))
		return nil, common.ErrInvalidParam
	}
	if identity == nil || !l.svcCtx.AccessControl.Enforce(roleOf(identity), "product", authz.ActionCreate, 0, identity.UserID, req.Status) {
		return nil, common.ErrForbidden
	}
	if bizErr := validateInitialStocks(req.SKUs); bizErr != nil {
		return nil, bizErr
	}
	if l.svcCtx.ProductCreateCoordinator != nil {
		data, bizErr := l.svcCtx.ProductCreateCoordinator.CreateProduct(ctx, req, identity.UserID)
		if bizErr != nil {
			span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(bizErr.Code)), attribute.String("biz.message", bizErr.Msg))
			return nil, bizErr
		}
		span.SetAttributes(attribute.Bool("biz.success", true))
		span.SetStatus(codes.Ok, "ok")
		return data, nil
	}

	txID := uuid.NewString()
	resp, err := l.svcCtx.ProductClient.CreateProductSaga(ctx, buildCreateProductSagaRPCRequest(req, identity.UserID, txID))
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc create failed")
		logx.L(ctx).Error("product rpc create failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(resp.Code)), attribute.String("biz.message", resp.Message))
		return nil, common.NewBizError(resp.Code, resp.Message)
	}

	if bizErr := l.initStocksAfterProductCreate(ctx, req, resp, identity.UserID, txID); bizErr != nil {
		return nil, bizErr
	}

	if req.Status == productStatusOnline {
		statusResp, statusErr := l.svcCtx.ProductClient.ChangeProductStatus(ctx, &productpb.ChangeProductStatusRequest{
			ProductId:  resp.ProductID,
			Status:     productStatusOnline,
			OperatorId: identity.UserID,
		})
		if statusErr != nil {
			l.compensateInventoryAndProduct(ctx, txID, resp.ProductID, resp.Skus, identity.UserID)
			return nil, logicutil.MapRPCError(statusErr)
		}
		if statusResp.Code != common.CodeOK {
			l.compensateInventoryAndProduct(ctx, txID, resp.ProductID, resp.Skus, identity.UserID)
			return nil, common.NewBizError(statusResp.Code, statusResp.Message)
		}
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return &types.CreateProductData{ProductID: resp.ProductID, SKUs: toCreatedProductSKUs(resp.Skus)}, nil
}

func buildCreateProductRPCRequest(req *types.CreateProductRequest, creatorID int64) *productpb.CreateProductRequest {
	return &productpb.CreateProductRequest{
		Title:       req.Title,
		SubTitle:    req.SubTitle,
		CategoryId:  req.CategoryID,
		Brand:       req.Brand,
		Description: req.Description,
		Status:      req.Status,
		Skus:        buildSKUInputs(req.SKUs),
		CreatorId:   creatorID,
	}
}

func buildCreateProductSagaRPCRequest(req *types.CreateProductRequest, creatorID int64, txID string) *productpb.CreateProductSagaRequest {
	return &productpb.CreateProductSagaRequest{
		GlobalTxId:   txID,
		BranchId:     sagaBranchIDProductCreate,
		Title:        req.Title,
		SubTitle:     req.SubTitle,
		CategoryId:   req.CategoryID,
		Brand:        req.Brand,
		Description:  req.Description,
		TargetStatus: req.Status,
		Skus:         buildSKUInputs(req.SKUs),
		CreatorId:    creatorID,
	}
}

func buildSKUInputs(items []types.ProductSkuInput) []*productpb.ProductSkuInput {
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

func validateInitialStocks(items []types.ProductSkuInput) *common.BizError {
	for _, item := range items {
		if item.InitialStock != nil && *item.InitialStock < 0 {
			return common.ErrInvalidParam
		}
	}
	return nil
}

func (l *CreateLogic) initStocksAfterProductCreate(ctx context.Context, req *types.CreateProductRequest, resp *productrpc.CreateProductResponse, operatorID int64, txID string) *common.BizError {
	initItems := buildInitStockItems(req.SKUs, resp.Skus)
	if len(initItems) != len(resp.Skus) {
		logx.L(ctx).Error("created sku ids do not match requested initial stocks", zap.Int64("product_id", resp.ProductID))
		l.compensateProductCreate(ctx, txID, resp.ProductID, operatorID)
		return common.ErrInternalError
	}

	initResp, err := l.svcCtx.InventoryClient.InitSkuStocksSaga(ctx, &inventorypb.InitSkuStocksSagaRequest{
		GlobalTxId: txID,
		BranchId:   sagaBranchIDInventoryInit,
		Stocks:     initItems,
	})
	if err != nil {
		logx.L(ctx).Error("inventory rpc init sku stocks failed after create product", zap.Error(err), zap.Int64("product_id", resp.ProductID))
		l.compensateProductCreate(ctx, txID, resp.ProductID, operatorID)
		return logicutil.MapRPCError(err)
	}
	if initResp.Code != common.CodeOK {
		l.compensateProductCreate(ctx, txID, resp.ProductID, operatorID)
		return common.NewBizError(initResp.Code, initResp.Message)
	}
	return nil
}

func buildInitStockItems(requestSKUs []types.ProductSkuInput, createdSKUs []*productpb.ProductSku) []*inventorypb.InitSkuStockItem {
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

func (l *CreateLogic) compensateProductCreate(ctx context.Context, txID string, productID int64, operatorID int64) {
	_, err := l.svcCtx.ProductClient.CompensateCreateProductSaga(ctx, &productpb.CompensateCreateProductSagaRequest{
		GlobalTxId: txID,
		BranchId:   sagaBranchIDProductCreate,
		ProductId:  productID,
		OperatorId: operatorID,
	})
	if err != nil {
		logx.L(ctx).Warn("compensate product create saga failed", zap.Error(err), zap.Int64("product_id", productID), zap.String("global_tx_id", txID))
	}
}

func (l *CreateLogic) compensateInventoryAndProduct(ctx context.Context, txID string, productID int64, skus []*productpb.ProductSku, operatorID int64) {
	skuIDs := make([]int64, 0, len(skus))
	for _, sku := range skus {
		if sku == nil {
			continue
		}
		skuIDs = append(skuIDs, sku.GetId())
	}
	if len(skuIDs) > 0 {
		_, err := l.svcCtx.InventoryClient.CompensateInitSkuStocksSaga(ctx, &inventorypb.CompensateInitSkuStocksSagaRequest{
			GlobalTxId: txID,
			BranchId:   sagaBranchIDInventoryInit,
			SkuIds:     skuIDs,
		})
		if err != nil {
			logx.L(ctx).Warn("compensate inventory init saga failed", zap.Error(err), zap.String("global_tx_id", txID), zap.String("sku_ids", fmt.Sprint(skuIDs)))
		}
	}
	l.compensateProductCreate(ctx, txID, productID, operatorID)
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
