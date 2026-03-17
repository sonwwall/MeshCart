package cart

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/logic/logicutil"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	cartpb "meshcart/kitex_gen/meshcart/cart"
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

func (l *UpdateLogic) Update(userID, itemID int64, req *types.UpdateCartItemRequest) (*types.CartItemData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.cart.update", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "cart"), attribute.String("biz.action", "update"), attribute.Int64("user_id", userID), attribute.Int64("item_id", itemID))

	if userID <= 0 || itemID <= 0 || req == nil || req.Quantity <= 0 {
		return nil, common.ErrInvalidParam
	}

	getResp, err := l.svcCtx.CartClient.GetCart(ctx, &cartpb.GetCartRequest{UserId: userID})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("cart rpc get before update failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if getResp.Code != common.CodeOK {
		return nil, common.NewBizError(getResp.Code, getResp.Message)
	}

	existing := findCartItem(getResp.Items, itemID)
	if existing == nil {
		return nil, common.ErrNotFound
	}

	detailResp, err := l.svcCtx.ProductClient.GetProductDetail(ctx, &productpb.GetProductDetailRequest{ProductId: existing.GetProductId()})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("product rpc detail before update cart failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if detailResp.Code != common.CodeOK || detailResp.Product == nil {
		return nil, common.NewBizError(detailResp.Code, detailResp.Message)
	}
	if detailResp.Product.GetStatus() != productStatusOnline {
		return nil, common.ErrNotFound
	}

	sku := findSKU(detailResp.Product, existing.GetSkuId())
	if sku == nil || sku.GetStatus() != skuStatusActive {
		return nil, common.ErrNotFound
	}

	stockResp, err := l.svcCtx.InventoryClient.CheckSaleableStock(ctx, &inventorypb.CheckSaleableStockRequest{
		SkuId:    existing.GetSkuId(),
		Quantity: req.Quantity,
	})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("inventory rpc check stock before update cart failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if stockResp.Code != common.CodeOK {
		return nil, common.NewBizError(stockResp.Code, stockResp.Message)
	}
	if !stockResp.Saleable {
		return nil, common.NewBizError(inventoryCodeInsufficientStock, "库存不足")
	}

	resp, err := l.svcCtx.CartClient.UpdateCartItem(ctx, &cartpb.UpdateCartItemRequest{
		UserId:   userID,
		ItemId:   itemID,
		Quantity: req.Quantity,
		Checked:  req.Checked,
	})
	if err != nil {
		span.RecordError(err)
		logx.L(ctx).Error("cart rpc update failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		return nil, common.NewBizError(resp.Code, resp.Message)
	}
	if resp.Item == nil {
		return nil, common.ErrInternalError
	}

	item := toCartData([]*cartpb.CartItem{resp.Item}).Items[0]
	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return &item, nil
}
