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

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return &types.CreateProductData{ProductID: resp.ProductID}, nil
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
