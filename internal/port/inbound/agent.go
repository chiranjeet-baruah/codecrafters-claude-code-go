package inbound

import "context"

type Agent interface {
	Start(ctx context.Context, prompt string) (Session, error)
}

type Session interface {
	Run(ctx context.Context) (string, error)
}
