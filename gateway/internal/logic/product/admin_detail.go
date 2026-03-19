package product

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type AdminDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDetailLogic {
	return &AdminDetailLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminDetailLogic) Get(productID int64, identity *middleware.AuthIdentity) (*types.AdminProductDetailData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.admin_detail", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "admin_detail"), attribute.Int64("product_id", productID))

	if productID <= 0 {
		return nil, common.ErrInvalidParam
	}
	if identity == nil {
		return nil, common.ErrUnauthorized
	}
	if role := roleOf(identity); role != authz.RoleAdmin && role != authz.RoleSuperAdmin {
		return nil, common.ErrForbidden
	}

	productResp, err := l.svcCtx.ProductClient.GetProductDetail(ctx, &productpb.GetProductDetailRequest{ProductId: productID})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc admin detail failed")
		logx.L(ctx).Error("product rpc admin detail failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if productResp.Code != common.CodeOK {
		logx.L(ctx).Warn("product rpc admin detail returned business error",
			zap.Int64("product_id", productID),
			zap.Int32("code", productResp.Code),
			zap.String("message", productResp.Message),
		)
		return nil, common.NewBizError(productResp.Code, productResp.Message)
	}
	if productResp.Product == nil {
		return nil, common.ErrNotFound
	}
	product := productResp.Product

	if !l.svcCtx.AccessControl.Enforce(roleOf(identity), "product", authz.ActionReadPrivate, product.GetCreatorId(), identity.UserID, product.GetStatus()) {
		return nil, common.ErrForbidden
	}

	stockMap, bizErr := l.batchGetInventoryBySKUs(ctx, product.GetSkus())
	if bizErr != nil {
		return nil, bizErr
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return toAdminDetailData(product, stockMap), nil
}

func (l *AdminDetailLogic) batchGetInventoryBySKUs(ctx context.Context, skus []*productpb.ProductSku) (map[int64]*types.InventoryStockData, *common.BizError) {
	if len(skus) == 0 {
		return map[int64]*types.InventoryStockData{}, nil
	}

	skuIDs := make([]int64, 0, len(skus))
	for _, sku := range skus {
		if sku == nil || sku.GetId() <= 0 {
			continue
		}
		skuIDs = append(skuIDs, sku.GetId())
	}
	if len(skuIDs) == 0 {
		return map[int64]*types.InventoryStockData{}, nil
	}

	stockResp, err := l.svcCtx.InventoryClient.BatchGetSkuStock(ctx, &inventorypb.BatchGetSkuStockRequest{SkuIds: skuIDs})
	if err != nil {
		logx.L(ctx).Error("inventory rpc batch get sku stock for admin detail failed", zap.Error(err), zap.Int64s("sku_ids", skuIDs))
		return nil, logicutil.MapRPCError(err)
	}
	if stockResp.Code != common.CodeOK {
		logx.L(ctx).Warn("inventory rpc batch get sku stock for admin detail returned business error",
			zap.Int64s("sku_ids", skuIDs),
			zap.Int32("code", stockResp.Code),
			zap.String("message", stockResp.Message),
		)
		return nil, common.NewBizError(stockResp.Code, stockResp.Message)
	}

	stockMap := make(map[int64]*types.InventoryStockData, len(stockResp.Stocks))
	for _, stock := range stockResp.Stocks {
		if stock == nil {
			continue
		}
		stockMap[stock.GetSkuId()] = toInventoryStockData(stock)
	}
	return stockMap, nil
}
