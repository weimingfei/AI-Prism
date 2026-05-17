package interviewcompat

import "time"

const AppName = "interview_compat"

type PageResult[T any] struct {
	Records []T `json:"records"`
	Total   int `json:"total"`
	Size    int `json:"size"`
	Current int `json:"current"`
	Pages   int `json:"pages"`
}

type Session struct {
	SessionID      string            `json:"sessionId"`
	UserID         string            `json:"userId"`
	Status         string            `json:"status"`
	Questions      map[string]string `json:"questions"`
	Suggestions    map[string]string `json:"suggestions"`
	KnowledgeList  []KnowledgeItem   `json:"knowledgeList"`
	MaterialName   string            `json:"resumeFileUrl"`
	OutlineTitle   string            `json:"outlineTitle"`
	OutlineSummary string            `json:"outlineSummary"`
	ResumeScore    int               `json:"resumeScore"`
	MaterialBytes  []byte            `json:"-"`
	ContentType    string            `json:"-"`
	CurrentIndex   int               `json:"-"`
	TotalScore     int               `json:"totalScore"`
	Turns          []Turn            `json:"-"`
	CreateTime     time.Time         `json:"createTime"`
	UpdateTime     time.Time         `json:"updateTime"`
}

type KnowledgeItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Keywords    []string `json:"keywords"`
	Difficulty  string   `json:"difficulty"`
	CheckPrompt string   `json:"checkPrompt"`
	Status      string   `json:"status"`
	Score       *int     `json:"score,omitempty"`
}

type KnowledgePointActionResult struct {
	Question      *AnswerResult   `json:"question,omitempty"`
	KnowledgeList []KnowledgeItem `json:"knowledgeList"`
	Finished      bool            `json:"finished"`
}

type Turn struct {
	QuestionNumber string    `json:"questionNumber"`
	Question       string    `json:"question"`
	Answer         string    `json:"answer"`
	Score          int       `json:"score"`
	Feedback       string    `json:"feedback"`
	IsFollowUp     bool      `json:"isFollowUp"`
	FollowUpCount  int       `json:"followUpCount"`
	CreateTime     time.Time `json:"createTime"`
}

type CreateSessionResult struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

type ConversationItem struct {
	SessionID         string    `json:"sessionId"`
	ConversationTitle string    `json:"conversationTitle"`
	Status            string    `json:"status"`
	InterviewType     string    `json:"interviewType"`
	ResumeFileURL     string    `json:"resumeFileUrl"`
	CreateTime        time.Time `json:"createTime"`
	UpdateTime        time.Time `json:"updateTime"`
}

type ExtractQuestionsResult struct {
	ID              string            `json:"id"`
	SessionID       string            `json:"sessionId"`
	UserName        string            `json:"userName"`
	AgentID         int               `json:"agentId"`
	Questions       map[string]string `json:"questions"`
	Suggestions     map[string]string `json:"suggestions"`
	KnowledgeList   []KnowledgeItem   `json:"knowledgeList"`
	OutlineTitle    string            `json:"outlineTitle"`
	OutlineSummary  string            `json:"outlineSummary"`
	InterviewType   string            `json:"interviewType"`
	ResumeFileURL   string            `json:"resumeFileUrl"`
	ResponseTime    int               `json:"responseTime"`
	TokenCount      int               `json:"tokenCount"`
	ResumeScore     int               `json:"resumeScore"`
	QuestionCount   int               `json:"questionCount"`
	SuggestionCount int               `json:"suggestionCount"`
	IsSuccess       int               `json:"isSuccess"`
	CreateTime      time.Time         `json:"createTime"`
	UpdateTime      time.Time         `json:"updateTime"`
}

type AnswerRequest struct {
	QuestionNumber   string `json:"questionNumber" form:"questionNumber"`
	AnswerContent    string `json:"answerContent" form:"answerContent"`
	RequestID        string `json:"requestId" form:"requestId"`
	MaxFollowUpCount int    `json:"maxFollowUpCount" form:"maxFollowUpCount"`
	FollowUpMode     string `json:"followUpMode" form:"followUpMode"`
}

type AnswerResult struct {
	QuestionNumber     string            `json:"questionNumber"`
	QuestionContent    string            `json:"questionContent"`
	Score              int               `json:"score"`
	TotalScore         int               `json:"totalScore"`
	IsSuccess          bool              `json:"isSuccess"`
	Feedback           string            `json:"feedback"`
	NextQuestion       *string           `json:"nextQuestion"`
	NextQuestionNumber *string           `json:"nextQuestionNumber"`
	IsFollowUp         bool              `json:"isFollowUp"`
	FollowUpNeeded     bool              `json:"followUpNeeded"`
	FollowUpCount      int               `json:"followUpCount"`
	AskToUser          *string           `json:"askToUser"`
	MissingPoints      map[string]string `json:"missingPoints"`
	KnowledgeList      []KnowledgeItem   `json:"knowledgeList"`
	Finished           bool              `json:"finished"`
	NeedsChoice        bool              `json:"needsChoice,omitempty"`
	ReferenceAnswer    string            `json:"referenceAnswer,omitempty"`
}

type RestoreResult struct {
	SessionID     string            `json:"sessionId"`
	Status        string            `json:"status"`
	CanResume     bool              `json:"canResume"`
	CanWrite      bool              `json:"canWrite"`
	ResumeFileURL string            `json:"resumeFileUrl"`
	ResumeScore   int               `json:"resumeScore"`
	InterviewType string            `json:"interviewType"`
	Suggestions   map[string]string `json:"suggestions"`
	KnowledgeList []KnowledgeItem   `json:"knowledgeList"`
	LoadMode      string            `json:"loadMode"`
	RestoreSource string            `json:"restoreSource"`
	Confidence    string            `json:"confidence"`
	CacheRebuilt  bool              `json:"cacheRebuilt"`
}

type RadarChart struct {
	ResumeScore          int           `json:"resumeScore"`
	InterviewPerformance int           `json:"interviewPerformance"`
	DemeanorEvaluation   int           `json:"demeanorEvaluation"`
	ProfessionalSkills   int           `json:"professionalSkills"`
	PotentialIndex       int           `json:"potentialIndex"`
	InterviewScore       int           `json:"interviewScore"`
	TotalScore           int           `json:"totalScore"`
	RadarMetrics         []RadarMetric `json:"radarMetrics"`
	RadarPoints          []RadarMetric `json:"radarPoints"`
}

type RadarMetric struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type Record struct {
	ID                      int64             `json:"id"`
	UserID                  int64             `json:"userId"`
	SessionID               string            `json:"sessionId"`
	ResumeScore             int               `json:"resumeScore"`
	InterviewScore          int               `json:"interviewScore"`
	InterviewStatus         string            `json:"interviewStatus"`
	QuestionCount           int               `json:"questionCount"`
	CompositeScore          int               `json:"compositeScore"`
	TotalScore              int               `json:"totalScore"`
	FinalScore              int               `json:"finalScore"`
	InterviewSuggestionsMap map[string]string `json:"interviewSuggestionsMap"`
	InterviewDirection      string            `json:"interviewDirection"`
	RadarChart              RadarChart        `json:"radarChart"`
	QAReviews               []QAReview        `json:"qaReviews"`
	ReviewFeedback          ReviewFeedback    `json:"reviewFeedback"`
	StartTime               time.Time         `json:"startTime"`
	EndTime                 time.Time         `json:"endTime"`
	DurationSeconds         int               `json:"durationSeconds"`
	CreateTime              time.Time         `json:"createTime"`
	UpdateTime              time.Time         `json:"updateTime"`
}

type QAReview struct {
	Seq            int    `json:"seq"`
	QuestionNumber string `json:"questionNumber"`
	Question       string `json:"question"`
	Answer         string `json:"answer"`
	Score          int    `json:"score"`
	Feedback       string `json:"feedback"`
	IsFollowUp     bool   `json:"isFollowUp"`
	FollowUpNeeded bool   `json:"followUpNeeded"`
	FollowUpCount  int    `json:"followUpCount"`
}

type ReviewFeedback struct {
	OverallComment  string   `json:"overallComment"`
	Highlights      []string `json:"highlights"`
	ImprovementTips []string `json:"improvementTips"`
	NextActions     []string `json:"nextActions"`
}

type UploadedFile struct {
	ID          int64     `json:"id"`
	AgentID     int64     `json:"agentId"`
	SessionID   string    `json:"sessionId"`
	BizType     string    `json:"bizType"`
	FileName    string    `json:"fileName"`
	FileSize    int64     `json:"fileSize"`
	ContentType string    `json:"contentType"`
	FileURL     string    `json:"fileUrl"`
	CreateTime  time.Time `json:"createTime"`
}
