package tx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"meshcart/app/common"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/types"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	productrpc "meshcart/gateway/rpc/product"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"
)

type stubProductWorkflowClient struct {
	createFn     func(context.Context, *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error)
	compensateFn func(context.Context, *productpb.CompensateCreateProductSagaRequest) (*productrpc.ChangeProductStatusResponse, error)
	changeFn     func(context.Context, *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error)
}

func (s *stubProductWorkflowClient) CreateProductSaga(ctx context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error) {
	return s.createFn(ctx, req)
}

func (s *stubProductWorkflowClient) CompensateCreateProductSaga(ctx context.Context, req *productpb.CompensateCreateProductSagaRequest) (*productrpc.ChangeProductStatusResponse, error) {
	if s.compensateFn != nil {
		return s.compensateFn(ctx, req)
	}
	return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func (s *stubProductWorkflowClient) ChangeProductStatus(ctx context.Context, req *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error) {
	if s.changeFn != nil {
		return s.changeFn(ctx, req)
	}
	return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
}

type stubInventoryWorkflowClient struct {
	initFn       func(context.Context, *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	compensateFn func(context.Context, *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventoryrpc.CompensateInitSkuStocksResponse, error)
}

func (s *stubInventoryWorkflowClient) InitSkuStocksSaga(ctx context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
	return s.initFn(ctx, req)
}

func (s *stubInventoryWorkflowClient) CompensateInitSkuStocksSaga(ctx context.Context, req *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventoryrpc.CompensateInitSkuStocksResponse, error) {
	if s.compensateFn != nil {
		return s.compensateFn(ctx, req)
	}
	return &inventoryrpc.CompensateInitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
}

func TestDTMProductCreateCoordinator_CreateProduct(t *testing.T) {
	dtmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/prepareWorkflow":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"transaction":{"gid":"gid","status":""},"progresses":[]}`))
		case "/registerBranch":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case "/submit":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer dtmServer.Close()

	coordinator := NewProductCreateCoordinator(config.DTMConfig{
		Server:              dtmServer.URL,
		WorkflowCallbackURL: "http://127.0.0.1:8080/api/internal/dtm/workflow",
	}, &stubProductWorkflowClient{
		createFn: func(_ context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error) {
			if req.GetGlobalTxId() == "" || req.GetBranchId() == "" {
				t.Fatalf("expected gid and branch id, got %+v", req)
			}
			return &productrpc.CreateProductResponse{
				Code:      common.CodeOK,
				Message:   "成功",
				ProductID: 2001,
				Skus:      []*productpb.ProductSku{{Id: 3001, SkuCode: "blue-xl"}},
			}, nil
		},
	}, &stubInventoryWorkflowClient{
		initFn: func(_ context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if len(req.GetStocks()) != 1 || req.GetStocks()[0].GetSkuId() != 3001 {
				t.Fatalf("unexpected inventory init request: %+v", req)
			}
			return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
	})

	data, bizErr := coordinator.CreateProduct(context.Background(), &types.CreateProductRequest{
		Title:  "Tee",
		Status: productStatusOffline,
		SKUs: []types.ProductSkuInput{
			{SKUCode: "blue-xl", Title: "Blue XL", SalePrice: 100, Status: 1},
		},
	}, 1)
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if data == nil || data.ProductID != 2001 || len(data.SKUs) != 1 || data.SKUs[0].ID != 3001 {
		t.Fatalf("unexpected data: %+v", data)
	}
}
