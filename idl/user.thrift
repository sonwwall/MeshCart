namespace go meshcart.user

// TODO: define user service IDL.

include "base.thrift"

struct UserLoginRequest {
    1: string username
    2: string password
}

struct UserLoginResponse {
    1: i64 user_id
    2: string username
    3: string role
    255: base.BaseResponse base
}

struct UserRegisterRequest {
    1: string username
    2: string password
}

struct UserRegisterResponse {
    255: base.BaseResponse base
}

struct UserGetRequest {
    1: i64 user_id
}

struct UserGetResponse {
    1: i64 user_id
    2: string username
    3: string role
    4: bool is_locked
    255: base.BaseResponse base
}

struct UserUpdateRoleRequest {
    1: i64 user_id
    2: string role
}

struct UserUpdateRoleResponse {
    255: base.BaseResponse base
}

service UserService {
    UserLoginResponse login(1: UserLoginRequest request)
    UserRegisterResponse register(1: UserRegisterRequest request)
    UserGetResponse getUser(1: UserGetRequest request)
    UserUpdateRoleResponse updateUserRole(1: UserUpdateRoleRequest request)
}
