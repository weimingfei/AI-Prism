package knowledge

import "time"

type KnowledgeBase struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	UserID      string    `json:"userId" gorm:"index;size:64"`
	Name        string    `json:"name" gorm:"size:128"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Document struct {
	ID              string    `json:"id" gorm:"primaryKey;size:36"`
	KnowledgeBaseID string    `json:"knowledgeBaseId" gorm:"index;size:36"`
	Title           string    `json:"title" gorm:"size:256"`
	Content         string    `json:"content,omitempty" gorm:"-"`
	SourceType      string    `json:"sourceType" gorm:"size:32"`
	Status          string    `json:"status" gorm:"size:32"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type KnowledgePoint struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Keywords    []string `json:"keywords"`
	Difficulty  string   `json:"difficulty"`
	CheckPrompt string   `json:"checkPrompt"`
}

type Outline struct {
	DocumentID string           `json:"documentId"`
	Title      string           `json:"title"`
	Points     []KnowledgePoint `json:"points"`
}

type CreateKnowledgeBaseRequest struct {
	UserID      string `json:"userId"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type CreateDocumentRequest struct {
	KnowledgeBaseID string `json:"knowledgeBaseId" binding:"required"`
	Title           string `json:"title" binding:"required"`
	Content         string `json:"content"`
	SourceType      string `json:"sourceType"`
}

type GenerateOutlineRequest struct {
	DocumentID string `json:"documentId" uri:"documentId" binding:"required"`
}

type GetDocumentRequest struct {
	DocumentID string `json:"documentId" uri:"documentId" binding:"required"`
}
