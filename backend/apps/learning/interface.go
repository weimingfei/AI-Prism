package learning

import "context"

const AppName = "learning"

type Service interface {
	CreateSession(context.Context, CreateSessionRequest) (*Session, error)
	SubmitExplanation(context.Context, SubmitExplanationRequest) (*ExplanationTurn, error)
	NextFollowUp(context.Context, NextFollowUpRequest) (*FollowUp, error)
	GenerateMarkdownCard(context.Context, GenerateMarkdownCardRequest) (*MarkdownCard, error)
	GenerateReviewPlan(context.Context, GenerateReviewPlanRequest) (*ReviewPlan, error)
	FinishSession(context.Context, FinishSessionRequest) (*Session, error)
}
