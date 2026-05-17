package impl

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ai-prism/backend/apps/knowledge"
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
		knowledgeBases: map[string]*knowledge.KnowledgeBase{},
		documents:      map[string]*knowledge.Document{},
		outlines:       map[string]*knowledge.Outline{},
	})
}

var ErrNotFound = errors.New("resource not found")

type ServiceImpl struct {
	ioc.ObjectImpl

	mu             sync.RWMutex
	db             *gorm.DB
	mongo          *mongo.Database
	outline        *agent.KnowledgeOutlineWorkflow
	knowledgeBases map[string]*knowledge.KnowledgeBase
	documents      map[string]*knowledge.Document
	outlines       map[string]*knowledge.Outline
}

type documentBody struct {
	ID              string    `bson:"_id"`
	KnowledgeBaseID string    `bson:"knowledgeBaseId"`
	Title           string    `bson:"title"`
	Content         string    `bson:"content"`
	SourceType      string    `bson:"sourceType"`
	CreatedAt       time.Time `bson:"createdAt"`
	UpdatedAt       time.Time `bson:"updatedAt"`
}

type outlineBody struct {
	ID         string              `bson:"_id"`
	DocumentID string              `bson:"documentId"`
	Title      string              `bson:"title"`
	Points     []knowledgePointDoc `bson:"points"`
	UpdatedAt  time.Time           `bson:"updatedAt"`
}

type knowledgePointDoc struct {
	ID          string   `bson:"id"`
	Title       string   `bson:"title"`
	Summary     string   `bson:"summary"`
	Keywords    []string `bson:"keywords"`
	Difficulty  string   `bson:"difficulty"`
	CheckPrompt string   `bson:"checkPrompt"`
}

type chunkBody struct {
	ID              string            `bson:"_id"`
	KnowledgeBaseID string            `bson:"knowledgeBaseId"`
	DocumentID      string            `bson:"documentId"`
	Content         string            `bson:"content"`
	Embedding       []float32         `bson:"embedding"`
	Metadata        map[string]string `bson:"metadata"`
	CreatedAt       time.Time         `bson:"createdAt"`
}

func (s *ServiceImpl) Name() string {
	return knowledge.AppName
}

func (s *ServiceImpl) Init() error {
	cfg := config.Load()
	if db, err := storage.OpenMySQL(cfg.MySQL); err == nil {
		s.db = db
		_ = s.db.AutoMigrate(&storage.KnowledgeBaseModel{}, &storage.KnowledgeDocumentModel{})
	}
	if client, err := storage.OpenMongo(context.Background(), cfg.Mongo); err == nil && client != nil {
		s.mongo = client.Database(cfg.Mongo.Database)
		s.ensureMongoIndexes(context.Background())
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
	return nil
}

func (s *ServiceImpl) CreateKnowledgeBase(ctx context.Context, req knowledge.CreateKnowledgeBaseRequest) (*knowledge.KnowledgeBase, error) {
	now := time.Now()
	item := &knowledge.KnowledgeBase{
		ID:          uuid.NewString(),
		UserID:      defaultUser(req.UserID),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if item.Name == "" {
		item.Name = "默认知识库"
	}

	if s.db != nil {
		model := storage.KnowledgeBaseModel{
			ID:          item.ID,
			UserID:      item.UserID,
			Name:        item.Name,
			Description: item.Description,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
		if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.knowledgeBases[item.ID] = item
	return item, nil
}

func (s *ServiceImpl) CreateDocument(ctx context.Context, req knowledge.CreateDocumentRequest) (*knowledge.Document, error) {
	if strings.TrimSpace(req.KnowledgeBaseID) == "" {
		return nil, ErrNotFound
	}
	if ok := s.knowledgeBaseExists(ctx, req.KnowledgeBaseID); !ok {
		return nil, ErrNotFound
	}

	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" {
		sourceType = "text"
	}
	now := time.Now()
	doc := &knowledge.Document{
		ID:              uuid.NewString(),
		KnowledgeBaseID: req.KnowledgeBaseID,
		Title:           strings.TrimSpace(req.Title),
		Content:         req.Content,
		SourceType:      sourceType,
		Status:          "OUTLINE_READY",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if doc.Title == "" {
		doc.Title = "未命名资料"
	}

	outline := s.generateOutline(ctx, doc)

	if s.db != nil {
		model := storage.KnowledgeDocumentModel{
			ID:              doc.ID,
			KnowledgeBaseID: doc.KnowledgeBaseID,
			Title:           doc.Title,
			SourceType:      doc.SourceType,
			Status:          doc.Status,
			CreatedAt:       doc.CreatedAt,
			UpdatedAt:       doc.UpdatedAt,
		}
		if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
			return nil, err
		}
	}
	if s.mongo != nil {
		if err := s.saveDocumentBody(ctx, doc, outline); err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents[doc.ID] = doc
	s.outlines[doc.ID] = outline
	return doc, nil
}

func (s *ServiceImpl) GenerateOutline(ctx context.Context, req knowledge.GenerateOutlineRequest) (*knowledge.Outline, error) {
	if outline, ok := s.outlineFromMemory(req.DocumentID); ok {
		return outline, nil
	}
	if s.mongo != nil {
		outline, err := s.loadOutline(ctx, req.DocumentID)
		if err == nil {
			s.mu.Lock()
			s.outlines[req.DocumentID] = outline
			s.mu.Unlock()
			return outline, nil
		}
	}
	doc, err := s.GetDocument(ctx, knowledge.GetDocumentRequest{DocumentID: req.DocumentID})
	if err != nil {
		return nil, err
	}
	outline := s.generateOutline(ctx, doc)
	if s.mongo != nil {
		_ = s.saveOutline(ctx, outline)
	}
	s.mu.Lock()
	s.outlines[doc.ID] = outline
	s.mu.Unlock()
	return outline, nil
}

func (s *ServiceImpl) GetDocument(ctx context.Context, req knowledge.GetDocumentRequest) (*knowledge.Document, error) {
	s.mu.RLock()
	if doc, ok := s.documents[req.DocumentID]; ok {
		cloned := *doc
		s.mu.RUnlock()
		return &cloned, nil
	}
	s.mu.RUnlock()

	var model storage.KnowledgeDocumentModel
	if s.db != nil {
		if err := s.db.WithContext(ctx).First(&model, "id = ?", req.DocumentID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	var body documentBody
	if s.mongo != nil {
		err := s.mongo.Collection("knowledge_documents").FindOne(ctx, bson.M{"_id": req.DocumentID}).Decode(&body)
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, err
		}
	}
	if model.ID == "" && body.ID == "" {
		return nil, ErrNotFound
	}
	doc := &knowledge.Document{
		ID:              firstNonEmpty(model.ID, body.ID),
		KnowledgeBaseID: firstNonEmpty(model.KnowledgeBaseID, body.KnowledgeBaseID),
		Title:           firstNonEmpty(model.Title, body.Title),
		Content:         body.Content,
		SourceType:      firstNonEmpty(model.SourceType, body.SourceType),
		Status:          model.Status,
		CreatedAt:       firstTime(model.CreatedAt, body.CreatedAt),
		UpdatedAt:       firstTime(model.UpdatedAt, body.UpdatedAt),
	}
	s.mu.Lock()
	s.documents[doc.ID] = doc
	s.mu.Unlock()
	return doc, nil
}

func (s *ServiceImpl) knowledgeBaseExists(ctx context.Context, id string) bool {
	s.mu.RLock()
	_, exists := s.knowledgeBases[id]
	s.mu.RUnlock()
	if exists {
		return true
	}
	if s.db == nil {
		return false
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&storage.KnowledgeBaseModel{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func (s *ServiceImpl) outlineFromMemory(documentID string) (*knowledge.Outline, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	outline, ok := s.outlines[documentID]
	return outline, ok
}

func (s *ServiceImpl) generateOutline(ctx context.Context, doc *knowledge.Document) *knowledge.Outline {
	if s.outline != nil {
		output := s.outline.Generate(ctx, agent.KnowledgeOutlineInput{
			FileName: doc.Title,
			Content:  []byte(doc.Content),
		})
		if len(output.Points) > 0 {
			points := make([]knowledge.KnowledgePoint, 0, len(output.Points))
			for _, point := range output.Points {
				points = append(points, knowledge.KnowledgePoint{
					ID:          firstNonEmpty(point.ID, uuid.NewString()),
					Title:       strings.TrimSpace(point.Title),
					Summary:     strings.TrimSpace(point.Summary),
					Keywords:    point.Keywords,
					Difficulty:  firstNonEmpty(point.Difficulty, "medium"),
					CheckPrompt: strings.TrimSpace(point.CheckPrompt),
				})
			}
			return &knowledge.Outline{DocumentID: doc.ID, Title: firstNonEmpty(output.Title, doc.Title), Points: points}
		}
	}
	return fallbackOutline(doc)
}

func fallbackOutline(doc *knowledge.Document) *knowledge.Outline {
	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = "学习资料"
	}
	return &knowledge.Outline{
		DocumentID: doc.ID,
		Title:      title,
		Points: []knowledge.KnowledgePoint{
			{
				ID:          uuid.NewString(),
				Title:       title + "核心概念",
				Summary:     "用自己的话解释该主题的定义、适用场景和边界。",
				Keywords:    []string{"定义", "场景", "边界"},
				Difficulty:  "medium",
				CheckPrompt: "请像教给初学者一样解释这个知识点，并举一个例子。",
			},
			{
				ID:          uuid.NewString(),
				Title:       title + "常见误区",
				Summary:     "识别该主题中容易混淆的概念和错误推理。",
				Keywords:    []string{"误区", "对比", "反例"},
				Difficulty:  "medium",
				CheckPrompt: "请说明这个知识点最容易被误解的地方是什么，并给出反例。",
			},
		},
	}
}

func (s *ServiceImpl) saveDocumentBody(ctx context.Context, doc *knowledge.Document, outline *knowledge.Outline) error {
	body := documentBody{
		ID:              doc.ID,
		KnowledgeBaseID: doc.KnowledgeBaseID,
		Title:           doc.Title,
		Content:         doc.Content,
		SourceType:      doc.SourceType,
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
	}
	_, err := s.mongo.Collection("knowledge_documents").ReplaceOne(ctx, bson.M{"_id": doc.ID}, body, options.Replace().SetUpsert(true))
	if err != nil {
		return err
	}
	if err := s.saveOutline(ctx, outline); err != nil {
		return err
	}
	chunks := buildChunks(doc, outline)
	if len(chunks) == 0 {
		return nil
	}
	_, _ = s.mongo.Collection("knowledge_chunks").DeleteMany(ctx, bson.M{"documentId": doc.ID})
	docs := make([]interface{}, 0, len(chunks))
	for _, chunk := range chunks {
		docs = append(docs, chunk)
	}
	_, err = s.mongo.Collection("knowledge_chunks").InsertMany(ctx, docs)
	return err
}

func (s *ServiceImpl) saveOutline(ctx context.Context, outline *knowledge.Outline) error {
	if outline == nil {
		return nil
	}
	body := outlineBody{
		ID:         outline.DocumentID,
		DocumentID: outline.DocumentID,
		Title:      outline.Title,
		Points:     pointDocs(outline.Points),
		UpdatedAt:  time.Now(),
	}
	_, err := s.mongo.Collection("knowledge_outlines").ReplaceOne(ctx, bson.M{"_id": outline.DocumentID}, body, options.Replace().SetUpsert(true))
	return err
}

func (s *ServiceImpl) loadOutline(ctx context.Context, documentID string) (*knowledge.Outline, error) {
	var body outlineBody
	if err := s.mongo.Collection("knowledge_outlines").FindOne(ctx, bson.M{"_id": documentID}).Decode(&body); err != nil {
		return nil, err
	}
	return &knowledge.Outline{
		DocumentID: body.DocumentID,
		Title:      body.Title,
		Points:     knowledgePoints(body.Points),
	}, nil
}

func (s *ServiceImpl) ensureMongoIndexes(ctx context.Context) {
	_, _ = s.mongo.Collection("knowledge_documents").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "knowledgeBaseId", Value: 1}}},
	})
	_, _ = s.mongo.Collection("knowledge_chunks").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "knowledgeBaseId", Value: 1}}},
		{Keys: bson.D{{Key: "documentId", Value: 1}}},
	})
}

func buildChunks(doc *knowledge.Document, outline *knowledge.Outline) []chunkBody {
	segments := splitText(doc.Content, 900)
	if len(segments) == 0 && outline != nil {
		for _, point := range outline.Points {
			segments = append(segments, strings.TrimSpace(point.Title+"\n"+point.Summary+"\n"+point.CheckPrompt))
		}
	}
	now := time.Now()
	chunks := make([]chunkBody, 0, len(segments))
	for index, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		chunks = append(chunks, chunkBody{
			ID:              uuid.NewString(),
			KnowledgeBaseID: doc.KnowledgeBaseID,
			DocumentID:      doc.ID,
			Content:         segment,
			Embedding:       localEmbedding(segment, 64),
			Metadata: map[string]string{
				"title": doc.Title,
				"index": strconv.Itoa(index),
			},
			CreatedAt: now,
		})
	}
	return chunks
}

func splitText(text string, size int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	var chunks []string
	for start := 0; start < len(runes); start += size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func localEmbedding(text string, dim int) []float32 {
	if dim <= 0 {
		dim = 64
	}
	vector := make([]float32, dim)
	for index, r := range []rune(text) {
		vector[(index+int(r))%dim] += 1
	}
	return vector
}

func pointDocs(points []knowledge.KnowledgePoint) []knowledgePointDoc {
	result := make([]knowledgePointDoc, 0, len(points))
	for _, point := range points {
		result = append(result, knowledgePointDoc{
			ID:          point.ID,
			Title:       point.Title,
			Summary:     point.Summary,
			Keywords:    point.Keywords,
			Difficulty:  point.Difficulty,
			CheckPrompt: point.CheckPrompt,
		})
	}
	return result
}

func knowledgePoints(points []knowledgePointDoc) []knowledge.KnowledgePoint {
	result := make([]knowledge.KnowledgePoint, 0, len(points))
	for _, point := range points {
		result = append(result, knowledge.KnowledgePoint{
			ID:          point.ID,
			Title:       point.Title,
			Summary:     point.Summary,
			Keywords:    point.Keywords,
			Difficulty:  point.Difficulty,
			CheckPrompt: point.CheckPrompt,
		})
	}
	return result
}

func defaultUser(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "demo-user"
	}
	return userID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Now()
}
