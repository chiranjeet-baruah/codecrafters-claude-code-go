package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
)

type fakeModel struct {
	responses []domain.Message
	requests  [][]domain.Message
}

func (m *fakeModel) Complete(_ context.Context, messages []domain.Message) (domain.Message, error) {
	m.requests = append(m.requests, append([]domain.Message(nil), messages...))
	return m.responses[len(m.requests)-1], nil
}

type fakeTools struct {
	name, arguments string
}

func (t *fakeTools) Execute(name, arguments string) string {
	t.name, t.arguments = name, arguments
	return "contents"
}

func TestAgentRunsToolLoop(t *testing.T) {
	call := domain.ToolCall{ID: "call_1", Name: "Read", Arguments: `{"file_path":"a.txt"}`}
	assistant := domain.Message{Role: "assistant", ToolCalls: []domain.ToolCall{call}}
	model := &fakeModel{responses: []domain.Message{assistant, {Role: "assistant", Content: "done"}}}
	tools := &fakeTools{}
	agent := NewAgent(model, tools, nil, nil)
	session, err := agent.Start(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(model.requests) != 0 {
		t.Fatal("ordinary prompt called the model during startup")
	}
	answer, err := session.Run(context.Background())
	if err != nil || answer != "done" {
		t.Fatalf("answer = %q, error = %v", answer, err)
	}
	if tools.name != "Read" || tools.arguments != call.Arguments {
		t.Fatalf("tool call = %q %q", tools.name, tools.arguments)
	}
	want := [][]domain.Message{
		{{Role: "user", Content: "hello"}},
		{{Role: "user", Content: "hello"}, assistant, {Role: "tool", ToolCallID: "call_1", Content: "contents"}},
	}
	if !reflect.DeepEqual(model.requests, want) {
		t.Fatalf("model requests = %+v, want %+v", model.requests, want)
	}
}
