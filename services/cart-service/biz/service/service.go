package service

import (
	"github.com/bwmarrin/snowflake"

	"meshcart/services/cart-service/biz/repository"
)

type CartService struct {
	repo repository.CartRepository
	node *snowflake.Node
}

func NewCartService(repo repository.CartRepository, node *snowflake.Node) *CartService {
	return &CartService{repo: repo, node: node}
}
