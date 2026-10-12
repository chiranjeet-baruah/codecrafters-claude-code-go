package local

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
)

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
			name, description, _, err := readSkillFrontmatter(reader)
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
			body, err := (SkillStore{}).ReadBody(path)
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

func TestSkillContext(t *testing.T) {
	tests := []struct {
		name, frontmatter, wantContext string
	}{
		{"absent", "", ""},
		{"plain fork", "context: fork\n", "fork"},
		{"no space", "context:fork\n", "fork"},
		{"double quoted", "context: \"fork\"\n", "fork"},
		{"single quoted", "context: 'fork'\n", "fork"},
		{"surrounding whitespace", " context : \tfork \t\n", "fork"},
		{"other context", "context: inline\n", "inline"},
		{"case sensitive", "context: Fork\n", "Fork"},
		{"agent ignored", "agent: fork\n", ""},
		{"fork with agent", "context: fork\nagent: reviewer\n", "fork"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRawSkill(t, dir, "apple", "---\nname: apple\ndescription: On-call rotation.\n"+tt.frontmatter+"---\n\nsecret body\n")
			skills, err := DiscoverSkills(dir)
			if err != nil {
				t.Fatal(err)
			}
			want := []domain.Skill{{Name: "apple", Description: "On-call rotation.", Context: tt.wantContext, Path: filepath.Join(dir, "apple", "SKILL.md")}}
			if !reflect.DeepEqual(skills, want) {
				t.Fatalf("skills = %+v, want %+v", skills, want)
			}
			body, err := (SkillStore{}).ReadBody(skills[0].Path)
			if err != nil || body != "secret body" {
				t.Fatalf("body = %q, error = %v", body, err)
			}
		})
	}
}
