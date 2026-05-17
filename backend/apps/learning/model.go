package learning

import "time"

type Session struct {
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	KnowledgeBaseID  string    `json:"knowledgeBaseId"`
	DocumentID       string    `json:"documentId"`
	KnowledgePointID string    `json:"knowledgePointId"`
	Status           string    `json:"status"`
	MasteryScore     int       `json:"masteryScore"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type ExplanationTurn struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"sessionId"`
	InputMode    string    `json:"inputMode"`
	Explanation  string    `json:"explanation"`
	Diagnosis    Diagnosis `json:"diagnosis"`
	FollowUp     FollowUp  `json:"followUp"`
	Correction   string    `json:"correction"`
	MasteryScore int       `json:"masteryScore"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Diagnosis struct {
	CorrectParts   []string `json:"correctParts"`
	MissingParts   []string `json:"missingParts"`
	Misconceptions []string `json:"misconceptions"`
	Clarity        string   `json:"clarity"`
}

type FollowUp struct {
	Question string   `json:"question"`
	Reason   string   `json:"reason"`
	Targets  []string `json:"targets"`
}

type MarkdownCard struct {
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
	Content   string `json:"content"`
}

type ReviewPlan struct {
	SessionID string       `json:"sessionId"`
	Score     int          `json:"score"`
	Items     []ReviewItem `json:"items"`
}

type ReviewItem struct {
	When   string `json:"when"`
	Focus  string `json:"focus"`
	Task   string `json:"task"`
	Target string `json:"target"`
}

type CreateSessionRequest struct {
	UserID           string `json:"userId"`
	KnowledgeBaseID  string `json:"knowledgeBaseId" binding:"required"`
	DocumentID       string `json:"documentId" binding:"required"`
	KnowledgePointID string `json:"knowledgePointId"`
}

type SubmitExplanationRequest struct {
	SessionID   string `json:"sessionId" uri:"sessionId"`
	InputMode   string `json:"inputMode"`
	Explanation string `json:"explanation" binding:"required"`
}

type NextFollowUpRequest struct {
	SessionID string `json:"sessionId" uri:"sessionId" binding:"required"`
}

type GenerateMarkdownCardRequest struct {
	SessionID string `json:"sessionId" uri:"sessionId" binding:"required"`
}

type GenerateReviewPlanRequest struct {
	SessionID string `json:"sessionId" uri:"sessionId" binding:"required"`
}

type FinishSessionRequest struct {
	SessionID string `json:"sessionId" uri:"sessionId" binding:"required"`
}
