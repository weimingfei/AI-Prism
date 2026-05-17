package impl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	compat "github.com/ai-prism/backend/apps/interviewcompat"
	coach "github.com/ai-prism/backend/apps/prism"
	"github.com/ai-prism/backend/internal/agent"
	"github.com/ai-prism/backend/internal/ai"
	"github.com/ai-prism/backend/internal/config"
	"github.com/ai-prism/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/infraboard/mcube/v2/ioc"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

func init() {
	ioc.Controller().Registry(&ServiceImpl{
		sessions: map[string]*compat.Session{},
		files:    map[string][]byte{},
	})
}

var ErrSessionNotFound = errors.New("interview compatibility session not found")

type ServiceImpl struct {
	ioc.ObjectImpl

	Coach coach.Service `ioc:"autowire=true;namespace=controllers"`

	mu       sync.RWMutex
	redis    *redis.Client
	db       *gorm.DB
	mongo    *mongo.Database
	outline  *agent.KnowledgeOutlineWorkflow
	coachWF  *agent.FeynmanCoachWorkflow
	sessions map[string]*compat.Session
	files    map[string][]byte
	nextFile int64
}

type coachSessionDocument struct {
	ID         string         `bson:"_id"`
	UserID     string         `bson:"userId"`
	Status     string         `bson:"status"`
	UpdateTime time.Time      `bson:"updateTime"`
	Payload    compat.Session `bson:"payload"`
}

func (s *ServiceImpl) Name() string {
	return compat.AppName
}

func (s *ServiceImpl) Init() error {
	cfg := config.Load()
	if db, err := storage.OpenMySQL(cfg.MySQL); err == nil {
		s.db = db
		_ = s.db.AutoMigrate(&storage.CoachSessionModel{})
	}
	if client, err := storage.OpenMongo(context.Background(), cfg.Mongo); err == nil && client != nil {
		s.mongo = client.Database(cfg.Mongo.Database)
		s.ensureMongoIndexes(context.Background())
	}
	s.redis = storage.OpenRedis(cfg.Redis)
	if err := s.redis.Ping(context.Background()).Err(); err != nil {
		s.redis = nil
	}
	s.outline = agent.NewKnowledgeOutlineWorkflow(
		ai.NewClient(cfg),
		cfg.AI.Model,
		agent.DocumentParserConfig{
			MinerU: agent.MinerUConfig{
				Enabled:        cfg.MinerU.Enabled,
				BaseURL:        cfg.MinerU.BaseURL,
				APIKey:         cfg.MinerU.APIKey,
				ParseEndpoint:  cfg.MinerU.ParseEndpoint,
				TimeoutSeconds: cfg.MinerU.TimeoutSeconds,
			},
		},
	)
	s.coachWF = agent.NewFeynmanCoachWorkflow(ai.NewClient(cfg), cfg.AI.Model)
	return nil
}

func (s *ServiceImpl) CreateSession(ctx context.Context, userID string) (*compat.CreateSessionResult, error) {
	now := time.Now()
	sessionID := uuid.NewString()
	session := &compat.Session{
		SessionID:    sessionID,
		UserID:       defaultUser(userID),
		Status:       "CREATED",
		Questions:    defaultQuestions("学习资料"),
		Suggestions:  defaultSuggestions(),
		CurrentIndex: 1,
		TotalScore:   0,
		CreateTime:   now,
		UpdateTime:   now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
	s.saveSession(ctx, session)
	return &compat.CreateSessionResult{SessionID: sessionID, Status: session.Status}, nil
}

func (s *ServiceImpl) PageConversations(ctx context.Context, current int, size int) (*compat.PageResult[compat.ConversationItem], error) {
	current, size = normalizePage(current, size)
	sessions := s.listSessions(ctx)
	s.mu.RLock()
	for _, session := range s.sessions {
		if _, exists := sessions[session.SessionID]; !exists {
			sessions[session.SessionID] = session
		}
	}
	s.mu.RUnlock()

	records := make([]compat.ConversationItem, 0, len(sessions))
	for _, session := range sessions {
		records = append(records, compat.ConversationItem{
			SessionID:         session.SessionID,
			ConversationTitle: conversationTitle(session),
			Status:            session.Status,
			InterviewType:     learningTopic(session),
			ResumeFileURL:     session.MaterialName,
			CreateTime:        session.CreateTime,
			UpdateTime:        session.UpdateTime,
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].UpdateTime.After(records[j].UpdateTime)
	})
	return page(records, current, size), nil
}

func (s *ServiceImpl) ExtractQuestions(ctx context.Context, sessionID string, username string, fileName string, content []byte, contentType string) (*compat.ExtractQuestionsResult, error) {
	unlockRuntime := s.acquireRuntimeLock(ctx, sessionID, 2*time.Minute)
	if unlockRuntime == nil {
		return nil, errorsNewRuntimeBusy()
	}
	defer unlockRuntime()

	s.mu.Lock()
	defer s.mu.Unlock()

	view := s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadWriteRequired, runtimeScopeFullRuntime)
	if view.Session == nil {
		return nil, ErrSessionNotFound
	}
	session := view.Session

	outline := s.outline.Generate(ctx, agent.KnowledgeOutlineInput{
		FileName: fileName,
		Content:  content,
	})
	title := strings.TrimSpace(outline.Title)
	if title == "" {
		title = materialTitle(fileName)
	}
	questions := questionsFromOutline(outline)
	suggestions := suggestionsFromOutline(outline)
	knowledgeList := knowledgeItemsFromOutline(outline)

	session.MaterialName = fileName
	session.MaterialBytes = append([]byte(nil), content...)
	session.ContentType = contentType
	session.OutlineTitle = title
	session.OutlineSummary = outline.Summary
	session.ResumeScore = outline.Score
	session.Questions = questions
	session.Suggestions = suggestions
	session.KnowledgeList = knowledgeList
	session.Status = "IN_PROGRESS"
	session.CurrentIndex = 1
	session.UpdateTime = time.Now()
	s.saveSession(ctx, session)

	return &compat.ExtractQuestionsResult{
		ID:              uuid.NewString(),
		SessionID:       sessionID,
		UserName:        defaultUser(username),
		AgentID:         1,
		Questions:       cloneMap(session.Questions),
		Suggestions:     cloneMap(session.Suggestions),
		KnowledgeList:   cloneKnowledgeList(session.KnowledgeList),
		OutlineTitle:    title,
		OutlineSummary:  outline.Summary,
		InterviewType:   title,
		ResumeFileURL:   fileName,
		ResponseTime:    0,
		TokenCount:      0,
		ResumeScore:     outline.Score,
		QuestionCount:   len(session.Questions),
		SuggestionCount: len(session.Suggestions),
		IsSuccess:       1,
		CreateTime:      session.CreateTime,
		UpdateTime:      session.UpdateTime,
	}, nil
}

func (s *ServiceImpl) PreviewMaterial(ctx context.Context, sessionID string) ([]byte, string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.getSessionLocked(ctx, sessionID)
	if !ok {
		return nil, "", "", ErrSessionNotFound
	}
	if len(session.MaterialBytes) == 0 {
		session.MaterialBytes = s.loadFile(ctx, session.SessionID)
	}
	if len(session.MaterialBytes) == 0 {
		return nil, "", "", errors.New("material preview is not ready")
	}
	contentType := session.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/pdf"
	}
	return append([]byte(nil), session.MaterialBytes...), session.MaterialName, contentType, nil
}

func (s *ServiceImpl) Restore(ctx context.Context, sessionID string) (*compat.RestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadOnly, runtimeScopeFullRuntime)
	if view.Session == nil {
		return nil, ErrSessionNotFound
	}
	session := view.Session
	return &compat.RestoreResult{
		SessionID:     session.SessionID,
		Status:        session.Status,
		CanResume:     session.Status != "COMPLETED",
		CanWrite:      view.CanWrite,
		ResumeFileURL: session.MaterialName,
		ResumeScore:   resumeScore(session),
		InterviewType: learningTopic(session),
		Suggestions:   cloneMap(session.Suggestions),
		KnowledgeList: cloneKnowledgeList(session.KnowledgeList),
		LoadMode:      view.LoadMode,
		RestoreSource: view.RestoreSource,
		Confidence:    view.Confidence,
		CacheRebuilt:  view.CacheRebuilt,
	}, nil
}

func (s *ServiceImpl) CurrentQuestion(ctx context.Context, sessionID string) (*compat.AnswerResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadWriteRequired, runtimeScopeHotRuntime)
	session := view.Session
	if session == nil {
		now := time.Now()
		session = &compat.Session{
			SessionID:    sessionID,
			UserID:       "demo-user",
			Status:       "IN_PROGRESS",
			Questions:    defaultQuestions("学习资料"),
			Suggestions:  defaultSuggestions(),
			CurrentIndex: 1,
			TotalScore:   0,
			CreateTime:   now,
			UpdateTime:   now,
		}
		s.sessions[sessionID] = session
		s.saveSession(ctx, session)
	}
	flow := s.ensureFlowLocked(ctx, session, 2)
	if strings.TrimSpace(flow.CurrentQuestionNumber) != "" && normalizeFlowStatus(flow.Status) != flowStatusCompleted {
		question := session.Questions[flow.CurrentQuestionNumber]
		if strings.TrimSpace(question) != "" {
			next := question
			return &compat.AnswerResult{
				QuestionNumber:  flow.CurrentQuestionNumber,
				QuestionContent: question,
				Score:           session.TotalScore,
				TotalScore:      session.TotalScore,
				IsSuccess:       true,
				NextQuestion:    &next,
				IsFollowUp:      normalizeFlowStatus(flow.Status) == flowStatusFollowUp,
				FollowUpNeeded:  false,
				FollowUpCount:   flow.FollowUpCount,
				MissingPoints:   map[string]string{},
				KnowledgeList:   cloneKnowledgeList(session.KnowledgeList),
				Finished:        false,
			}, nil
		}
	}
	return questionResult(session, session.Status == "COMPLETED"), nil
}

func (s *ServiceImpl) SelectKnowledgePoint(ctx context.Context, sessionID string, pointID string) (*compat.KnowledgePointActionResult, error) {
	unlockRuntime := s.acquireRuntimeLock(ctx, sessionID, 20*time.Second)
	if unlockRuntime == nil {
		return nil, errorsNewRuntimeBusy()
	}
	defer unlockRuntime()

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.getSessionLocked(ctx, sessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}
	index := knowledgeIndexByID(session.KnowledgeList, pointID)
	if index < 0 {
		return nil, errors.New("knowledge point not found")
	}
	session.CurrentIndex = index + 1
	for itemIndex := range session.KnowledgeList {
		if session.KnowledgeList[itemIndex].Status == "active" {
			session.KnowledgeList[itemIndex].Status = "pending"
		}
	}
	if session.KnowledgeList[index].Status == "" || session.KnowledgeList[index].Status == "pending" {
		session.KnowledgeList[index].Status = "active"
	}
	session.UpdateTime = time.Now()
	_, _ = s.moveFlowToAskingLocked(ctx, session, s.ensureFlowLocked(ctx, session, 2))
	s.saveSession(ctx, session)
	question := questionResult(session, false)
	return &compat.KnowledgePointActionResult{
		Question:      question,
		KnowledgeList: cloneKnowledgeList(session.KnowledgeList),
		Finished:      allKnowledgePointsClosed(session.KnowledgeList),
	}, nil
}

func (s *ServiceImpl) SkipKnowledgePoint(ctx context.Context, sessionID string, pointID string) (*compat.KnowledgePointActionResult, error) {
	unlockRuntime := s.acquireRuntimeLock(ctx, sessionID, 20*time.Second)
	if unlockRuntime == nil {
		return nil, errorsNewRuntimeBusy()
	}
	defer unlockRuntime()

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.getSessionLocked(ctx, sessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}
	index := knowledgeIndexByID(session.KnowledgeList, pointID)
	if index < 0 {
		return nil, errors.New("knowledge point not found")
	}
	if session.KnowledgeList[index].Status != "passed" && session.KnowledgeList[index].Status != "failed" {
		session.KnowledgeList[index].Status = "skipped"
	}
	if session.CurrentIndex == index+1 {
		session.CurrentIndex = nextPendingKnowledgeIndex(session.KnowledgeList)
	}
	finished := allKnowledgePointsClosed(session.KnowledgeList)
	if finished {
		session.Status = "COMPLETED"
		_, _ = s.markFlowCompletedLocked(ctx, session, s.ensureFlowLocked(ctx, session, 2))
	} else {
		_, _ = s.moveFlowToAskingLocked(ctx, session, s.ensureFlowLocked(ctx, session, 2))
	}
	session.UpdateTime = time.Now()
	s.saveSession(ctx, session)
	return &compat.KnowledgePointActionResult{
		KnowledgeList: cloneKnowledgeList(session.KnowledgeList),
		Finished:      finished,
	}, nil
}

func (s *ServiceImpl) FinishCurrentKnowledgePoint(ctx context.Context, sessionID string) (*compat.KnowledgePointActionResult, error) {
	unlockRuntime := s.acquireRuntimeLock(ctx, sessionID, 20*time.Second)
	if unlockRuntime == nil {
		return nil, errorsNewRuntimeBusy()
	}
	defer unlockRuntime()

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.getSessionLocked(ctx, sessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}
	index := session.CurrentIndex - 1
	if index < 0 || index >= len(session.KnowledgeList) {
		return nil, errors.New("current knowledge point not found")
	}
	if session.KnowledgeList[index].Status != "failed" && session.KnowledgeList[index].Status != "skipped" {
		session.KnowledgeList[index].Status = "passed"
	}
	if session.KnowledgeList[index].Score == nil {
		score := session.TotalScore
		if score == 0 {
			score = 80
		}
		session.KnowledgeList[index].Score = &score
	}
	finished := allKnowledgePointsClosed(session.KnowledgeList)
	if finished {
		session.Status = "COMPLETED"
		_, _ = s.markFlowCompletedLocked(ctx, session, s.ensureFlowLocked(ctx, session, 2))
	} else {
		_, _ = s.moveFlowToAskingLocked(ctx, session, s.ensureFlowLocked(ctx, session, 2))
	}
	session.UpdateTime = time.Now()
	s.saveSession(ctx, session)
	return &compat.KnowledgePointActionResult{
		KnowledgeList: cloneKnowledgeList(session.KnowledgeList),
		Finished:      finished,
	}, nil
}

func (s *ServiceImpl) Answer(ctx context.Context, sessionID string, req compat.AnswerRequest) (*compat.AnswerResult, error) {
	unlockRuntime := s.acquireRuntimeLock(ctx, sessionID, 2*time.Minute)
	if unlockRuntime == nil {
		return nil, errorsNewRuntimeBusy()
	}
	defer unlockRuntime()

	answer := strings.TrimSpace(req.AnswerContent)
	if answer == "" {
		answer = "我还没有讲清楚这个知识点。"
	}
	maxFollowUps := clampFollowUpLimit(req.MaxFollowUpCount)

	s.mu.Lock()
	view := s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadWriteRequired, runtimeScopeHotRuntime)
	session := view.Session
	if session == nil {
		s.mu.Unlock()
		return nil, ErrSessionNotFound
	}
	if !view.CanWrite {
		s.mu.Unlock()
		return nil, fmt.Errorf("当前会话已结束或只能只读恢复，不能继续答题")
	}
	flow := s.ensureFlowLocked(ctx, session, maxFollowUps)
	if normalizeFlowStatus(flow.Status) == flowStatusCompleted {
		s.mu.Unlock()
		return questionResult(session, true), nil
	}
	requestedQuestionNumber := normalizeQuestionNumber(req.QuestionNumber, session.CurrentIndex)
	questionNumber := strings.TrimSpace(flow.CurrentQuestionNumber)
	if questionNumber == "" {
		questionNumber = requestedQuestionNumber
	}
	if requestedQuestionNumber != questionNumber {
		s.mu.Unlock()
		return nil, fmt.Errorf("题号已过期，请刷新当前问题后重试")
	}
	question := session.Questions[questionNumber]
	if strings.TrimSpace(question) == "" {
		s.mu.Unlock()
		return nil, fmt.Errorf("当前问题不存在或已过期")
	}
	flow, err := s.moveFlowToEvaluatingLocked(ctx, session, flow)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Unlock()

	diagnosis, err := s.diagnoseAnswer(ctx, coach.DiagnoseRequest{
		SessionID:    sessionID,
		Explanation:  answer,
		FollowUpMode: normalizeFollowUpMode(req.FollowUpMode),
	})
	if err != nil {
		return nil, err
	}

	score := diagnosis.MasteryScore
	currentFollowUpCount := followUpDepth(questionNumber)
	missing := mapFromSlice(diagnosis.Diagnosis.MissingParts)
	feedback := strings.TrimSpace(diagnosis.Correction)
	if feedback == "" {
		feedback = "建议按“定义 -> 例子 -> 反例 -> 边界”的结构重新讲解。"
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	view = s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadWriteRequired, runtimeScopeHotRuntime)
	session = view.Session
	if session == nil {
		return nil, ErrSessionNotFound
	}
	if !view.CanWrite {
		return nil, fmt.Errorf("当前会话已结束或只能只读恢复，不能继续答题")
	}
	lockedFlow := s.ensureFlowLocked(ctx, session, maxFollowUps)
	if lockedFlow.CurrentQuestionNumber != flow.CurrentQuestionNumber || normalizeFlowStatus(lockedFlow.Status) != flowStatusEvaluating {
		return nil, fmt.Errorf("题目状态已变化，请刷新当前问题后重试")
	}

	if diagnosis.Intent == "unknown" || diagnosis.Intent == "show_reference" {
		session.TotalScore = 0
		markKnowledgePointResult(session, questionNumber, 0)
		nextFlow, err := s.moveFlowToAskingLocked(ctx, session, lockedFlow)
		if err != nil {
			return nil, err
		}
		feedback = "我判断你现在还没有形成可讲解的答案。你可以选择“结束当前知识点”跳过，也可以先看下面的完整参考答案，再回到知识点清单继续练习。\n\n" + referenceAnswerForQuestion(session, questionNumber, question)
		session.Turns = append(session.Turns, compat.Turn{
			QuestionNumber: questionNumber,
			Question:       question,
			Answer:         answer,
			Score:          0,
			Feedback:       feedback,
			IsFollowUp:     currentFollowUpCount > 0,
			FollowUpCount:  currentFollowUpCount,
			CreateTime:     now,
		})
		session.UpdateTime = now
		s.saveSession(ctx, session)
		return &compat.AnswerResult{
			QuestionNumber:  questionNumber,
			QuestionContent: question,
			Score:           0,
			TotalScore:      0,
			IsSuccess:       true,
			Feedback:        feedback,
			FollowUpNeeded:  false,
			FollowUpCount:   currentFollowUpCount,
			MissingPoints:   map[string]string{"1": "尚未形成可讲解答案"},
			KnowledgeList:   cloneKnowledgeList(session.KnowledgeList),
			Finished:        normalizeFlowStatus(nextFlow.Status) == flowStatusCompleted,
			NeedsChoice:     true,
			ReferenceAnswer: referenceAnswerForQuestion(session, questionNumber, question),
		}, nil
	}

	session.TotalScore = score
	markKnowledgePointResult(session, questionNumber, score)
	decision, nextFlow, err := s.decideAndAdvanceAnswerFlowLocked(ctx, session, lockedFlow, score, strings.TrimSpace(diagnosis.FollowUp.Question), maxFollowUps)
	if err != nil {
		return nil, err
	}
	session.Turns = append(session.Turns, compat.Turn{
		QuestionNumber: questionNumber,
		Question:       question,
		Answer:         answer,
		Score:          score,
		Feedback:       feedback,
		IsFollowUp:     currentFollowUpCount > 0,
		FollowUpCount:  currentFollowUpCount,
		CreateTime:     now,
	})
	session.UpdateTime = now
	s.saveSession(ctx, session)
	if strings.TrimSpace(req.RequestID) != "" {
		s.refreshRuntimeSnapshots(ctx, session, "ANSWER_COMMITTED", req.RequestID)
	}

	if !decision.NeedFollowUp {
		return &compat.AnswerResult{
			QuestionNumber:  questionNumber,
			QuestionContent: question,
			Score:           score,
			TotalScore:      score,
			IsSuccess:       true,
			Feedback:        feedback + "\n\n本知识点问答已结束，你可以从知识点清单选择下一个知识点继续练习。",
			IsFollowUp:      currentFollowUpCount > 0,
			FollowUpNeeded:  false,
			FollowUpCount:   currentFollowUpCount,
			MissingPoints:   missing,
			KnowledgeList:   cloneKnowledgeList(session.KnowledgeList),
			Finished:        decision.Finished || normalizeFlowStatus(nextFlow.Status) == flowStatusCompleted,
		}, nil
	}

	return &compat.AnswerResult{
		QuestionNumber:     questionNumber,
		QuestionContent:    question,
		Score:              score,
		TotalScore:         score,
		IsSuccess:          true,
		Feedback:           feedback,
		NextQuestion:       &decision.NextQuestion,
		NextQuestionNumber: &decision.NextQuestionNumber,
		IsFollowUp:         true,
		FollowUpNeeded:     true,
		FollowUpCount:      decision.NextFollowUpCount,
		AskToUser:          &decision.NextQuestion,
		MissingPoints:      missing,
		KnowledgeList:      cloneKnowledgeList(session.KnowledgeList),
		Finished:           false,
	}, nil
}

func (s *ServiceImpl) answerLegacy(ctx context.Context, sessionID string, req compat.AnswerRequest) (*compat.AnswerResult, error) {
	unlockRuntime := s.acquireRuntimeLock(ctx, sessionID, 2*time.Minute)
	if unlockRuntime == nil {
		return nil, errorsNewRuntimeBusy()
	}
	defer unlockRuntime()

	answer := strings.TrimSpace(req.AnswerContent)
	if answer == "" {
		answer = "我还没有讲清楚这个知识点。"
	}
	maxFollowUps := clampFollowUpLimit(req.MaxFollowUpCount)

	s.mu.Lock()
	view := s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadWriteRequired, runtimeScopeHotRuntime)
	session := view.Session
	if session == nil {
		s.mu.Unlock()
		return nil, ErrSessionNotFound
	}
	if !view.CanWrite {
		s.mu.Unlock()
		return nil, fmt.Errorf("当前会话已结束或只能只读恢复，不能继续答题")
	}
	questionNumber := normalizeQuestionNumber(req.QuestionNumber, session.CurrentIndex)
	question := session.Questions[questionNumber]
	s.mu.Unlock()

	diagnosis, err := s.diagnoseAnswer(ctx, coach.DiagnoseRequest{
		SessionID:    sessionID,
		Explanation:  answer,
		FollowUpMode: normalizeFollowUpMode(req.FollowUpMode),
	})
	if err != nil {
		return nil, err
	}

	score := diagnosis.MasteryScore
	currentFollowUpCount := followUpDepth(questionNumber)
	nextFollowUpCount := currentFollowUpCount + 1
	nextQuestion := optimizeFollowUpQuestion(strings.TrimSpace(diagnosis.FollowUp.Question), nextFollowUpCount)
	if nextQuestion == "" {
		nextQuestion = fmt.Sprintf("第 %d 次追问：请再用一个反例说明这个知识点的边界。", nextFollowUpCount)
	}
	shouldFollowUp := nextFollowUpCount <= maxFollowUps
	nextNumber := questionNumber + "-F1"
	if shouldFollowUp {
		nextNumber = fmt.Sprintf("%s-F%d", baseQuestionNumber(questionNumber), nextFollowUpCount)
	}
	missing := mapFromSlice(diagnosis.Diagnosis.MissingParts)
	feedback := strings.TrimSpace(diagnosis.Correction)
	if feedback == "" {
		feedback = "建议按“定义 -> 例子 -> 反例 -> 边界”的结构重新讲解。"
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	view = s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadWriteRequired, runtimeScopeHotRuntime)
	session = view.Session
	if session == nil {
		return nil, ErrSessionNotFound
	}
	if !view.CanWrite {
		return nil, fmt.Errorf("当前会话已结束或只能只读恢复，不能继续答题")
	}
	session.TotalScore = score
	markKnowledgePointResult(session, questionNumber, score)
	finished := allKnowledgePointsClosed(session.KnowledgeList)
	if shouldFollowUp {
		session.Questions[nextNumber] = nextQuestion
	} else if finished {
		session.Status = "COMPLETED"
	}
	session.Turns = append(session.Turns, compat.Turn{
		QuestionNumber: questionNumber,
		Question:       question,
		Answer:         answer,
		Score:          score,
		Feedback:       feedback,
		IsFollowUp:     currentFollowUpCount > 0,
		FollowUpCount:  currentFollowUpCount,
		CreateTime:     now,
	})
	session.UpdateTime = now
	s.saveSession(ctx, session)
	if strings.TrimSpace(req.RequestID) != "" {
		s.refreshRuntimeSnapshots(ctx, session, "ANSWER_COMMITTED", req.RequestID)
	}

	if !shouldFollowUp {
		return &compat.AnswerResult{
			QuestionNumber:  questionNumber,
			QuestionContent: question,
			Score:           score,
			TotalScore:      score,
			IsSuccess:       true,
			Feedback:        feedback + "\n\n本知识点问答已结束，你可以从知识点清单选择下一个知识点继续练习。",
			IsFollowUp:      currentFollowUpCount > 0,
			FollowUpNeeded:  false,
			FollowUpCount:   currentFollowUpCount,
			MissingPoints:   missing,
			KnowledgeList:   cloneKnowledgeList(session.KnowledgeList),
			Finished:        finished,
		}, nil
	}

	return &compat.AnswerResult{
		QuestionNumber:     questionNumber,
		QuestionContent:    question,
		Score:              score,
		TotalScore:         score,
		IsSuccess:          true,
		Feedback:           feedback,
		NextQuestion:       &nextQuestion,
		NextQuestionNumber: &nextNumber,
		IsFollowUp:         true,
		FollowUpNeeded:     true,
		FollowUpCount:      nextFollowUpCount,
		AskToUser:          &nextQuestion,
		MissingPoints:      missing,
		KnowledgeList:      cloneKnowledgeList(session.KnowledgeList),
		Finished:           false,
	}, nil
}

func (s *ServiceImpl) Finish(ctx context.Context, sessionID string) error {
	unlockRuntime := s.acquireRuntimeLock(ctx, sessionID, 20*time.Second)
	if unlockRuntime == nil {
		return errorsNewRuntimeBusy()
	}
	defer unlockRuntime()

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.getSessionLocked(ctx, sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	session.Status = "COMPLETED"
	session.UpdateTime = time.Now()
	_, _ = s.markFlowCompletedLocked(ctx, session, s.ensureFlowLocked(ctx, session, 2))
	s.saveSession(ctx, session)
	return nil
}

func (s *ServiceImpl) Radar(ctx context.Context, sessionID string) (*compat.RadarChart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadOnly, runtimeScopeScoreOnly)
	if view.Session == nil {
		return nil, ErrSessionNotFound
	}
	session := view.Session
	radar := radarFromScore(session.TotalScore)
	return &radar, nil
}

func (s *ServiceImpl) Record(ctx context.Context, sessionID string) (*compat.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := s.ensureRuntimeLocked(ctx, sessionID, runtimeLoadModeReadOnly, runtimeScopeFullRuntime)
	if view.Session == nil {
		return nil, ErrSessionNotFound
	}
	session := view.Session
	record := recordFromSession(session)
	return &record, nil
}

func (s *ServiceImpl) PageRecords(ctx context.Context, current int, size int) (*compat.PageResult[compat.Record], error) {
	current, size = normalizePage(current, size)
	sessions := s.listSessions(ctx)
	s.mu.RLock()
	for _, session := range s.sessions {
		if _, exists := sessions[session.SessionID]; !exists {
			sessions[session.SessionID] = session
		}
	}
	s.mu.RUnlock()
	records := make([]compat.Record, 0, len(sessions))
	for _, session := range sessions {
		records = append(records, recordFromSession(session))
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].UpdateTime.After(records[j].UpdateTime)
	})
	return page(records, current, size), nil
}

func (s *ServiceImpl) UploadFile(ctx context.Context, file compat.UploadedFile, content []byte) (*compat.UploadedFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextFile++
	if s.redis != nil {
		if id, err := s.redis.Incr(ctx, interviewFileSeqKey()).Result(); err == nil {
			s.nextFile = id
		}
	}
	file.ID = s.nextFile
	file.CreateTime = time.Now()
	file.FileURL = fmt.Sprintf("/api/lingzhi/v1/agents/files/%d", file.ID)
	s.files[file.FileURL] = append([]byte(nil), content...)
	s.saveFile(ctx, file.FileURL, content)
	return &file, nil
}

func (s *ServiceImpl) Demeanor(ctx context.Context, sessionID string) (string, error) {
	return "学习状态良好，请继续保持清晰、分步骤的讲解节奏。", nil
}

func (s *ServiceImpl) diagnoseAnswer(ctx context.Context, req coach.DiagnoseRequest) (*coach.CoachResult, error) {
	if s.Coach != nil {
		if result, err := s.Coach.Diagnose(ctx, req); err == nil && result != nil {
			return result, nil
		}
	}
	if s.coachWF == nil {
		cfg := config.Load()
		s.coachWF = agent.NewFeynmanCoachWorkflow(ai.NewClient(cfg), cfg.AI.Model)
	}
	result := s.coachWF.Diagnose(ctx, agent.FeynmanCoachInput{
		SessionID:    req.SessionID,
		Explanation:  req.Explanation,
		FollowUpMode: req.FollowUpMode,
	})
	return &coach.CoachResult{
		Diagnosis: coach.Diagnosis{
			CorrectParts:   result.Diagnosis.CorrectParts,
			MissingParts:   result.Diagnosis.MissingParts,
			Misconceptions: result.Diagnosis.Misconceptions,
			Clarity:        result.Diagnosis.Clarity,
		},
		FollowUp: coach.FollowUp{
			Question: result.FollowUp.Question,
			Reason:   result.FollowUp.Reason,
			Targets:  result.FollowUp.Targets,
		},
		Correction:   result.Correction,
		MasteryScore: result.MasteryScore,
		Intent:       result.Intent,
	}, nil
}

func (s *ServiceImpl) getSessionLocked(ctx context.Context, sessionID string) (*compat.Session, bool) {
	if session, ok := s.sessions[sessionID]; ok {
		return session, true
	}
	if s.redis != nil {
		data, err := s.redis.Get(ctx, interviewSessionKey(sessionID)).Bytes()
		if err == nil {
			var session compat.Session
			if json.Unmarshal(data, &session) == nil {
				s.sessions[sessionID] = &session
				return &session, true
			}
		}
	}
	if s.mongo != nil {
		var doc coachSessionDocument
		err := s.mongo.Collection("coach_sessions").FindOne(ctx, bson.M{"_id": sessionID}).Decode(&doc)
		if err == nil {
			session := doc.Payload
			s.sessions[sessionID] = &session
			s.saveSessionToRedis(ctx, &session)
			return &session, true
		}
	}
	return nil, false
}

func (s *ServiceImpl) saveSession(ctx context.Context, session *compat.Session) {
	if session == nil {
		return
	}
	if s.db != nil {
		model := storage.CoachSessionModel{
			ID:           session.SessionID,
			UserID:       session.UserID,
			Status:       session.Status,
			Title:        conversationTitle(session),
			MaterialName: session.MaterialName,
			TotalScore:   session.TotalScore,
			CreatedAt:    session.CreateTime,
			UpdatedAt:    session.UpdateTime,
		}
		_ = s.db.WithContext(ctx).Save(&model).Error
	}
	if s.mongo != nil {
		_, _ = s.mongo.Collection("coach_sessions").ReplaceOne(ctx, bson.M{"_id": session.SessionID}, coachSessionDocument{
			ID:         session.SessionID,
			UserID:     session.UserID,
			Status:     session.Status,
			UpdateTime: session.UpdateTime,
			Payload:    *session,
		}, options.Replace().SetUpsert(true))
	}
	s.saveSessionToRedis(ctx, session)
	s.refreshRuntimeSnapshots(ctx, session, "AUTO", "")
}

func (s *ServiceImpl) saveSessionToRedis(ctx context.Context, session *compat.Session) {
	if s.redis == nil || session == nil {
		return
	}
	data, err := json.Marshal(session)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, interviewSessionKey(session.SessionID), data, 7*24*time.Hour).Err()
	_ = s.redis.ZAdd(ctx, interviewSessionIndexKey(), redis.Z{
		Score:  float64(session.UpdateTime.UnixMilli()),
		Member: session.SessionID,
	}).Err()
}

func (s *ServiceImpl) listSessions(ctx context.Context) map[string]*compat.Session {
	result := map[string]*compat.Session{}
	if s.mongo != nil {
		cursor, err := s.mongo.Collection("coach_sessions").Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "updateTime", Value: -1}}))
		if err == nil {
			defer cursor.Close(ctx)
			var docs []coachSessionDocument
			if cursor.All(ctx, &docs) == nil {
				for index := range docs {
					session := docs[index].Payload
					result[session.SessionID] = &session
				}
			}
		}
	}
	if s.redis == nil {
		return result
	}
	ids, err := s.redis.ZRevRange(ctx, interviewSessionIndexKey(), 0, -1).Result()
	if err != nil {
		return result
	}
	for _, id := range ids {
		data, err := s.redis.Get(ctx, interviewSessionKey(id)).Bytes()
		if err != nil {
			continue
		}
		var session compat.Session
		if json.Unmarshal(data, &session) == nil {
			result[session.SessionID] = &session
		}
	}
	return result
}

func (s *ServiceImpl) saveFile(ctx context.Context, fileURL string, content []byte) {
	if strings.TrimSpace(fileURL) == "" {
		return
	}
	if s.redis != nil {
		_ = s.redis.Set(ctx, interviewFileKey(fileURL), content, 7*24*time.Hour).Err()
	}
	if s.mongo != nil {
		_, _ = s.mongo.Collection("coach_files").ReplaceOne(ctx, bson.M{"_id": fileURL}, bson.M{
			"_id":       fileURL,
			"content":   content,
			"updatedAt": time.Now(),
		}, options.Replace().SetUpsert(true))
	}
}

func (s *ServiceImpl) loadFile(ctx context.Context, sessionID string) []byte {
	if s.mongo == nil {
		return nil
	}
	var doc coachSessionDocument
	if err := s.mongo.Collection("coach_sessions").FindOne(ctx, bson.M{"_id": sessionID}).Decode(&doc); err != nil {
		return nil
	}
	return append([]byte(nil), doc.Payload.MaterialBytes...)
}

func (s *ServiceImpl) ensureMongoIndexes(ctx context.Context) {
	if s.mongo == nil {
		return
	}
	_, _ = s.mongo.Collection("coach_sessions").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "updateTime", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
	})
	_, _ = s.mongo.Collection("coach_runtime_hot_snapshots").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "sessionId", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "snapshotUpdatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "sessionStatus", Value: 1}}},
	})
	_, _ = s.mongo.Collection("coach_runtime_cold_snapshots").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "sessionId", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "materialUpdatedAt", Value: -1}}},
	})
	_, _ = s.mongo.Collection("coach_runtime_turn_archives").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "sessionId", Value: 1}, {Key: "seq", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "sessionId", Value: 1}, {Key: "digest", Value: 1}}},
	})
}

func interviewSessionKey(sessionID string) string {
	return "ai-prism:coach:session:" + sessionID
}

func interviewSessionIndexKey() string {
	return "ai-prism:coach:sessions"
}

func interviewFileKey(fileURL string) string {
	return "ai-prism:coach:file:" + fileURL
}

func interviewFileSeqKey() string {
	return "ai-prism:coach:file_seq"
}

func defaultQuestions(title string) map[string]string {
	return map[string]string{
		"1": fmt.Sprintf("请用自己的话讲清楚《%s》的核心概念，并举一个初学者能理解的例子。", title),
		"2": "这个知识点最容易和哪个概念混淆？请说明区别。",
		"3": "请给出一个不适用场景或反例，说明它的边界。",
	}
}

func questionsFromOutline(outline agent.KnowledgeOutlineOutput) map[string]string {
	if len(outline.Points) == 0 {
		return defaultQuestions(outline.Title)
	}
	result := make(map[string]string, len(outline.Points))
	for index, point := range outline.Points {
		prompt := strings.TrimSpace(point.CheckPrompt)
		if prompt == "" {
			prompt = fmt.Sprintf("请用自己的话讲解“%s”，并举一个例子。", point.Title)
		}
		result[fmt.Sprintf("%d", index+1)] = prompt
	}
	return result
}

func suggestionsFromOutline(outline agent.KnowledgeOutlineOutput) map[string]string {
	if len(outline.Suggestions) == 0 {
		return defaultSuggestions()
	}
	result := make(map[string]string, len(outline.Suggestions))
	for index, item := range outline.Suggestions {
		result[fmt.Sprintf("%d", index+1)] = item
	}
	return result
}

func knowledgeItemsFromOutline(outline agent.KnowledgeOutlineOutput) []compat.KnowledgeItem {
	items := make([]compat.KnowledgeItem, 0, len(outline.Points))
	for _, point := range outline.Points {
		items = append(items, compat.KnowledgeItem{
			ID:          point.ID,
			Title:       point.Title,
			Summary:     point.Summary,
			Keywords:    append([]string(nil), point.Keywords...),
			Difficulty:  point.Difficulty,
			CheckPrompt: point.CheckPrompt,
			Status:      "pending",
		})
	}
	return items
}

func defaultSuggestions() map[string]string {
	return map[string]string{
		"1": "先用一句话定义知识点，再补充使用场景。",
		"2": "每次讲解至少包含一个例子和一个反例。",
		"3": "把不确定的概念沉淀成 Markdown 知识卡片。",
	}
}

func questionResult(session *compat.Session, finished bool) *compat.AnswerResult {
	number := fmt.Sprintf("%d", session.CurrentIndex)
	question := session.Questions[number]
	if strings.TrimSpace(question) == "" {
		number = "1"
		question = session.Questions[number]
	}
	next := question
	return &compat.AnswerResult{
		QuestionNumber:  number,
		QuestionContent: question,
		Score:           session.TotalScore,
		TotalScore:      session.TotalScore,
		IsSuccess:       true,
		Feedback:        "",
		NextQuestion:    &next,
		IsFollowUp:      false,
		FollowUpNeeded:  false,
		MissingPoints:   map[string]string{},
		KnowledgeList:   cloneKnowledgeList(session.KnowledgeList),
		Finished:        finished,
	}
}

func knowledgeIndexByID(items []compat.KnowledgeItem, pointID string) int {
	pointID = strings.TrimSpace(pointID)
	for index, item := range items {
		if item.ID == pointID || fmt.Sprintf("%d", index+1) == pointID {
			return index
		}
	}
	return -1
}

func markKnowledgePointResult(session *compat.Session, questionNumber string, score int) {
	index := questionIndex(questionNumber) - 1
	if index < 0 || index >= len(session.KnowledgeList) {
		return
	}
	session.KnowledgeList[index].Score = &score
	if score >= 60 {
		session.KnowledgeList[index].Status = "passed"
		return
	}
	session.KnowledgeList[index].Status = "failed"
}

func isUnknownAnswer(answer string) bool {
	normalized := strings.TrimSpace(strings.ToLower(answer))
	if normalized == "" {
		return true
	}
	unknownPhrases := []string{
		"不会", "不知道", "不清楚", "忘了", "我忘了", "没思路", "不会答", "不知道答案",
		"i don't know", "dont know", "do not know", "no idea",
	}
	for _, phrase := range unknownPhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

func referenceAnswerForQuestion(session *compat.Session, questionNumber string, question string) string {
	index := questionIndex(questionNumber) - 1
	if index >= 0 && index < len(session.KnowledgeList) {
		item := session.KnowledgeList[index]
		parts := []string{
			"参考答案：",
			"1. 先说明核心概念：" + strings.TrimSpace(item.Title),
		}
		if strings.TrimSpace(item.Summary) != "" {
			parts = append(parts, "2. 用自己的话解释："+strings.TrimSpace(item.Summary))
		}
		if strings.TrimSpace(item.CheckPrompt) != "" {
			parts = append(parts, "3. 练习时重点回答："+strings.TrimSpace(item.CheckPrompt))
		}
		if len(item.Keywords) > 0 {
			parts = append(parts, "4. 关键词："+strings.Join(item.Keywords, "、"))
		}
		parts = append(parts, "5. 最后补一个例子或反例，说明这个知识点在什么场景下成立、在什么边界下不适用。")
		return strings.Join(parts, "\n")
	}
	return "参考答案：\n1. 先直接回答问题：" + strings.TrimSpace(question) + "\n2. 再补充一个初学者能理解的例子。\n3. 最后说明这个解释的边界、反例或容易误解的地方。"
}

func questionIndex(questionNumber string) int {
	questionNumber = strings.TrimSpace(questionNumber)
	base := baseQuestionNumber(questionNumber)
	var index int
	_, _ = fmt.Sscanf(base, "%d", &index)
	return index
}

func baseQuestionNumber(questionNumber string) string {
	questionNumber = strings.TrimSpace(questionNumber)
	if questionNumber == "" {
		return "1"
	}
	return strings.Split(questionNumber, "-")[0]
}

func followUpDepth(questionNumber string) int {
	questionNumber = strings.TrimSpace(questionNumber)
	if questionNumber == "" {
		return 0
	}
	parts := strings.Split(questionNumber, "-F")
	if len(parts) < 2 {
		return 0
	}
	var round int
	_, _ = fmt.Sscanf(parts[len(parts)-1], "%d", &round)
	return round
}

func clampFollowUpLimit(limit int) int {
	if limit < 0 {
		return 0
	}
	if limit > 5 {
		return 5
	}
	return limit
}

func optimizeFollowUpQuestion(question string, round int) string {
	question = strings.TrimSpace(strings.ReplaceAll(question, "\n", " "))
	if question == "" {
		return ""
	}
	runes := []rune(question)
	if len(runes) > 120 {
		question = string(runes[:120]) + "..."
	}
	return fmt.Sprintf("第 %d 次追问：%s", round, question)
}

func normalizeFollowUpMode(mode string) string {
	if strings.TrimSpace(mode) == "divergent" {
		return "divergent"
	}
	return "boundary"
}

func nextPendingKnowledgeIndex(items []compat.KnowledgeItem) int {
	for index, item := range items {
		if item.Status == "" || item.Status == "pending" || item.Status == "active" {
			return index + 1
		}
	}
	return 1
}

func allKnowledgePointsClosed(items []compat.KnowledgeItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		switch item.Status {
		case "passed", "failed", "skipped":
		default:
			return false
		}
	}
	return true
}

func recordFromSession(session *compat.Session) compat.Record {
	score := session.TotalScore
	if score == 0 {
		score = 60
	}
	qa := make([]compat.QAReview, 0, len(session.Turns))
	for index, turn := range session.Turns {
		qa = append(qa, compat.QAReview{
			Seq:            index + 1,
			QuestionNumber: turn.QuestionNumber,
			Question:       turn.Question,
			Answer:         turn.Answer,
			Score:          turn.Score,
			Feedback:       turn.Feedback,
			IsFollowUp:     turn.IsFollowUp,
			FollowUpNeeded: true,
			FollowUpCount:  turn.FollowUpCount,
		})
	}
	return compat.Record{
		ID:                      1,
		UserID:                  1,
		SessionID:               session.SessionID,
		ResumeScore:             resumeScore(session),
		InterviewScore:          score,
		InterviewStatus:         session.Status,
		QuestionCount:           len(session.Questions),
		CompositeScore:          (resumeScore(session) + score) / 2,
		TotalScore:              score,
		FinalScore:              score,
		InterviewSuggestionsMap: cloneMap(session.Suggestions),
		InterviewDirection:      learningTopic(session),
		RadarChart:              radarFromScore(score),
		QAReviews:               qa,
		ReviewFeedback: compat.ReviewFeedback{
			OverallComment:  "本次学习记录已生成。建议继续围绕概念边界、反例和迁移应用做复习。",
			Highlights:      []string{"已经开始用自己的话讲解知识点"},
			ImprovementTips: []string{"补充反例", "压缩成初学者能听懂的版本"},
			NextActions:     []string{"生成知识卡片", "明天进行一次不看资料的复述"},
		},
		StartTime:       session.CreateTime,
		EndTime:         session.UpdateTime,
		DurationSeconds: int(session.UpdateTime.Sub(session.CreateTime).Seconds()),
		CreateTime:      session.CreateTime,
		UpdateTime:      session.UpdateTime,
	}
}

func radarFromScore(score int) compat.RadarChart {
	if score == 0 {
		score = 60
	}
	points := []compat.RadarMetric{
		{Label: "概念理解", Value: score},
		{Label: "讲解结构", Value: clamp(score - 5)},
		{Label: "表达清晰", Value: clamp(score + 3)},
		{Label: "追问应对", Value: clamp(score - 8)},
		{Label: "迁移应用", Value: clamp(score - 3)},
	}
	return compat.RadarChart{
		ResumeScore:          80,
		InterviewPerformance: score,
		DemeanorEvaluation:   75,
		ProfessionalSkills:   score,
		PotentialIndex:       clamp(score + 5),
		InterviewScore:       score,
		TotalScore:           score,
		RadarMetrics:         points,
		RadarPoints:          points,
	}
}

func clamp(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func defaultUser(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "demo-user"
	}
	return value
}

func materialTitle(fileName string) string {
	title := strings.TrimSpace(fileName)
	if title == "" {
		return "学习资料"
	}
	title = strings.TrimSuffix(title, ".pdf")
	return title
}

func learningTopic(session *compat.Session) string {
	if session == nil {
		return "学习资料"
	}
	if title := strings.TrimSpace(session.OutlineTitle); title != "" {
		return title
	}
	return materialTitle(session.MaterialName)
}

func resumeScore(session *compat.Session) int {
	if session == nil || session.ResumeScore == 0 {
		return 80
	}
	return session.ResumeScore
}

func conversationTitle(session *compat.Session) string {
	if title := learningTopic(session); title != "" && title != "学习资料" {
		return title
	}
	return "AI 棱镜"
}

func normalizeQuestionNumber(value string, fallback int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Sprintf("%d", fallback)
	}
	return value
}

func mapFromSlice(items []string) map[string]string {
	result := map[string]string{}
	for index, item := range items {
		result[fmt.Sprintf("%d", index+1)] = item
	}
	if len(result) == 0 {
		result["1"] = "请补充概念边界和反例。"
	}
	return result
}

func cloneMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneKnowledgeList(input []compat.KnowledgeItem) []compat.KnowledgeItem {
	output := make([]compat.KnowledgeItem, len(input))
	copy(output, input)
	for index := range output {
		output[index].Keywords = append([]string(nil), input[index].Keywords...)
	}
	return output
}

func normalizePage(current int, size int) (int, int) {
	if current <= 0 {
		current = 1
	}
	if size <= 0 {
		size = 10
	}
	return current, size
}

func page[T any](records []T, current int, size int) *compat.PageResult[T] {
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
	return &compat.PageResult[T]{
		Records: records[start:end],
		Total:   total,
		Size:    size,
		Current: current,
		Pages:   pages,
	}
}
