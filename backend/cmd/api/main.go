package main

import (
	"context"
	"log"

	"github.com/ai-prism/backend/internal/agent"
	"github.com/ai-prism/backend/internal/ai"
	"github.com/ai-prism/backend/internal/config"
	"github.com/cloudwego/eino-ext/devops"
	"github.com/infraboard/mcube/v2/ioc/server/cmd"

	_ "github.com/infraboard/mcube/v2/ioc/apps/health/gin"
	_ "github.com/infraboard/mcube/v2/ioc/apps/metric/gin"
	_ "github.com/infraboard/mcube/v2/ioc/config/gin"

	_ "github.com/ai-prism/backend/apps/chat/api"
	_ "github.com/ai-prism/backend/apps/chat/impl"
	_ "github.com/ai-prism/backend/apps/interviewcompat/api"
	_ "github.com/ai-prism/backend/apps/interviewcompat/impl"
	_ "github.com/ai-prism/backend/apps/knowledge/api"
	_ "github.com/ai-prism/backend/apps/knowledge/impl"
	_ "github.com/ai-prism/backend/apps/learning/api"
	_ "github.com/ai-prism/backend/apps/learning/impl"
	_ "github.com/ai-prism/backend/apps/prism/impl"
	_ "github.com/ai-prism/backend/apps/user/api"
	_ "github.com/ai-prism/backend/apps/user/impl"
)

func main() {
	initEinoDev()
	cmd.Start()
}

func initEinoDev() {
	cfg := config.Load()
	if !cfg.EinoDev.Enabled {
		return
	}

	err := devops.Init(
		context.Background(),
		devops.WithDevServerIP(cfg.EinoDev.IP),
		devops.WithDevServerPort(cfg.EinoDev.Port),
		devops.AppendType(agent.DocumentQualityInput{}),
		devops.AppendType(agent.DocumentQualityOutput{}),
		devops.AppendType(agent.KnowledgeOutlineInput{}),
		devops.AppendType(agent.KnowledgeOutlineOutput{}),
		devops.AppendType(agent.QuestionGeneratorInput{}),
		devops.AppendType(agent.QuestionGeneratorOutput{}),
		devops.AppendType(agent.AnswerScoringInput{}),
		devops.AppendType(agent.AnswerScoringOutput{}),
		devops.AppendType(agent.FollowUpQuestionInput{}),
		devops.AppendType(agent.FollowUpQuestionOutput{}),
		devops.AppendType(agent.LearningCardReviewInput{}),
		devops.AppendType(agent.LearningCardReviewOutput{}),
		devops.AppendType(agent.FeynmanCoachInput{}),
		devops.AppendType(agent.FeynmanCoachOutput{}),
	)
	if err != nil {
		log.Printf("init eino devops failed: %v", err)
		return
	}

	agent.RegisterDevWorkflows(ai.NewClient(cfg), cfg.AI.Model)
}
