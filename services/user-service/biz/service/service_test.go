package service

import (
	"context"
	"errors"
	"testing"

	"meshcart/app/common"
	"meshcart/services/user-service/biz/errno"
	"meshcart/services/user-service/biz/repository"
	dalmodel "meshcart/services/user-service/dal/model"

	"github.com/bwmarrin/snowflake"
	"golang.org/x/crypto/bcrypt"
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

func TestRegister_InvalidParam(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		createFn:        func(ctx context.Context, user *dalmodel.User) error { return nil },
	})

	bizErr := svc.Register(context.Background(), " ", "")
	if bizErr != common.ErrInvalidParam {
		t.Fatalf("expected ErrInvalidParam, got %+v", bizErr)
	}
}

func TestRegister_PasswordIllegal(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		createFn:        func(ctx context.Context, user *dalmodel.User) error { return nil },
	})

	bizErr := svc.Register(context.Background(), "tester", "123")
	if bizErr != errno.ErrPasswordIllegal {
		t.Fatalf("expected ErrPasswordIllegal, got %+v", bizErr)
	}
}

func TestRegister_UserExists(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		createFn: func(ctx context.Context, user *dalmodel.User) error {
			return repository.ErrUserAlreadyExists
		},
	})

	bizErr := svc.Register(context.Background(), "tester", "123456")
	if bizErr != errno.ErrUserExists {
		t.Fatalf("expected ErrUserExists, got %+v", bizErr)
	}
}

func TestRegister_InternalError(t *testing.T) {
	svc := newTestUserService(t, &stubUserRepository{
		getByUsernameFn: func(ctx context.Context, username string) (*dalmodel.User, error) { return nil, nil },
		createFn: func(ctx context.Context, user *dalmodel.User) error {
			return errors.New("db failed")
		},
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
		createFn: func(ctx context.Context, user *dalmodel.User) error {
			created = user
			return nil
		},
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
	if created.Password == "123456" {
		t.Fatal("expected hashed password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(created.Password), []byte("123456")); err != nil {
		t.Fatalf("expected valid bcrypt password, got %v", err)
	}
}
