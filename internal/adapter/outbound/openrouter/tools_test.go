package openrouter

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/openai/openai-go/v3"
)

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
