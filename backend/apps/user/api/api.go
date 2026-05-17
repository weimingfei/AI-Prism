package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ai-prism/backend/apps/user"
	userimpl "github.com/ai-prism/backend/apps/user/impl"
	"github.com/ai-prism/backend/internal/httpx"
	"github.com/gin-gonic/gin"
	"github.com/infraboard/mcube/v2/ioc"
	iocgin "github.com/infraboard/mcube/v2/ioc/config/gin"
)

func init() {
	ioc.Api().Registry(&Handler{})
}

type Handler struct {
	ioc.ObjectImpl

	Svc user.Service `ioc:"autowire=true;namespace=controllers"`
}

func (h *Handler) Name() string {
	return user.AppName
}

func (h *Handler) Meta() ioc.ObjectMeta {
	meta := ioc.DefaultObjectMeta()
	meta.CustomPathPrefix = "/api/lingzhi/v1/users"
	return meta
}

func (h *Handler) Init() error {
	h.registerRoutes(iocgin.ObjectRouter(h))
	return nil
}

func (h *Handler) registerRoutes(router gin.IRouter) {
	router.POST("/register", h.Register)
	router.POST("/login", h.Login)
	router.GET("/check-login", h.CheckLogin)
	router.POST("/logout", h.Logout)
	router.GET("/has-username", h.HasUsername)
	router.GET("/:username", h.GetUser)
	router.GET("/actual/:username", h.GetUser)
}

func (h *Handler) Register(c *gin.Context) {
	var req user.RegisterRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	_, err := h.Svc.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, userimpl.ErrUserExists) {
			httpx.Error(c, http.StatusConflict, "409", "username already exists")
			return
		}
		httpx.Error(c, http.StatusBadRequest, "400", err.Error())
		return
	}
	httpx.OK(c, nil)
}

func (h *Handler) Login(c *gin.Context) {
	var req user.LoginRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.Svc.Login(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, "401", "invalid username or password")
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) CheckLogin(c *gin.Context) {
	resp, err := h.Svc.CheckLogin(c.Request.Context(), user.CheckLoginRequest{
		Token: bearerToken(c),
	})
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, "401", "user is not logged in")
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) Logout(c *gin.Context) {
	_ = h.Svc.Logout(c.Request.Context(), user.LogoutRequest{
		Token: bearerToken(c),
	})
	httpx.OK(c, nil)
}

func (h *Handler) GetUser(c *gin.Context) {
	resp, err := h.Svc.GetUser(c.Request.Context(), user.GetUserRequest{
		Username: c.Param("username"),
	})
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "404", "user not found")
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) HasUsername(c *gin.Context) {
	resp, err := h.Svc.HasUsername(c.Request.Context(), user.HasUsernameRequest{
		Username: c.Query("username"),
	})
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func bearerToken(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	if header != "" {
		return header
	}
	return strings.TrimSpace(c.GetHeader("satoken"))
}
