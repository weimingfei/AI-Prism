package impl

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	compat "github.com/ai-prism/backend/apps/interviewcompat"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	runtimeLoadModeReadOnly          = "READ_ONLY"
	runtimeLoadModeReadWriteRequired = "READ_WRITE_REQUIRED"

	runtimeScopeFlowOnly    = "FLOW_ONLY"
	runtimeScopeScoreOnly   = "SCORE_ONLY"
	runtimeScopePlayback    = "PLAYBACK_ONLY"
	runtimeScopeMaterial    = "MATERIAL_ONLY"
	runtimeScopeHotRuntime  = "HOT_RUNTIME"
	runtimeScopeFullRuntime = "FULL_RUNTIME"

	runtimeConfidenceExact    = "EXACT"
	runtimeConfidenceDerived  = "DERIVED"
	runtimeConfidenceReadOnly = "READ_ONLY"
	runtimeConfidenceTerminal = "TERMINAL"

	runtimeRestoreSourceNone            = "NONE"
	runtimeRestoreSourceCache           = "CACHE"
	runtimeRestoreSourceRuntimeSnapshot = "RUNTIME_SNAPSHOT"
	runtimeRestoreSourceSessionDocument = "SESSION_DOCUMENT"
)

const recentTurnLimit = 20

type runtimeFlowState struct {
	Status                string    `json:"status" bson:"status"`
	CurrentIndex          int       `json:"currentIndex" bson:"currentIndex"`
	CurrentQuestionNumber string    `json:"currentQuestionNumber" bson:"currentQuestionNumber"`
	FollowUpCount         int       `json:"followUpCount" bson:"followUpCount"`
	MaxFollowUp           int       `json:"maxFollowUp" bson:"maxFollowUp"`
	TotalQuestions        int       `json:"totalQuestions" bson:"totalQuestions"`
	Version               int64     `json:"version" bson:"version"`
	UpdatedAt             time.Time `json:"updatedAt" bson:"updatedAt"`
}

type runtimeScoreAggregate struct {
	AnsweredCount int     `json:"answeredCount" bson:"answeredCount"`
	PassedCount   int     `json:"passedCount" bson:"passedCount"`
	FailedCount   int     `json:"failedCount" bson:"failedCount"`
	SkippedCount  int     `json:"skippedCount" bson:"skippedCount"`
	TotalScore    int     `json:"totalScore" bson:"totalScore"`
	AverageScore  float64 `json:"averageScore" bson:"averageScore"`
}

type runtimeHotSnapshot struct {
	ID                          string                `bson:"_id"`
	SessionID                   string                `bson:"sessionId"`
	UserID                      string                `bson:"userId"`
	SessionStatus               string                `bson:"sessionStatus"`
	SnapshotLevel               string                `bson:"snapshotLevel"`
	SnapshotVersion             int64                 `bson:"snapshotVersion"`
	RebuildConfidence           string                `bson:"rebuildConfidence"`
	Flow                        runtimeFlowState      `bson:"flow"`
	ScoreAggregate              runtimeScoreAggregate `bson:"scoreAggregate"`
	RecentTurns                 []compat.Turn         `bson:"recentTurns"`
	RecentTurnCount             int                   `bson:"recentTurnCount"`
	LastTurnSeq                 int64                 `bson:"lastTurnSeq"`
	ArchiveWatermark            int64                 `bson:"archiveWatermark"`
	LastAppliedRequestID        string                `bson:"lastAppliedRequestId"`
	LastCommittedQuestionNumber string                `bson:"lastCommittedQuestionNumber"`
	LastCommittedTurnDigest     string                `bson:"lastCommittedTurnDigest"`
	SnapshotUpdatedAt           time.Time             `bson:"snapshotUpdatedAt"`
}

type runtimeColdSnapshot struct {
	ID                string                 `bson:"_id"`
	SessionID         string                 `bson:"sessionId"`
	UserID            string                 `bson:"userId"`
	MaterialVersion   int64                  `bson:"materialVersion"`
	MaterialName      string                 `bson:"materialName"`
	ContentType       string                 `bson:"contentType"`
	Questions         map[string]string      `bson:"questions"`
	Suggestions       map[string]string      `bson:"suggestions"`
	KnowledgeList     []compat.KnowledgeItem `bson:"knowledgeList"`
	MaterialUpdatedAt time.Time              `bson:"materialUpdatedAt"`
}

type runtimeTurnArchive struct {
	ID        string      `bson:"_id"`
	SessionID string      `bson:"sessionId"`
	Seq       int64       `bson:"seq"`
	Digest    string      `bson:"digest"`
	Turn      compat.Turn `bson:"turn"`
	CreatedAt time.Time   `bson:"createdAt"`
}

type runtimeView struct {
	Session       *compat.Session
	LoadMode      string
	Scope         string
	RestoreSource string
	Confidence    string
	CacheRebuilt  bool
	CanWrite      bool
	HotSnapshot   *runtimeHotSnapshot
	ColdSnapshot  *runtimeColdSnapshot
}

func (s *ServiceImpl) ensureRuntimeLocked(ctx context.Context, sessionID string, loadMode string, scope string) runtimeView {
	if strings.TrimSpace(loadMode) == "" {
		loadMode = runtimeLoadModeReadOnly
	}
	if strings.TrimSpace(scope) == "" {
		scope = runtimeScopeFullRuntime
	}
	if sessionID == "" {
		return runtimeView{LoadMode: loadMode, Scope: scope, RestoreSource: runtimeRestoreSourceNone, Confidence: runtimeConfidenceReadOnly}
	}
	if session, ok := s.getSessionLocked(ctx, sessionID); ok {
		view := runtimeView{
			Session:       session,
			LoadMode:      loadMode,
			Scope:         scope,
			RestoreSource: runtimeRestoreSourceCache,
			Confidence:    runtimeConfidenceExact,
			CanWrite:      canWriteRuntime(loadMode, runtimeConfidenceExact, session),
		}
		hot, cold := s.findRuntimeSnapshots(ctx, sessionID)
		view.HotSnapshot = hot
		view.ColdSnapshot = cold
		if !s.runtimeReady(ctx, sessionID, scope) {
			s.refreshRuntimeSnapshots(ctx, session, "CACHE_REBUILD", "")
			view.CacheRebuilt = true
		}
		return view
	}

	// 内存和 Redis 都没有命中时，用短锁保护快照恢复，避免多个请求同时重建同一会话。
	unlock := s.acquireRuntimeLock(ctx, sessionID, 8*time.Second)
	if unlock != nil {
		defer unlock()
	} else {
		for i := 0; i < 4; i++ {
			time.Sleep(80 * time.Millisecond)
			if session, ok := s.getSessionLocked(ctx, sessionID); ok && s.runtimeReady(ctx, sessionID, scope) {
				return runtimeView{
					Session:       session,
					LoadMode:      loadMode,
					Scope:         scope,
					RestoreSource: runtimeRestoreSourceCache,
					Confidence:    runtimeConfidenceExact,
					CanWrite:      canWriteRuntime(loadMode, runtimeConfidenceExact, session),
				}
			}
		}
	}

	if session := s.rebuildSessionFromSnapshots(ctx, sessionID); session != nil {
		s.sessions[sessionID] = session
		s.saveSessionToRedis(ctx, session)
		s.writeRuntimeCache(ctx, session)
		hot, cold := s.findRuntimeSnapshots(ctx, sessionID)
		confidence := runtimeConfidenceDerived
		if session.Status == "COMPLETED" {
			confidence = runtimeConfidenceTerminal
		}
		return runtimeView{
			Session:       session,
			LoadMode:      loadMode,
			Scope:         scope,
			RestoreSource: runtimeRestoreSourceRuntimeSnapshot,
			Confidence:    confidence,
			CacheRebuilt:  true,
			CanWrite:      canWriteRuntime(loadMode, confidence, session),
			HotSnapshot:   hot,
			ColdSnapshot:  cold,
		}
	}
	return runtimeView{LoadMode: loadMode, Scope: scope, RestoreSource: runtimeRestoreSourceNone, Confidence: runtimeConfidenceReadOnly}
}

func (s *ServiceImpl) refreshRuntimeSnapshots(ctx context.Context, session *compat.Session, level string, requestID string) {
	if session == nil {
		return
	}
	if strings.TrimSpace(level) == "" {
		level = "AUTO"
	}
	s.writeRuntimeCache(ctx, session)
	if s.mongo == nil {
		return
	}
	// 热快照保存会话推进所需的轻量状态，冷快照保存资料和题目这类重建基础数据。
	now := time.Now()
	hot, _ := s.findRuntimeSnapshots(ctx, session.SessionID)
	version := int64(1)
	if hot != nil && hot.SnapshotVersion > 0 {
		version = hot.SnapshotVersion + 1
	}
	turns := limitRecentTurns(session.Turns)
	archiveWatermark := int64(len(session.Turns))
	lastQuestion := ""
	lastDigest := ""
	if len(session.Turns) > 0 {
		last := session.Turns[len(session.Turns)-1]
		lastQuestion = last.QuestionNumber
		lastDigest = turnDigest(last)
	}
	flow := flowFromSession(session, version)
	if existing, ok := s.loadFlowFromRedis(ctx, session.SessionID); ok {
		flow = existing
		flow.TotalQuestions = len(session.Questions)
		flow.UpdatedAt = session.UpdateTime
		if session.Status == "COMPLETED" {
			flow.Status = flowStatusCompleted
		}
	}
	hotDoc := runtimeHotSnapshot{
		ID:                          session.SessionID,
		SessionID:                   session.SessionID,
		UserID:                      session.UserID,
		SessionStatus:               session.Status,
		SnapshotLevel:               level,
		SnapshotVersion:             version,
		RebuildConfidence:           confidenceForSession(session),
		Flow:                        flow,
		ScoreAggregate:              scoreAggregateFromSession(session),
		RecentTurns:                 turns,
		RecentTurnCount:             len(turns),
		LastTurnSeq:                 archiveWatermark,
		ArchiveWatermark:            archiveWatermark,
		LastAppliedRequestID:        strings.TrimSpace(requestID),
		LastCommittedQuestionNumber: lastQuestion,
		LastCommittedTurnDigest:     lastDigest,
		SnapshotUpdatedAt:           now,
	}
	_, _ = s.mongo.Collection("coach_runtime_hot_snapshots").ReplaceOne(ctx, bson.M{"_id": session.SessionID}, hotDoc, options.Replace().SetUpsert(true))
	coldDoc := runtimeColdSnapshot{
		ID:                session.SessionID,
		SessionID:         session.SessionID,
		UserID:            session.UserID,
		MaterialVersion:   version,
		MaterialName:      session.MaterialName,
		ContentType:       session.ContentType,
		Questions:         cloneMap(session.Questions),
		Suggestions:       cloneMap(session.Suggestions),
		KnowledgeList:     cloneKnowledgeList(session.KnowledgeList),
		MaterialUpdatedAt: now,
	}
	_, _ = s.mongo.Collection("coach_runtime_cold_snapshots").ReplaceOne(ctx, bson.M{"_id": session.SessionID}, coldDoc, options.Replace().SetUpsert(true))
	s.archiveTurns(ctx, session)
}

func (s *ServiceImpl) writeRuntimeCache(ctx context.Context, session *compat.Session) {
	if s.redis == nil || session == nil {
		return
	}
	ttl := 24 * time.Hour
	writeJSON := func(key string, value any) {
		data, err := json.Marshal(value)
		if err == nil {
			_ = s.redis.Set(ctx, key, data, ttl).Err()
		}
	}
	flow := flowFromSession(session, time.Now().UnixMilli())
	if existing, ok := s.loadFlowFromRedis(ctx, session.SessionID); ok {
		flow = existing
		flow.TotalQuestions = len(session.Questions)
		flow.UpdatedAt = session.UpdateTime
		if session.Status == "COMPLETED" {
			flow.Status = flowStatusCompleted
		}
	}
	writeJSON(runtimeFlowKey(session.SessionID), flow)
	writeJSON(runtimeScoreKey(session.SessionID), scoreAggregateFromSession(session))
	writeJSON(runtimeTurnsKey(session.SessionID), limitRecentTurns(session.Turns))
	writeJSON(runtimeMaterialKey(session.SessionID), runtimeColdSnapshot{
		SessionID:     session.SessionID,
		UserID:        session.UserID,
		MaterialName:  session.MaterialName,
		ContentType:   session.ContentType,
		Questions:     cloneMap(session.Questions),
		Suggestions:   cloneMap(session.Suggestions),
		KnowledgeList: cloneKnowledgeList(session.KnowledgeList),
	})
}

func (s *ServiceImpl) runtimeReady(ctx context.Context, sessionID string, scope string) bool {
	if s.redis == nil {
		return false
	}
	needs := []string{}
	switch scope {
	case runtimeScopeFlowOnly:
		needs = []string{runtimeFlowKey(sessionID)}
	case runtimeScopeScoreOnly:
		needs = []string{runtimeScoreKey(sessionID)}
	case runtimeScopePlayback:
		needs = []string{runtimeTurnsKey(sessionID)}
	case runtimeScopeMaterial:
		needs = []string{runtimeMaterialKey(sessionID)}
	case runtimeScopeHotRuntime:
		needs = []string{runtimeFlowKey(sessionID), runtimeScoreKey(sessionID), runtimeTurnsKey(sessionID)}
	default:
		needs = []string{runtimeFlowKey(sessionID), runtimeScoreKey(sessionID), runtimeTurnsKey(sessionID), runtimeMaterialKey(sessionID)}
	}
	for _, key := range needs {
		ok, err := s.redis.Exists(ctx, key).Result()
		if err != nil || ok == 0 {
			return false
		}
	}
	return true
}

func (s *ServiceImpl) rebuildSessionFromSnapshots(ctx context.Context, sessionID string) *compat.Session {
	hot, cold := s.findRuntimeSnapshots(ctx, sessionID)
	if hot == nil && cold == nil {
		return nil
	}
	now := time.Now()
	session := &compat.Session{
		SessionID:    sessionID,
		Status:       "IN_PROGRESS",
		Questions:    map[string]string{},
		Suggestions:  map[string]string{},
		CurrentIndex: 1,
		CreateTime:   now,
		UpdateTime:   now,
	}
	if cold != nil {
		session.UserID = cold.UserID
		session.MaterialName = cold.MaterialName
		session.ContentType = cold.ContentType
		session.Questions = cloneMap(cold.Questions)
		session.Suggestions = cloneMap(cold.Suggestions)
		session.KnowledgeList = cloneKnowledgeList(cold.KnowledgeList)
		session.UpdateTime = cold.MaterialUpdatedAt
	}
	if hot != nil {
		session.UserID = firstNonBlank(session.UserID, hot.UserID)
		session.Status = firstNonBlank(hot.SessionStatus, session.Status)
		session.CurrentIndex = hot.Flow.CurrentIndex
		if session.CurrentIndex <= 0 {
			session.CurrentIndex = questionIndex(hot.Flow.CurrentQuestionNumber)
		}
		session.TotalScore = hot.ScoreAggregate.TotalScore
		session.Turns = append([]compat.Turn(nil), hot.RecentTurns...)
		if !hot.SnapshotUpdatedAt.IsZero() {
			session.UpdateTime = hot.SnapshotUpdatedAt
		}
	}
	if session.UserID == "" {
		session.UserID = "demo-user"
	}
	if session.CreateTime.IsZero() {
		session.CreateTime = session.UpdateTime
	}
	if session.UpdateTime.IsZero() {
		session.UpdateTime = now
	}
	if len(session.Questions) == 0 {
		return nil
	}
	if session.CurrentIndex <= 0 {
		session.CurrentIndex = 1
	}
	return session
}

func (s *ServiceImpl) findRuntimeSnapshots(ctx context.Context, sessionID string) (*runtimeHotSnapshot, *runtimeColdSnapshot) {
	if s.mongo == nil {
		return nil, nil
	}
	var hot runtimeHotSnapshot
	var cold runtimeColdSnapshot
	var hotPtr *runtimeHotSnapshot
	var coldPtr *runtimeColdSnapshot
	if err := s.mongo.Collection("coach_runtime_hot_snapshots").FindOne(ctx, bson.M{"_id": sessionID}).Decode(&hot); err == nil {
		hotPtr = &hot
	}
	if err := s.mongo.Collection("coach_runtime_cold_snapshots").FindOne(ctx, bson.M{"_id": sessionID}).Decode(&cold); err == nil {
		coldPtr = &cold
	}
	return hotPtr, coldPtr
}

func (s *ServiceImpl) archiveTurns(ctx context.Context, session *compat.Session) {
	if s.mongo == nil || session == nil {
		return
	}
	for index, turn := range session.Turns {
		seq := int64(index + 1)
		digest := turnDigest(turn)
		id := fmt.Sprintf("%s:%d", session.SessionID, seq)
		_, _ = s.mongo.Collection("coach_runtime_turn_archives").ReplaceOne(ctx, bson.M{"_id": id}, runtimeTurnArchive{
			ID:        id,
			SessionID: session.SessionID,
			Seq:       seq,
			Digest:    digest,
			Turn:      turn,
			CreatedAt: turn.CreateTime,
		}, options.Replace().SetUpsert(true))
	}
}

func (s *ServiceImpl) acquireRuntimeLock(ctx context.Context, sessionID string, ttl time.Duration) func() {
	if s.redis == nil {
		return func() {}
	}
	token := uuid.NewString()
	key := runtimeLockKey(sessionID)
	ok, err := s.redis.SetNX(ctx, key, token, ttl).Result()
	if err != nil || !ok {
		return nil
	}
	return func() {
		_ = s.redis.Eval(ctx, `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`, []string{key}, token).Err()
	}
}

func (s *ServiceImpl) withRuntimeWriteLock(ctx context.Context, sessionID string, fn func() error) error {
	unlock := s.acquireRuntimeLock(ctx, sessionID, 20*time.Second)
	if unlock == nil {
		return errorsNewRuntimeBusy()
	}
	defer unlock()
	return fn()
}

func flowFromSession(session *compat.Session, version int64) runtimeFlowState {
	number := fmt.Sprintf("%d", session.CurrentIndex)
	if strings.TrimSpace(session.Questions[number]) == "" {
		number = "1"
	}
	status := flowStatusAsking
	if session.Status == "COMPLETED" {
		status = flowStatusCompleted
	}
	return runtimeFlowState{
		Status:                status,
		CurrentIndex:          session.CurrentIndex,
		CurrentQuestionNumber: number,
		FollowUpCount:         followUpDepth(number),
		MaxFollowUp:           2,
		TotalQuestions:        len(session.Questions),
		Version:               version,
		UpdatedAt:             session.UpdateTime,
	}
}

func scoreAggregateFromSession(session *compat.Session) runtimeScoreAggregate {
	var total, answered, passed, failed, skipped int
	for _, item := range session.KnowledgeList {
		switch item.Status {
		case "passed":
			passed++
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
		if item.Score != nil {
			answered++
			total += *item.Score
		}
	}
	if answered == 0 && len(session.Turns) > 0 {
		answered = len(session.Turns)
		for _, turn := range session.Turns {
			total += turn.Score
		}
	}
	average := 0.0
	if answered > 0 {
		average = math.Round(float64(total)/float64(answered)*100) / 100
	}
	return runtimeScoreAggregate{
		AnsweredCount: answered,
		PassedCount:   passed,
		FailedCount:   failed,
		SkippedCount:  skipped,
		TotalScore:    session.TotalScore,
		AverageScore:  average,
	}
}

func limitRecentTurns(turns []compat.Turn) []compat.Turn {
	if len(turns) <= recentTurnLimit {
		return append([]compat.Turn(nil), turns...)
	}
	return append([]compat.Turn(nil), turns[len(turns)-recentTurnLimit:]...)
}

func turnDigest(turn compat.Turn) string {
	sum := sha1.Sum([]byte(turn.QuestionNumber + "\n" + turn.Question + "\n" + turn.Answer + "\n" + turn.Feedback))
	return hex.EncodeToString(sum[:])
}

func confidenceForSession(session *compat.Session) string {
	if session == nil {
		return runtimeConfidenceReadOnly
	}
	if session.Status == "COMPLETED" {
		return runtimeConfidenceTerminal
	}
	return runtimeConfidenceExact
}

func canWriteRuntime(loadMode string, confidence string, session *compat.Session) bool {
	if loadMode != runtimeLoadModeReadWriteRequired || session == nil {
		return false
	}
	if session.Status == "COMPLETED" || confidence == runtimeConfidenceReadOnly || confidence == runtimeConfidenceTerminal {
		return false
	}
	return true
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func runtimeFlowKey(sessionID string) string {
	return "ai-prism:practice:runtime:flow:" + sessionID
}

func runtimeScoreKey(sessionID string) string {
	return "ai-prism:practice:runtime:score:" + sessionID
}

func runtimeTurnsKey(sessionID string) string {
	return "ai-prism:practice:runtime:turns:" + sessionID
}

func runtimeMaterialKey(sessionID string) string {
	return "ai-prism:practice:runtime:material:" + sessionID
}

func runtimeLockKey(sessionID string) string {
	return "ai-prism:practice:runtime:rehydrate:lock:" + sessionID
}

func errorsNewRuntimeBusy() error {
	return fmt.Errorf("当前会话正在恢复或推进，请稍后重试")
}

var _ = redis.Nil
