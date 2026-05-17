package feynman

import "context"

const AppName = "feynman"

type Service interface {
	Diagnose(context.Context, DiagnoseRequest) (*CoachResult, error)
}
