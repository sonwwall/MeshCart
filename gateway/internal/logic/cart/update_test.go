package cart

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/internal/types"
	cartrpc "meshcart/gateway/rpc/cart"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	productrpc "meshcart/gateway/rpc/product"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"
)

func TestUpdateLogic_ProductOffline(t *testing.T) {
	logic := NewUpdateLogic(context.Background(), newCartTestServiceContext(t, &stubCartClient{
		getCartFn: func(context.Context, *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error) {
			return &cartrpc.GetCartResponse{
				Code: common.CodeOK,
				Items: []*cartpb.CartItem{
					{Id: 11, ProductId: 2001, SkuId: 3001, Quantity: 1},
				},
			}, nil
		},
	}, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{Id: 2001, Status: 1},
			}, nil
		},
	}, &stubInventoryClient{}))

	item, bizErr := logic.Update(101, 11, &types.UpdateCartItemRequest{Quantity: 2})
	if item != nil {
		t.Fatalf("expected nil item, got %+v", item)
	}
	if bizErr != common.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %+v", bizErr)
	}
}

func TestUpdateLogic_SKUInactive(t *testing.T) {
	logic := NewUpdateLogic(context.Background(), newCartTestServiceContext(t, &stubCartClient{
		getCartFn: func(context.Context, *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error) {
			return &cartrpc.GetCartResponse{
				Code: common.CodeOK,
				Items: []*cartpb.CartItem{
					{Id: 11, ProductId: 2001, SkuId: 3001, Quantity: 1},
				},
			}, nil
		},
	}, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{
					Id:     2001,
					Status: 2,
					Skus: []*productpb.ProductSku{
						{Id: 3001, Status: 2},
					},
				},
			}, nil
		},
	}, &stubInventoryClient{}))

	item, bizErr := logic.Update(101, 11, &types.UpdateCartItemRequest{Quantity: 2})
	if item != nil {
		t.Fatalf("expected nil item, got %+v", item)
	}
	if bizErr != common.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %+v", bizErr)
	}
}

func TestUpdateLogic_InsufficientStock(t *testing.T) {
	logic := NewUpdateLogic(context.Background(), newCartTestServiceContext(t, &stubCartClient{
		getCartFn: func(context.Context, *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error) {
			return &cartrpc.GetCartResponse{
				Code: common.CodeOK,
				Items: []*cartpb.CartItem{
					{Id: 11, ProductId: 2001, SkuId: 3001, Quantity: 1},
				},
			}, nil
		},
	}, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{
					Id:     2001,
					Status: 2,
					Skus: []*productpb.ProductSku{
						{Id: 3001, Status: 1},
					},
				},
			}, nil
		},
	}, &stubInventoryClient{
		checkSaleableStockFn: func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
			return &inventoryrpc.CheckSaleableStockResponse{Code: 2050002, Message: "库存不足", Saleable: false, AvailableStock: 1}, nil
		},
	}))

	item, bizErr := logic.Update(101, 11, &types.UpdateCartItemRequest{Quantity: 2})
	if item != nil {
		t.Fatalf("expected nil item, got %+v", item)
	}
	if bizErr == nil || bizErr.Code != 2050002 {
		t.Fatalf("expected insufficient stock error, got %+v", bizErr)
	}
}

func TestUpdateLogic_Success(t *testing.T) {
	logic := NewUpdateLogic(context.Background(), newCartTestServiceContext(t, &stubCartClient{
		getCartFn: func(context.Context, *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error) {
			return &cartrpc.GetCartResponse{
				Code: common.CodeOK,
				Items: []*cartpb.CartItem{
					{Id: 11, ProductId: 2001, SkuId: 3001, Quantity: 1, Checked: true},
				},
			}, nil
		},
		updateCartItemFn: func(_ context.Context, req *cartpb.UpdateCartItemRequest) (*cartrpc.UpdateCartItemResponse, error) {
			if req.GetUserId() != 101 || req.GetItemId() != 11 || req.GetQuantity() != 3 {
				t.Fatalf("unexpected update request: %+v", req)
			}
			return &cartrpc.UpdateCartItemResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Item: &cartpb.CartItem{
					Id:        11,
					ProductId: 2001,
					SkuId:     3001,
					Quantity:  3,
					Checked:   true,
				},
			}, nil
		},
	}, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{
					Id:     2001,
					Status: 2,
					Skus: []*productpb.ProductSku{
						{Id: 3001, Status: 1},
					},
				},
			}, nil
		},
	}, &stubInventoryClient{
		checkSaleableStockFn: func(_ context.Context, req *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
			if req.GetSkuId() != 3001 || req.GetQuantity() != 3 {
				t.Fatalf("unexpected stock check request: %+v", req)
			}
			return &inventoryrpc.CheckSaleableStockResponse{Code: common.CodeOK, Message: "成功", Saleable: true, AvailableStock: 10}, nil
		},
	}))

	item, bizErr := logic.Update(101, 11, &types.UpdateCartItemRequest{Quantity: 3})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if item == nil || item.Quantity != 3 || item.ProductID != 2001 || item.SKUID != 3001 {
		t.Fatalf("unexpected item: %+v", item)
	}
}
