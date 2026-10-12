package openrouter

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
)

func TestMessageHelpers(t *testing.T) {
	toolCall := domain.ToolCall{ID: "call_1", Name: "Read", Arguments: `{"file_path":"a.txt"}`}
	tests := []struct {
		name string
		make func() domain.Message
		want string
	}{
		{"user", func() domain.Message { return domain.Message{Role: "user", Content: "hello"} }, `{"role":"user","content":"hello"}`},
		{"tool", func() domain.Message { return domain.Message{Role: "tool", ToolCallID: "call_1", Content: "contents"} }, `{"role":"tool","tool_call_id":"call_1","content":"contents"}`},
		{"assistant text", func() domain.Message {
			return domain.Message{Role: "assistant", Content: "done"}
		}, `{"role":"assistant","content":"done"}`},
		{"assistant tool call", func() domain.Message {
			return domain.Message{Role: "assistant", ToolCalls: []domain.ToolCall{toolCall}}
		}, `{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"Read","arguments":"{\"file_path\":\"a.txt\"}"}}]}`},
		{"assistant text and tool call", func() domain.Message {
			return domain.Message{Role: "assistant", Content: "reading", ToolCalls: []domain.ToolCall{toolCall}}
		}, `{"role":"assistant","content":"reading","tool_calls":[{"id":"call_1","type":"function","function":{"name":"Read","arguments":"{\"file_path\":\"a.txt\"}"}}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, err := json.Marshal(messageParam(tt.make()))
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(gotJSON, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("message = %s, want %s", gotJSON, tt.want)
			}
		})
	}
}
