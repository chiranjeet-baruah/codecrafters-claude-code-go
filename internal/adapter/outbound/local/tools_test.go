package local

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			got := (ToolRunner{}).Execute(tt.tool, args)
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
