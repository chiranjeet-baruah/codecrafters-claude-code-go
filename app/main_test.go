package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func chatResponse(content string) string {
	return fmt.Sprintf(`{"id":"chat_1","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":"stop"}]}`, content)
}

func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "agent")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	tests := []struct {
		name       string
		prompt     string
		setup      func(*testing.T, string)
		responses  func(*testing.T, string) []string
		wantCalls  int
		wantOutput string
		wantError  bool
		check      func(*testing.T, []testMessage)
	}{
		{
			name: "ordinary prompt", prompt: "hello", wantCalls: 1, wantOutput: "done",
			check: func(t *testing.T, messages []testMessage) {
				if len(messages) != 1 || messages[0] != (testMessage{Role: "user", Content: "hello"}) {
					t.Errorf("messages = %+v", messages)
				}
			},
		},
		{
			name: "slash command", prompt: "/apple ignored", wantCalls: 1, wantOutput: "done",
			setup: func(t *testing.T, root string) {
				writeTestSkill(t, filepath.Join(root, ".claude", "skills"), "apple", "Reports the deploy target.", "blueberry")
				writeTestSkill(t, filepath.Join(root, ".claude", "skills"), "banana", "Other skill.", "secret banana body")
			},
			check: func(t *testing.T, messages []testMessage) {
				if len(messages) != 2 || !strings.Contains(messages[0].Content, "- banana: Other skill.") || messages[1] != (testMessage{Role: "user", Content: "blueberry"}) {
					t.Errorf("messages = %+v", messages)
				}
				for _, message := range messages {
					if strings.Contains(message.Content, "secret banana body") || strings.Contains(message.Content, "/apple") {
						t.Errorf("unexpected disclosure: %+v", message)
					}
				}
			},
		},
		{
			name: "slash command arguments", prompt: "/apple staging eu-west", wantCalls: 1, wantOutput: "done",
			setup: func(t *testing.T, root string) {
				writeTestSkill(t, filepath.Join(root, ".claude", "skills"), "apple", "Deploy service.", "Deploy to $0 in region $ARGUMENTS[1]. Full request was: $ARGUMENTS")
				writeTestSkill(t, filepath.Join(root, ".claude", "skills"), "banana", "Other skill.", "secret banana body $0")
			},
			check: func(t *testing.T, messages []testMessage) {
				want := "Deploy to staging in region eu-west. Full request was: staging eu-west"
				if len(messages) != 2 || messages[0] != (testMessage{Role: "system", Content: "You have access to the following skills:\n\n- apple: Deploy service.\n- banana: Other skill."}) || messages[1] != (testMessage{Role: "user", Content: want}) {
					t.Errorf("messages = %+v", messages)
				}
			},
		},
		{
			name: "stacked skills", prompt: "/apple /grape 4127", wantCalls: 1, wantOutput: "blueberry-4127\ncherry-4127",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, ".claude", "skills")
				writeTestSkill(t, dir, "apple", "Apple service.", "Add this exact line to your response: blueberry-$ARGUMENTS")
				writeTestSkill(t, dir, "banana", "Banana service.", "secret banana body")
				writeTestSkill(t, dir, "grape", "Grape service.", "Add this exact line to your response: cherry-$ARGUMENTS")
			},
			responses: func(t *testing.T, root string) []string {
				return []string{chatResponse("blueberry-4127\ncherry-4127")}
			},
			check: func(t *testing.T, messages []testMessage) {
				want := []testMessage{
					{"system", "You have access to the following skills:\n\n- apple: Apple service.\n- banana: Banana service.\n- grape: Grape service."},
					{"user", "Add this exact line to your response: blueberry-4127"},
					{"user", "Add this exact line to your response: cherry-4127"},
				}
				if !reflect.DeepEqual(messages, want) {
					t.Errorf("messages = %+v, want %+v", messages, want)
				}
			},
		},
		{name: "invalid command never calls API", prompt: "/missing", wantCalls: 0, wantOutput: "unknown skill", wantError: true},
		{
			name: "tool call loop", prompt: "read the file", wantCalls: 2, wantOutput: "done",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "input.txt"), []byte("file contents"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			responses: func(t *testing.T, root string) []string {
				args, err := json.Marshal(map[string]string{"file_path": filepath.Join(root, "input.txt")})
				if err != nil {
					t.Fatal(err)
				}
				return []string{
					fmt.Sprintf(`{"id":"chat_1","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"Read","arguments":%q}}]},"finish_reason":"tool_calls"}]}`, string(args)),
					chatResponse("done"),
				}
			},
			check: func(t *testing.T, messages []testMessage) {
				if len(messages) != 3 || messages[0].Role != "user" || messages[1].Role != "assistant" || messages[2] != (testMessage{Role: "tool", Content: "file contents"}) {
					t.Errorf("tool loop messages = %+v", messages)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.setup != nil {
				tt.setup(t, root)
			}
			responses := []string{chatResponse("done")}
			if tt.responses != nil {
				responses = tt.responses(t, root)
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := int(calls.Add(1)) - 1
				if call >= len(responses) {
					t.Errorf("unexpected API call %d", call+1)
					http.Error(w, "unexpected call", http.StatusBadRequest)
					return
				}
				var request struct {
					Messages []testMessage `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
				}
				if tt.check != nil && call == len(responses)-1 {
					tt.check(t, request.Messages)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, responses[call])
			}))
			defer server.Close()
			command := exec.Command(binary, "-p", tt.prompt)
			command.Dir = root
			command.Env = append(os.Environ(), "OPENROUTER_API_KEY=test-key", "OPENROUTER_BASE_URL="+server.URL)
			output, err := command.CombinedOutput()
			if (err != nil) != tt.wantError || !strings.Contains(string(output), tt.wantOutput) {
				t.Fatalf("CLI output = %q, error = %v; want output containing %q and error %v", output, err, tt.wantOutput, tt.wantError)
			}
			if int(calls.Load()) != tt.wantCalls {
				t.Fatalf("API calls = %d, want %d", calls.Load(), tt.wantCalls)
			}
		})
	}
}
