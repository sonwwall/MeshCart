package service

import (
	"meshcart/services/user-service/biz/repository"

	"github.com/bwmarrin/snowflake"
)

type UserService struct {
	repo repository.UserRepository
	node *snowflake.Node
}

func NewUserService(repo repository.UserRepository, node *snowflake.Node) *UserService {
	return &UserService{
		repo: repo,
		node: node,
	}
}
