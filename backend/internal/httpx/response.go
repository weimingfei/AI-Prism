package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDKey = "requestId"

type Response struct {
	Code      string `json:"code"`
	Message   string `json:"message,omitempty"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId,omitempty"`
	Success   bool   `json:"success"`
}

func NewRequestID() string {
	return uuid.NewString()
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:      "0",
		Data:      data,
		RequestID: requestID(c),
		Success:   true,
	})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Code:      "0",
		Data:      data,
		RequestID: requestID(c),
		Success:   true,
	})
}

func Error(c *gin.Context, status int, code string, message string) {
	c.JSON(status, Response{
		Code:      code,
		Message:   message,
		Data:      nil,
		RequestID: requestID(c),
		Success:   false,
	})
}

func BindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		Error(c, http.StatusBadRequest, "400", err.Error())
		return false
	}
	return true
}

func requestID(c *gin.Context) string {
	if value, ok := c.Get(RequestIDKey); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}
