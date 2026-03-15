package product

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	productrpc "meshcart/gateway/rpc/product"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"
)

func TestAdminDetailLogic_GetAggregatesInventory(t *testing.T) {
	logic := NewAdminDetailLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code: common.CodeOK,
				Product: &productpb.Product{
					Id:          2001,
					Title:       "Tee",
					Status:      productStatusOffline,
					CreatorId:   1,
					Description: "desc",
					Skus: []*productpb.ProductSku{
						{Id: 3001, SpuId: 2001, Title: "Blue", Status: 1},
						{Id: 3002, SpuId: 2001, Title: "White", Status: 0},
					},
				},
			}, nil
		},
	}, &stubInventoryClient{
		batchGetSkuStockFn: func(_ context.Context, req *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error) {
			if len(req.GetSkuIds()) != 2 || req.GetSkuIds()[0] != 3001 || req.GetSkuIds()[1] != 3002 {
				t.Fatalf("unexpected sku ids: %+v", req.GetSkuIds())
			}
			return &inventoryrpc.BatchGetSkuStockResponse{
				Code: common.CodeOK,
				Stocks: []*inventorypb.SkuStock{
					{SkuId: 3001, AvailableStock: 8, Status: 1},
					{SkuId: 3002, AvailableStock: 0, Status: 0},
				},
			}, nil
		},
	}))

	data, bizErr := logic.Get(2001, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.CreatorID != 1 || len(data.SKUs) != 2 {
		t.Fatalf("unexpected data: %+v", data)
	}
	if data.SKUs[0].Inventory == nil || data.SKUs[0].Inventory.AvailableStock != 8 {
		t.Fatalf("expected inventory to be aggregated, got %+v", data.SKUs[0].Inventory)
	}
	if data.SKUs[1].Inventory == nil || data.SKUs[1].Inventory.Status != 0 {
		t.Fatalf("expected inactive sku inventory status, got %+v", data.SKUs[1].Inventory)
	}
}

func TestAdminDetailLogic_GetForbiddenForOtherAdmin(t *testing.T) {
	logic := NewAdminDetailLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code: common.CodeOK,
				Product: &productpb.Product{
					Id:        2001,
					Status:    productStatusOffline,
					CreatorId: 2,
				},
			}, nil
		},
	}, &stubInventoryClient{
		batchGetSkuStockFn: func(context.Context, *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error) {
			t.Fatal("batch get sku stock should not be called when forbidden")
			return nil, nil
		},
	}))

	_, bizErr := logic.Get(2001, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != common.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %+v", bizErr)
	}
}
