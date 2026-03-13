package service

import (
	"context"
	"testing"

	"github.com/bwmarrin/snowflake"

	"meshcart/app/common"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	"meshcart/services/cart-service/biz/repository"
	dalmodel "meshcart/services/cart-service/dal/model"
)

type stubCartRepository struct {
	listByUserIDFn    func(context.Context, int64) ([]*dalmodel.CartItem, error)
	addOrAccumulateFn func(context.Context, *dalmodel.CartItem) (*dalmodel.CartItem, error)
	updateByIDFn      func(context.Context, int64, int64, int32, *bool) (*dalmodel.CartItem, error)
	deleteByIDFn      func(context.Context, int64, int64) error
	clearByUserIDFn   func(context.Context, int64) error
}

func (s *stubCartRepository) ListByUserID(ctx context.Context, userID int64) ([]*dalmodel.CartItem, error) {
	if s.listByUserIDFn != nil {
		return s.listByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (s *stubCartRepository) AddOrAccumulate(ctx context.Context, item *dalmodel.CartItem) (*dalmodel.CartItem, error) {
	if s.addOrAccumulateFn != nil {
		return s.addOrAccumulateFn(ctx, item)
	}
	return item, nil
}

func (s *stubCartRepository) UpdateByID(ctx context.Context, userID, itemID int64, quantity int32, checked *bool) (*dalmodel.CartItem, error) {
	if s.updateByIDFn != nil {
		return s.updateByIDFn(ctx, userID, itemID, quantity, checked)
	}
	return nil, nil
}

func (s *stubCartRepository) DeleteByID(ctx context.Context, userID, itemID int64) error {
	if s.deleteByIDFn != nil {
		return s.deleteByIDFn(ctx, userID, itemID)
	}
	return nil
}

func (s *stubCartRepository) ClearByUserID(ctx context.Context, userID int64) error {
	if s.clearByUserIDFn != nil {
		return s.clearByUserIDFn(ctx, userID)
	}
	return nil
}

func TestCartService_AddCartItem_InvalidParam(t *testing.T) {
	node, _ := snowflake.NewNode(1)
	svc := NewCartService(&stubCartRepository{}, node)

	item, bizErr := svc.AddCartItem(context.Background(), &cartpb.AddCartItemRequest{
		UserId:    101,
		ProductId: 2001,
		SkuId:     3001,
		Quantity:  1,
	})
	if item != nil {
		t.Fatalf("expected nil item, got %+v", item)
	}
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestCartService_UpdateCartItem_NotFound(t *testing.T) {
	node, _ := snowflake.NewNode(1)
	svc := NewCartService(&stubCartRepository{
		updateByIDFn: func(context.Context, int64, int64, int32, *bool) (*dalmodel.CartItem, error) {
			return nil, repository.ErrCartItemNotFound
		},
	}, node)

	item, bizErr := svc.UpdateCartItem(context.Background(), &cartpb.UpdateCartItemRequest{
		UserId:   101,
		ItemId:   1,
		Quantity: 2,
	})
	if item != nil {
		t.Fatalf("expected nil item, got %+v", item)
	}
	if bizErr == nil || bizErr.Code == common.CodeOK {
		t.Fatalf("expected business error, got %+v", bizErr)
	}
}
