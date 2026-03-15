package product

import (
	"context"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/types"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	productrpc "meshcart/gateway/rpc/product"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"
)

func TestUpdateLogic_InitStocksForNewSKUs(t *testing.T) {
	existingID := int64(3001)
	initial := int64(8)

	logic := NewUpdateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code: common.CodeOK,
				Product: &productpb.Product{
					Id:        2001,
					Status:    productStatusOffline,
					CreatorId: 1,
					Skus: []*productpb.ProductSku{
						{Id: existingID, SpuId: 2001, SkuCode: "existing"},
					},
				},
			}, nil
		},
		updateProductFn: func(_ context.Context, req *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error) {
			if len(req.GetSkus()) != 2 {
				t.Fatalf("expected two skus, got %+v", req.GetSkus())
			}
			return &productrpc.UpdateProductResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Skus: []*productpb.ProductSku{
					{Id: existingID, SkuCode: "existing", SpuId: 2001},
					{Id: 3002, SkuCode: "new", SpuId: 2001},
				},
			}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksFn: func(_ context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if len(req.GetStocks()) != 1 {
				t.Fatalf("expected one init stock item, got %+v", req.GetStocks())
			}
			if req.GetStocks()[0].GetSkuId() != 3002 || req.GetStocks()[0].GetTotalStock() != 8 {
				t.Fatalf("unexpected init stock item: %+v", req.GetStocks()[0])
			}
			return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}))

	bizErr := logic.Update(2001, &types.UpdateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{ID: &existingID, SKUCode: "existing", Title: "Existing", SalePrice: 100, Status: 1},
			{SKUCode: "new", Title: "New", SalePrice: 120, Status: 1, InitialStock: &initial},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
}

func TestUpdateLogic_DefaultsNewSKUInitialStockToZero(t *testing.T) {
	logic := NewUpdateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code: common.CodeOK,
				Product: &productpb.Product{
					Id:        2001,
					Status:    productStatusOffline,
					CreatorId: 1,
					Skus:      []*productpb.ProductSku{},
				},
			}, nil
		},
		updateProductFn: func(_ context.Context, req *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error) {
			return &productrpc.UpdateProductResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Skus: []*productpb.ProductSku{
					{Id: 3003, SkuCode: "", SpuId: 2001},
				},
			}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksFn: func(_ context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if len(req.GetStocks()) != 1 {
				t.Fatalf("expected one init stock item, got %+v", req.GetStocks())
			}
			if req.GetStocks()[0].GetSkuId() != 3003 || req.GetStocks()[0].GetTotalStock() != 0 {
				t.Fatalf("expected default zero stock, got %+v", req.GetStocks()[0])
			}
			return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}))

	bizErr := logic.Update(2001, &types.UpdateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{Title: "New", SalePrice: 120, Status: 1},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
}

func TestUpdateLogic_DoesNotInitStocksWhenNoNewSKU(t *testing.T) {
	existingID := int64(3001)
	logic := NewUpdateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code: common.CodeOK,
				Product: &productpb.Product{
					Id:        2001,
					Status:    productStatusOffline,
					CreatorId: 1,
					Skus: []*productpb.ProductSku{
						{Id: existingID, SpuId: 2001, SkuCode: "existing"},
					},
				},
			}, nil
		},
		updateProductFn: func(_ context.Context, req *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error) {
			return &productrpc.UpdateProductResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Skus: []*productpb.ProductSku{
					{Id: existingID, SkuCode: "existing", SpuId: 2001},
				},
			}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksFn: func(_ context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			t.Fatalf("init sku stocks should not be called, got %+v", req)
			return nil, nil
		},
	}))

	bizErr := logic.Update(2001, &types.UpdateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{ID: &existingID, SKUCode: "existing", Title: "Existing", SalePrice: 100, Status: 1},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
}

func TestUpdateLogic_FreezesDeletedSKUs(t *testing.T) {
	existingID := int64(3001)
	deletedID := int64(3002)

	logic := NewUpdateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code: common.CodeOK,
				Product: &productpb.Product{
					Id:        2001,
					Status:    productStatusOffline,
					CreatorId: 1,
					Skus: []*productpb.ProductSku{
						{Id: existingID, SpuId: 2001, SkuCode: "existing"},
						{Id: deletedID, SpuId: 2001, SkuCode: "deleted"},
					},
				},
			}, nil
		},
		updateProductFn: func(_ context.Context, req *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error) {
			if len(req.GetSkus()) != 1 || req.GetSkus()[0].GetId() != existingID {
				t.Fatalf("unexpected update skus: %+v", req.GetSkus())
			}
			return &productrpc.UpdateProductResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Skus: []*productpb.ProductSku{
					{Id: existingID, SkuCode: "existing", SpuId: 2001},
				},
			}, nil
		},
	}, &stubInventoryClient{
		initSkuStocksFn: func(_ context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			t.Fatalf("init sku stocks should not be called, got %+v", req)
			return nil, nil
		},
		freezeSkuStocksFn: func(_ context.Context, req *inventorypb.FreezeSkuStocksRequest) (*inventoryrpc.FreezeSkuStocksResponse, error) {
			if len(req.GetSkuIds()) != 1 || req.GetSkuIds()[0] != deletedID {
				t.Fatalf("unexpected freeze sku ids: %+v", req.GetSkuIds())
			}
			if req.GetOperatorId() != 1 || req.GetReason() != "product sku removed" {
				t.Fatalf("unexpected freeze request: %+v", req)
			}
			return &inventoryrpc.FreezeSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	}))

	bizErr := logic.Update(2001, &types.UpdateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{ID: &existingID, SKUCode: "existing", Title: "Existing", SalePrice: 100, Status: 1},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
}

func TestUpdateLogic_RejectsInitialStockForExistingSKU(t *testing.T) {
	existingID := int64(3001)
	initial := int64(5)
	logic := NewUpdateLogic(context.Background(), newCreateProductSvcCtx(t, &stubProductClient{
		getProductDetailFn: func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			t.Fatal("product detail should not be called when request is invalid")
			return nil, nil
		},
	}, &stubInventoryClient{}))

	bizErr := logic.Update(2001, &types.UpdateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{ID: &existingID, SKUCode: "existing", Title: "Existing", SalePrice: 100, Status: 1, InitialStock: &initial},
		},
	}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}
