package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	"meshcart/services/user-service/biz/dto"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"
	dalmodel "meshcart/services/user-service/dal/model"

	"github.com/bwmarrin/snowflake"
	"golang.org/x/crypto/bcrypt"
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

func (s *UserService) Login(ctx context.Context, username, password string) (*dto.LoginResult, *common.BizError) {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return nil, errno.ErrUserNotFound
		}
		return nil, common.ErrInternalError
	}

	if user.IsLocked {
		return nil, errno.ErrUserLocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errno.ErrPasswordInvalid
	}

	return &dto.LoginResult{
		UserID:   user.ID,
		Username: user.Username,
	}, nil
}

func (s *UserService) Register(ctx context.Context, username, password string) *common.BizError {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return common.ErrInvalidParam
	}
	if len(password) < 6 {
		return errno.ErrPasswordIllegal
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return common.ErrInternalError
	}

	newUser := &dalmodel.User{
		ID:       s.node.Generate().Int64(),
		Username: username,
		Password: string(hashedPassword),
	}
	if err := s.repo.Create(ctx, newUser); err != nil {
		if err == repository.ErrUserAlreadyExists {
			return errno.ErrUserExists
		}
		return common.ErrInternalError
	}

	return nil
}
