package user

import "time"

type User struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`
	RealName   string    `json:"realName,omitempty"`
	Phone      string    `json:"phone,omitempty"`
	Mail       string    `json:"mail,omitempty"`
	Avatar     string    `json:"avatar,omitempty"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
	DelFlag    int       `json:"delFlag"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	RealName string `json:"realName"`
	Phone    string `json:"phone"`
	Mail     string `json:"mail"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type CheckLoginRequest struct {
	Token string
}

type LogoutRequest struct {
	Token string
}

type GetUserRequest struct {
	Username string `uri:"username" binding:"required"`
}

type HasUsernameRequest struct {
	Username string `form:"username" binding:"required"`
}

type AuthPayload struct {
	Token       string `json:"token"`
	User        *User  `json:"user"`
	CurrentUser *User  `json:"currentUser"`
}
