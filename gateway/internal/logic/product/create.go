package product

import (
	"context"
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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type CreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

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

	resp, err := l.svcCtx.ProductClient.CreateProduct(ctx, buildCreateProductRPCRequest(req, identity.UserID))
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

	if bizErr := l.initStocksAfterProductCreate(ctx, req, resp, identity.UserID); bizErr != nil {
		return nil, bizErr
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

func (l *CreateLogic) initStocksAfterProductCreate(ctx context.Context, req *types.CreateProductRequest, resp *productrpc.CreateProductResponse, operatorID int64) *common.BizError {
	initItems := buildInitStockItems(req.SKUs, resp.Skus)
	if len(initItems) != countRequestedInitialStocks(req.SKUs) {
		logx.L(ctx).Error("created sku ids do not match requested initial stocks", zap.Int64("product_id", resp.ProductID))
		l.bestEffortDemoteProduct(ctx, resp.ProductID, req.Status, operatorID)
		return common.ErrInternalError
	}
	if len(initItems) == 0 {
		return nil
	}

	initResp, err := l.svcCtx.InventoryClient.InitSkuStocks(ctx, &inventorypb.InitSkuStocksRequest{Stocks: initItems})
	if err != nil {
		logx.L(ctx).Error("inventory rpc init sku stocks failed after create product", zap.Error(err), zap.Int64("product_id", resp.ProductID))
		l.bestEffortDemoteProduct(ctx, resp.ProductID, req.Status, operatorID)
		return logicutil.MapRPCError(err)
	}
	if initResp.Code != common.CodeOK {
		l.bestEffortDemoteProduct(ctx, resp.ProductID, req.Status, operatorID)
		return common.NewBizError(initResp.Code, initResp.Message)
	}
	return nil
}

func buildInitStockItems(requestSKUs []types.ProductSkuInput, createdSKUs []*productpb.ProductSku) []*inventorypb.InitSkuStockItem {
	if len(requestSKUs) == 0 || len(createdSKUs) == 0 {
		return nil
	}
	stockByCode := make(map[string]int64, len(requestSKUs))
	for _, sku := range requestSKUs {
		if sku.InitialStock == nil {
			continue
		}
		stockByCode[strings.TrimSpace(sku.SKUCode)] = *sku.InitialStock
	}
	result := make([]*inventorypb.InitSkuStockItem, 0, len(stockByCode))
	for _, sku := range createdSKUs {
		stock, ok := stockByCode[strings.TrimSpace(sku.GetSkuCode())]
		if !ok {
			continue
		}
		result = append(result, &inventorypb.InitSkuStockItem{
			SkuId:      sku.GetId(),
			TotalStock: stock,
		})
	}
	return result
}

func countRequestedInitialStocks(items []types.ProductSkuInput) int {
	count := 0
	for _, item := range items {
		if item.InitialStock != nil {
			count++
		}
	}
	return count
}

func (l *CreateLogic) bestEffortDemoteProduct(ctx context.Context, productID int64, requestedStatus int32, operatorID int64) {
	if requestedStatus != productStatusOnline {
		return
	}
	_, err := l.svcCtx.ProductClient.ChangeProductStatus(ctx, &productpb.ChangeProductStatusRequest{
		ProductId:  productID,
		Status:     productStatusOffline,
		OperatorId: operatorID,
	})
	if err != nil {
		logx.L(ctx).Warn("best effort demote product after inventory init failure failed", zap.Error(err), zap.Int64("product_id", productID))
	}
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
