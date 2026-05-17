package chat

import "context"

type Service interface {
	ListAIProperties(context.Context) (*PageResult[AIProperty], error)
	CreateConversation(context.Context, CreateConversationRequest) (*CreateConversationResponse, error)
	PageConversations(context.Context, PageConversationsRequest) (*PageResult[Conversation], error)
	ListHistory(context.Context, string) ([]Message, error)
	PageHistory(context.Context, PageHistoryRequest) (*PageResult[Message], error)
	Chat(context.Context, ChatRequest) (string, error)
}

type PageConversationsRequest struct {
	Username string
	Current  int
	Size     int
}

type PageHistoryRequest struct {
	SessionID string
	Current   int
	Size      int
}
