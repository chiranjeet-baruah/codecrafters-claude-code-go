package main

import (
	"bytes"
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

func TestForkCLI(t *testing.T) {
	binary := buildTestCLI(t)
	const question = "Who is on the on-call rotation right now?"
	const system = "You have access to the following skills:\n\n- apple: Use this skill when the user asks about the on-call rotation.\n- grape: Use this skill when the user asks to format source code." + skillInstruction
	const body = "Respond with exactly one word: blueberry"
	const framed = "Skill apple ran in a separate context and returned: blueberry"
	const forkError = `error running skill "apple" in separate context: no choices in response`
	const final = "The on-call engineer is blueberry."
	message := func(role, content string) map[string]any {
		return map[string]any{"role": role, "content": content}
	}
	toolCall := func(id, name, arguments string) map[string]any {
		return map[string]any{"role": "assistant", "tool_calls": []any{
			map[string]any{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": arguments}},
		}}
	}
	toolResult := func(id, content string) map[string]any {
		return map[string]any{"role": "tool", "tool_call_id": id, "content": content}
	}
	tests := []struct {
		name, prompt, arguments, body, wantBody, context, childTool string
		childError, mainError, changeContext                        bool
	}{
		{name: "model invoked fork", prompt: question, arguments: `{"name":"apple"}`, body: body, wantBody: body, context: "fork"},
		{
			name: "model fork with arguments", prompt: question, arguments: `{"name":"apple","args":" staging\t eu-west "}`,
			body: "  Deploy $0 to $ARGUMENTS[1].\n\nFull: $ARGUMENTS; again $0", wantBody: "  Deploy staging to eu-west.\n\nFull: staging\t eu-west; again staging", context: "fork",
		},
		{
			name: "model fork with literal placeholders", prompt: question, arguments: `{"name":"apple","args":"$1 $ARGUMENTS[0]"}`,
			body: "$0/$ARGUMENTS[1]/$ARGUMENTS", wantBody: "$1/$ARGUMENTS[0]/$1 $ARGUMENTS[0]", context: "fork",
		},
		{name: "child executes Read", prompt: question, arguments: `{"name":"apple"}`, body: body, wantBody: body, context: "fork", childTool: "Read"},
		{name: "child uses startup skill catalog", prompt: question, arguments: `{"name":"apple"}`, body: body, wantBody: body, context: "fork", childTool: "Skill"},
		{name: "child API failure returns to main", prompt: question, arguments: `{"name":"apple"}`, body: body, wantBody: body, context: "fork", childError: true},
		{name: "startup context is retained", prompt: question, arguments: `{"name":"apple"}`, body: body, wantBody: body, context: "fork", changeContext: true},
		{name: "other context stays inline", prompt: question, arguments: `{"name":"apple"}`, body: body, wantBody: body, context: "inline"},
		{name: "context is case sensitive", prompt: question, arguments: `{"name":"apple"}`, body: body, wantBody: body, context: "Fork"},
		{name: "slash fork alone", prompt: "/apple", body: body, wantBody: body, context: "fork"},
		{
			name: "slash fork ends expansion", prompt: "/apple /grape 4127", context: "fork",
			body: "$0/$ARGUMENTS[1]/$ARGUMENTS", wantBody: "/grape/4127//grape 4127",
		},
		{
			name: "slash fork argument whitespace", prompt: "/apple\t\n/grape\u2003 staging\t \neu-west \n", context: "fork",
			body: "$0|$1|$ARGUMENTS", wantBody: "/grape|staging|/grape\u2003 staging\t \neu-west",
		},
		{
			name: "normal slash stops before fork", prompt: "/grape /apple 4127", context: "fork", body: body,
			wantBody: "Format /apple 4127 (/apple/4127)",
		},
		{
			name: "normal stack stops before fork", prompt: "/grape /grape\t/apple staging\t \neu-west \n", context: "fork", body: body,
			wantBody: "Format /apple staging\t \neu-west (/apple/staging)",
		},
		{name: "slash fork failure exits", prompt: "/apple", body: body, wantBody: body, context: "fork", childError: true},
		{name: "main API failure exits", prompt: question, body: body, context: "fork", mainError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".claude", "skills")
			writeRawSkill(t, dir, "apple", "---\nname: apple\ndescription: Use this skill when the user asks about the on-call rotation.\ncontext: "+tt.context+"\n---\n\n"+tt.body+"\n")
			writeTestSkill(t, dir, "grape", "Use this skill when the user asks to format source code.", "Format $ARGUMENTS ($0/$1)")
			catalog := system
			if tt.childTool == "Skill" {
				writeTestSkill(t, dir, "helper", "Helper instructions.", "private child skill body $ARGUMENTS")
				catalog = strings.TrimSuffix(system, skillInstruction) + "\n- helper: Helper instructions." + skillInstruction
			}
			readArgs, err := json.Marshal(map[string]string{"file_path": filepath.Join(root, "input.txt")})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "input.txt"), []byte("private child tool contents"), 0644); err != nil {
				t.Fatal(err)
			}

			var responses []string
			var requests [][]map[string]any
			mainMessages := []map[string]any{message("system", catalog), message("user", tt.prompt)}
			wantOutput, wantError, wantStderr := final, false, ""
			slash := strings.HasPrefix(tt.prompt, "/")
			switch {
			case tt.mainError:
				requests = append(requests, mainMessages)
				responses = append(responses, `{"choices":[]}`)
				wantOutput, wantError, wantStderr = "", true, "no choices in response"
			case strings.HasPrefix(tt.prompt, "/grape"):
				mainMessages = []map[string]any{message("system", catalog), message("user", tt.wantBody)}
				if strings.HasPrefix(tt.prompt, "/grape /grape") {
					mainMessages = append(mainMessages, message("user", tt.wantBody))
				}
				requests = append(requests, mainMessages)
				responses = append(responses, chatResponse(final))
			default:
				if !slash {
					requests = append(requests, mainMessages)
					responses = append(responses, toolResponse("parent_skill", "Skill", tt.arguments))
				}
				result := tt.wantBody
				if tt.context == "fork" {
					childMessages := []map[string]any{message("user", tt.wantBody)}
					requests = append(requests, childMessages)
					if tt.childTool != "" {
						arguments, content := string(readArgs), "private child tool contents"
						if tt.childTool == "Skill" {
							arguments, content = `{"name":"helper","args":"staging"}`, "private child skill body staging"
						}
						responses = append(responses, toolResponse("child_tool", tt.childTool, arguments))
						childMessages = append(childMessages, toolCall("child_tool", tt.childTool, arguments), toolResult("child_tool", content))
						requests = append(requests, childMessages)
					}
					result = framed
					if tt.childError {
						responses = append(responses, `{"choices":[]}`)
						result = forkError
					} else {
						responses = append(responses, chatResponse("blueberry"))
					}
				}
				if slash && tt.childError {
					wantOutput, wantError, wantStderr = "", true, strings.TrimPrefix(forkError, "error ")
				} else {
					if slash {
						mainMessages = []map[string]any{message("system", catalog), message("user", result)}
					} else {
						mainMessages = append(mainMessages, toolCall("parent_skill", "Skill", tt.arguments), toolResult("parent_skill", result))
					}
					requests = append(requests, mainMessages)
					responses = append(responses, chatResponse(final))
				}
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
					Messages []map[string]any `json:"messages"`
					Tools    []struct {
						Function struct{ Name string } `json:"function"`
					} `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
				}
				if !reflect.DeepEqual(request.Messages, requests[call]) {
					t.Errorf("request %d messages = %+v, want %+v", call+1, request.Messages, requests[call])
				}
				var names []string
				for _, tool := range request.Tools {
					names = append(names, tool.Function.Name)
				}
				if !reflect.DeepEqual(names, []string{"Read", "Write", "Bash", "Skill"}) {
					t.Errorf("request %d tools = %v", call+1, names)
				}
				if call == 0 && tt.changeContext {
					content := "---\nname: apple\ndescription: Changed metadata.\n---\n\n" + tt.body
					if err := os.WriteFile(filepath.Join(dir, "apple", "SKILL.md"), []byte(content), 0644); err != nil {
						t.Errorf("change skill context: %v", err)
						http.Error(w, "cannot change skill", http.StatusBadRequest)
						return
					}
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, responses[call])
			}))
			defer server.Close()
			command := exec.Command(binary, "-p", tt.prompt)
			command.Dir = root
			command.Env = append(os.Environ(), "OPENROUTER_API_KEY=test-key", "OPENROUTER_BASE_URL="+server.URL)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			err = command.Run()
			if (err != nil) != wantError || stdout.String() != wantOutput || !strings.Contains(stderr.String(), wantStderr) {
				t.Fatalf("stdout = %q, stderr = %q, error = %v; want stdout %q, error %v, stderr containing %q", stdout.String(), stderr.String(), err, wantOutput, wantError, wantStderr)
			}
			if int(calls.Load()) != len(requests) {
				t.Fatalf("API calls = %d, want %d", calls.Load(), len(requests))
			}
		})
	}
}
