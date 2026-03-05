package user

import (
	"context"
	"strings"

	"meshcart/app/common"
	logx "meshcart/app/log"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	userrpc "meshcart/gateway/rpc/user"

	"go.uber.org/zap"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.UserLoginRequest) (*types.UserLoginData, *common.BizError) {
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return nil, common.ErrInvalidParam
	}

	resp, err := l.svcCtx.UserClient.Login(l.ctx, &userrpc.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		logx.L(l.ctx).Error("user rpc login failed", zap.Error(err))
		return nil, common.ErrInternalError
	}
	if resp.Code != common.CodeOK {
		return nil, common.NewBizError(resp.Code, resp.Message)
	}

	return &types.UserLoginData{
		UserID:   resp.UserID,
		Token:    resp.Token,
		Username: resp.Username,
	}, nil
}
