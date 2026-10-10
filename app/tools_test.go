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
			got := handleToolCall(call, nil, nil)
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

func TestHandleSkillToolCall(t *testing.T) {
	tests := []struct {
		name, arguments, body, want, wantPrefix string
		noSkills                                bool
		afterDiscovery                          func(*testing.T, string)
	}{
		{name: "selected body only", arguments: `{"name":"apple"}`, body: "Respond with exactly one word: blueberry", want: "Respond with exactly one word: blueberry"},
		{name: "omitted arguments", arguments: `{"name":"apple"}`, body: "[$ARGUMENTS][$0][$ARGUMENTS[1]]", want: "[][][]"},
		{name: "supplied arguments", arguments: `{"name":"apple","args":"staging eu-west"}`, body: "Deploy to $0 in $ARGUMENTS[1]. Request: $ARGUMENTS. Again: $0/$1", want: "Deploy to staging in eu-west. Request: staging eu-west. Again: staging/eu-west"},
		{name: "formatting and whitespace", arguments: `{"name":"apple","args":" \tstaging\t \neu-west \n"}`, body: "  $0\n\n$1\n$ARGUMENTS  ", want: "  staging\n\neu-west\nstaging\t \neu-west  "},
		{name: "inserted placeholders stay literal", arguments: `{"name":"apple","args":"$1 $ARGUMENTS[0]"}`, body: "$0|$1|$ARGUMENTS", want: "$1|$ARGUMENTS[0]|$1 $ARGUMENTS[0]"},
		{name: "invalid JSON", arguments: "{", wantPrefix: "error parsing arguments:"},
		{name: "invalid name type", arguments: `{"name":42}`, wantPrefix: "error parsing arguments:"},
		{name: "invalid args type", arguments: `{"name":"apple","args":[]}`, wantPrefix: "error parsing arguments:"},
		{name: "missing name", arguments: `{}`, want: `error: invalid skill name ""`},
		{name: "empty name", arguments: `{"name":""}`, want: `error: invalid skill name ""`},
		{name: "unsafe name", arguments: `{"name":"../apple"}`, want: `error: invalid skill name "../apple"`},
		{name: "leading slash", arguments: `{"name":"/apple"}`, want: `error: invalid skill name "/apple"`},
		{name: "unknown skill", arguments: `{"name":"pear"}`, want: `error: unknown skill "pear"`},
		{name: "case sensitive", arguments: `{"name":"Apple"}`, want: `error: unknown skill "Apple"`},
		{name: "no catalog", arguments: `{"name":"apple"}`, noSkills: true, want: `error: unknown skill "apple"`},
		{
			name: "removed after discovery", arguments: `{"name":"apple"}`, wantPrefix: `error loading skill "apple": open skill body:`,
			afterDiscovery: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, "apple", "SKILL.md")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "corrupted after discovery", arguments: `{"name":"apple"}`, wantPrefix: `error loading skill "apple": parse skill body:`,
			afterDiscovery: func(t *testing.T, dir string) {
				writeRawSkill(t, dir, "apple", "no frontmatter")
			},
		},
		{
			name: "unselected body is never loaded", arguments: `{"name":"apple"}`, body: "blueberry", want: "blueberry",
			afterDiscovery: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, "grape", "SKILL.md")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "new skill is not in startup catalog", arguments: `{"name":"pear"}`, want: `error: unknown skill "pear"`,
			afterDiscovery: func(t *testing.T, dir string) {
				writeTestSkill(t, dir, "pear", "Other skill.", "undiscovered instructions")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.noSkills {
				writeTestSkill(t, dir, "apple", "Migration status.", tt.body)
				writeTestSkill(t, dir, "grape", "Format code.", "secret grape instructions")
			}
			skills, err := discoverSkills(dir)
			if err != nil {
				t.Fatal(err)
			}
			if tt.afterDiscovery != nil {
				tt.afterDiscovery(t, dir)
			}
			call := openai.ChatCompletionMessageToolCallUnion{
				Function: openai.ChatCompletionMessageFunctionToolCallFunction{Name: "Skill", Arguments: tt.arguments},
			}
			got := handleToolCall(call, skills, nil)
			if tt.wantPrefix != "" {
				if !strings.HasPrefix(got, tt.wantPrefix) {
					t.Fatalf("result = %q, want prefix %q", got, tt.wantPrefix)
				}
			} else if got != tt.want {
				t.Fatalf("result = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolDefinitions(t *testing.T) {
	tests := []struct {
		name       string
		definition func() openai.ChatCompletionToolUnionParam
		required   []string
		wantJSON   string
	}{
		{"Read", readFileTool, []string{"file_path"}, ""},
		{"Write", writeFileTool, []string{"file_path", "content"}, ""},
		{"Bash", bashTool, []string{"command"}, ""},
		{"Skill", skillTool, []string{"name"}, `{"type":"function","function":{"name":"Skill","description":"Load a skill's instructions into the conversation","parameters":{"type":"object","required":["name"],"properties":{"name":{"type":"string","description":"The name of the skill to use"},"args":{"type":"string","description":"Optional arguments for the skill"}}}}}`},
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
			if tt.wantJSON != "" {
				var actual, expected any
				if err := json.Unmarshal(data, &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(tt.wantJSON), &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Fatalf("definition = %s, want %s", data, tt.wantJSON)
				}
			}
		})
	}
}
