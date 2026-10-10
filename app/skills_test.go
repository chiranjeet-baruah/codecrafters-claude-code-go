package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type testMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

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
	messages, err := initialMessages(prompt, root)
	if err != nil {
		if messages != nil {
			t.Fatalf("got messages on error: %+v", messages)
		}
		return nil, err
	}
	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []testMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded, nil
}

func TestInitialMessages(t *testing.T) {
	const system = "You have access to the following skills:\n\n- apple: Reports the deploy target.\n- banana: Explains the banana service."
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
			want: []testMessage{{"system", "You have access to the following skills:\n\n- apple: Deploy $0."}, {"user", "Deploy to staging in region eu-west. Full request was: staging  eu-west"}},
		},
		{
			name:   "ordinary prompt keeps prompt and skill descriptions",
			prompt: "Where should I deploy $0 $ARGUMENTS[1] $ARGUMENTS?",
			setup: func(t *testing.T, root string) {
				writeTestSkill(t, root, "apple", "Reports the deploy target.", "hidden instructions")
			},
			want:   []testMessage{{"system", "You have access to the following skills:\n\n- apple: Reports the deploy target."}, {"user", "Where should I deploy $0 $ARGUMENTS[1] $ARGUMENTS?"}},
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
	const system = "You have access to the following skills:\n\n- apple: Apple service.\n- banana: Banana service.\n- grape: Grape service."
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

func TestSubstituteSkillArguments(t *testing.T) {
	tests := []struct {
		name, body, arguments, want string
	}{
		{
			name:      "deployment example",
			body:      "Deploy to $0 in region $1. Full request was: $ARGUMENTS",
			arguments: "staging eu-west",
			want:      "Deploy to staging in region eu-west. Full request was: staging eu-west",
		},
		{
			name:      "mixed repeated placeholders",
			body:      "$ARGUMENTS[0]/$0/$ARGUMENTS[1]/$1/$ARGUMENTS/$ARGUMENTS/$0",
			arguments: "staging eu-west",
			want:      "staging/staging/eu-west/eu-west/staging eu-west/staging eu-west/staging",
		},
		{
			name:      "multi digit indexes",
			body:      "$10/$ARGUMENTS[10]/$1/$ARGUMENTS[11]",
			arguments: "a b c d e f g h i j k l",
			want:      "k/k/b/l",
		},
		{
			name:      "whitespace and body formatting",
			body:      "  $0\n\n$ARGUMENTS[1]\n$ARGUMENTS  ",
			arguments: " \tstaging\t \neu-west\r\n ",
			want:      "  staging\n\neu-west\nstaging\t \neu-west  ",
		},
		{name: "no arguments", body: "[$ARGUMENTS][$0][$ARGUMENTS[1]]", want: "[][][]"},
		{name: "missing argument", body: "$0/$1/$ARGUMENTS[2]", arguments: "staging", want: "staging//"},
		{name: "index overflow", body: "[$999999999999999999999999999999][$ARGUMENTS[999999999999999999999999999999]]", arguments: "staging", want: "[][]"},
		{
			name:      "argument placeholders remain literal",
			body:      "$0|$1|$ARGUMENTS[2]|$ARGUMENTS",
			arguments: "$1 $ARGUMENTS[0] $ARGUMENTS",
			want:      "$1|$ARGUMENTS[0]|$ARGUMENTS|$1 $ARGUMENTS[0] $ARGUMENTS",
		},
		{name: "no placeholders", body: "Deploy blueberry.", arguments: "staging", want: "Deploy blueberry."},
		{name: "quotes are literal", body: "$0|$1", arguments: "\"staging eu-west\"", want: "\"staging|eu-west\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := substituteSkillArguments(tt.body, tt.arguments); got != tt.want {
				t.Fatalf("substituteSkillArguments(%q, %q) = %q, want %q", tt.body, tt.arguments, got, tt.want)
			}
		})
	}
}

func TestValidSkillName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"apple", true}, {"apple-2_test", true}, {"", false}, {"../apple", false}, {"apple/banana", false}, {"apple space", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validSkillName(tt.name); got != tt.want {
				t.Fatalf("validSkillName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestReadSkillFrontmatter(t *testing.T) {
	tests := []struct {
		name, input, wantName, wantDescription, wantBody, wantErr string
	}{
		{"plain", "---\nname: apple\ndescription: Deploy target\n---\nbody", "apple", "Deploy target", "body", ""},
		{"quoted CRLF", "---\r\nname: \"apple\"\r\ndescription: 'Bob''s target'\r\n---\r\nbody", "apple", "Bob's target", "body", ""},
		{"missing opening", "name: apple\n---\n", "", "", "", "missing opening"},
		{"missing closing", "---\nname: apple\ndescription: target", "", "", "", "missing closing"},
		{"missing metadata", "---\nname: apple\n---\n", "", "", "", "missing name or description"},
		{"invalid quoted metadata", "---\nname: \"apple\ndescription: target\n---\n", "", "", "", "invalid quoted name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			name, description, err := readSkillFrontmatter(reader)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if name != tt.wantName || description != tt.wantDescription || string(body) != tt.wantBody {
				t.Fatalf("got (%q, %q, %q), want (%q, %q, %q)", name, description, body, tt.wantName, tt.wantDescription, tt.wantBody)
			}
		})
	}
}

func TestReadSkillBody(t *testing.T) {
	tests := []struct {
		name, content, wantBody, wantErr string
	}{
		{"preserves internal formatting", "---\nname: apple\ndescription: Target\n---\n\n  first line\n\nsecond line  \n", "  first line\n\nsecond line  ", ""},
		{"missing file", "", "", "open skill body"},
		{"malformed frontmatter", "body only", "", "parse skill body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "SKILL.md")
			if tt.name != "missing file" {
				if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			body, err := readSkillBody(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || body != tt.wantBody {
				t.Fatalf("body = %q, error = %v; want %q", body, err, tt.wantBody)
			}
		})
	}
}
