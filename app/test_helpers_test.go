package main

import (
	"os"
	"path/filepath"
	"testing"
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
