package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	chatapp "github.com/ai-prism/backend/apps/chat"
	"github.com/ai-prism/backend/internal/ai"
	"github.com/ai-prism/backend/internal/asr"
	"github.com/ai-prism/backend/internal/config"
	"github.com/ai-prism/backend/internal/httpx"
	"github.com/ai-prism/backend/internal/xunfei"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/infraboard/mcube/v2/ioc"
	iocgin "github.com/infraboard/mcube/v2/ioc/config/gin"
)

func init() {
	ioc.Api().Registry(&Handler{})
}

type Handler struct {
	ioc.ObjectImpl

	Svc chatapp.Service `ioc:"autowire=true;namespace=controllers"`
}

var audioToTextUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) Name() string {
	return chatapp.AppName
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
	router.GET("/ai-properties", h.ListAIProperties)
	router.POST("/ai/conversations", h.CreateConversation)
	router.GET("/ai/conversations", h.PageConversations)
	router.GET("/ai/history/:sessionId", h.ListHistory)
	router.GET("/ai/history/page", h.PageHistory)
	router.POST("/ai/sessions/:sessionId/chat", h.Chat)
	router.POST("/agents/sessions", h.CreateAgentSession)
	router.POST("/agents/sessions/:sessionId/chat", h.AgentChat)
	router.POST("/xunfei/tts/synthesize", h.SynthesizeTTS)
	router.POST("/xunfei/tts/tasks", h.SynthesizeTTS)
	router.GET("/xunfei/tts/tasks/:taskId", h.QueryTTSTask)
	router.GET("/xunfei/audio-to-text/:userId", h.AudioToText)
}

func (h *Handler) ListAIProperties(c *gin.Context) {
	resp, err := h.Svc.ListAIProperties(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) CreateConversation(c *gin.Context) {
	var req chatapp.CreateConversationRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.Svc.CreateConversation(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) PageConversations(c *gin.Context) {
	resp, err := h.Svc.PageConversations(c.Request.Context(), chatapp.PageConversationsRequest{
		Username: c.Query("username"),
		Current:  queryInt(c, "current", 1),
		Size:     queryInt(c, "size", 20),
	})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) ListHistory(c *gin.Context) {
	resp, err := h.Svc.ListHistory(c.Request.Context(), c.Param("sessionId"))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) PageHistory(c *gin.Context) {
	resp, err := h.Svc.PageHistory(c.Request.Context(), chatapp.PageHistoryRequest{
		SessionID: c.Query("sessionId"),
		Current:   queryInt(c, "current", 1),
		Size:      queryInt(c, "size", 20),
	})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) Chat(c *gin.Context) {
	var req chatapp.ChatRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	req.SessionID = c.Param("sessionId")
	if req.UserName == "" {
		req.UserName = c.Query("username")
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	content, err := h.Svc.Chat(aiContext(c), req)
	if err != nil {
		writeSSE(c, "message", gin.H{"content": "模型调用失败：" + err.Error()})
		writeSSE(c, "done", gin.H{"done": true})
		return
	}
	writeSSE(c, "message", gin.H{"content": content})
	writeSSE(c, "done", gin.H{"done": true})
}

func (h *Handler) CreateAgentSession(c *gin.Context) {
	var req struct {
		AgentID      int64  `json:"agentId"`
		FirstMessage string `json:"firstMessage"`
		UserName     string `json:"userName"`
	}
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.Svc.CreateConversation(c.Request.Context(), chatapp.CreateConversationRequest{
		UserName:     req.UserName,
		FirstMessage: req.FirstMessage,
		AIID:         req.AgentID,
	})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "500", err.Error())
		return
	}
	httpx.OK(c, resp)
}

func (h *Handler) AgentChat(c *gin.Context) {
	var req chatapp.ChatRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	req.SessionID = c.Param("sessionId")
	if req.UserName == "" {
		req.UserName = c.Query("username")
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	content, err := h.Svc.Chat(aiContext(c), req)
	if err != nil {
		writeSSE(c, "message", gin.H{"content": "智能体调用失败：" + err.Error()})
		writeSSE(c, "done", gin.H{"done": true})
		return
	}
	writeSSE(c, "message", gin.H{"content": content})
	writeSSE(c, "done", gin.H{"done": true})
}

func (h *Handler) SynthesizeTTS(c *gin.Context) {
	cfg := config.Load().Xunfei
	client := xunfei.NewClient(cfg)
	if client.ConfiguredForTTS() {
		var req struct {
			Text     string `json:"text"`
			VCN      string `json:"vcn"`
			Speed    int    `json:"speed"`
			Volume   int    `json:"volume"`
			Pitch    int    `json:"pitch"`
			Encoding string `json:"audioEncoding"`
		}
		if !httpx.BindJSON(c, &req) {
			return
		}
		audio, err := client.Synthesize(xunfei.TTSRequest{
			Text:     req.Text,
			Voice:    req.VCN,
			Speed:    req.Speed,
			Volume:   req.Volume,
			Pitch:    req.Pitch,
			Encoding: ttsEncoding(req.Encoding),
		})
		if err != nil {
			httpx.OK(c, gin.H{
				"taskId":      "tts-failed",
				"taskStatus":  "4",
				"code":        500,
				"message":     err.Error(),
				"audioBase64": nil,
				"audioUrl":    nil,
				"completed":   true,
				"success":     false,
			})
			return
		}
		httpx.OK(c, gin.H{
			"taskId":      fmt.Sprintf("tts-%d", time.Now().UnixNano()),
			"taskStatus":  "5",
			"code":        0,
			"message":     "success",
			"audioBase64": base64.StdEncoding.EncodeToString(audio),
			"audioUrl":    nil,
			"completed":   true,
			"success":     true,
		})
		return
	}

	httpx.OK(c, gin.H{
		"taskId":      "tts-disabled",
		"taskStatus":  "4",
		"code":        501,
		"message":     "当前未配置语音合成服务，已跳过自动朗读。",
		"audioBase64": nil,
		"audioUrl":    nil,
		"completed":   true,
		"success":     false,
	})
}

func (h *Handler) QueryTTSTask(c *gin.Context) {
	h.SynthesizeTTS(c)
}

func (h *Handler) AudioToText(c *gin.Context) {
	browserConn, err := audioToTextUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer browserConn.Close()

	client := xunfei.NewClient(config.Load().Xunfei)
	if !client.ConfiguredForASR() {
		writeWSJSON(browserConn, gin.H{
			"type":      "connected",
			"message":   "ASR WebSocket 已连接，但当前未配置真实语音识别服务。",
			"timestamp": time.Now().UnixMilli(),
		})
		writeWSJSON(browserConn, gin.H{
			"type":      "error",
			"message":   "当前未配置语音识别服务。请检查 application.toml 的 [xunfei] 配置。",
			"timestamp": time.Now().UnixMilli(),
		})
		return
	}

	xunfeiConn, err := client.DialASR()
	if err != nil {
		writeWSJSON(browserConn, gin.H{
			"type":      "error",
			"message":   "连接讯飞语音听写失败：" + err.Error(),
			"timestamp": time.Now().UnixMilli(),
		})
		return
	}
	defer xunfeiConn.Close()

	var writeMu sync.Mutex
	writeBrowser := func(payload any) {
		writeMu.Lock()
		defer writeMu.Unlock()
		writeWSJSON(browserConn, payload)
	}
	writeBrowser(gin.H{
		"type":      "connected",
		"message":   "讯飞语音听写已连接",
		"timestamp": time.Now().UnixMilli(),
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		assembler := asr.NewRealtimeAssembler()
		for {
			_, payload, err := xunfeiConn.ReadMessage()
			if err != nil {
				if !isClosedNetworkError(err) {
					writeBrowser(gin.H{
						"type":      "error",
						"message":   "读取讯飞语音听写结果失败：" + err.Error(),
						"timestamp": time.Now().UnixMilli(),
					})
				}
				return
			}
			rawUpdate, err := xunfei.ExtractASRUpdate(payload)
			if err != nil {
				writeBrowser(gin.H{
					"type":      "error",
					"message":   err.Error(),
					"timestamp": time.Now().UnixMilli(),
				})
				return
			}
			update, changed := assembler.Apply(xunfei.ASRPacketFromUpdate(rawUpdate))
			if changed {
				writeBrowser(gin.H{
					"type":          "transcription",
					"message":       "Partial snapshot",
					"data":          update.FullText,
					"text":          update.FullText,
					"fullText":      update.FullText,
					"displayText":   update.DisplayText,
					"committedText": update.CommittedText,
					"liveText":      update.LiveText,
					"isSnapshot":    true,
					"updateAction":  "replace",
					"revision":      update.Revision,
					"resultStatus":  update.ResultStatus,
					"segmentId":     update.SegmentID,
					"segmentText":   update.SegmentText,
					"pgs":           update.PGS,
					"rg":            update.RG,
					"bg":            update.BG,
					"ed":            update.ED,
					"finalPacket":   update.FinalPacket,
					"timestamp":     time.Now().UnixMilli(),
				})
			}
			if rawUpdate.Status == 2 || rawUpdate.FinalPacket {
				finalText := assembler.BuildSnapshot()
				writeBrowser(gin.H{
					"type":         "final",
					"message":      "Transcription completed",
					"data":         finalText,
					"text":         finalText,
					"fullText":     finalText,
					"displayText":  finalText,
					"isSnapshot":   true,
					"updateAction": "archive",
					"timestamp":    time.Now().UnixMilli(),
				})
				return
			}
		}
	}()

	started := false
	for {
		select {
		case <-done:
			return
		default:
		}
		messageType, payload, err := browserConn.ReadMessage()
		if err != nil {
			if started {
				_ = xunfeiConn.WriteJSON(xunfei.NewASREndFrame())
			}
			return
		}
		if messageType == websocket.BinaryMessage {
			if len(payload) == 0 {
				continue
			}
			if !started {
				if err := xunfeiConn.WriteJSON(client.NewASRStartFrame(payload)); err != nil {
					writeBrowser(gin.H{"type": "error", "message": err.Error(), "timestamp": time.Now().UnixMilli()})
					return
				}
				started = true
				continue
			}
			if err := xunfeiConn.WriteJSON(xunfei.NewASRContinueFrame(payload)); err != nil {
				writeBrowser(gin.H{"type": "error", "message": err.Error(), "timestamp": time.Now().UnixMilli()})
				return
			}
			continue
		}
		if messageType != websocket.TextMessage {
			continue
		}

		var msg struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(payload, &msg)
		switch msg.Type {
		case "ping", "get_status":
			writeBrowser(gin.H{
				"type":      "heartbeat",
				"timestamp": time.Now().UnixMilli(),
			})
		case "start_transcription":
			writeBrowser(gin.H{
				"type":      "transcription_started",
				"message":   "语音转写已开始",
				"timestamp": time.Now().UnixMilli(),
			})
		case "stop_transcription":
			if started {
				_ = xunfeiConn.WriteJSON(xunfei.NewASREndFrame())
				<-done
				return
			}
			writeBrowser(gin.H{"type": "archive", "text": "", "timestamp": time.Now().UnixMilli()})
			return
		}
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

func writeWSJSON(conn *websocket.Conn, payload any) {
	_ = conn.WriteJSON(payload)
}

func writeSSE(c *gin.Context, event string, data any) {
	payload, _ := json.Marshal(data)
	_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, payload)
	c.Writer.Flush()
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value := fallback
	if raw := c.Query(key); raw != "" {
		_, _ = fmt.Sscanf(raw, "%d", &value)
	}
	return value
}

func ttsEncoding(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "raw", "lame", "speex", "speex-wb":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "lame"
	}
}

func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return true
	}
	return err == io.EOF || strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}
