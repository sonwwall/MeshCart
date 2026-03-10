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
	getByIDFn       func(ctx context.Context, userID int64) (*dalmodel.User, error)
	countFn         func(ctx context.Context) (int64, error)
	countByRoleFn   func(ctx context.Context, role string) (int64, error)
	createFn        func(ctx context.Context, user *dalmodel.User) error
	updateRoleFn    func(ctx context.Context, userID int64, role string) error
}

func (s *stubUserRepository) GetByUsername(ctx context.Context, username string) (*dalmodel.User, error) {
	return s.getByUsernameFn(ctx, username)
}

func (s *stubUserRepository) GetByID(ctx context.Context, userID int64) (*dalmodel.User, error) {
	return s.getByIDFn(ctx, userID)
}

func (s *stubUserRepository) Count(ctx context.Context) (int64, error) {
	return s.countFn(ctx)
}

func (s *stubUserRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	return s.countByRoleFn(ctx, role)
}

func (s *stubUserRepository) Create(ctx context.Context, user *dalmodel.User) error {
	return s.createFn(ctx, user)
}

func (s *stubUserRepository) UpdateRole(ctx context.Context, userID int64, role string) error {
	return s.updateRoleFn(ctx, userID, role)
}

func newTestUserService(t *testing.T, repo repository.UserRepository) *UserService {
	t.Helper()
	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("init snowflake node: %v", err)
	}
	return NewUserService(repo, node)
}
