package product

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	productrpc "meshcart/gateway/rpc/product"
	productpb "meshcart/kitex_gen/meshcart/product"
)

func TestDetailLogic_PublicDetailHidesInactiveSKUs(t *testing.T) {
	logic := NewDetailLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code: common.CodeOK,
				Product: &productpb.Product{
					Id:     2001,
					Status: productStatusOnline,
					Skus: []*productpb.ProductSku{
						{Id: 3001, Status: 1, Title: "Active"},
						{Id: 3002, Status: 0, Title: "Inactive"},
					},
				},
			}, nil
		},
	}, &stubInventoryClient{}))

	data, bizErr := logic.Get(2001, nil)
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if len(data.SKUs) != 1 || data.SKUs[0].ID != 3001 {
		t.Fatalf("expected only active skus, got %+v", data.SKUs)
	}
}

func TestDetailLogic_PrivateDetailKeepsInactiveSKUs(t *testing.T) {
	logic := NewDetailLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code: common.CodeOK,
				Product: &productpb.Product{
					Id:        2001,
					Status:    productStatusOffline,
					CreatorId: 1,
					Skus: []*productpb.ProductSku{
						{Id: 3001, Status: 1, Title: "Active"},
						{Id: 3002, Status: 0, Title: "Inactive"},
					},
				},
			}, nil
		},
	}, &stubInventoryClient{}))

	data, bizErr := logic.Get(2001, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if len(data.SKUs) != 2 {
		t.Fatalf("expected both skus for private detail, got %+v", data.SKUs)
	}
}
