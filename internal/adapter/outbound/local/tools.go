package local

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

type ToolRunner struct{}

func (ToolRunner) Execute(name, arguments string) string {
	switch name {
	case "Read":
		return readFile(arguments)
	case "Write":
		return writeFile(arguments)
	case "Bash":
		return executeBashCommand(arguments)
	default:
		return fmt.Sprintf("error: unknown tool %q", name)
	}
}

func readFile(rawArgs string) string {
	var args struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}

	contents, err := os.ReadFile(args.FilePath)
	if err != nil {
		return fmt.Sprintf("error reading file: %v", err)
	}

	return string(contents)
}

func writeFile(rawArgs string) string {
	var args struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}

	if err := os.WriteFile(args.FilePath, []byte(args.Content), 0644); err != nil {
		return fmt.Sprintf("error writing file: %v", err)
	}

	return "success"
}

func executeBashCommand(rawArgs string) string {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}

	cmd := exec.Command("bash", "-c", args.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("error executing command: %v", err)
	}

	return string(output)
}
