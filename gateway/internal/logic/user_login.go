package logic

import (
	"context"
	"strings"

	"meshcart/app/common"
	"meshcart/gateway/internal/svc"
	"meshcart/gateway/internal/types"
	userrpc "meshcart/gateway/rpc/user"
)

type UserLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserLoginLogic {
	return &UserLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserLoginLogic) Login(req *types.UserLoginRequest) (*types.UserLoginData, *common.BizError) {
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return nil, common.ErrInvalidParam
	}

	resp, err := l.svcCtx.UserClient.Login(l.ctx, &userrpc.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
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
