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

type UpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogic {
	return &UpdateLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateLogic) Update(productID int64, req *types.UpdateProductRequest, identity *middleware.AuthIdentity) *common.BizError {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.update", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "update"), attribute.Int64("product_id", productID))

	if productID <= 0 || strings.TrimSpace(req.Title) == "" || len(req.SKUs) == 0 {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(common.ErrInvalidParam.Code)), attribute.String("biz.message", common.ErrInvalidParam.Msg))
		return common.ErrInvalidParam
	}
	if bizErr := validateUpdateInitialStocks(req.SKUs); bizErr != nil {
		return bizErr
	}

	if identity == nil {
		return common.ErrUnauthorized
	}
	detailResp, err := l.svcCtx.ProductClient.GetProductDetail(ctx, &productpb.GetProductDetailRequest{ProductId: productID})
	if err != nil {
		logx.L(ctx).Error("product rpc detail before update failed", zap.Error(err))
		return logicutil.MapRPCError(err)
	}
	if detailResp.Code != common.CodeOK || detailResp.Product == nil {
		return common.NewBizError(detailResp.Code, detailResp.Message)
	}
	role := roleOf(identity)
	if !l.svcCtx.AccessControl.Enforce(role, "product", authz.ActionWriteOwn, detailResp.Product.GetCreatorId(), identity.UserID, detailResp.Product.GetStatus()) {
		logx.L(ctx).Warn("product update forbidden",
			zap.Int64("product_id", productID),
			zap.Int64("user_id", identity.UserID),
			zap.String("role", role),
			zap.Int64("creator_id", detailResp.Product.GetCreatorId()),
			zap.Int32("status", detailResp.Product.GetStatus()),
		)
		if role == authz.RoleAdmin {
			return errOwnProductRequired
		}
		return common.ErrForbidden
	}
	deletedSKUIds := findDeletedSKUIds(detailResp.Product.GetSkus(), req.SKUs)

	resp, err := l.svcCtx.ProductClient.UpdateProduct(ctx, &productpb.UpdateProductRequest{
		ProductId:   productID,
		Title:       req.Title,
		SubTitle:    req.SubTitle,
		CategoryId:  req.CategoryID,
		Brand:       req.Brand,
		Description: req.Description,
		Status:      req.Status,
		Skus:        buildSKUInputs(req.SKUs),
		OperatorId:  identity.UserID,
	})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc update failed")
		logx.L(ctx).Error("product rpc update failed", zap.Error(err))
		return logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(resp.Code)), attribute.String("biz.message", resp.Message))
		return common.NewBizError(resp.Code, resp.Message)
	}
	if bizErr := l.syncStocksAfterProductUpdate(ctx, req, resp, deletedSKUIds, identity.UserID); bizErr != nil {
		return bizErr
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return nil
}

func (l *UpdateLogic) syncStocksAfterProductUpdate(ctx context.Context, req *types.UpdateProductRequest, resp *productrpc.UpdateProductResponse, deletedSKUIds []int64, operatorID int64) *common.BizError {
	expectedNewSKUCount := countNewSKUs(req.SKUs)
	if expectedNewSKUCount > 0 {
		if len(req.SKUs) != len(resp.Skus) {
			logx.L(ctx).Error("updated sku ids do not match requested skus", zap.Int("request_sku_count", len(req.SKUs)), zap.Int("response_sku_count", len(resp.Skus)))
			return common.ErrInternalError
		}

		initItems := buildInitStockItemsForNewSKUs(req.SKUs, resp.Skus)
		if len(initItems) != expectedNewSKUCount {
			logx.L(ctx).Error("updated new sku ids do not match requested initial stocks", zap.Int("expected_new_sku_count", expectedNewSKUCount), zap.Int("actual_init_stock_count", len(initItems)))
			return common.ErrInternalError
		}

		initResp, err := l.svcCtx.InventoryClient.InitSkuStocks(ctx, &inventorypb.InitSkuStocksRequest{Stocks: initItems})
		if err != nil {
			logx.L(ctx).Error("inventory rpc init sku stocks failed after update product", zap.Error(err))
			return logicutil.MapRPCError(err)
		}
		if initResp.Code != common.CodeOK {
			return common.NewBizError(initResp.Code, initResp.Message)
		}
	}

	if len(deletedSKUIds) > 0 {
		freezeResp, err := l.svcCtx.InventoryClient.FreezeSkuStocks(ctx, &inventorypb.FreezeSkuStocksRequest{
			SkuIds:     deletedSKUIds,
			OperatorId: operatorID,
			Reason:     stringPtr("product sku removed"),
		})
		if err != nil {
			logx.L(ctx).Error("inventory rpc freeze sku stocks failed after update product", zap.Error(err), zap.Int64s("sku_ids", deletedSKUIds))
			return logicutil.MapRPCError(err)
		}
		if freezeResp.Code != common.CodeOK {
			return common.NewBizError(freezeResp.Code, freezeResp.Message)
		}
	}
	return nil
}

func buildInitStockItemsForNewSKUs(requestSKUs []types.ProductSkuInput, updatedSKUs []*productpb.ProductSku) []*inventorypb.InitSkuStockItem {
	if len(requestSKUs) == 0 || len(updatedSKUs) == 0 || len(requestSKUs) != len(updatedSKUs) {
		return nil
	}

	result := make([]*inventorypb.InitSkuStockItem, 0, len(updatedSKUs))
	for idx, sku := range requestSKUs {
		if sku.ID != nil && *sku.ID > 0 {
			continue
		}
		stock := int64(0)
		if sku.InitialStock != nil {
			stock = *sku.InitialStock
		}
		result = append(result, &inventorypb.InitSkuStockItem{
			SkuId:      updatedSKUs[idx].GetId(),
			TotalStock: stock,
		})
	}
	return result
}

func countNewSKUs(items []types.ProductSkuInput) int {
	count := 0
	for _, sku := range items {
		if sku.ID == nil || *sku.ID == 0 {
			count++
		}
	}
	return count
}

func validateUpdateInitialStocks(items []types.ProductSkuInput) *common.BizError {
	for _, sku := range items {
		if sku.InitialStock != nil && sku.ID != nil && *sku.ID > 0 {
			return common.ErrInvalidParam
		}
	}
	return validateInitialStocks(items)
}

func findDeletedSKUIds(existingSKUs []*productpb.ProductSku, requestSKUs []types.ProductSkuInput) []int64 {
	if len(existingSKUs) == 0 {
		return nil
	}

	requested := make(map[int64]struct{}, len(requestSKUs))
	for _, sku := range requestSKUs {
		if sku.ID != nil && *sku.ID > 0 {
			requested[*sku.ID] = struct{}{}
		}
	}

	deleted := make([]int64, 0)
	for _, sku := range existingSKUs {
		if sku == nil || sku.GetId() <= 0 {
			continue
		}
		if _, ok := requested[sku.GetId()]; !ok {
			deleted = append(deleted, sku.GetId())
		}
	}
	return deleted
}

func stringPtr(v string) *string {
	return &v
}
