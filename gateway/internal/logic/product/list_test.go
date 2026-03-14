package product

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/types"
	productrpc "meshcart/gateway/rpc/product"
	productpb "meshcart/kitex_gen/meshcart/product"
)

func TestList_PublicListAlwaysUsesOnlineStatus(t *testing.T) {
	logic := NewListLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		listProductsFn: func(_ context.Context, req *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
			if req.Status == nil || req.GetStatus() != 2 {
				t.Fatalf("expected public list to always request online status, got %+v", req.Status)
			}
			if req.CreatorId != nil {
				t.Fatalf("expected public list to not set creator filter, got %+v", req.CreatorId)
			}
			return &productrpc.ListProductsResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}, &stubInventoryClient{}))

	_, bizErr := logic.List(&types.ListProductsRequest{
		Page:     1,
		PageSize: 5,
		Status:   int32Ptr(0),
	}, &middleware.AuthIdentity{UserID: 88, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
}

func TestListOwned_AdminOnlySeesOwnProducts(t *testing.T) {
	logic := NewListLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		listProductsFn: func(_ context.Context, req *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
			if req.GetPage() != 1 || req.GetPageSize() != 5 {
				t.Fatalf("unexpected paging request: %+v", req)
			}
			if req.CreatorId == nil || req.GetCreatorId() != 88 {
				t.Fatalf("expected creator filter to be current admin, got %+v", req)
			}
			if req.Status == nil || req.GetStatus() != 2 {
				t.Fatalf("expected status filter to be forwarded, got %+v", req)
			}
			return &productrpc.ListProductsResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Products: []*productpb.ProductListItem{
					{Id: 1001, Title: "Own Tee", Status: 2},
				},
				Total: 1,
			}, nil
		},
	}, &stubInventoryClient{}))

	data, bizErr := logic.ListOwned(&types.ListProductsRequest{
		Page:     1,
		PageSize: 5,
		Status:   int32Ptr(2),
	}, &middleware.AuthIdentity{UserID: 88, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if data == nil || data.Total != 1 || len(data.Products) != 1 || data.Products[0].ID != 1001 {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestListOwned_AdminDefaultsToAllStatusesOfOwnProducts(t *testing.T) {
	logic := NewListLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		listProductsFn: func(_ context.Context, req *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
			if req.CreatorId == nil || req.GetCreatorId() != 66 {
				t.Fatalf("expected creator filter to be current admin, got %+v", req)
			}
			if req.Status != nil {
				t.Fatalf("expected nil status filter for owned list default, got %+v", req.Status)
			}
			return &productrpc.ListProductsResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}, &stubInventoryClient{}))

	_, bizErr := logic.ListOwned(&types.ListProductsRequest{}, &middleware.AuthIdentity{UserID: 66, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
}

func TestListOwned_RejectsNonAdmin(t *testing.T) {
	logic := NewListLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{}, &stubInventoryClient{}))

	_, bizErr := logic.ListOwned(&types.ListProductsRequest{}, &middleware.AuthIdentity{UserID: 12, Role: authz.RoleUser})
	if bizErr == nil || bizErr.Code != common.ErrForbidden.Code {
		t.Fatalf("expected forbidden, got %+v", bizErr)
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}
