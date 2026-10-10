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
			name:   "ordinary prompt keeps prompt and skill descriptions",
			prompt: "Where should I deploy?",
			setup: func(t *testing.T, root string) {
				writeTestSkill(t, root, "apple", "Reports the deploy target.", "hidden instructions")
			},
			want:   []testMessage{{"system", "You have access to the following skills:\n\n- apple: Reports the deploy target."}, {"user", "Where should I deploy?"}},
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
