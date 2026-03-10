package service

import (
	"github.com/bwmarrin/snowflake"

	"meshcart/services/product-service/biz/repository"
)

const (
	ProductStatusDraft   int32 = 0
	ProductStatusOffline int32 = 1
	ProductStatusOnline  int32 = 2

	SKUStatusInactive int32 = 0
	SKUStatusActive   int32 = 1
)

type ProductService struct {
	repo repository.ProductRepository
	node *snowflake.Node
}

func NewProductService(repo repository.ProductRepository, node *snowflake.Node) *ProductService {
	return &ProductService{
		repo: repo,
		node: node,
	}
}

func isValidProductStatus(status int32) bool {
	return status == ProductStatusDraft || status == ProductStatusOffline || status == ProductStatusOnline
}

func isValidSKUStatus(status int32) bool {
	return status == SKUStatusInactive || status == SKUStatusActive
}
