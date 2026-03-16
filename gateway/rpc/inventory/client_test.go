package inventory

import (
	"context"
	"errors"
	"testing"

	callopt "github.com/cloudwego/kitex/client/callopt"
	basepb "meshcart/kitex_gen/meshcart/base"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	inventoryservice "meshcart/kitex_gen/meshcart/inventory/inventoryservice"
)

type stubKitexInventoryClient struct {
	getSkuStockFn                 func(context.Context, *inventorypb.GetSkuStockRequest) (*inventorypb.GetSkuStockResponse, error)
	batchGetSkuStockFn            func(context.Context, *inventorypb.BatchGetSkuStockRequest) (*inventorypb.BatchGetSkuStockResponse, error)
	checkSaleableFn               func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventorypb.CheckSaleableStockResponse, error)
	initSkuStocksFn               func(context.Context, *inventorypb.InitSkuStocksRequest) (*inventorypb.InitSkuStocksResponse, error)
	initSkuStocksSagaFn           func(context.Context, *inventorypb.InitSkuStocksSagaRequest) (*inventorypb.InitSkuStocksResponse, error)
	compensateInitSkuStocksSagaFn func(context.Context, *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventorypb.CompensateInitSkuStocksSagaResponse, error)
	freezeSkuStocksFn             func(context.Context, *inventorypb.FreezeSkuStocksRequest) (*inventorypb.FreezeSkuStocksResponse, error)
	adjustStockFn                 func(context.Context, *inventorypb.AdjustStockRequest) (*inventorypb.AdjustStockResponse, error)
}

var _ inventoryservice.Client = (*stubKitexInventoryClient)(nil)

func (s *stubKitexInventoryClient) GetSkuStock(ctx context.Context, request *inventorypb.GetSkuStockRequest, _ ...callopt.Option) (*inventorypb.GetSkuStockResponse, error) {
	return s.getSkuStockFn(ctx, request)
}

func (s *stubKitexInventoryClient) BatchGetSkuStock(ctx context.Context, request *inventorypb.BatchGetSkuStockRequest, _ ...callopt.Option) (*inventorypb.BatchGetSkuStockResponse, error) {
	return s.batchGetSkuStockFn(ctx, request)
}

func (s *stubKitexInventoryClient) CheckSaleableStock(ctx context.Context, request *inventorypb.CheckSaleableStockRequest, _ ...callopt.Option) (*inventorypb.CheckSaleableStockResponse, error) {
	return s.checkSaleableFn(ctx, request)
}

func (s *stubKitexInventoryClient) InitSkuStocks(ctx context.Context, request *inventorypb.InitSkuStocksRequest, _ ...callopt.Option) (*inventorypb.InitSkuStocksResponse, error) {
	return s.initSkuStocksFn(ctx, request)
}

func (s *stubKitexInventoryClient) InitSkuStocksSaga(ctx context.Context, request *inventorypb.InitSkuStocksSagaRequest, _ ...callopt.Option) (*inventorypb.InitSkuStocksResponse, error) {
	return s.initSkuStocksSagaFn(ctx, request)
}

func (s *stubKitexInventoryClient) CompensateInitSkuStocksSaga(ctx context.Context, request *inventorypb.CompensateInitSkuStocksSagaRequest, _ ...callopt.Option) (*inventorypb.CompensateInitSkuStocksSagaResponse, error) {
	return s.compensateInitSkuStocksSagaFn(ctx, request)
}

func (s *stubKitexInventoryClient) FreezeSkuStocks(ctx context.Context, request *inventorypb.FreezeSkuStocksRequest, _ ...callopt.Option) (*inventorypb.FreezeSkuStocksResponse, error) {
	return s.freezeSkuStocksFn(ctx, request)
}

func (s *stubKitexInventoryClient) AdjustStock(ctx context.Context, request *inventorypb.AdjustStockRequest, _ ...callopt.Option) (*inventorypb.AdjustStockResponse, error) {
	return s.adjustStockFn(ctx, request)
}

func TestClient_InitSkuStocks_NilResponse(t *testing.T) {
	c := &kitexClient{cli: &stubKitexInventoryClient{
		initSkuStocksFn: func(context.Context, *inventorypb.InitSkuStocksRequest) (*inventorypb.InitSkuStocksResponse, error) {
			return nil, nil
		},
	}}

	resp, err := c.InitSkuStocks(context.Background(), &inventorypb.InitSkuStocksRequest{})
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !errors.Is(err, errNilInitSkuStocksResponse) {
		t.Fatalf("expected errNilInitSkuStocksResponse, got %v", err)
	}
}

func TestClient_AdjustStock_NilResponse(t *testing.T) {
	c := &kitexClient{cli: &stubKitexInventoryClient{
		adjustStockFn: func(context.Context, *inventorypb.AdjustStockRequest) (*inventorypb.AdjustStockResponse, error) {
			return nil, nil
		},
	}}

	resp, err := c.AdjustStock(context.Background(), &inventorypb.AdjustStockRequest{})
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !errors.Is(err, errNilAdjustStockResponse) {
		t.Fatalf("expected errNilAdjustStockResponse, got %v", err)
	}
}

func TestClient_FreezeSkuStocks_NilResponse(t *testing.T) {
	c := &kitexClient{cli: &stubKitexInventoryClient{
		freezeSkuStocksFn: func(context.Context, *inventorypb.FreezeSkuStocksRequest) (*inventorypb.FreezeSkuStocksResponse, error) {
			return nil, nil
		},
	}}

	resp, err := c.FreezeSkuStocks(context.Background(), &inventorypb.FreezeSkuStocksRequest{})
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !errors.Is(err, errNilFreezeSkuStocksResponse) {
		t.Fatalf("expected errNilFreezeSkuStocksResponse, got %v", err)
	}
}

func TestClient_CheckSaleableStock_MapsBaseResponse(t *testing.T) {
	c := &kitexClient{cli: &stubKitexInventoryClient{
		checkSaleableFn: func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventorypb.CheckSaleableStockResponse, error) {
			return &inventorypb.CheckSaleableStockResponse{
				Base:           &basepb.BaseResponse{Code: 123, Message: "库存不足"},
				Saleable:       false,
				AvailableStock: 1,
			}, nil
		},
	}}

	resp, err := c.CheckSaleableStock(context.Background(), &inventorypb.CheckSaleableStockRequest{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Code != 123 || resp.Message != "库存不足" || resp.AvailableStock != 1 || resp.Saleable {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
