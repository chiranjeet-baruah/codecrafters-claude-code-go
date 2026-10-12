package application

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/codecrafters-io/claude-code-starter-go/internal/adapter/outbound/local"
	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
)

type testMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const skillInstruction = "\n\nIf a skill matches the user's request, call the Skill tool with its name and follow the instructions it returns."

func writeTestSkill(t *testing.T, root, name, description, body string) {
	t.Helper()
	writeRawSkill(t, root, name, "---\nname: "+name+"\ndescription: "+description+"\n---\n\n"+body+"\n")
}

func writeRawSkill(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func decodeMessages(t *testing.T, prompt, root string) ([]testMessage, error) {
	t.Helper()
	skills, err := local.DiscoverSkills(root)
	if err != nil {
		return nil, err
	}
	messages, err := NewAgent(nil, nil, local.SkillStore{}, skills).initialMessages(context.Background(), prompt)
	if err != nil {
		if messages != nil {
			t.Fatalf("got messages on error: %+v", messages)
		}
		return nil, err
	}
	decoded := make([]testMessage, len(messages))
	for i, message := range messages {
		decoded[i] = testMessage{Role: message.Role, Content: message.Content}
	}
	return decoded, nil
}

func TestInitialMessages(t *testing.T) {
	const system = "You have access to the following skills:\n\n- apple: Reports the deploy target.\n- banana: Explains the banana service." + skillInstruction
	const body = "Respond with exactly one word: blueberry\n\nDo not add any other text."
	tests := []struct {
		name    string
		prompt  string
		setup   func(*testing.T, string)
		want    []testMessage
		wantErr string
		forbid  []string
	}{
		{
			name:   "slash command loads only selected body",
			prompt: "/apple ignored arguments",
			setup: func(t *testing.T, root string) {
				writeTestSkill(t, root, "apple", "Reports the deploy target.", body)
				writeTestSkill(t, root, "banana", "Explains the banana service.", "secret banana instructions")
			},
			want:   []testMessage{{"system", system}, {"user", body}},
			forbid: []string{"secret banana instructions", "/apple"},
		},
		{
			name:   "slash command substitutes arguments only in body",
			prompt: "/apple\t staging  eu-west \n",
			setup: func(t *testing.T, root string) {
				writeTestSkill(t, root, "apple", "Deploy $0.", "Deploy to $0 in region $ARGUMENTS[1]. Full request was: $ARGUMENTS")
			},
			want: []testMessage{{"system", "You have access to the following skills:\n\n- apple: Deploy $0." + skillInstruction}, {"user", "Deploy to staging in region eu-west. Full request was: staging  eu-west"}},
		},
		{
			name:   "ordinary prompt keeps prompt and skill descriptions",
			prompt: "Where should I deploy $0 $ARGUMENTS[1] $ARGUMENTS?",
			setup: func(t *testing.T, root string) {
				writeTestSkill(t, root, "apple", "Reports the deploy target.", "hidden instructions")
			},
			want:   []testMessage{{"system", "You have access to the following skills:\n\n- apple: Reports the deploy target." + skillInstruction}, {"user", "Where should I deploy $0 $ARGUMENTS[1] $ARGUMENTS?"}},
			forbid: []string{"hidden instructions"},
		},
		{name: "no skills directory", prompt: "ordinary prompt", want: []testMessage{{"user", "ordinary prompt"}}},
		{
			name: "empty skills directory", prompt: "ordinary prompt", want: []testMessage{{"user", "ordinary prompt"}},
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(root, 0755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "ignore entries without SKILL.md", prompt: "ordinary prompt", want: []testMessage{{"user", "ordinary prompt"}},
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "incomplete"), 0755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unknown command", prompt: "/missing", wantErr: "unknown skill",
			setup: func(t *testing.T, root string) {
				writeTestSkill(t, root, "apple", "Reports the deploy target.", "blueberry")
			},
		},
		{name: "path traversal", prompt: "/../apple", wantErr: "invalid skill name"},
		{name: "empty command", prompt: "/", wantErr: "invalid skill name"},
		{
			name: "malformed skill", prompt: "/apple", wantErr: "missing closing ---",
			setup: func(t *testing.T, root string) {
				writeRawSkill(t, root, "apple", "---\nname: apple\ndescription: Broken")
			},
		},
		{
			name: "mismatched metadata name", prompt: "/apple", wantErr: "mismatched name",
			setup: func(t *testing.T, root string) {
				writeRawSkill(t, root, "apple", "---\nname: banana\ndescription: Wrong name\n---\nbody")
			},
		},
		{
			name: "unreadable skill", prompt: "/apple", wantErr: "parse skill",
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "apple", "SKILL.md"), 0755); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "skills")
			if tt.setup != nil {
				tt.setup(t, root)
			}
			got, err := decodeMessages(t, tt.prompt, root)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("messages = %+v, want %+v", got, tt.want)
			}
			for _, message := range got {
				for _, secret := range tt.forbid {
					if strings.Contains(message.Content, secret) {
						t.Fatalf("message disclosed %q: %+v", secret, message)
					}
				}
			}
		})
	}
}

func TestStackedSkillMessages(t *testing.T) {
	const system = "You have access to the following skills:\n\n- apple: Apple service.\n- banana: Banana service.\n- grape: Grape service." + skillInstruction
	tests := []struct {
		name, prompt string
		want         []string
	}{
		{"shared arguments", "/apple /grape 4127", []string{"blueberry-4127 (4127/)", "cherry-4127 (4127/)"}},
		{"prompt order", "/grape /apple 4127", []string{"cherry-4127 (4127/)", "blueberry-4127 (4127/)"}},
		{"ordinary argument ends expansion", "/apple 4127 /grape", []string{"blueberry-4127 /grape (4127//grape)"}},
		{"skill name without slash is an argument", "/apple grape /grape", []string{"blueberry-grape /grape (grape//grape)"}},
		{"unknown skill ends expansion", "/apple /pear 4127 /grape", []string{"blueberry-/pear 4127 /grape (/pear/4127)"}},
		{"unknown skill after stack is shared argument", "/apple /grape /pear 4127", []string{"blueberry-/pear 4127 (/pear/4127)", "cherry-/pear 4127 (/pear/4127)"}},
		{"malformed command ends expansion", "/apple /../grape 4127", []string{"blueberry-/../grape 4127 (/../grape/4127)"}},
		{"no arguments", "/apple /grape \t\n", []string{"blueberry- (/)", "cherry- (/)"}},
		{"repeated skill", "/apple /apple 4127", []string{"blueberry-4127 (4127/)", "blueberry-4127 (4127/)"}},
		{"whitespace preserved in shared arguments", "/apple\t\n/grape\u2003 staging\t \neu-west \n", []string{"blueberry-staging\t \neu-west (staging/eu-west)", "cherry-staging\t \neu-west (staging/eu-west)"}},
		{"inserted placeholders stay literal", "/apple /grape $1 $ARGUMENTS[0]", []string{"blueberry-$1 $ARGUMENTS[0] ($1/$ARGUMENTS[0])", "cherry-$1 $ARGUMENTS[0] ($1/$ARGUMENTS[0])"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestSkill(t, root, "apple", "Apple service.", "blueberry-$ARGUMENTS ($0/$ARGUMENTS[1])")
			writeTestSkill(t, root, "banana", "Banana service.", "secret banana body")
			writeTestSkill(t, root, "grape", "Grape service.", "cherry-$ARGUMENTS ($ARGUMENTS[0]/$1)")
			got, err := decodeMessages(t, tt.prompt, root)
			if err != nil {
				t.Fatal(err)
			}
			want := []testMessage{{"system", system}}
			for _, body := range tt.want {
				want = append(want, testMessage{"user", body})
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("messages = %+v, want %+v", got, want)
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
			skills, err := local.DiscoverSkills(dir)
			if err != nil {
				t.Fatal(err)
			}
			if tt.afterDiscovery != nil {
				tt.afterDiscovery(t, dir)
			}
			agent := NewAgent(nil, nil, local.SkillStore{}, skills)
			got := agent.handleToolCall(context.Background(), domain.ToolCall{Name: "Skill", Arguments: tt.arguments})
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
