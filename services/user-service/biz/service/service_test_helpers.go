package service

import (
	"context"
	"testing"

	"meshcart/services/user-service/biz/repository"
	dalmodel "meshcart/services/user-service/dal/model"

	"github.com/bwmarrin/snowflake"
)

type stubUserRepository struct {
	getByUsernameFn func(ctx context.Context, username string) (*dalmodel.User, error)
	createFn        func(ctx context.Context, user *dalmodel.User) error
}

func (s *stubUserRepository) GetByUsername(ctx context.Context, username string) (*dalmodel.User, error) {
	return s.getByUsernameFn(ctx, username)
}

func (s *stubUserRepository) Create(ctx context.Context, user *dalmodel.User) error {
	return s.createFn(ctx, user)
}

func newTestUserService(t *testing.T, repo repository.UserRepository) *UserService {
	t.Helper()
	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("init snowflake node: %v", err)
	}
	return NewUserService(repo, node)
}
