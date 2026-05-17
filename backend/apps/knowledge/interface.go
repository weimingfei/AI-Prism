package knowledge

import "context"

const AppName = "knowledge"

type Service interface {
	CreateKnowledgeBase(context.Context, CreateKnowledgeBaseRequest) (*KnowledgeBase, error)
	CreateDocument(context.Context, CreateDocumentRequest) (*Document, error)
	GenerateOutline(context.Context, GenerateOutlineRequest) (*Outline, error)
	GetDocument(context.Context, GetDocumentRequest) (*Document, error)
}
