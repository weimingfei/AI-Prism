package api

import (
	"errors"
	"net/http"

	knowledgeimpl "github.com/ai-prism/backend/apps/knowledge/impl"
	"github.com/ai-prism/backend/apps/learning"
	learningimpl "github.com/ai-prism/backend/apps/learning/impl"
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

	Svc learning.Service `ioc:"autowire=true;namespace=controllers"`
}

func (h *Handler) Name() string {
	return learning.AppName
}

func (h *Handler) Meta() ioc.ObjectMeta {
	meta := ioc.DefaultObjectMeta()
	meta.CustomPathPrefix = "/api/v1"
	return meta
}

func (h *Handler) Init() error {
	router := iocgin.ObjectRouter(h)
	router.POST("/learning/sessions", h.CreateSession)
	router.POST("/learning/sessions/:sessionId/explanations", h.SubmitExplanation)
	router.POST("/learning/sessions/:sessionId/followups", h.NextFollowUp)
	router.POST("/learning/sessions/:sessionId/cards", h.GenerateMarkdownCard)
	router.POST("/learning/sessions/:sessionId/review-plan", h.GenerateReviewPlan)
	router.POST("/learning/sessions/:sessionId/finish", h.FinishSession)
	return nil
}

func (h *Handler) CreateSession(c *gin.Context) {
	var req learning.CreateSessionRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.Svc.CreateSession(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, knowledgeimpl.ErrNotFound) {
			httpx.Error(c, http.StatusNotFound, "404", "document not found")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.Created(c, resp)
}

func (h *Handler) SubmitExplanation(c *gin.Context) {
	var req learning.SubmitExplanationRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	req.SessionID = c.Param("sessionId")
	resp, err := h.Svc.SubmitExplanation(c.Request.Context(), req)
	if err != nil {
		writeSessionError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) NextFollowUp(c *gin.Context) {
	req := learning.NextFollowUpRequest{SessionID: c.Param("sessionId")}
	resp, err := h.Svc.NextFollowUp(c.Request.Context(), req)
	if err != nil {
		writeSessionError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) GenerateMarkdownCard(c *gin.Context) {
	req := learning.GenerateMarkdownCardRequest{SessionID: c.Param("sessionId")}
	resp, err := h.Svc.GenerateMarkdownCard(c.Request.Context(), req)
	if err != nil {
		writeSessionError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) GenerateReviewPlan(c *gin.Context) {
	req := learning.GenerateReviewPlanRequest{SessionID: c.Param("sessionId")}
	resp, err := h.Svc.GenerateReviewPlan(c.Request.Context(), req)
	if err != nil {
		writeSessionError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) FinishSession(c *gin.Context) {
	req := learning.FinishSessionRequest{SessionID: c.Param("sessionId")}
	resp, err := h.Svc.FinishSession(c.Request.Context(), req)
	if err != nil {
		writeSessionError(c, err)
		return
	}
	httpx.OK(c, resp)
}

func writeSessionError(c *gin.Context, err error) {
	if errors.Is(err, learningimpl.ErrSessionNotFound) {
		httpx.Error(c, http.StatusNotFound, "404", "learning session not found")
		return
	}
	httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
}
