package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	compat "github.com/ai-prism/backend/apps/interviewcompat"
	compatimpl "github.com/ai-prism/backend/apps/interviewcompat/impl"
	"github.com/ai-prism/backend/internal/ai"
	"github.com/ai-prism/backend/internal/config"
	"github.com/ai-prism/backend/internal/filetype"
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

	Svc compat.Service `ioc:"autowire=true;namespace=controllers"`
}

func (h *Handler) Name() string {
	return compat.AppName
}

func (h *Handler) Meta() ioc.ObjectMeta {
	meta := ioc.DefaultObjectMeta()
	meta.CustomPathPrefix = "/api/lingzhi/v1"
	return meta
}

func (h *Handler) Init() error {
	h.registerRoutes(iocgin.ObjectRouter(h))
	return nil
}

func (h *Handler) registerRoutes(router gin.IRouter) {
	router.POST("/interview/sessions", h.CreateSession)
	router.GET("/interview/conversations", h.PageConversations)
	router.POST("/interview/sessions/:sessionId/interview-questions", h.ExtractQuestions)
	router.GET("/interview/sessions/:sessionId/resume/preview", h.PreviewMaterial)
	router.GET("/interview/sessions/:sessionId/restore", h.Restore)
	router.GET("/interview/sessions/:sessionId/current-question", h.CurrentQuestion)
	router.GET("/interview/sessions/:sessionId/next-question", h.CurrentQuestion)
	router.POST("/interview/sessions/:sessionId/knowledge-points/:pointId/select", h.SelectKnowledgePoint)
	router.POST("/interview/sessions/:sessionId/knowledge-points/:pointId/skip", h.SkipKnowledgePoint)
	router.POST("/interview/sessions/:sessionId/knowledge-points/current/finish", h.FinishCurrentKnowledgePoint)
	router.POST("/interview/sessions/:sessionId/interview/answer-json", h.AnswerJSON)
	router.POST("/interview/sessions/:sessionId/interview/answer", h.AnswerForm)
	router.PUT("/interview/sessions/:sessionId/finish", h.Finish)
	router.GET("/interview/sessions/:sessionId/radar-chart", h.Radar)
	router.POST("/interview/sessions/:sessionId/demeanor-evaluation", h.Demeanor)
	router.POST("/interview/interview/record", h.SaveRecord)
	router.POST("/interview/record", h.SaveRecord)
	router.POST("/interview/interview/record/save-from-redis/:sessionId", h.SaveRecordFromRedis)
	router.POST("/interview/record/save-from-redis/:sessionId", h.SaveRecordFromRedis)
	router.GET("/interview/interview/records", h.PageRecords)
	router.GET("/interview/records", h.PageRecords)
	router.GET("/interview/interview/record/:sessionId", h.Record)
	router.GET("/interview/record/:sessionId", h.Record)
	router.POST("/agents/files/upload", h.UploadFile)
}

func (h *Handler) CreateSession(c *gin.Context) {
	resp, err := h.Svc.CreateSession(c.Request.Context(), currentUser(c))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) PageConversations(c *gin.Context) {
	resp, err := h.Svc.PageConversations(c.Request.Context(), queryInt(c, "current", 1), queryInt(c, "size", 10))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) ExtractQuestions(c *gin.Context) {
	file, err := c.FormFile("resumePdf")
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", "resumePdf is required")
		return
	}
	content, err := readMultipartFile(file)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", err.Error())
		return
	}
	detected, err := filetype.Validate(content, file.Filename, filetype.KindPDF)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", uploadValidationMessage(err))
		return
	}
	resp, err := h.Svc.ExtractQuestions(
		aiContext(c),
		c.Param("sessionId"),
		currentUser(c),
		file.Filename,
		content,
		detected.ContentType,
	)
	if err != nil {
		status := http.StatusInternalServerError
		if err == compatimpl.ErrSessionNotFound {
			status = http.StatusNotFound
		}
		httpx.Error(c, status, strconv.Itoa(status), err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) PreviewMaterial(c *gin.Context) {
	content, fileName, contentType, err := h.Svc.PreviewMaterial(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "404", err.Error())
		return
	}
	if contentType == "" {
		contentType = "application/pdf"
	}
	c.Header("Cache-Control", "no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Content-Disposition", `inline; filename="`+fileName+`"`)
	c.Data(http.StatusOK, contentType, content)
}

func (h *Handler) Restore(c *gin.Context) {
	resp, err := h.Svc.Restore(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "404", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) CurrentQuestion(c *gin.Context) {
	resp, err := h.Svc.CurrentQuestion(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "404", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) SelectKnowledgePoint(c *gin.Context) {
	resp, err := h.Svc.SelectKnowledgePoint(c.Request.Context(), c.Param("sessionId"), c.Param("pointId"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) SkipKnowledgePoint(c *gin.Context) {
	resp, err := h.Svc.SkipKnowledgePoint(c.Request.Context(), c.Param("sessionId"), c.Param("pointId"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) FinishCurrentKnowledgePoint(c *gin.Context) {
	resp, err := h.Svc.FinishCurrentKnowledgePoint(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) AnswerJSON(c *gin.Context) {
	var req compat.AnswerRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	h.answer(c, req)
}

func (h *Handler) AnswerForm(c *gin.Context) {
	req := compat.AnswerRequest{
		QuestionNumber:   c.PostForm("questionNumber"),
		AnswerContent:    c.PostForm("answerContent"),
		RequestID:        c.PostForm("requestId"),
		MaxFollowUpCount: queryFormInt(c, "maxFollowUpCount", 2),
		FollowUpMode:     c.PostForm("followUpMode"),
	}
	h.answer(c, req)
}

func (h *Handler) answer(c *gin.Context, req compat.AnswerRequest) {
	resp, err := h.Svc.Answer(aiContext(c), c.Param("sessionId"), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) Finish(c *gin.Context) {
	if err := h.Svc.Finish(c.Request.Context(), c.Param("sessionId")); err != nil {
		httpx.Error(c, http.StatusNotFound, "404", err.Error())
		return
	}
	httpx.OK(c, nil)
}

func (h *Handler) Radar(c *gin.Context) {
	resp, err := h.Svc.Radar(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "404", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) Demeanor(c *gin.Context) {
	resp, err := h.Svc.Demeanor(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "404", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) SaveRecord(c *gin.Context) {
	httpx.OK(c, nil)
}

func (h *Handler) SaveRecordFromRedis(c *gin.Context) {
	httpx.OK(c, nil)
}

func (h *Handler) Record(c *gin.Context) {
	resp, err := h.Svc.Record(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "404", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) PageRecords(c *gin.Context) {
	current := queryInt(c, "pageNum", queryInt(c, "current", 1))
	size := queryInt(c, "pageSize", queryInt(c, "size", 10))
	resp, err := h.Svc.PageRecords(c.Request.Context(), current, size)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", "file is required")
		return
	}
	content, err := readMultipartFile(file)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", err.Error())
		return
	}
	detected, err := filetype.Validate(
		content,
		file.Filename,
		filetype.KindPDF,
		filetype.KindImage,
		filetype.KindAudio,
		filetype.KindVideo,
	)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", uploadValidationMessage(err))
		return
	}
	agentID, _ := strconv.ParseInt(c.PostForm("agentId"), 10, 64)
	resp, err := h.Svc.UploadFile(c.Request.Context(), compat.UploadedFile{
		AgentID:     agentID,
		SessionID:   c.PostForm("sessionId"),
		BizType:     c.DefaultPostForm("bizType", "material"),
		FileName:    file.Filename,
		FileSize:    file.Size,
		ContentType: detected.ContentType,
	}, content)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func uploadValidationMessage(err error) string {
	switch {
	case errors.Is(err, filetype.ErrEmptyFile):
		return "上传文件不能为空"
	case errors.Is(err, filetype.ErrUnsupportedType):
		return "不支持的文件类型，或文件内容与真实格式不匹配"
	case errors.Is(err, filetype.ErrFileTooLarge):
		return "上传文件超过大小限制"
	case errors.Is(err, filetype.ErrExtensionMismatch):
		return "文件扩展名与真实文件类型不一致"
	default:
		return err.Error()
	}
}

func aiContext(c *gin.Context) context.Context {
	return ai.WithProviderOverride(c.Request.Context(), config.AIConfig{
		Provider: c.GetHeader("X-Lingzhi-AI-Provider"),
		BaseURL:  c.GetHeader("X-Lingzhi-AI-Base-URL"),
		APIKey:   c.GetHeader("X-Lingzhi-AI-API-Key"),
		Model:    c.GetHeader("X-Lingzhi-AI-Model"),
	})
}

func readMultipartFile(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func currentUser(c *gin.Context) string {
	if username := c.Query("username"); username != "" {
		return username
	}
	return "demo-user"
}

func queryInt(c *gin.Context, key string, fallback int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryFormInt(c *gin.Context, key string, fallback int) int {
	value := fallback
	if raw := c.PostForm(key); raw != "" {
		_, _ = fmt.Sscanf(raw, "%d", &value)
	}
	return value
}
