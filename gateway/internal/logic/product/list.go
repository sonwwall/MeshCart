package product

import (
	"context"

	"meshcart/app/common"
	logx "meshcart/app/log"
	tracex "meshcart/app/trace"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	productpb "meshcart/kitex_gen/meshcart/product"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ListLogic) List(req *types.ListProductsRequest) (*types.ListProductsData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.list", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "list"))

	rpcReq := &productpb.ListProductsRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
	}
	if req.Status != nil {
		rpcReq.Status = req.Status
	}
	if req.CategoryID != nil {
		rpcReq.CategoryId = req.CategoryID
	}
	if req.Keyword != "" {
		keyword := req.Keyword
		rpcReq.Keyword = &keyword
	}

	resp, err := l.svcCtx.ProductClient.ListProducts(ctx, rpcReq)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc list failed")
		logx.L(ctx).Error("product rpc list failed", zap.Error(err))
		return nil, common.ErrInternalError
	}
	if resp.Code != common.CodeOK {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(resp.Code)), attribute.String("biz.message", resp.Message))
		return nil, common.NewBizError(resp.Code, resp.Message)
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	return &types.ListProductsData{
		Products: resp.Products,
		Total:    resp.Total,
	}, nil
}
