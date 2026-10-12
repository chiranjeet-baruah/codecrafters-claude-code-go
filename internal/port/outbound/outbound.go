package outbound

import (
	"context"

	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
)

type Model interface {
	Complete(ctx context.Context, messages []domain.Message) (domain.Message, error)
}

type ToolRunner interface {
	Execute(name, arguments string) string
}

type SkillStore interface {
	ReadBody(path string) (string, error)
}
