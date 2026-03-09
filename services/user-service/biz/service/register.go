package service

import (
	"context"
	"strings"

	"meshcart/app/common"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"
	dalmodel "meshcart/services/user-service/dal/model"

	"golang.org/x/crypto/bcrypt"
)

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
