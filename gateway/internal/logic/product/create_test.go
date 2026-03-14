package product

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
	if s.changeStatusFn != nil {
		return s.changeStatusFn(ctx, req)
	}
	return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
}
func (s *stubProductClient) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
	return s.getProductDetailFn(ctx, req)
}
func (s *stubProductClient) ListProducts(ctx context.Context, req *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
	return s.listProductsFn(ctx, req)
}
func (s *stubProductClient) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
	if s.batchGetSkuFn != nil {
		return s.batchGetSkuFn(ctx, req)
	}
	return &productrpc.BatchGetSKUResponse{Code: common.CodeOK, Message: "成功"}, nil
}

type stubInventoryClient struct {
	getSkuStockFn        func(context.Context, *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error)
	batchGetSkuStockFn   func(context.Context, *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error)
	checkSaleableStockFn func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error)
	initSkuStocksFn      func(context.Context, *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	adjustStockFn        func(context.Context, *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error)
}

func (s *stubInventoryClient) GetSkuStock(ctx context.Context, req *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error) {
	return s.getSkuStockFn(ctx, req)
}
func (s *stubInventoryClient) BatchGetSkuStock(ctx context.Context, req *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error) {
	return s.batchGetSkuStockFn(ctx, req)
}
func (s *stubInventoryClient) CheckSaleableStock(ctx context.Context, req *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
	return s.checkSaleableStockFn(ctx, req)
}
func (s *stubInventoryClient) InitSkuStocks(ctx context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
	return s.initSkuStocksFn(ctx, req)
}
func (s *stubInventoryClient) AdjustStock(ctx context.Context, req *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error) {
	return s.adjustStockFn(ctx, req)
}

func newCreateProductSvcCtx(t *testing.T, productClient productrpc.Client, inventoryClient inventoryrpc.Client) *svc.ServiceContext {
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
		ProductClient:   productClient,
		InventoryClient: inventoryClient,
		AccessControl:   ac,
		JWT:             jwtMiddleware,
	}
}

func TestCreateLogic_InitStocksAfterCreate(t *testing.T) {
	initial := int64(12)
	logic := NewCreateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		createProductFn: func(context.Context, *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error) {
			return &productrpc.CreateProductResponse{
				Code:      common.CodeOK,
				Message:   "成功",
				ProductID: 2001,
				Skus: []*productpb.ProductSku{
					{Id: 3001, SkuCode: "blue-xl"},
				},
			}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksFn: func(_ context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if len(req.GetStocks()) != 1 || req.GetStocks()[0].GetSkuId() != 3001 || req.GetStocks()[0].GetTotalStock() != 12 {
				t.Fatalf("unexpected init stock request: %+v", req)
			}
			return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}))

	data, bizErr := logic.Create(&types.CreateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{SKUCode: "blue-xl", Title: "Blue XL", SalePrice: 100, Status: 1, InitialStock: &initial},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || len(data.SKUs) != 1 || data.SKUs[0].ID != 3001 {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestCreateLogic_DefaultInitialStockToZero(t *testing.T) {
	logic := NewCreateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		createProductFn: func(context.Context, *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error) {
			return &productrpc.CreateProductResponse{
				Code:      common.CodeOK,
				Message:   "成功",
				ProductID: 2002,
				Skus: []*productpb.ProductSku{
					{Id: 3002, SkuCode: "white-m"},
				},
			}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksFn: func(_ context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if len(req.GetStocks()) != 1 {
				t.Fatalf("expected one init stock item, got %+v", req.GetStocks())
			}
			if req.GetStocks()[0].GetSkuId() != 3002 || req.GetStocks()[0].GetTotalStock() != 0 {
				t.Fatalf("expected default zero stock, got %+v", req.GetStocks()[0])
			}
			return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}))

	data, bizErr := logic.Create(&types.CreateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{SKUCode: "white-m", Title: "White M", SalePrice: 100, Status: 1},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || len(data.SKUs) != 1 || data.SKUs[0].ID != 3002 {
		t.Fatalf("unexpected data: %+v", data)
	}
}
