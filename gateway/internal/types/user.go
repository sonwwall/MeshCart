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
	UserID   int64  `json:"user_id"`
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" form:"role"`
}
