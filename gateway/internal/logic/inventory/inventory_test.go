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
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
)

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

func TestGetLogic_Success(t *testing.T) {
	logic := NewGetLogic(context.Background(), newInventorySvcCtx(t, &stubInventoryClient{
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
	logic := NewAdjustLogic(context.Background(), newInventorySvcCtx(t, &stubInventoryClient{
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
	}))

	data, bizErr := logic.Adjust(3001, &types.AdjustInventoryStockRequest{TotalStock: 20, Reason: "fix"}, &middleware.AuthIdentity{UserID: 1, Role: authz.RoleAdmin})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if data == nil || data.TotalStock != 20 {
		t.Fatalf("unexpected data: %+v", data)
	}
}
