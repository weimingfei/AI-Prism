package chat

import "time"

const AppName = "chat"

type PageResult[T any] struct {
	Records []T `json:"records"`
	Total   int `json:"total"`
	Size    int `json:"size"`
	Current int `json:"current"`
	Pages   int `json:"pages"`
}

type AIProperty struct {
	ID          int64     `json:"id"`
	AIName      string    `json:"aiName"`
	AIType      string    `json:"aiType"`
	APIURL      string    `json:"apiUrl,omitempty"`
	ModelName   string    `json:"modelName,omitempty"`
	Temperature float32   `json:"temperature"`
	IsEnabled   int       `json:"isEnabled"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
	DelFlag     int       `json:"delFlag"`
}

type Conversation struct {
	ID                string    `json:"_id,omitempty"`
	SessionID         string    `json:"sessionId"`
	Username          string    `json:"username"`
	AIID              int64     `json:"aiId"`
	AIName            string    `json:"aiName,omitempty"`
	Title             string    `json:"title,omitempty"`
	Status            int       `json:"status"`
	MessageCount      int       `json:"messageCount"`
	LastMessageTime   time.Time `json:"lastMessageTime"`
	CreateTime        time.Time `json:"createTime"`
	UpdateTime        time.Time `json:"updateTime"`
	DelFlag           int       `json:"delFlag"`
	ConversationTitle string    `json:"conversationTitle,omitempty"`
}

type Message struct {
	ID               string    `json:"id"`
	SessionID        string    `json:"sessionId"`
	MessageType      int       `json:"messageType"`
	MessageContent   string    `json:"messageContent"`
	MessageSeq       int       `json:"messageSeq"`
	ReasoningContent string    `json:"reasoningContent,omitempty"`
	CreateTime       time.Time `json:"createTime"`
	UpdateTime       time.Time `json:"updateTime"`
	DelFlag          int       `json:"delFlag"`
}

type CreateConversationRequest struct {
	UserName     string `json:"userName"`
	FirstMessage string `json:"firstMessage"`
	AIID         int64  `json:"aiId"`
}

type CreateConversationResponse struct {
	SessionID         string `json:"sessionId"`
	ConversationTitle string `json:"conversationTitle"`
}

type ChatRequest struct {
	SessionID    string   `json:"sessionId"`
	InputMessage string   `json:"inputMessage"`
	UserName     string   `json:"userName"`
	AIID         int64    `json:"aiId"`
	MessageSeq   int      `json:"messageSeq"`
	ImageURLs    []string `json:"imageUrls"`
}
