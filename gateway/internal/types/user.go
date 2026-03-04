package types

type UserLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserLoginData struct {
	UserID   int64  `json:"user_id"`
	Token    string `json:"token"`
	Username string `json:"username"`
}
