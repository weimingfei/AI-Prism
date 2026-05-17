package impl

import (
	"context"

	"github.com/ai-prism/backend/apps/feynman"
	"github.com/ai-prism/backend/internal/agent"
	"github.com/ai-prism/backend/internal/ai"
	"github.com/ai-prism/backend/internal/config"
	"github.com/infraboard/mcube/v2/ioc"
)

func init() {
	ioc.Controller().Registry(&ServiceImpl{})
}

type ServiceImpl struct {
	ioc.ObjectImpl

	coach *agent.FeynmanCoachWorkflow
}

func (s *ServiceImpl) Name() string {
	return feynman.AppName
}

func (s *ServiceImpl) Init() error {
	cfg := config.Load()
	s.coach = agent.NewFeynmanCoachWorkflow(ai.NewClient(cfg), cfg.AI.Model)
	return nil
}

func (s *ServiceImpl) Diagnose(ctx context.Context, req feynman.DiagnoseRequest) (*feynman.CoachResult, error) {
	result := s.coach.Diagnose(ctx, agent.FeynmanCoachInput{
		SessionID:    req.SessionID,
		Explanation:  req.Explanation,
		FollowUpMode: req.FollowUpMode,
	})

	return &feynman.CoachResult{
		Diagnosis: feynman.Diagnosis{
			CorrectParts:   result.Diagnosis.CorrectParts,
			MissingParts:   result.Diagnosis.MissingParts,
			Misconceptions: result.Diagnosis.Misconceptions,
			Clarity:        result.Diagnosis.Clarity,
		},
		FollowUp: feynman.FollowUp{
			Question: result.FollowUp.Question,
			Reason:   result.FollowUp.Reason,
			Targets:  result.FollowUp.Targets,
		},
		Correction:   result.Correction,
		MasteryScore: result.MasteryScore,
		Intent:       result.Intent,
	}, nil
}
