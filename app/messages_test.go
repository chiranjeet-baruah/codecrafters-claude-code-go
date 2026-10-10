package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/openai/openai-go/v3"
)

func TestMessageHelpers(t *testing.T) {
	toolCall := openai.ChatCompletionMessageToolCallUnion{
		ID: "call_1",
		Function: openai.ChatCompletionMessageFunctionToolCallFunction{
			Name: "Read", Arguments: `{"file_path":"a.txt"}`,
		},
	}
	tests := []struct {
		name string
		make func() openai.ChatCompletionMessageParamUnion
		want string
	}{
		{"user", func() openai.ChatCompletionMessageParamUnion { return userMessage("hello") }, `{"role":"user","content":"hello"}`},
		{"tool", func() openai.ChatCompletionMessageParamUnion { return toolMessage("call_1", "contents") }, `{"role":"tool","tool_call_id":"call_1","content":"contents"}`},
		{"assistant text", func() openai.ChatCompletionMessageParamUnion {
			return assistantMessage(openai.ChatCompletionMessage{Content: "done"})
		}, `{"role":"assistant","content":"done"}`},
		{"assistant tool call", func() openai.ChatCompletionMessageParamUnion {
			return assistantMessage(openai.ChatCompletionMessage{ToolCalls: []openai.ChatCompletionMessageToolCallUnion{toolCall}})
		}, `{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"Read","arguments":"{\"file_path\":\"a.txt\"}"}}]}`},
		{"assistant text and tool call", func() openai.ChatCompletionMessageParamUnion {
			return assistantMessage(openai.ChatCompletionMessage{Content: "reading", ToolCalls: []openai.ChatCompletionMessageToolCallUnion{toolCall}})
		}, `{"role":"assistant","content":"reading","tool_calls":[{"id":"call_1","type":"function","function":{"name":"Read","arguments":"{\"file_path\":\"a.txt\"}"}}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, err := json.Marshal(tt.make())
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
