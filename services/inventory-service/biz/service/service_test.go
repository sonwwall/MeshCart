package service

import (
	"context"
	"testing"

	"meshcart/app/common"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	"meshcart/services/inventory-service/biz/errno"
	"meshcart/services/inventory-service/biz/repository"
	dalmodel "meshcart/services/inventory-service/dal/model"
)

type stubInventoryRepository struct {
	getBySKUIDFn   func(context.Context, int64) (*dalmodel.InventoryStock, error)
	listBySKUIDsFn func(context.Context, []int64) ([]*dalmodel.InventoryStock, error)
	createBatchFn  func(context.Context, []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error)
	adjustStockFn  func(context.Context, int64, int64) (*dalmodel.InventoryStock, error)
}

func (s *stubInventoryRepository) GetBySKUID(ctx context.Context, skuID int64) (*dalmodel.InventoryStock, error) {
	if s.getBySKUIDFn != nil {
		return s.getBySKUIDFn(ctx, skuID)
	}
	return nil, repository.ErrStockNotFound
}

func (s *stubInventoryRepository) ListBySKUIDs(ctx context.Context, skuIDs []int64) ([]*dalmodel.InventoryStock, error) {
	if s.listBySKUIDsFn != nil {
		return s.listBySKUIDsFn(ctx, skuIDs)
	}
	return []*dalmodel.InventoryStock{}, nil
}

func (s *stubInventoryRepository) CreateBatch(ctx context.Context, stocks []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error) {
	if s.createBatchFn != nil {
		return s.createBatchFn(ctx, stocks)
	}
	return stocks, nil
}

func (s *stubInventoryRepository) AdjustTotalStock(ctx context.Context, skuID int64, totalStock int64) (*dalmodel.InventoryStock, error) {
	if s.adjustStockFn != nil {
		return s.adjustStockFn(ctx, skuID, totalStock)
	}
	return nil, repository.ErrStockNotFound
}

func TestInventoryService_GetSkuStock_NotFound(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{})

	stock, bizErr := svc.GetSkuStock(context.Background(), 3001)
	if stock != nil {
		t.Fatalf("expected nil stock, got %+v", stock)
	}
	if bizErr != errno.ErrInventoryStockNotFound {
		t.Fatalf("expected ErrInventoryStockNotFound, got %+v", bizErr)
	}
}

func TestInventoryService_BatchGetSkuStock_Success(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		listBySKUIDsFn: func(context.Context, []int64) ([]*dalmodel.InventoryStock, error) {
			return []*dalmodel.InventoryStock{
				{SKUID: 3001, TotalStock: 100, ReservedStock: 5, AvailableStock: 95},
			}, nil
		},
	})

	stocks, bizErr := svc.BatchGetSkuStock(context.Background(), []int64{3001})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if len(stocks) != 1 || stocks[0].GetSaleableStock() != 95 {
		t.Fatalf("unexpected stocks: %+v", stocks)
	}
}

func TestInventoryService_BatchGetSkuStock_InvalidParam(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{})

	stocks, bizErr := svc.BatchGetSkuStock(context.Background(), []int64{3001, 0})
	if stocks != nil {
		t.Fatalf("expected nil stocks, got %+v", stocks)
	}
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestInventoryService_BatchGetSkuStock_RepositoryError(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		listBySKUIDsFn: func(context.Context, []int64) ([]*dalmodel.InventoryStock, error) {
			return nil, context.DeadlineExceeded
		},
	})

	stocks, bizErr := svc.BatchGetSkuStock(context.Background(), []int64{3001})
	if stocks != nil {
		t.Fatalf("expected nil stocks, got %+v", stocks)
	}
	if bizErr != common.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %+v", bizErr)
	}
}

func TestInventoryService_CheckSaleableStock_InvalidParam(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{})

	saleable, available, bizErr := svc.CheckSaleableStock(context.Background(), &inventorypb.CheckSaleableStockRequest{
		SkuId:    3001,
		Quantity: 0,
	})
	if saleable || available != 0 {
		t.Fatalf("unexpected result saleable=%v available=%d", saleable, available)
	}
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestInventoryService_CheckSaleableStock_Insufficient(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		getBySKUIDFn: func(context.Context, int64) (*dalmodel.InventoryStock, error) {
			return &dalmodel.InventoryStock{SKUID: 3001, AvailableStock: 1}, nil
		},
	})

	saleable, available, bizErr := svc.CheckSaleableStock(context.Background(), &inventorypb.CheckSaleableStockRequest{
		SkuId:    3001,
		Quantity: 2,
	})
	if saleable || available != 1 {
		t.Fatalf("unexpected result saleable=%v available=%d", saleable, available)
	}
	if bizErr != errno.ErrInsufficientStock {
		t.Fatalf("expected ErrInsufficientStock, got %+v", bizErr)
	}
}

func TestInventoryService_CheckSaleableStock_NotFoundTreatsAsInsufficient(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{})

	saleable, available, bizErr := svc.CheckSaleableStock(context.Background(), &inventorypb.CheckSaleableStockRequest{
		SkuId:    3001,
		Quantity: 2,
	})
	if saleable || available != 0 {
		t.Fatalf("unexpected result saleable=%v available=%d", saleable, available)
	}
	if bizErr != errno.ErrInsufficientStock {
		t.Fatalf("expected ErrInsufficientStock, got %+v", bizErr)
	}
}

func TestInventoryService_CheckSaleableStock_RepositoryError(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		getBySKUIDFn: func(context.Context, int64) (*dalmodel.InventoryStock, error) {
			return nil, context.DeadlineExceeded
		},
	})

	saleable, available, bizErr := svc.CheckSaleableStock(context.Background(), &inventorypb.CheckSaleableStockRequest{
		SkuId:    3001,
		Quantity: 2,
	})
	if saleable || available != 0 {
		t.Fatalf("unexpected result saleable=%v available=%d", saleable, available)
	}
	if bizErr != common.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %+v", bizErr)
	}
}

func TestInventoryService_CheckSaleableStock_Success(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		getBySKUIDFn: func(context.Context, int64) (*dalmodel.InventoryStock, error) {
			return &dalmodel.InventoryStock{SKUID: 3001, AvailableStock: 8}, nil
		},
	})

	saleable, available, bizErr := svc.CheckSaleableStock(context.Background(), &inventorypb.CheckSaleableStockRequest{
		SkuId:    3001,
		Quantity: 2,
	})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if !saleable || available != 8 {
		t.Fatalf("unexpected result saleable=%v available=%d", saleable, available)
	}
}

func TestInventoryService_InitSkuStocks_Success(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		createBatchFn: func(_ context.Context, stocks []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error) {
			if len(stocks) != 1 || stocks[0].SKUID != 3001 || stocks[0].AvailableStock != 20 {
				t.Fatalf("unexpected init stocks: %+v", stocks)
			}
			return stocks, nil
		},
	})

	stocks, bizErr := svc.InitSkuStocks(context.Background(), &inventorypb.InitSkuStocksRequest{
		Stocks: []*inventorypb.InitSkuStockItem{{SkuId: 3001, TotalStock: 20}},
	})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if len(stocks) != 1 || stocks[0].GetTotalStock() != 20 {
		t.Fatalf("unexpected response: %+v", stocks)
	}
}

func TestInventoryService_InitSkuStocks_InvalidParam(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{})

	stocks, bizErr := svc.InitSkuStocks(context.Background(), &inventorypb.InitSkuStocksRequest{
		Stocks: []*inventorypb.InitSkuStockItem{{SkuId: 3001, TotalStock: -1}},
	})
	if stocks != nil {
		t.Fatalf("expected nil stocks, got %+v", stocks)
	}
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestInventoryService_InitSkuStocks_RepositoryConflict(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		createBatchFn: func(context.Context, []*dalmodel.InventoryStock) ([]*dalmodel.InventoryStock, error) {
			return nil, repository.ErrStockExists
		},
	})

	stocks, bizErr := svc.InitSkuStocks(context.Background(), &inventorypb.InitSkuStocksRequest{
		Stocks: []*inventorypb.InitSkuStockItem{{SkuId: 3001, TotalStock: 20}},
	})
	if stocks != nil {
		t.Fatalf("expected nil stocks, got %+v", stocks)
	}
	if bizErr != errno.ErrStockAlreadyExists {
		t.Fatalf("expected ErrStockAlreadyExists, got %+v", bizErr)
	}
}

func TestInventoryService_AdjustStock_Success(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		adjustStockFn: func(context.Context, int64, int64) (*dalmodel.InventoryStock, error) {
			return &dalmodel.InventoryStock{SKUID: 3001, TotalStock: 50, AvailableStock: 45, ReservedStock: 5}, nil
		},
	})

	stock, bizErr := svc.AdjustStock(context.Background(), &inventorypb.AdjustStockRequest{SkuId: 3001, TotalStock: 50})
	if bizErr != nil {
		t.Fatalf("expected nil error, got %+v", bizErr)
	}
	if stock == nil || stock.GetAvailableStock() != 45 {
		t.Fatalf("unexpected stock: %+v", stock)
	}
}

func TestInventoryService_AdjustStock_InvalidParam(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{})

	stock, bizErr := svc.AdjustStock(context.Background(), &inventorypb.AdjustStockRequest{SkuId: 3001, TotalStock: -1})
	if stock != nil {
		t.Fatalf("expected nil stock, got %+v", stock)
	}
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestInventoryService_AdjustStock_InvalidQuantityFromRepository(t *testing.T) {
	svc := NewInventoryService(&stubInventoryRepository{
		adjustStockFn: func(context.Context, int64, int64) (*dalmodel.InventoryStock, error) {
			return nil, repository.ErrInvalidQuantity
		},
	})

	stock, bizErr := svc.AdjustStock(context.Background(), &inventorypb.AdjustStockRequest{SkuId: 3001, TotalStock: 1})
	if stock != nil {
		t.Fatalf("expected nil stock, got %+v", stock)
	}
	if bizErr != errno.ErrInvalidStockQuantity {
		t.Fatalf("expected ErrInvalidStockQuantity, got %+v", bizErr)
	}
}
