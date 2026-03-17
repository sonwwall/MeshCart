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
	return s.createProductFn(ctx, req)
}
func (s *stubProductClient) CreateProductSaga(ctx context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error) {
	if s.createProductSagaFn != nil {
		return s.createProductSagaFn(ctx, req)
	}
	if s.createProductFn != nil {
		return s.createProductFn(ctx, &productpb.CreateProductRequest{
			Title: req.GetTitle(), SubTitle: req.GetSubTitle(), CategoryId: req.GetCategoryId(), Brand: req.GetBrand(), Description: req.GetDescription(), Status: req.GetTargetStatus(), Skus: req.GetSkus(), CreatorId: req.GetCreatorId(),
		})
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
	getSkuStockFn                 func(context.Context, *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error)
	batchGetSkuStockFn            func(context.Context, *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error)
	checkSaleableStockFn          func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error)
	initSkuStocksFn               func(context.Context, *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	initSkuStocksSagaFn           func(context.Context, *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	compensateInitSkuStocksSagaFn func(context.Context, *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventoryrpc.CompensateInitSkuStocksResponse, error)
	freezeSkuStocksFn             func(context.Context, *inventorypb.FreezeSkuStocksRequest) (*inventoryrpc.FreezeSkuStocksResponse, error)
	adjustStockFn                 func(context.Context, *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error)
	reserveSkuStocksFn            func(context.Context, *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error)
	releaseReservedSkuStocksFn    func(context.Context, *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error)
	confirmDeductReservedFn       func(context.Context, *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error)
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
func (s *stubInventoryClient) InitSkuStocksSaga(ctx context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
	if s.initSkuStocksSagaFn != nil {
		return s.initSkuStocksSagaFn(ctx, req)
	}
	if s.initSkuStocksFn != nil {
		return s.initSkuStocksFn(ctx, &inventorypb.InitSkuStocksRequest{Stocks: req.GetStocks()})
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
	return s.adjustStockFn(ctx, req)
}

func (s *stubInventoryClient) ReserveSkuStocks(ctx context.Context, req *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error) {
	if s.reserveSkuStocksFn != nil {
		return s.reserveSkuStocksFn(ctx, req)
	}
	return &inventoryrpc.ReserveSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubInventoryClient) ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error) {
	if s.releaseReservedSkuStocksFn != nil {
		return s.releaseReservedSkuStocksFn(ctx, req)
	}
	return &inventoryrpc.ReleaseReservedSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubInventoryClient) ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error) {
	if s.confirmDeductReservedFn != nil {
		return s.confirmDeductReservedFn(ctx, req)
	}
	return &inventoryrpc.ConfirmDeductReservedSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
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

func TestCreateLogic_AllowsEmptySKUCodeAndInitializesByOrder(t *testing.T) {
	initial := int64(8)
	logic := NewCreateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		createProductFn: func(_ context.Context, req *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error) {
			if len(req.GetSkus()) != 2 || req.GetSkus()[0].GetSkuCode() != "" || req.GetSkus()[1].GetSkuCode() != "" {
				t.Fatalf("expected empty sku_code values to be allowed, got %+v", req.GetSkus())
			}
			return &productrpc.CreateProductResponse{
				Code:      common.CodeOK,
				Message:   "成功",
				ProductID: 2003,
				Skus: []*productpb.ProductSku{
					{Id: 3003, SkuCode: ""},
					{Id: 3004, SkuCode: ""},
				},
			}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksFn: func(_ context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if len(req.GetStocks()) != 2 {
				t.Fatalf("expected two init stock items, got %+v", req.GetStocks())
			}
			if req.GetStocks()[0].GetSkuId() != 3003 || req.GetStocks()[0].GetTotalStock() != 0 {
				t.Fatalf("unexpected first init stock item: %+v", req.GetStocks()[0])
			}
			if req.GetStocks()[1].GetSkuId() != 3004 || req.GetStocks()[1].GetTotalStock() != 8 {
				t.Fatalf("unexpected second init stock item: %+v", req.GetStocks()[1])
			}
			return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}))

	data, bizErr := logic.Create(&types.CreateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{Title: "Blue M", SalePrice: 100, Status: 1},
			{Title: "Blue L", SalePrice: 120, Status: 1, InitialStock: &initial},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || len(data.SKUs) != 2 || data.SKUs[0].ID != 3003 || data.SKUs[1].ID != 3004 {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestCreateLogic_CompensatesProductWhenInventoryInitFails(t *testing.T) {
	compensated := false
	logic := NewCreateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		createProductSagaFn: func(_ context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error) {
			if req.GetBranchId() != sagaBranchIDProductCreate {
				t.Fatalf("unexpected product branch id: %s", req.GetBranchId())
			}
			return &productrpc.CreateProductResponse{
				Code:      common.CodeOK,
				Message:   "成功",
				ProductID: 2004,
				Skus:      []*productpb.ProductSku{{Id: 3005, SkuCode: "black-l"}},
			}, nil
		},
		compensateCreateProductSagaFn: func(_ context.Context, req *productpb.CompensateCreateProductSagaRequest) (*productrpc.ChangeProductStatusResponse, error) {
			compensated = true
			if req.GetProductId() != 2004 || req.GetBranchId() != sagaBranchIDProductCreate {
				t.Fatalf("unexpected compensate request: %+v", req)
			}
			return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksSagaFn: func(_ context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if req.GetBranchId() != sagaBranchIDInventoryInit {
				t.Fatalf("unexpected inventory branch id: %s", req.GetBranchId())
			}
			return &inventoryrpc.InitSkuStocksResponse{Code: common.ErrInternalError.Code, Message: "库存初始化失败"}, nil
		},
	}))

	data, bizErr := logic.Create(&types.CreateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{SKUCode: "black-l", Title: "Black L", SalePrice: 100, Status: 1},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if data != nil {
		t.Fatalf("expected nil data, got %+v", data)
	}
	if bizErr == nil {
		t.Fatal("expected inventory failure to be returned")
	}
	if !compensated {
		t.Fatal("expected product compensation to be triggered")
	}
}

func TestCreateLogic_OnlineProductPromotesAfterInventoryInit(t *testing.T) {
	statusChanged := false
	logic := NewCreateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		createProductSagaFn: func(_ context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error) {
			if req.GetTargetStatus() != productStatusOnline {
				t.Fatalf("expected target status online, got %d", req.GetTargetStatus())
			}
			return &productrpc.CreateProductResponse{
				Code:      common.CodeOK,
				Message:   "成功",
				ProductID: 2005,
				Skus:      []*productpb.ProductSku{{Id: 3006, SkuCode: "green-m"}},
			}, nil
		},
		changeStatusFn: func(_ context.Context, req *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error) {
			statusChanged = true
			if req.GetProductId() != 2005 || req.GetStatus() != productStatusOnline {
				t.Fatalf("unexpected change status request: %+v", req)
			}
			return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksSagaFn: func(_ context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if len(req.GetStocks()) != 1 || req.GetStocks()[0].GetSkuId() != 3006 {
				t.Fatalf("unexpected init saga request: %+v", req)
			}
			return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}))

	data, bizErr := logic.Create(&types.CreateProductRequest{
		Title:  "Tee",
		Status: productStatusOnline,
		SKUs: []types.ProductSkuInput{
			{SKUCode: "green-m", Title: "Green M", SalePrice: 100, Status: 1},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.ProductID != 2005 {
		t.Fatalf("unexpected data: %+v", data)
	}
	if !statusChanged {
		t.Fatal("expected online product to be promoted after inventory init")
	}
}
