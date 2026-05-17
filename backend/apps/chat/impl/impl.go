package impl

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	chatapp "github.com/ai-prism/backend/apps/chat"
	"github.com/ai-prism/backend/internal/ai"
	"github.com/ai-prism/backend/internal/config"
	"github.com/ai-prism/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/infraboard/mcube/v2/ioc"
	"github.com/redis/go-redis/v9"
)

func init() {
	ioc.Controller().Registry(&ServiceImpl{
		conversations: map[string]*chatapp.Conversation{},
		messages:      map[string][]chatapp.Message{},
	})
}

type ServiceImpl struct {
	ioc.ObjectImpl

	mu            sync.RWMutex
	redis         *redis.Client
	cfg           config.AIConfig
	client        ai.Client
	conversations map[string]*chatapp.Conversation
	messages      map[string][]chatapp.Message
}

func (s *ServiceImpl) Name() string {
	return chatapp.AppName
}

func (s *ServiceImpl) Init() error {
	cfg := config.Load()
	s.cfg = cfg.AI
	s.client = ai.NewClient(cfg)
	s.redis = storage.OpenRedis(cfg.Redis)
	if err := s.redis.Ping(context.Background()).Err(); err != nil {
		s.redis = nil
	}
	return nil
}

func (s *ServiceImpl) ListAIProperties(ctx context.Context) (*chatapp.PageResult[chatapp.AIProperty], error) {
	now := time.Now()
	record := chatapp.AIProperty{
		ID:          1,
		AIName:      "AI 棱镜 本地模型",
		AIType:      s.cfg.Provider,
		APIURL:      s.cfg.BaseURL,
		ModelName:   s.cfg.Model,
		Temperature: s.cfg.Temperature,
		IsEnabled:   1,
		CreateTime:  now,
		UpdateTime:  now,
		DelFlag:     0,
	}
	return &chatapp.PageResult[chatapp.AIProperty]{
		Records: []chatapp.AIProperty{record},
		Total:   1,
		Size:    100,
		Current: 1,
		Pages:   1,
	}, nil
}

func (s *ServiceImpl) CreateConversation(ctx context.Context, req chatapp.CreateConversationRequest) (*chatapp.CreateConversationResponse, error) {
	now := time.Now()
	sessionID := uuid.NewString()
	title := titleFromMessage(req.FirstMessage)
	conversation := &chatapp.Conversation{
		ID:                sessionID,
		SessionID:         sessionID,
		Username:          defaultUsername(req.UserName),
		AIID:              defaultAIID(req.AIID),
		AIName:            "AI 棱镜 本地模型",
		Title:             title,
		Status:            1,
		MessageCount:      0,
		LastMessageTime:   now,
		CreateTime:        now,
		UpdateTime:        now,
		DelFlag:           0,
		ConversationTitle: title,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.conversations[sessionID] = conversation
	s.saveConversation(ctx, conversation)
	return &chatapp.CreateConversationResponse{
		SessionID:         sessionID,
		ConversationTitle: title,
	}, nil
}

func (s *ServiceImpl) PageConversations(ctx context.Context, req chatapp.PageConversationsRequest) (*chatapp.PageResult[chatapp.Conversation], error) {
	current, size := normalizePage(req.Current, req.Size)
	username := strings.TrimSpace(req.Username)

	records := s.loadConversations(ctx)
	if len(records) == 0 {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, item := range s.conversations {
			if username != "" && item.Username != username {
				continue
			}
			records = append(records, cloneConversation(item))
		}
	}
	filtered := make([]chatapp.Conversation, 0, len(records))
	for _, item := range records {
		if username != "" && item.Username != username {
			continue
		}
		filtered = append(filtered, item)
	}
	return page(filtered, current, size), nil
}

func (s *ServiceImpl) ListHistory(ctx context.Context, sessionID string) ([]chatapp.Message, error) {
	if messages := s.loadMessages(ctx, sessionID); len(messages) > 0 {
		return messages, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]chatapp.Message(nil), s.messages[sessionID]...), nil
}

func (s *ServiceImpl) PageHistory(ctx context.Context, req chatapp.PageHistoryRequest) (*chatapp.PageResult[chatapp.Message], error) {
	current, size := normalizePage(req.Current, req.Size)
	if messages := s.loadMessages(ctx, req.SessionID); len(messages) > 0 {
		return page(messages, current, size), nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return page(append([]chatapp.Message(nil), s.messages[req.SessionID]...), current, size), nil
}

func (s *ServiceImpl) Chat(ctx context.Context, req chatapp.ChatRequest) (string, error) {
	input := strings.TrimSpace(req.InputMessage)
	if input == "" {
		return "", nil
	}

	s.appendMessage(ctx, req.SessionID, 1, input)
	history := s.historyMessages(ctx, req.SessionID)
	response, err := s.client.Chat(ai.WithSingleFlightStage(ctx, "chat-completion"), ai.ChatRequest{
		Model: s.cfg.Model,
		Messages: append([]ai.Message{
			{
				Role:    "system",
				Content: "你是 AI 棱镜的知识练习助手。请用中文回答，帮助用户讲清知识点，优先追问概念边界、例子、反例和应用场景。",
			},
		}, history...),
	})
	if err != nil {
		return "", err
	}

	content := strings.TrimSpace(response.Content)
	s.appendMessage(ctx, req.SessionID, 2, content)
	return content, nil
}

func (s *ServiceImpl) appendMessage(ctx context.Context, sessionID string, messageType int, content string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	nextSeq := len(s.messages[sessionID]) + 1
	if s.redis != nil {
		if count, err := s.redis.LLen(ctx, chatMessagesKey(sessionID)).Result(); err == nil && int(count)+1 > nextSeq {
			nextSeq = int(count) + 1
		}
	}
	msg := chatapp.Message{
		ID:             uuid.NewString(),
		SessionID:      sessionID,
		MessageType:    messageType,
		MessageContent: content,
		MessageSeq:     nextSeq,
		CreateTime:     now,
		UpdateTime:     now,
		DelFlag:        0,
	}
	s.messages[sessionID] = append(s.messages[sessionID], msg)

	if conversation, ok := s.conversations[sessionID]; ok {
		conversation.MessageCount = len(s.messages[sessionID])
		conversation.LastMessageTime = now
		conversation.UpdateTime = now
	}
	s.saveMessage(ctx, msg)
	if conversation, ok := s.conversations[sessionID]; ok {
		s.saveConversation(ctx, conversation)
	}
}

func (s *ServiceImpl) historyMessages(ctx context.Context, sessionID string) []ai.Message {
	if redisMessages := s.loadMessages(ctx, sessionID); len(redisMessages) > 0 {
		return toAIMessages(redisMessages)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	return toAIMessages(s.messages[sessionID])
}

func toAIMessages(items []chatapp.Message) []ai.Message {
	messages := make([]ai.Message, 0, len(items))
	for _, item := range items {
		role := "assistant"
		if item.MessageType == 1 {
			role = "user"
		}
		messages = append(messages, ai.Message{
			Role:    role,
			Content: item.MessageContent,
		})
	}
	return messages
}

func defaultUsername(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return "demo-user"
	}
	return username
}

func defaultAIID(aiID int64) int64 {
	if aiID <= 0 {
		return 1
	}
	return aiID
}

func titleFromMessage(message string) string {
	title := strings.TrimSpace(message)
	if title == "" {
		return "新的学习对话"
	}
	runes := []rune(title)
	if len(runes) > 24 {
		return string(runes[:24])
	}
	return title
}

func normalizePage(current int, size int) (int, int) {
	if current <= 0 {
		current = 1
	}
	if size <= 0 {
		size = 20
	}
	return current, size
}

func page[T any](records []T, current int, size int) *chatapp.PageResult[T] {
	total := len(records)
	start := (current - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	pages := 0
	if total > 0 {
		pages = (total + size - 1) / size
	}
	return &chatapp.PageResult[T]{
		Records: records[start:end],
		Total:   total,
		Size:    size,
		Current: current,
		Pages:   pages,
	}
}

func cloneConversation(item *chatapp.Conversation) chatapp.Conversation {
	if item == nil {
		return chatapp.Conversation{}
	}
	return *item
}

func (s *ServiceImpl) saveConversation(ctx context.Context, conversation *chatapp.Conversation) {
	if s.redis == nil || conversation == nil {
		return
	}
	data, err := json.Marshal(conversation)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, chatConversationKey(conversation.SessionID), data, 0).Err()
	_ = s.redis.ZAdd(ctx, chatConversationIndexKey(), redis.Z{
		Score:  float64(conversation.UpdateTime.UnixMilli()),
		Member: conversation.SessionID,
	}).Err()
}

func (s *ServiceImpl) saveMessage(ctx context.Context, msg chatapp.Message) {
	if s.redis == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_ = s.redis.RPush(ctx, chatMessagesKey(msg.SessionID), data).Err()
}

func (s *ServiceImpl) loadConversations(ctx context.Context) []chatapp.Conversation {
	if s.redis == nil {
		return nil
	}
	ids, err := s.redis.ZRevRange(ctx, chatConversationIndexKey(), 0, -1).Result()
	if err != nil {
		return nil
	}
	records := make([]chatapp.Conversation, 0, len(ids))
	for _, id := range ids {
		data, err := s.redis.Get(ctx, chatConversationKey(id)).Bytes()
		if err != nil {
			continue
		}
		var item chatapp.Conversation
		if json.Unmarshal(data, &item) == nil {
			records = append(records, item)
		}
	}
	return records
}

func (s *ServiceImpl) loadMessages(ctx context.Context, sessionID string) []chatapp.Message {
	if s.redis == nil {
		return nil
	}
	items, err := s.redis.LRange(ctx, chatMessagesKey(sessionID), 0, -1).Result()
	if err != nil {
		return nil
	}
	messages := make([]chatapp.Message, 0, len(items))
	for _, item := range items {
		var msg chatapp.Message
		if json.Unmarshal([]byte(item), &msg) == nil {
			messages = append(messages, msg)
		}
	}
	return messages
}

func chatConversationKey(sessionID string) string {
	return "ai-prism:chat:conversation:" + sessionID
}

func chatConversationIndexKey() string {
	return "ai-prism:chat:conversations"
}

func chatMessagesKey(sessionID string) string {
	return "ai-prism:chat:messages:" + sessionID
}
