package handler

import "meshcart/services/product-service/biz/service"

type ProductServiceImpl struct {
	svc *service.ProductService
}

func NewProductServiceImpl(svc *service.ProductService) *ProductServiceImpl {
	return &ProductServiceImpl{svc: svc}
}
