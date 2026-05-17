package storage

import (
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type KnowledgeBaseModel struct {
	ID          string `gorm:"primaryKey;size:36"`
	UserID      string `gorm:"index;size:64;not null"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (KnowledgeBaseModel) TableName() string {
	return "knowledge_bases"
}

type KnowledgeDocumentModel struct {
	ID              string `gorm:"primaryKey;size:36"`
	KnowledgeBaseID string `gorm:"index;size:36;not null"`
	Title           string `gorm:"size:256;not null"`
	SourceType      string `gorm:"size:32;not null"`
	Status          string `gorm:"size:32;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (KnowledgeDocumentModel) TableName() string {
	return "knowledge_documents"
}

type LearningSessionModel struct {
	ID               string `gorm:"primaryKey;size:36"`
	UserID           string `gorm:"index;size:64;not null"`
	KnowledgeBaseID  string `gorm:"index;size:36;not null"`
	DocumentID       string `gorm:"index;size:36;not null"`
	KnowledgePointID string `gorm:"index;size:64"`
	Status           string `gorm:"size:32;not null"`
	MasteryScore     int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func (LearningSessionModel) TableName() string {
	return "learning_sessions"
}

type CoachSessionModel struct {
	ID           string `gorm:"primaryKey;size:36"`
	UserID       string `gorm:"index;size:64;not null"`
	Status       string `gorm:"index;size:32;not null"`
	Title        string `gorm:"size:256"`
	MaterialName string `gorm:"size:512"`
	TotalScore   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (CoachSessionModel) TableName() string {
	return "coach_sessions"
}
