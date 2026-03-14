package inventory

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	productrpc "meshcart/gateway/rpc/product"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"
)

type stubInventoryClient struct {
	getSkuStockFn        func(context.Context, *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error)
	batchGetSkuStockFn   func(context.Context, *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error)
	checkSaleableStockFn func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error)
	initSkuStocksFn      func(context.Context, *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	adjustStockFn        func(context.Context, *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error)
}

type stubProductClient struct {
	createProductFn    func(context.Context, *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error)
	updateProductFn    func(context.Context, *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error)
	changeStatusFn     func(context.Context, *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error)
	getProductDetailFn func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error)
	listProductsFn     func(context.Context, *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error)
	batchGetSkuFn      func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error)
}

func (s *stubProductClient) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error) {
	return s.createProductFn(ctx, req)
}
func (s *stubProductClient) UpdateProduct(ctx context.Context, req *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error) {
	return s.updateProductFn(ctx, req)
}
func (s *stubProductClient) ChangeProductStatus(ctx context.Context, req *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error) {
	return s.changeStatusFn(ctx, req)
}
func (s *stubProductClient) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
	return s.getProductDetailFn(ctx, req)
}
func (s *stubProductClient) ListProducts(ctx context.Context, req *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
	return s.listProductsFn(ctx, req)
}
func (s *stubProductClient) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
	return s.batchGetSkuFn(ctx, req)
}

func (s *stubInventoryClient) GetSkuStock(ctx context.Context, req *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error) {
	return s.getSkuStockFn(ctx, req)
}

func (s *stubInventoryClient) BatchGetSkuStock(ctx context.Context, req *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error) {
	return s.batchGetSkuStockFn(ctx, req)
}

func (s *stubInventoryClient) CheckSaleableStock(ctx context.Context, req *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
	if s.checkSaleableStockFn != nil {
		return s.checkSaleableStockFn(ctx, req)
	}
	return &inventoryrpc.CheckSaleableStockResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubInventoryClient) InitSkuStocks(ctx context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
	if s.initSkuStocksFn != nil {
		return s.initSkuStocksFn(ctx, req)
	}
	return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubInventoryClient) AdjustStock(ctx context.Context, req *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error) {
	return s.adjustStockFn(ctx, req)
}

func newInventorySvcCtx(t *testing.T, client inventoryrpc.Client) *svc.ServiceContext {
	t.Helper()
	ac, err := authz.NewAccessController()
	if err != nil {
		t.Fatalf("new access controller: %v", err)
	}
	jwtMiddleware, err := middleware.NewJWT(config.JWTConfig{
		Secret:            "test-secret",
		Issuer:            "meshcart.gateway",
		TimeoutMinutes:    30,
		MaxRefreshMinutes: 60,
	})
	if err != nil {
		t.Fatalf("new jwt middleware: %v", err)
	}
	return &svc.ServiceContext{
		InventoryClient: client,
		AccessControl:   ac,
		JWT:             jwtMiddleware,
	}
}

func newInventorySvcCtxWithProduct(t *testing.T, inventoryClient inventoryrpc.Client, productClient productrpc.Client) *svc.ServiceContext {
	t.Helper()
	ctx := newInventorySvcCtx(t, inventoryClient)
	ctx.ProductClient = productClient
	return ctx
}

func TestGetLogic_Success(t *testing.T) {
	logic := NewGetLogic(context.Background(), newInventorySvcCtxWithProduct(t, &stubInventoryClient{
		getSkuStockFn: func(context.Context, *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error) {
			return &inventoryrpc.GetSkuStockResponse{
				Code: common.CodeOK,
				Stock: &inventorypb.SkuStock{
					SkuId:          3001,
					TotalStock:     10,
					ReservedStock:  1,
					AvailableStock: 9,
					SaleableStock:  9,
				},
			}, nil
		},
	}, &stubProductClient{
		batchGetSkuFn: func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
			return &productrpc.BatchGetSKUResponse{Code: common.CodeOK, Skus: []*productpb.ProductSku{{Id: 3001, SpuId: 2001}}}, nil
		},
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{Code: common.CodeOK, Product: &productpb.Product{Id: 2001, CreatorId: 1}}, nil
		},
	}))

	data, bizErr := logic.Get(3001, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.AvailableStock != 9 {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestAdjustLogic_Success(t *testing.T) {
	logic := NewAdjustLogic(context.Background(), newInventorySvcCtxWithProduct(t, &stubInventoryClient{
		adjustStockFn: func(_ context.Context, req *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error) {
			if req.GetSkuId() != 3001 || req.GetTotalStock() != 20 {
				t.Fatalf("unexpected adjust request: %+v", req)
			}
			return &inventoryrpc.AdjustStockResponse{
				Code: common.CodeOK,
				Stock: &inventorypb.SkuStock{
					SkuId:          3001,
					TotalStock:     20,
					ReservedStock:  0,
					AvailableStock: 20,
					SaleableStock:  20,
				},
			}, nil
		},
	}, &stubProductClient{
		batchGetSkuFn: func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
			return &productrpc.BatchGetSKUResponse{Code: common.CodeOK, Skus: []*productpb.ProductSku{{Id: 3001, SpuId: 2001}}}, nil
		},
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{Code: common.CodeOK, Product: &productpb.Product{Id: 2001, CreatorId: 1}}, nil
		},
	}))

	data, bizErr := logic.Adjust(3001, &types.AdjustInventoryStockRequest{TotalStock: 20, Reason: "fix"}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.TotalStock != 20 {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestAdjustLogic_ForbiddenForOtherAdminProduct(t *testing.T) {
	logic := NewAdjustLogic(context.Background(), newInventorySvcCtxWithProduct(t, &stubInventoryClient{
		adjustStockFn: func(_ context.Context, _ *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error) {
			t.Fatal("adjust stock should not be called when ownership check fails")
			return nil, nil
		},
	}, &stubProductClient{
		batchGetSkuFn: func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
			return &productrpc.BatchGetSKUResponse{Code: common.CodeOK, Skus: []*productpb.ProductSku{{Id: 3001, SpuId: 2001}}}, nil
		},
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{Code: common.CodeOK, Product: &productpb.Product{Id: 2001, CreatorId: 2}}, nil
		},
	}))

	data, bizErr := logic.Adjust(3001, &types.AdjustInventoryStockRequest{TotalStock: 20, Reason: "fix"}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if data != nil {
		t.Fatalf("expected nil data, got %+v", data)
	}
	if bizErr != errOwnInventoryRequired {
		t.Fatalf("expected errOwnInventoryRequired, got %+v", bizErr)
	}
}

func TestGetLogic_SuperAdminCanReadAnyInventory(t *testing.T) {
	logic := NewGetLogic(context.Background(), newInventorySvcCtxWithProduct(t, &stubInventoryClient{
		getSkuStockFn: func(context.Context, *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error) {
			return &inventoryrpc.GetSkuStockResponse{
				Code: common.CodeOK,
				Stock: &inventorypb.SkuStock{
					SkuId:          3001,
					TotalStock:     10,
					ReservedStock:  1,
					AvailableStock: 9,
					SaleableStock:  9,
				},
			}, nil
		},
	}, &stubProductClient{
		batchGetSkuFn: func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
			t.Fatal("superadmin should bypass ownership lookup")
			return nil, nil
		},
	}))

	data, bizErr := logic.Get(3001, &middleware.AuthIdentity{UserID: 99, Role: authz.RoleSuperAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.SKUID != 3001 {
		t.Fatalf("unexpected data: %+v", data)
	}
}
