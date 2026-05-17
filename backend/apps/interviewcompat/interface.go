package interviewcompat

import "context"

type Service interface {
	CreateSession(context.Context, string) (*CreateSessionResult, error)
	PageConversations(context.Context, int, int) (*PageResult[ConversationItem], error)
	ExtractQuestions(context.Context, string, string, string, []byte, string) (*ExtractQuestionsResult, error)
	PreviewMaterial(context.Context, string) ([]byte, string, string, error)
	Restore(context.Context, string) (*RestoreResult, error)
	CurrentQuestion(context.Context, string) (*AnswerResult, error)
	SelectKnowledgePoint(context.Context, string, string) (*KnowledgePointActionResult, error)
	SkipKnowledgePoint(context.Context, string, string) (*KnowledgePointActionResult, error)
	FinishCurrentKnowledgePoint(context.Context, string) (*KnowledgePointActionResult, error)
	Answer(context.Context, string, AnswerRequest) (*AnswerResult, error)
	Finish(context.Context, string) error
	Radar(context.Context, string) (*RadarChart, error)
	Record(context.Context, string) (*Record, error)
	PageRecords(context.Context, int, int) (*PageResult[Record], error)
	UploadFile(context.Context, UploadedFile, []byte) (*UploadedFile, error)
	Demeanor(context.Context, string) (string, error)
}
