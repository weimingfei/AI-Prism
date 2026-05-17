package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	compat "github.com/ai-prism/backend/apps/interviewcompat"
)

const (
	flowStatusInit       = "INIT"
	flowStatusAsking     = "ASKING"
	flowStatusEvaluating = "EVALUATING"
	flowStatusFollowUp   = "FOLLOW_UP"
	flowStatusCompleted  = "COMPLETED"
)

var legalFlowTransitions = map[string]map[string]bool{
	flowStatusInit: {
		flowStatusAsking:    true,
		flowStatusCompleted: true,
	},
	flowStatusAsking: {
		flowStatusEvaluating: true,
		flowStatusFollowUp:   true,
		flowStatusCompleted:  true,
	},
	flowStatusEvaluating: {
		flowStatusAsking:    true,
		flowStatusFollowUp:  true,
		flowStatusCompleted: true,
	},
	flowStatusFollowUp: {
		flowStatusEvaluating: true,
		flowStatusAsking:     true,
		flowStatusCompleted:  true,
	},
	flowStatusCompleted: {},
}

// 回答链路只通过这个状态机推进。这样做是为了把“评分中、追问中、已完成”
// 这些中间状态显式落到 Redis，刷新页面或后端重启后仍能恢复到正确位置。
type answerFlowDecision struct {
	NeedFollowUp         bool
	NextQuestionNumber   string
	NextQuestion         string
	NextFollowUpCount    int
	ResolvedMaxFollowUps int
	Finished             bool
}

func (s *ServiceImpl) ensureFlowLocked(ctx context.Context, session *compat.Session, maxFollowUps int) runtimeFlowState {
	if session == nil {
		return runtimeFlowState{Status: flowStatusInit}
	}
	if flow, ok := s.loadFlowFromRedis(ctx, session.SessionID); ok {
		if maxFollowUps >= 0 {
			flow.MaxFollowUp = maxFollowUps
		}
		if flow.TotalQuestions <= 0 {
			flow.TotalQuestions = len(session.Questions)
		}
		if strings.TrimSpace(flow.CurrentQuestionNumber) == "" {
			flow.CurrentQuestionNumber = currentMainQuestionNumber(session)
		}
		s.saveFlowToRedis(ctx, session.SessionID, flow)
		return flow
	}
	status := flowStatusAsking
	if session.Status == "COMPLETED" {
		status = flowStatusCompleted
	}
	flow := runtimeFlowState{
		Status:                status,
		CurrentIndex:          session.CurrentIndex,
		CurrentQuestionNumber: currentMainQuestionNumber(session),
		FollowUpCount:         0,
		MaxFollowUp:           maxFollowUps,
		TotalQuestions:        len(session.Questions),
		Version:               1,
		UpdatedAt:             time.Now(),
	}
	if flow.CurrentIndex <= 0 {
		flow.CurrentIndex = questionIndex(flow.CurrentQuestionNumber)
	}
	if flow.MaxFollowUp < 0 {
		flow.MaxFollowUp = 2
	}
	s.saveFlowToRedis(ctx, session.SessionID, flow)
	return flow
}

func (s *ServiceImpl) moveFlowToEvaluatingLocked(ctx context.Context, session *compat.Session, flow runtimeFlowState) (runtimeFlowState, error) {
	return s.transitionFlowLocked(ctx, session, flow, flowStatusEvaluating)
}

func (s *ServiceImpl) startFollowUpQuestionLocked(ctx context.Context, session *compat.Session, flow runtimeFlowState, questionNumber string, maxFollowUps int) (runtimeFlowState, error) {
	if !isLegalFlowTransition(flow.Status, flowStatusFollowUp) && flow.Status != flowStatusFollowUp {
		return flow, illegalFlowTransition(flow.Status, flowStatusFollowUp)
	}
	flow.Status = flowStatusFollowUp
	flow.CurrentQuestionNumber = strings.TrimSpace(questionNumber)
	flow.CurrentIndex = questionIndex(questionNumber)
	flow.FollowUpCount = followUpDepth(questionNumber)
	flow.MaxFollowUp = maxFollowUps
	flow.TotalQuestions = len(session.Questions)
	flow.Version++
	flow.UpdatedAt = time.Now()
	s.saveFlowToRedis(ctx, session.SessionID, flow)
	return flow, nil
}

func (s *ServiceImpl) moveFlowToAskingLocked(ctx context.Context, session *compat.Session, flow runtimeFlowState) (runtimeFlowState, error) {
	next, err := s.transitionFlowLocked(ctx, session, flow, flowStatusAsking)
	if err != nil {
		return flow, err
	}
	next.CurrentQuestionNumber = currentMainQuestionNumber(session)
	next.CurrentIndex = questionIndex(next.CurrentQuestionNumber)
	next.FollowUpCount = 0
	s.saveFlowToRedis(ctx, session.SessionID, next)
	return next, nil
}

func (s *ServiceImpl) markFlowCompletedLocked(ctx context.Context, session *compat.Session, flow runtimeFlowState) (runtimeFlowState, error) {
	next, err := s.transitionFlowLocked(ctx, session, flow, flowStatusCompleted)
	if err != nil {
		return flow, err
	}
	session.Status = "COMPLETED"
	s.saveFlowToRedis(ctx, session.SessionID, next)
	return next, nil
}

func (s *ServiceImpl) transitionFlowLocked(ctx context.Context, session *compat.Session, flow runtimeFlowState, target string) (runtimeFlowState, error) {
	source := normalizeFlowStatus(flow.Status)
	target = normalizeFlowStatus(target)
	if source == target {
		return flow, nil
	}
	if !isLegalFlowTransition(source, target) {
		return flow, illegalFlowTransition(source, target)
	}
	flow.Status = target
	flow.TotalQuestions = len(session.Questions)
	flow.Version++
	flow.UpdatedAt = time.Now()
	s.saveFlowToRedis(ctx, session.SessionID, flow)
	return flow, nil
}

func (s *ServiceImpl) decideAndAdvanceAnswerFlowLocked(ctx context.Context, session *compat.Session, flow runtimeFlowState, diagnosisScore int, suggestedFollowUp string, maxFollowUps int) (answerFlowDecision, runtimeFlowState, error) {
	currentFollowUpCount := followUpDepth(flow.CurrentQuestionNumber)
	nextFollowUpCount := currentFollowUpCount + 1
	maxFollowUps = clampFollowUpLimit(maxFollowUps)
	shouldFollowUp := strings.TrimSpace(suggestedFollowUp) != "" && nextFollowUpCount <= maxFollowUps && diagnosisScore < 90
	finished := allKnowledgePointsClosed(session.KnowledgeList)

	if shouldFollowUp {
		// 追问次数由运行态里的题号深度计算，不依赖前端传值，避免刷新后重新从 0 开始。
		nextQuestion := optimizeFollowUpQuestion(suggestedFollowUp, nextFollowUpCount)
		if nextQuestion == "" {
			nextQuestion = fmt.Sprintf("第 %d 次追问：请再用一个反例说明这个知识点的边界。", nextFollowUpCount)
		}
		nextNumber := fmt.Sprintf("%s-F%d", baseQuestionNumber(flow.CurrentQuestionNumber), nextFollowUpCount)
		session.Questions[nextNumber] = nextQuestion
		nextFlow, err := s.startFollowUpQuestionLocked(ctx, session, flow, nextNumber, maxFollowUps)
		if err != nil {
			return answerFlowDecision{}, flow, err
		}
		return answerFlowDecision{
			NeedFollowUp:         true,
			NextQuestionNumber:   nextNumber,
			NextQuestion:         nextQuestion,
			NextFollowUpCount:    nextFollowUpCount,
			ResolvedMaxFollowUps: maxFollowUps,
			Finished:             false,
		}, nextFlow, nil
	}

	if finished {
		nextFlow, err := s.markFlowCompletedLocked(ctx, session, flow)
		if err != nil {
			return answerFlowDecision{}, flow, err
		}
		return answerFlowDecision{ResolvedMaxFollowUps: maxFollowUps, Finished: true}, nextFlow, nil
	}

	nextFlow, err := s.moveFlowToAskingLocked(ctx, session, flow)
	if err != nil {
		return answerFlowDecision{}, flow, err
	}
	return answerFlowDecision{ResolvedMaxFollowUps: maxFollowUps, Finished: false}, nextFlow, nil
}

func (s *ServiceImpl) loadFlowFromRedis(ctx context.Context, sessionID string) (runtimeFlowState, bool) {
	if s.redis == nil {
		return runtimeFlowState{}, false
	}
	data, err := s.redis.Get(ctx, runtimeFlowKey(sessionID)).Bytes()
	if err != nil {
		return runtimeFlowState{}, false
	}
	var flow runtimeFlowState
	if json.Unmarshal(data, &flow) != nil {
		return runtimeFlowState{}, false
	}
	flow.Status = normalizeFlowStatus(flow.Status)
	return flow, true
}

func (s *ServiceImpl) saveFlowToRedis(ctx context.Context, sessionID string, flow runtimeFlowState) {
	if s.redis == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	flow.Status = normalizeFlowStatus(flow.Status)
	data, err := json.Marshal(flow)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, runtimeFlowKey(sessionID), data, 24*time.Hour).Err()
}

func isLegalFlowTransition(source string, target string) bool {
	allowed := legalFlowTransitions[normalizeFlowStatus(source)]
	return allowed != nil && allowed[normalizeFlowStatus(target)]
}

func illegalFlowTransition(source string, target string) error {
	return fmt.Errorf("非法练习流程状态流转：%s -> %s", normalizeFlowStatus(source), normalizeFlowStatus(target))
}

func normalizeFlowStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case flowStatusInit, flowStatusAsking, flowStatusEvaluating, flowStatusFollowUp, flowStatusCompleted:
		return strings.ToUpper(strings.TrimSpace(status))
	case "CREATED":
		return flowStatusInit
	case "IN_PROGRESS":
		return flowStatusAsking
	default:
		return flowStatusAsking
	}
}

func currentMainQuestionNumber(session *compat.Session) string {
	if session == nil {
		return "1"
	}
	number := fmt.Sprintf("%d", session.CurrentIndex)
	if strings.TrimSpace(session.Questions[number]) == "" {
		return "1"
	}
	return number
}
