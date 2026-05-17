package embedding

import "context"

type Request struct {
	Model string
	Texts []string
}

type Response struct {
	Vectors [][]float32
}

type Client interface {
	Embed(ctx context.Context, request Request) (Response, error)
}
