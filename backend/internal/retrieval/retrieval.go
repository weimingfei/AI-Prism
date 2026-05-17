package retrieval

import "context"

type Query struct {
	KnowledgeBaseID string
	Text            string
	TopK            int
}

type Chunk struct {
	ID         string
	DocumentID string
	Content    string
	Score      float64
	Metadata   map[string]string
}

type Retriever interface {
	Retrieve(ctx context.Context, query Query) ([]Chunk, error)
}
