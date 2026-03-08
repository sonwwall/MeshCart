namespace go meshcart.user

// TODO: define user service IDL.

include "base.thrift"

struct UserLoginRequest {
    1: string username
    2: string password
}

struct UserLoginResponse {
    255: base.BaseResponse base
}

struct UserRegisterRequest {
    1: string username
    2: string password
}

struct UserRegisterResponse {
    255: base.BaseResponse base
}

service UserService {
    UserLoginResponse login(1: UserLoginRequest request)
    UserRegisterResponse register(1: UserRegisterRequest request)
}
