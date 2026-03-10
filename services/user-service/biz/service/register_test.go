package service

import (
	"context"
	"errors"
	"testing"

	"meshcart/app/common"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"
	dalmodel "meshcart/services/user-service/dal/model"

	"golang.org/x/crypto/bcrypt"
)

func TestRegister_InvalidParam(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		getByIDFn:       func(ctx context.Context, userID int64) (*dalmodel.User, error) { return nil, nil },
		countFn:         func(ctx context.Context) (int64, error) { return 0, nil },
		countByRoleFn:   func(ctx context.Context, role string) (int64, error) { return 0, nil },
		createFn:        func(ctx context.Context, user *dalmodel.User) error { return nil },
		updateRoleFn:    func(ctx context.Context, userID int64, role string) error { return nil },
	})

	bizErr := svc.Register(context.Background(), " ", "")
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestRegister_PasswordIllegal(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		getByIDFn:       func(ctx context.Context, userID int64) (*dalmodel.User, error) { return nil, nil },
		countFn:         func(ctx context.Context) (int64, error) { return 0, nil },
		countByRoleFn:   func(ctx context.Context, role string) (int64, error) { return 0, nil },
		createFn:        func(ctx context.Context, user *dalmodel.User) error { return nil },
		updateRoleFn:    func(ctx context.Context, userID int64, role string) error { return nil },
	})

	bizErr := svc.Register(context.Background(), "tester", "123")
	if bizErr != errno.ErrPasswordIllegal {
		t.Fatalf("expected ErrPasswordIllegal, got %+v", bizErr)
	}
}

func TestRegister_UserExists(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		getByIDFn:       func(ctx context.Context, userID int64) (*dalmodel.User, error) { return nil, nil },
		countFn:         func(ctx context.Context) (int64, error) { return 1, nil },
		countByRoleFn:   func(ctx context.Context, role string) (int64, error) { return 0, nil },
		createFn: func(ctx context.Context, user *dalmodel.User) error {
			return repository.ErrUserAlreadyExists
		},
		updateRoleFn: func(ctx context.Context, userID int64, role string) error { return nil },
	})

	bizErr := svc.Register(context.Background(), "tester", "123456")
	if bizErr != errno.ErrUserExists {
		t.Fatalf("expected ErrUserExists, got %+v", bizErr)
	}
}

func TestRegister_InternalError(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		getByIDFn:       func(ctx context.Context, userID int64) (*dalmodel.User, error) { return nil, nil },
		countFn:         func(ctx context.Context) (int64, error) { return 1, nil },
		countByRoleFn:   func(ctx context.Context, role string) (int64, error) { return 0, nil },
		createFn: func(ctx context.Context, user *dalmodel.User) error {
			return errors.New("db failed")
		},
		updateRoleFn: func(ctx context.Context, userID int64, role string) error { return nil },
	})

	bizErr := svc.Register(context.Background(), "tester", "123456")
	if bizErr != common.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %+v", bizErr)
	}
}

func TestRegister_Success(t *testing.T) {
	var created *dalmodel.User
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		getByIDFn:       func(ctx context.Context, userID int64) (*dalmodel.User, error) { return nil, nil },
		countFn:         func(ctx context.Context) (int64, error) { return 1, nil },
		countByRoleFn:   func(ctx context.Context, role string) (int64, error) { return 0, nil },
		createFn: func(ctx context.Context, user *dalmodel.User) error {
			created = user
			return nil
		},
		updateRoleFn: func(ctx context.Context, userID int64, role string) error { return nil },
	})

	bizErr := svc.Register(context.Background(), "tester", "123456")
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if created == nil {
		t.Fatal("expected created user")
	}
	if created.ID == 0 {
		t.Fatal("expected generated user id")
	}
	if created.Username != "tester" {
		t.Fatalf("expected username tester, got %s", created.Username)
	}
	if created.Role != "user" {
		t.Fatalf("expected role user, got %s", created.Role)
	}
	if created.Password == "123456" {
		t.Fatal("expected hashed password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(created.Password), []byte("123456")); err != nil {
		t.Fatalf("expected valid bcrypt password, got %v", err)
	}
}

func TestRegister_FirstUserBecomesSuperAdmin(t *testing.T) {
	var created *dalmodel.User
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		getByIDFn:       func(ctx context.Context, userID int64) (*dalmodel.User, error) { return nil, nil },
		countFn:         func(ctx context.Context) (int64, error) { return 0, nil },
		countByRoleFn:   func(ctx context.Context, role string) (int64, error) { return 0, nil },
		createFn: func(ctx context.Context, user *dalmodel.User) error {
			created = user
			return nil
		},
		updateRoleFn: func(ctx context.Context, userID int64, role string) error { return nil },
	})

	bizErr := svc.Register(context.Background(), "founder", "123456")
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if created == nil || created.Role != "superadmin" {
		t.Fatalf("expected first user to be superadmin, got %+v", created)
	}
}
