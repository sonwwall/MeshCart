package service

import (
	"context"
	"testing"

	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"
	dalmodel "meshcart/services/user-service/dal/model"

	"golang.org/x/crypto/bcrypt"
)

func TestLogin_UserNotFound(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) {
			return nil, repository.ErrUserNotFound
		},
		createFn: func(ctx context.Context, user *dalmodel.User) error { return nil },
	})

	result, bizErr := svc.Login(context.Background(), "tester", "123456")
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if bizErr != errno.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %+v", bizErr)
	}
}

func TestLogin_UserLocked(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) {
			return &dalmodel.User{ID: 1, Username: "tester", Password: string(hashedPassword), IsLocked: true}, nil
		},
		createFn: func(ctx context.Context, user *dalmodel.User) error { return nil },
	})

	result, bizErr := svc.Login(context.Background(), "tester", "123456")
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if bizErr != errno.ErrUserLocked {
		t.Fatalf("expected ErrUserLocked, got %+v", bizErr)
	}
}

func TestLogin_PasswordInvalid(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) {
			return &dalmodel.User{ID: 1, Username: "tester", Password: string(hashedPassword)}, nil
		},
		createFn: func(ctx context.Context, user *dalmodel.User) error { return nil },
	})

	result, bizErr := svc.Login(context.Background(), "tester", "wrong")
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if bizErr != errno.ErrPasswordInvalid {
		t.Fatalf("expected ErrPasswordInvalid, got %+v", bizErr)
	}
}

func TestLogin_Success(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) {
			return &dalmodel.User{ID: 99, Username: "tester", Password: string(hashedPassword)}, nil
		},
		createFn: func(ctx context.Context, user *dalmodel.User) error { return nil },
	})

	result, bizErr := svc.Login(context.Background(), "tester", "123456")
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if result == nil || result.UserID != 99 || result.Username != "tester" {
		t.Fatalf("unexpected login result: %+v", result)
	}
}
