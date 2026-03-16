package cart

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	cartrpc "meshcart/gateway/rpc/cart"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	productrpc "meshcart/gateway/rpc/product"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"
)

type stubCartClient struct {
	getCartFn        func(context.Context, *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error)
	addCartItemFn    func(context.Context, *cartpb.AddCartItemRequest) (*cartrpc.AddCartItemResponse, error)
	updateCartItemFn func(context.Context, *cartpb.UpdateCartItemRequest) (*cartrpc.UpdateCartItemResponse, error)
	removeCartItemFn func(context.Context, *cartpb.RemoveCartItemRequest) (*cartrpc.RemoveCartItemResponse, error)
	clearCartFn      func(context.Context, *cartpb.ClearCartRequest) (*cartrpc.ClearCartResponse, error)
}

func (s *stubCartClient) GetCart(ctx context.Context, req *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error) {
	if s.getCartFn != nil {
		return s.getCartFn(ctx, req)
	}
	return &cartrpc.GetCartResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubCartClient) AddCartItem(ctx context.Context, req *cartpb.AddCartItemRequest) (*cartrpc.AddCartItemResponse, error) {
	if s.addCartItemFn != nil {
		return s.addCartItemFn(ctx, req)
	}
	return &cartrpc.AddCartItemResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubCartClient) UpdateCartItem(ctx context.Context, req *cartpb.UpdateCartItemRequest) (*cartrpc.UpdateCartItemResponse, error) {
	if s.updateCartItemFn != nil {
		return s.updateCartItemFn(ctx, req)
	}
	return &cartrpc.UpdateCartItemResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubCartClient) RemoveCartItem(ctx context.Context, req *cartpb.RemoveCartItemRequest) (*cartrpc.RemoveCartItemResponse, error) {
	if s.removeCartItemFn != nil {
		return s.removeCartItemFn(ctx, req)
	}
	return &cartrpc.RemoveCartItemResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubCartClient) ClearCart(ctx context.Context, req *cartpb.ClearCartRequest) (*cartrpc.ClearCartResponse, error) {
	if s.clearCartFn != nil {
		return s.clearCartFn(ctx, req)
	}
	return &cartrpc.ClearCartResponse{Code: common.CodeOK, Message: "成功"}, nil
}

type stubProductClient struct {
	createProductFn               func(context.Context, *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error)
	createProductSagaFn           func(context.Context, *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error)
	compensateCreateProductSagaFn func(context.Context, *productpb.CompensateCreateProductSagaRequest) (*productrpc.ChangeProductStatusResponse, error)
	updateProductFn               func(context.Context, *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error)
	changeStatusFn                func(context.Context, *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error)
	getProductDetailFn            func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error)
	listProductsFn                func(context.Context, *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error)
	batchGetSkuFn                 func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error)
}

func (s *stubProductClient) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error) {
	if s.createProductFn != nil {
		return s.createProductFn(ctx, req)
	}
	return &productrpc.CreateProductResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubProductClient) CreateProductSaga(ctx context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error) {
	if s.createProductSagaFn != nil {
		return s.createProductSagaFn(ctx, req)
	}
	return &productrpc.CreateProductResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubProductClient) CompensateCreateProductSaga(ctx context.Context, req *productpb.CompensateCreateProductSagaRequest) (*productrpc.ChangeProductStatusResponse, error) {
	if s.compensateCreateProductSagaFn != nil {
		return s.compensateCreateProductSagaFn(ctx, req)
	}
	return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubProductClient) UpdateProduct(ctx context.Context, req *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error) {
	if s.updateProductFn != nil {
		return s.updateProductFn(ctx, req)
	}
	return &productrpc.UpdateProductResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubProductClient) ChangeProductStatus(ctx context.Context, req *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error) {
	if s.changeStatusFn != nil {
		return s.changeStatusFn(ctx, req)
	}
	return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubProductClient) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
	if s.getProductDetailFn != nil {
		return s.getProductDetailFn(ctx, req)
	}
	return &productrpc.GetProductDetailResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubProductClient) ListProducts(ctx context.Context, req *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
	if s.listProductsFn != nil {
		return s.listProductsFn(ctx, req)
	}
	return &productrpc.ListProductsResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubProductClient) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
	if s.batchGetSkuFn != nil {
		return s.batchGetSkuFn(ctx, req)
	}
	return &productrpc.BatchGetSKUResponse{Code: common.CodeOK, Message: "成功"}, nil
}

type stubInventoryClient struct {
	getSkuStockFn                 func(context.Context, *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error)
	batchGetSkuStockFn            func(context.Context, *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error)
	checkSaleableStockFn          func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error)
	initSkuStocksFn               func(context.Context, *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	initSkuStocksSagaFn           func(context.Context, *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	compensateInitSkuStocksSagaFn func(context.Context, *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventoryrpc.CompensateInitSkuStocksResponse, error)
	freezeSkuStocksFn             func(context.Context, *inventorypb.FreezeSkuStocksRequest) (*inventoryrpc.FreezeSkuStocksResponse, error)
	adjustStockFn                 func(context.Context, *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error)
}

func (s *stubInventoryClient) GetSkuStock(ctx context.Context, req *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error) {
	if s.getSkuStockFn != nil {
		return s.getSkuStockFn(ctx, req)
	}
	return &inventoryrpc.GetSkuStockResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubInventoryClient) BatchGetSkuStock(ctx context.Context, req *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error) {
	if s.batchGetSkuStockFn != nil {
		return s.batchGetSkuStockFn(ctx, req)
	}
	return &inventoryrpc.BatchGetSkuStockResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubInventoryClient) CheckSaleableStock(ctx context.Context, req *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
	if s.checkSaleableStockFn != nil {
		return s.checkSaleableStockFn(ctx, req)
	}
	return &inventoryrpc.CheckSaleableStockResponse{Code: common.CodeOK, Message: "成功", Saleable: true, AvailableStock: 100}, nil
}

func (s *stubInventoryClient) InitSkuStocks(ctx context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
	if s.initSkuStocksFn != nil {
		return s.initSkuStocksFn(ctx, req)
	}
	return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubInventoryClient) InitSkuStocksSaga(ctx context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
	if s.initSkuStocksSagaFn != nil {
		return s.initSkuStocksSagaFn(ctx, req)
	}
	return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubInventoryClient) CompensateInitSkuStocksSaga(ctx context.Context, req *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventoryrpc.CompensateInitSkuStocksResponse, error) {
	if s.compensateInitSkuStocksSagaFn != nil {
		return s.compensateInitSkuStocksSagaFn(ctx, req)
	}
	return &inventoryrpc.CompensateInitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubInventoryClient) FreezeSkuStocks(ctx context.Context, req *inventorypb.FreezeSkuStocksRequest) (*inventoryrpc.FreezeSkuStocksResponse, error) {
	if s.freezeSkuStocksFn != nil {
		return s.freezeSkuStocksFn(ctx, req)
	}
	return &inventoryrpc.FreezeSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubInventoryClient) AdjustStock(ctx context.Context, req *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error) {
	if s.adjustStockFn != nil {
		return s.adjustStockFn(ctx, req)
	}
	return &inventoryrpc.AdjustStockResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func newCartTestServiceContext(t *testing.T, cartClient cartrpc.Client, productClient productrpc.Client, inventoryClient inventoryrpc.Client) *svc.ServiceContext {
	t.Helper()

	jwtMiddleware, err := middleware.NewJWT(config.JWTConfig{
		Secret:            "test-secret",
		Issuer:            "meshcart.gateway",
		TimeoutMinutes:    120,
		MaxRefreshMinutes: 720,
	})
	if err != nil {
		t.Fatalf("create jwt middleware: %v", err)
	}
	accessController, err := authz.NewAccessController()
	if err != nil {
		t.Fatalf("create access controller: %v", err)
	}

	return &svc.ServiceContext{
		CartClient:      cartClient,
		ProductClient:   productClient,
		InventoryClient: inventoryClient,
		JWT:             jwtMiddleware,
		AccessControl:   accessController,
	}
}

func TestAddLogic_ProductOffline(t *testing.T) {
	logic := NewAddLogic(context.Background(), newCartTestServiceContext(t, &stubCartClient{}, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{Id: 2001, Status: 1},
			}, nil
		},
	}, &stubInventoryClient{}))

	item, bizErr := logic.Add(101, &types.AddCartItemRequest{ProductID: 2001, SKUID: 3001, Quantity: 1})
	if item != nil {
		t.Fatalf("expected nil item, got %+v", item)
	}
	if bizErr != common.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %+v", bizErr)
	}
}

func TestAddLogic_Success(t *testing.T) {
	logic := NewAddLogic(context.Background(), newCartTestServiceContext(t, &stubCartClient{
		addCartItemFn: func(_ context.Context, req *cartpb.AddCartItemRequest) (*cartrpc.AddCartItemResponse, error) {
			if req.GetTitleSnapshot() != "MeshCart Tee" || req.GetSkuTitleSnapshot() != "Blue XL" {
				t.Fatalf("unexpected snapshots: %+v", req)
			}
			return &cartrpc.AddCartItemResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Item: &cartpb.CartItem{
					Id:                1,
					ProductId:         req.GetProductId(),
					SkuId:             req.GetSkuId(),
					Quantity:          req.GetQuantity(),
					Checked:           true,
					TitleSnapshot:     req.GetTitleSnapshot(),
					SkuTitleSnapshot:  req.GetSkuTitleSnapshot(),
					SalePriceSnapshot: req.GetSalePriceSnapshot(),
					CoverUrlSnapshot:  req.GetCoverUrlSnapshot(),
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
					Title:  "MeshCart Tee",
					Status: 2,
					Skus: []*productpb.ProductSku{
						{Id: 3001, Title: "Blue XL", SalePrice: 1999, Status: 1, CoverUrl: "https://example.test/cover.png"},
					},
				},
			}, nil
		},
	}, &stubInventoryClient{
		checkSaleableStockFn: func(_ context.Context, req *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
			if req.GetSkuId() != 3001 || req.GetQuantity() != 2 {
				t.Fatalf("unexpected stock check request: %+v", req)
			}
			return &inventoryrpc.CheckSaleableStockResponse{Code: common.CodeOK, Message: "成功", Saleable: true, AvailableStock: 10}, nil
		},
	}))

	item, bizErr := logic.Add(101, &types.AddCartItemRequest{ProductID: 2001, SKUID: 3001, Quantity: 2})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if item == nil || item.Quantity != 2 || item.SKUTitleSnapshot != "Blue XL" {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestAddLogic_InsufficientStock(t *testing.T) {
	logic := NewAddLogic(context.Background(), newCartTestServiceContext(t, &stubCartClient{}, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{
					Id:     2001,
					Title:  "MeshCart Tee",
					Status: 2,
					Skus: []*productpb.ProductSku{
						{Id: 3001, Title: "Blue XL", SalePrice: 1999, Status: 1},
					},
				},
			}, nil
		},
	}, &stubInventoryClient{
		checkSaleableStockFn: func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
			return &inventoryrpc.CheckSaleableStockResponse{Code: 2050002, Message: "库存不足", Saleable: false, AvailableStock: 1}, nil
		},
	}))

	item, bizErr := logic.Add(101, &types.AddCartItemRequest{ProductID: 2001, SKUID: 3001, Quantity: 2})
	if item != nil {
		t.Fatalf("expected nil item, got %+v", item)
	}
	if bizErr == nil || bizErr.Code != 2050002 {
		t.Fatalf("expected insufficient stock error, got %+v", bizErr)
	}
}
