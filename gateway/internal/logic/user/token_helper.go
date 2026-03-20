package user

import (
	"context"
	"time"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"

	"go.uber.org/zap"
)

func buildTokenResponse(ctx context.Context, svcCtx *svc.ServiceContext, sessionID string, userID int64, username, role, refreshToken string, refreshExpireAt time.Time) (*types.UserRefreshTokenData, *common.BizError) {
	accessToken, accessExpireAt, err := svcCtx.JWT.TokenGenerator(&middleware.AuthIdentity{
		SessionID: sessionID,
		UserID:    userID,
		Username:  username,
		Role:      role,
	})
	if err != nil {
		logx.L(ctx).Error("jwt token generate failed", zap.Error(err))
		return nil, common.ErrInternalError
	}

	return &types.UserRefreshTokenData{
		SessionID:       sessionID,
		TokenType:       "Bearer",
		AccessToken:     middleware.FormatBearerToken(accessToken),
		AccessExpireAt:  accessExpireAt.Format(time.RFC3339),
		RefreshToken:    refreshToken,
		RefreshExpireAt: refreshExpireAt.Format(time.RFC3339),
	}, nil
}
