package prism

import "context"

const AppName = "prism"

type Service interface {
	Diagnose(context.Context, DiagnoseRequest) (*CoachResult, error)
}
