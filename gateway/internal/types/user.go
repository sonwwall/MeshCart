package types

type UserLoginRequest struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

type UserRegisterRequest struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

type UserLoginData struct {
	UserID          int64  `json:"user_id"`
	Username        string `json:"username"`
	Role            string `json:"role"`
	SessionID       string `json:"session_id"`
	TokenType       string `json:"token_type"`
	AccessToken     string `json:"access_token"`
	AccessExpireAt  string `json:"access_expire_at"`
	RefreshToken    string `json:"refresh_token"`
	RefreshExpireAt string `json:"refresh_expire_at"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" form:"role"`
}

type UserRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
}

type UserRefreshTokenData struct {
	SessionID       string `json:"session_id"`
	TokenType       string `json:"token_type"`
	AccessToken     string `json:"access_token"`
	AccessExpireAt  string `json:"access_expire_at"`
	RefreshToken    string `json:"refresh_token"`
	RefreshExpireAt string `json:"refresh_expire_at"`
}

type UserLogoutRequest struct {
	SessionID string `json:"session_id" form:"session_id"`
}
