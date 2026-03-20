package user

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"meshcart/app/common"
	"meshcart/gateway/internal/auth"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/types"
	userrpc "meshcart/gateway/rpc/user"
)

func TestRefreshToken_SuccessRotatesSession(t *testing.T) {
	svcCtx := newTestServiceContext(t, &stubUserClient{
		getUserFn: func(ctx context.Context, req *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error) {
			return &userrpc.GetUserResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   req.UserID,
				Username: "tester",
				Role:     "admin",
			}, nil
		},
	})

	refreshToken, err := auth.NewRefreshToken()
	if err != nil {
		t.Fatalf("new refresh token: %v", err)
	}
	now := time.Now()
	err = svcCtx.SessionStore.Save(context.Background(), &auth.Session{
		SessionID:        "session-1",
		UserID:           100,
		Username:         "tester",
		Role:             "user",
		RefreshTokenHash: auth.HashRefreshToken(refreshToken),
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		t.Fatalf("save session: %v", err)
	}

	logic := NewRefreshTokenLogic(context.Background(), svcCtx)
	data, bizErr := logic.Refresh(&types.UserRefreshTokenRequest{RefreshToken: refreshToken})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if data == nil {
		t.Fatal("expected refresh data")
	}
	if data.SessionID != "session-1" {
		t.Fatalf("expected session-1, got %s", data.SessionID)
	}
	if data.RefreshToken == "" || data.RefreshToken == refreshToken {
		t.Fatalf("expected rotated refresh token, got %q", data.RefreshToken)
	}

	_, err = svcCtx.SessionStore.GetByRefreshTokenHash(context.Background(), auth.HashRefreshToken(refreshToken))
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("expected old refresh token invalidated, got %v", err)
	}

	session, err := svcCtx.SessionStore.GetByRefreshTokenHash(context.Background(), auth.HashRefreshToken(data.RefreshToken))
	if err != nil {
		t.Fatalf("load rotated session: %v", err)
	}
	if session.Role != "admin" {
		t.Fatalf("expected role refreshed to admin, got %s", session.Role)
	}
}

func TestRefreshToken_ConcurrentOnlyOneSucceeds(t *testing.T) {
	svcCtx := newTestServiceContext(t, &stubUserClient{
		getUserFn: func(ctx context.Context, req *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error) {
			return &userrpc.GetUserResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   req.UserID,
				Username: "tester",
				Role:     "user",
			}, nil
		},
	})

	refreshToken, err := auth.NewRefreshToken()
	if err != nil {
		t.Fatalf("new refresh token: %v", err)
	}
	now := time.Now()
	err = svcCtx.SessionStore.Save(context.Background(), &auth.Session{
		SessionID:        "session-2",
		UserID:           101,
		Username:         "tester",
		Role:             "user",
		RefreshTokenHash: auth.HashRefreshToken(refreshToken),
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		t.Fatalf("save session: %v", err)
	}

	logic := NewRefreshTokenLogic(context.Background(), svcCtx)
	var wg sync.WaitGroup
	results := make(chan *common.BizError, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, bizErr := logic.Refresh(&types.UserRefreshTokenRequest{RefreshToken: refreshToken})
			results <- bizErr
		}()
	}
	wg.Wait()
	close(results)

	successCount := 0
	unauthorizedCount := 0
	for bizErr := range results {
		if bizErr == nil {
			successCount++
			continue
		}
		if bizErr == common.ErrUnauthorized {
			unauthorizedCount++
			continue
		}
		t.Fatalf("unexpected refresh error: %+v", bizErr)
	}

	if successCount != 1 || unauthorizedCount != 1 {
		t.Fatalf("expected 1 success and 1 unauthorized, got success=%d unauthorized=%d", successCount, unauthorizedCount)
	}
}

func TestLogout_DeletesSession(t *testing.T) {
	svcCtx := newTestServiceContext(t, &stubUserClient{})
	now := time.Now()
	err := svcCtx.SessionStore.Save(context.Background(), &auth.Session{
		SessionID:        "session-logout",
		UserID:           102,
		Username:         "tester",
		Role:             "user",
		RefreshTokenHash: auth.HashRefreshToken("refresh"),
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		t.Fatalf("save session: %v", err)
	}

	logic := NewLogoutLogic(context.Background(), svcCtx)
	bizErr := logic.Logout(&types.UserLogoutRequest{}, &middleware.AuthIdentity{
		SessionID: "session-logout",
		UserID:    102,
		Username:  "tester",
		Role:      "user",
	})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}

	_, err = svcCtx.SessionStore.Get(context.Background(), "session-logout")
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("expected session deleted, got %v", err)
	}
}
