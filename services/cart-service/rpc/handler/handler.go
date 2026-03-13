package handler

import "meshcart/services/cart-service/biz/service"

type CartServiceImpl struct {
	svc *service.CartService
}

func NewCartServiceImpl(svc *service.CartService) *CartServiceImpl {
	return &CartServiceImpl{svc: svc}
}
