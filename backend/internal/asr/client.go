package asr

import "context"

type TranscribeRequest struct {
	FileName    string
	ContentType string
	Audio       []byte
	Language    string
}

type TranscribeResponse struct {
	Text     string
	Duration int
}

type Client interface {
	Transcribe(ctx context.Context, request TranscribeRequest) (TranscribeResponse, error)
}
