package impl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ai-prism/backend/apps/knowledge"
	"github.com/ai-prism/backend/apps/learning"
	coach "github.com/ai-prism/backend/apps/prism"
	"github.com/ai-prism/backend/internal/agent"
	"github.com/ai-prism/backend/internal/ai"
	"github.com/ai-prism/backend/internal/config"
	"github.com/ai-prism/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/infraboard/mcube/v2/ioc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

func init() {
	ioc.Controller().Registry(&ServiceImpl{
		sessions: map[string]*learning.Session{},
		turns:    map[string][]*learning.ExplanationTurn{},
	})
}

var ErrSessionNotFound = errors.New("learning session not found")

type ServiceImpl struct {
	ioc.ObjectImpl

	Knowledge knowledge.Service `ioc:"autowire=true;namespace=controllers"`
	Coach     coach.Service     `ioc:"autowire=true;namespace=controllers"`

	mu         sync.RWMutex
	db         *gorm.DB
	mongo      *mongo.Database
	cardReview *agent.LearningCardReviewWorkflow
	sessions   map[string]*learning.Session
	turns      map[string][]*learning.ExplanationTurn
}

type turnDocument struct {
	ID           string             `bson:"_id"`
	SessionID    string             `bson:"sessionId"`
	InputMode    string             `bson:"inputMode"`
	Explanation  string             `bson:"explanation"`
	Diagnosis    learning.Diagnosis `bson:"diagnosis"`
	FollowUp     learning.FollowUp  `bson:"followUp"`
	Correction   string             `bson:"correction"`
	MasteryScore int                `bson:"masteryScore"`
	CreatedAt    time.Time          `bson:"createdAt"`
}

type cardDocument struct {
	ID        string    `bson:"_id"`
	SessionID string    `bson:"sessionId"`
	Title     string    `bson:"title"`
	Content   string    `bson:"content"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

type reviewPlanDocument struct {
	ID        string                `bson:"_id"`
	SessionID string                `bson:"sessionId"`
	Score     int                   `bson:"score"`
	Items     []learning.ReviewItem `bson:"items"`
	UpdatedAt time.Time             `bson:"updatedAt"`
}

func (s *ServiceImpl) Name() string {
	return learning.AppName
}

func (s *ServiceImpl) Init() error {
	cfg := config.Load()
	if db, err := storage.OpenMySQL(cfg.MySQL); err == nil {
		s.db = db
		_ = s.db.AutoMigrate(&storage.LearningSessionModel{})
	}
	if client, err := storage.OpenMongo(context.Background(), cfg.Mongo); err == nil && client != nil {
		s.mongo = client.Database(cfg.Mongo.Database)
		s.ensureMongoIndexes(context.Background())
	}
	s.cardReview = agent.NewLearningCardReviewWorkflow(ai.NewClient(cfg), cfg.AI.Model)
	return nil
}

func (s *ServiceImpl) CreateSession(ctx context.Context, req learning.CreateSessionRequest) (*learning.Session, error) {
	if _, err := s.Knowledge.GetDocument(ctx, knowledge.GetDocumentRequest{DocumentID: req.DocumentID}); err != nil {
		return nil, err
	}

	now := time.Now()
	session := &learning.Session{
		ID:               uuid.NewString(),
		UserID:           defaultUser(req.UserID),
		KnowledgeBaseID:  req.KnowledgeBaseID,
		DocumentID:       req.DocumentID,
		KnowledgePointID: req.KnowledgePointID,
		Status:           "IN_PROGRESS",
		MasteryScore:     0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.saveSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ServiceImpl) SubmitExplanation(ctx context.Context, req learning.SubmitExplanationRequest) (*learning.ExplanationTurn, error) {
	session, err := s.getSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	result, err := s.Coach.Diagnose(ctx, coach.DiagnoseRequest{
		SessionID:   req.SessionID,
		Explanation: req.Explanation,
	})
	if err != nil {
		return nil, err
	}

	mode := strings.TrimSpace(req.InputMode)
	if mode == "" {
		mode = "text"
	}
	turn := &learning.ExplanationTurn{
		ID:          uuid.NewString(),
		SessionID:   req.SessionID,
		InputMode:   mode,
		Explanation: strings.TrimSpace(req.Explanation),
		Diagnosis: learning.Diagnosis{
			CorrectParts:   result.Diagnosis.CorrectParts,
			MissingParts:   result.Diagnosis.MissingParts,
			Misconceptions: result.Diagnosis.Misconceptions,
			Clarity:        result.Diagnosis.Clarity,
		},
		FollowUp: learning.FollowUp{
			Question: result.FollowUp.Question,
			Reason:   result.FollowUp.Reason,
			Targets:  result.FollowUp.Targets,
		},
		Correction:   result.Correction,
		MasteryScore: result.MasteryScore,
		CreatedAt:    time.Now(),
	}
	session.MasteryScore = result.MasteryScore
	session.UpdatedAt = time.Now()
	if err := s.saveTurn(ctx, turn); err != nil {
		return nil, err
	}
	if err := s.saveSession(ctx, session); err != nil {
		return nil, err
	}
	return turn, nil
}

func (s *ServiceImpl) NextFollowUp(ctx context.Context, req learning.NextFollowUpRequest) (*learning.FollowUp, error) {
	if _, err := s.getSession(ctx, req.SessionID); err != nil {
		return nil, err
	}
	turns := s.loadTurns(ctx, req.SessionID)
	if len(turns) == 0 {
		return &learning.FollowUp{
			Question: "请先用自己的话讲解这个知识点的定义、使用场景和一个例子。",
			Reason:   "讲解练习需要先暴露你的当前理解。",
			Targets:  []string{"定义", "场景", "例子"},
		}, nil
	}
	return &turns[len(turns)-1].FollowUp, nil
}

func (s *ServiceImpl) GenerateMarkdownCard(ctx context.Context, req learning.GenerateMarkdownCardRequest) (*learning.MarkdownCard, error) {
	session, err := s.getSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}
	turns := s.loadTurns(ctx, req.SessionID)
	title := s.knowledgeTitle(ctx, session)
	card := s.generateCard(ctx, title, session, turns)
	if s.mongo != nil {
		_, _ = s.mongo.Collection("learning_cards").ReplaceOne(ctx, bson.M{"_id": req.SessionID}, cardDocument{
			ID:        req.SessionID,
			SessionID: req.SessionID,
			Title:     card.Title,
			Content:   card.Content,
			UpdatedAt: time.Now(),
		}, options.Replace().SetUpsert(true))
	}
	return card, nil
}

func (s *ServiceImpl) GenerateReviewPlan(ctx context.Context, req learning.GenerateReviewPlanRequest) (*learning.ReviewPlan, error) {
	session, err := s.getSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}
	turns := s.loadTurns(ctx, req.SessionID)
	title := s.knowledgeTitle(ctx, session)
	output := s.cardReview.Generate(ctx, agent.LearningCardReviewInput{
		KnowledgeTitle: title,
		Question:       latestQuestion(turns),
		Answer:         latestAnswer(turns),
		Feedback:       latestFeedback(turns),
		Score:          session.MasteryScore,
		Turns:          reviewTurns(turns),
	})
	items := reviewItems(output.ReviewPlan)
	if len(items) == 0 {
		items = fallbackReviewItems(session.MasteryScore)
	}
	plan := &learning.ReviewPlan{SessionID: req.SessionID, Score: session.MasteryScore, Items: items}
	if s.mongo != nil {
		_, _ = s.mongo.Collection("learning_review_plans").ReplaceOne(ctx, bson.M{"_id": req.SessionID}, reviewPlanDocument{
			ID:        req.SessionID,
			SessionID: req.SessionID,
			Score:     plan.Score,
			Items:     plan.Items,
			UpdatedAt: time.Now(),
		}, options.Replace().SetUpsert(true))
	}
	return plan, nil
}

func (s *ServiceImpl) FinishSession(ctx context.Context, req learning.FinishSessionRequest) (*learning.Session, error) {
	session, err := s.getSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}
	session.Status = "COMPLETED"
	session.UpdatedAt = time.Now()
	if err := s.saveSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ServiceImpl) getSession(ctx context.Context, id string) (*learning.Session, error) {
	s.mu.RLock()
	if session, ok := s.sessions[id]; ok {
		cloned := *session
		s.mu.RUnlock()
		return &cloned, nil
	}
	s.mu.RUnlock()
	if s.db == nil {
		return nil, ErrSessionNotFound
	}
	var model storage.LearningSessionModel
	if err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	session := sessionFromModel(model)
	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()
	return session, nil
}

func (s *ServiceImpl) saveSession(ctx context.Context, session *learning.Session) error {
	if s.db != nil {
		model := sessionModel(session)
		if err := s.db.WithContext(ctx).Save(&model).Error; err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()
	return nil
}

func (s *ServiceImpl) saveTurn(ctx context.Context, turn *learning.ExplanationTurn) error {
	if s.mongo != nil {
		doc := turnDocument{
			ID:           turn.ID,
			SessionID:    turn.SessionID,
			InputMode:    turn.InputMode,
			Explanation:  turn.Explanation,
			Diagnosis:    turn.Diagnosis,
			FollowUp:     turn.FollowUp,
			Correction:   turn.Correction,
			MasteryScore: turn.MasteryScore,
			CreatedAt:    turn.CreatedAt,
		}
		if _, err := s.mongo.Collection("learning_turns").InsertOne(ctx, doc); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.turns[turn.SessionID] = append(s.turns[turn.SessionID], turn)
	s.mu.Unlock()
	return nil
}

func (s *ServiceImpl) loadTurns(ctx context.Context, sessionID string) []*learning.ExplanationTurn {
	if s.mongo != nil {
		cursor, err := s.mongo.Collection("learning_turns").Find(ctx, bson.M{"sessionId": sessionID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
		if err == nil {
			defer cursor.Close(ctx)
			var docs []turnDocument
			if cursor.All(ctx, &docs) == nil {
				turns := make([]*learning.ExplanationTurn, 0, len(docs))
				for _, doc := range docs {
					turns = append(turns, &learning.ExplanationTurn{
						ID:           doc.ID,
						SessionID:    doc.SessionID,
						InputMode:    doc.InputMode,
						Explanation:  doc.Explanation,
						Diagnosis:    doc.Diagnosis,
						FollowUp:     doc.FollowUp,
						Correction:   doc.Correction,
						MasteryScore: doc.MasteryScore,
						CreatedAt:    doc.CreatedAt,
					})
				}
				if len(turns) > 0 {
					s.mu.Lock()
					s.turns[sessionID] = turns
					s.mu.Unlock()
					return turns
				}
			}
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]*learning.ExplanationTurn(nil), s.turns[sessionID]...)
}

func (s *ServiceImpl) generateCard(ctx context.Context, title string, session *learning.Session, turns []*learning.ExplanationTurn) *learning.MarkdownCard {
	output := s.cardReview.Generate(ctx, agent.LearningCardReviewInput{
		KnowledgeTitle: title,
		Question:       latestQuestion(turns),
		Answer:         latestAnswer(turns),
		Feedback:       latestFeedback(turns),
		Score:          session.MasteryScore,
		Turns:          reviewTurns(turns),
	})
	content := strings.TrimSpace(output.MarkdownCard)
	if content == "" {
		content = fallbackCard(title, session, turns)
	}
	return &learning.MarkdownCard{SessionID: session.ID, Title: title, Content: content}
}

func (s *ServiceImpl) knowledgeTitle(ctx context.Context, session *learning.Session) string {
	doc, err := s.Knowledge.GetDocument(ctx, knowledge.GetDocumentRequest{DocumentID: session.DocumentID})
	if err == nil && strings.TrimSpace(doc.Title) != "" {
		return doc.Title
	}
	return "AI 棱镜知识卡片"
}

func (s *ServiceImpl) ensureMongoIndexes(ctx context.Context) {
	_, _ = s.mongo.Collection("learning_turns").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "sessionId", Value: 1}, {Key: "createdAt", Value: 1}}},
	})
}

func sessionModel(session *learning.Session) storage.LearningSessionModel {
	return storage.LearningSessionModel{
		ID:               session.ID,
		UserID:           session.UserID,
		KnowledgeBaseID:  session.KnowledgeBaseID,
		DocumentID:       session.DocumentID,
		KnowledgePointID: session.KnowledgePointID,
		Status:           session.Status,
		MasteryScore:     session.MasteryScore,
		CreatedAt:        session.CreatedAt,
		UpdatedAt:        session.UpdatedAt,
	}
}

func sessionFromModel(model storage.LearningSessionModel) *learning.Session {
	return &learning.Session{
		ID:               model.ID,
		UserID:           model.UserID,
		KnowledgeBaseID:  model.KnowledgeBaseID,
		DocumentID:       model.DocumentID,
		KnowledgePointID: model.KnowledgePointID,
		Status:           model.Status,
		MasteryScore:     model.MasteryScore,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func fallbackCard(title string, session *learning.Session, turns []*learning.ExplanationTurn) string {
	latest := latestTurn(turns)
	return fmt.Sprintf(`# %s

## 核心讲解

%s

## AI 诊断

- 讲清楚的部分：%s
- 缺失部分：%s
- 可能误区：%s

## 修正版解释

%s

## 自测追问

%s

## 掌握度

%d/100
`,
		title,
		latestString(latest, func(t *learning.ExplanationTurn) string { return t.Explanation }),
		latestJoin(latest, func(t *learning.ExplanationTurn) []string { return t.Diagnosis.CorrectParts }),
		latestJoin(latest, func(t *learning.ExplanationTurn) []string { return t.Diagnosis.MissingParts }),
		latestJoin(latest, func(t *learning.ExplanationTurn) []string { return t.Diagnosis.Misconceptions }),
		latestString(latest, func(t *learning.ExplanationTurn) string { return t.Correction }),
		latestString(latest, func(t *learning.ExplanationTurn) string { return t.FollowUp.Question }),
		session.MasteryScore,
	)
}

func fallbackReviewItems(score int) []learning.ReviewItem {
	items := []learning.ReviewItem{
		{When: "today", Focus: "补齐概念定义和边界", Task: "重新用 3 句话讲解知识点，并写出一个反例。", Target: "能清楚区分概念适用和不适用的场景。"},
		{When: "tomorrow", Focus: "迁移应用", Task: "用一个新的例子解释该知识点。", Target: "能脱离原资料完成解释。"},
		{When: "day_3", Focus: "主动回忆", Task: "不看笔记完成一次口头讲解，再回答 AI 追问。", Target: "追问回答完整率达到 80% 以上。"},
	}
	if score >= 80 {
		return items[1:]
	}
	return items
}

func reviewItems(tasks []agent.ReviewTask) []learning.ReviewItem {
	items := make([]learning.ReviewItem, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, learning.ReviewItem{
			When:   task.When,
			Focus:  task.Focus,
			Task:   task.Task,
			Target: task.Target,
		})
	}
	return items
}

func reviewTurns(turns []*learning.ExplanationTurn) []agent.LearningReviewTurn {
	result := make([]agent.LearningReviewTurn, 0, len(turns))
	for _, turn := range turns {
		result = append(result, agent.LearningReviewTurn{
			Question: turn.FollowUp.Question,
			Answer:   turn.Explanation,
			Score:    turn.MasteryScore,
			Feedback: turn.Correction,
		})
	}
	return result
}

func latestTurn(turns []*learning.ExplanationTurn) *learning.ExplanationTurn {
	if len(turns) == 0 {
		return nil
	}
	return turns[len(turns)-1]
}

func latestQuestion(turns []*learning.ExplanationTurn) string {
	return latestString(latestTurn(turns), func(t *learning.ExplanationTurn) string { return t.FollowUp.Question })
}

func latestAnswer(turns []*learning.ExplanationTurn) string {
	return latestString(latestTurn(turns), func(t *learning.ExplanationTurn) string { return t.Explanation })
}

func latestFeedback(turns []*learning.ExplanationTurn) string {
	return latestString(latestTurn(turns), func(t *learning.ExplanationTurn) string { return t.Correction })
}

func defaultUser(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "demo-user"
	}
	return userID
}

func latestString(turn *learning.ExplanationTurn, picker func(*learning.ExplanationTurn) string) string {
	if turn == nil {
		return "待补充"
	}
	value := strings.TrimSpace(picker(turn))
	if value == "" {
		return "待补充"
	}
	return value
}

func latestJoin(turn *learning.ExplanationTurn, picker func(*learning.ExplanationTurn) []string) string {
	if turn == nil {
		return "待补充"
	}
	value := strings.Join(picker(turn), "；")
	if strings.TrimSpace(value) == "" {
		return "待补充"
	}
	return value
}
