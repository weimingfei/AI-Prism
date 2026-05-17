package api

import (
	"errors"
	"net/http"

	"github.com/ai-prism/backend/apps/knowledge"
	knowledgeimpl "github.com/ai-prism/backend/apps/knowledge/impl"
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

	Svc knowledge.Service `ioc:"autowire=true;namespace=controllers"`
}

func (h *Handler) Name() string {
	return knowledge.AppName
}

func (h *Handler) Meta() ioc.ObjectMeta {
	meta := ioc.DefaultObjectMeta()
	meta.CustomPathPrefix = "/api/v1"
	return meta
}

func (h *Handler) Init() error {
	router := iocgin.ObjectRouter(h)
	router.POST("/knowledge-bases", h.CreateKnowledgeBase)
	router.POST("/documents", h.CreateDocument)
	router.POST("/documents/:documentId/outline", h.GenerateOutline)
	return nil
}

func (h *Handler) CreateKnowledgeBase(c *gin.Context) {
	var req knowledge.CreateKnowledgeBaseRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.Svc.CreateKnowledgeBase(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.Created(c, resp)
}

func (h *Handler) CreateDocument(c *gin.Context) {
	var req knowledge.CreateDocumentRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.Svc.CreateDocument(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, knowledgeimpl.ErrNotFound) {
			httpx.Error(c, http.StatusNotFound, "404", "knowledge base not found")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.Created(c, resp)
}

func (h *Handler) GenerateOutline(c *gin.Context) {
	var req knowledge.GenerateOutlineRequest
	if err := c.ShouldBindUri(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "400", err.Error())
		return
	}
	resp, err := h.Svc.GenerateOutline(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "404", "document not found")
		return
	}
	httpx.OK(c, resp)
}
