package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
)

func toolArguments(t *testing.T, values map[string]string) string {
	t.Helper()
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestHandleToolCall(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		arguments  func(*testing.T, string) string
		want       string
		wantPrefix string
		verify     func(*testing.T, string)
	}{
		{
			name: "read file", tool: "Read",
			arguments: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "input.txt")
				if err := os.WriteFile(path, []byte("contents"), 0644); err != nil {
					t.Fatal(err)
				}
				return toolArguments(t, map[string]string{"file_path": path})
			},
			want: "contents",
		},
		{
			name: "write file", tool: "Write",
			arguments: func(t *testing.T, dir string) string {
				return toolArguments(t, map[string]string{"file_path": filepath.Join(dir, "output.txt"), "content": "written"})
			},
			want: "success",
			verify: func(t *testing.T, dir string) {
				data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
				if err != nil || string(data) != "written" {
					t.Fatalf("written file = %q, error %v", data, err)
				}
			},
		},
		{name: "bash", tool: "Bash", arguments: func(t *testing.T, _ string) string {
			return toolArguments(t, map[string]string{"command": "printf ok"})
		}, want: "ok"},
		{name: "unknown tool", tool: "Missing", want: `error: unknown tool "Missing"`},
		{name: "read invalid JSON", tool: "Read", arguments: func(*testing.T, string) string { return "{" }, wantPrefix: "error parsing arguments:"},
		{name: "write invalid JSON", tool: "Write", arguments: func(*testing.T, string) string { return "{" }, wantPrefix: "error parsing arguments:"},
		{name: "bash invalid JSON", tool: "Bash", arguments: func(*testing.T, string) string { return "{" }, wantPrefix: "error parsing arguments:"},
		{name: "read missing file", tool: "Read", arguments: func(t *testing.T, dir string) string {
			return toolArguments(t, map[string]string{"file_path": filepath.Join(dir, "missing.txt")})
		}, wantPrefix: "error reading file:"},
		{name: "write missing directory", tool: "Write", arguments: func(t *testing.T, dir string) string {
			return toolArguments(t, map[string]string{"file_path": filepath.Join(dir, "missing", "output.txt"), "content": "x"})
		}, wantPrefix: "error writing file:"},
		{name: "bash failure", tool: "Bash", arguments: func(t *testing.T, _ string) string {
			return toolArguments(t, map[string]string{"command": "exit 7"})
		}, wantPrefix: "error executing command:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var args string
			if tt.arguments != nil {
				args = tt.arguments(t, dir)
			}
			call := openai.ChatCompletionMessageToolCallUnion{
				Function: openai.ChatCompletionMessageFunctionToolCallFunction{Name: tt.tool, Arguments: args},
			}
			got := handleToolCall(call)
			if tt.wantPrefix != "" {
				if !strings.HasPrefix(got, tt.wantPrefix) {
					t.Fatalf("result = %q, want prefix %q", got, tt.wantPrefix)
				}
			} else if got != tt.want {
				t.Fatalf("result = %q, want %q", got, tt.want)
			}
			if tt.verify != nil {
				tt.verify(t, dir)
			}
		})
	}
}

func TestToolDefinitions(t *testing.T) {
	tests := []struct {
		name       string
		definition func() openai.ChatCompletionToolUnionParam
		required   []string
	}{
		{"Read", readFileTool, []string{"file_path"}},
		{"Write", writeFileTool, []string{"file_path", "content"}},
		{"Bash", bashTool, []string{"command"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.definition())
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Type     string `json:"type"`
				Function struct {
					Name       string `json:"name"`
					Parameters struct {
						Required   []string                   `json:"required"`
						Properties map[string]json.RawMessage `json:"properties"`
					} `json:"parameters"`
				} `json:"function"`
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.Type != "function" || got.Function.Name != tt.name || !reflect.DeepEqual(got.Function.Parameters.Required, tt.required) {
				t.Fatalf("definition = %s, want function %q with required %v", data, tt.name, tt.required)
			}
			for _, property := range tt.required {
				if _, ok := got.Function.Parameters.Properties[property]; !ok {
					t.Fatalf("definition %q has no %q property", tt.name, property)
				}
			}
		})
	}
}
