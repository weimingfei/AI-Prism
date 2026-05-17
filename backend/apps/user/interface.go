package user

import "context"

const AppName = "user"

type Service interface {
	Register(context.Context, RegisterRequest) (*User, error)
	Login(context.Context, LoginRequest) (*AuthPayload, error)
	CheckLogin(context.Context, CheckLoginRequest) (*AuthPayload, error)
	Logout(context.Context, LogoutRequest) error
	GetUser(context.Context, GetUserRequest) (*User, error)
	HasUsername(context.Context, HasUsernameRequest) (bool, error)
}
