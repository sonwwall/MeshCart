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

func (l *ListLogic) List(req *types.ListProductsRequest, identity *middleware.AuthIdentity) (*types.ListProductsData, *common.BizError) {
	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.list", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "list"))

	rpcReq, bizErr := l.buildPublicListRequest(req, identity)
	if bizErr != nil {
		return nil, bizErr
	}
	resp, err := l.svcCtx.ProductClient.ListProducts(ctx, rpcReq)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc list failed")
		logx.L(ctx).Error("product rpc list failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(resp.Code)), attribute.String("biz.message", resp.Message))
		return nil, common.NewBizError(resp.Code, resp.Message)
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	items := make([]types.ProductListItemData, 0, len(resp.Products))
	for _, item := range resp.Products {
		items = append(items, toListItemData(item))
	}
	return &types.ListProductsData{
		Products: items,
		Total:    resp.Total,
	}, nil
}

func (l *ListLogic) ListOwned(req *types.ListProductsRequest, identity *middleware.AuthIdentity) (*types.ListProductsData, *common.BizError) {
	if identity == nil {
		return nil, common.ErrUnauthorized
	}
	if role := roleOf(identity); role != authz.RoleAdmin && role != authz.RoleSuperAdmin {
		return nil, common.ErrForbidden
	}

	ctx, span := tracex.StartSpan(l.ctx, "meshcart.gateway", "gateway.logic.product.list_owned", oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("biz.module", "product"), attribute.String("biz.action", "list_owned"))

	rpcReq, bizErr := l.buildOwnedListRequest(req, identity)
	if bizErr != nil {
		return nil, bizErr
	}
	resp, err := l.svcCtx.ProductClient.ListProducts(ctx, rpcReq)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "technical"))
		span.SetStatus(codes.Error, "product rpc owned list failed")
		logx.L(ctx).Error("product rpc owned list failed", zap.Error(err))
		return nil, logicutil.MapRPCError(err)
	}
	if resp.Code != common.CodeOK {
		span.SetAttributes(attribute.Bool("biz.success", false), attribute.String("biz.type", "business"), attribute.Int("biz.code", int(resp.Code)), attribute.String("biz.message", resp.Message))
		return nil, common.NewBizError(resp.Code, resp.Message)
	}

	span.SetAttributes(attribute.Bool("biz.success", true))
	span.SetStatus(codes.Ok, "ok")
	items := make([]types.ProductListItemData, 0, len(resp.Products))
	for _, item := range resp.Products {
		items = append(items, toListItemData(item))
	}
	return &types.ListProductsData{
		Products: items,
		Total:    resp.Total,
	}, nil
}

func (l *ListLogic) buildPublicListRequest(req *types.ListProductsRequest, identity *middleware.AuthIdentity) (*productpb.ListProductsRequest, *common.BizError) {
	rpcReq := buildBaseListRequest(req)
	_ = identity
	status := int32(2)
	rpcReq.Status = &status
	return rpcReq, nil
}

func (l *ListLogic) buildOwnedListRequest(req *types.ListProductsRequest, identity *middleware.AuthIdentity) (*productpb.ListProductsRequest, *common.BizError) {
	rpcReq := buildBaseListRequest(req)
	if req.Status != nil {
		status := *req.Status
		if status < 0 || status > 2 {
			return nil, common.ErrInvalidParam
		}
		rpcReq.Status = &status
	}
	if roleOf(identity) != authz.RoleSuperAdmin {
		creatorID := identity.UserID
		rpcReq.CreatorId = &creatorID
	}
	return rpcReq, nil
}

func buildBaseListRequest(req *types.ListProductsRequest) *productpb.ListProductsRequest {
	rpcReq := &productpb.ListProductsRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
	}
	if req.CategoryID != nil {
		rpcReq.CategoryId = req.CategoryID
	}
	if req.Keyword != "" {
		keyword := req.Keyword
		rpcReq.Keyword = &keyword
	}
	return rpcReq
}
