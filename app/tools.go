package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func handleToolCall(toolCall openai.ChatCompletionMessageToolCallUnion, skills []skillInfo) string {
	switch toolCall.Function.Name {
	case "Read":
		return readFile(toolCall.Function.Arguments)
	case "Write":
		return writeFile(toolCall.Function.Arguments)
	case "Bash":
		return executeBashCommand(toolCall.Function.Arguments)
	case "Skill":
		return invokeSkill(toolCall.Function.Arguments, skills)
	default:
		return fmt.Sprintf("error: unknown tool %q", toolCall.Function.Name)
	}
}

func invokeSkill(rawArgs string, skills []skillInfo) string {
	var args struct {
		Name string `json:"name"`
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}
	if !validSkillName(args.Name) {
		return fmt.Sprintf("error: invalid skill name %q", args.Name)
	}
	for _, skill := range skills {
		if skill.name == args.Name {
			body, err := readSkillBody(skill.path)
			if err != nil {
				return fmt.Sprintf("error loading skill %q: %v", args.Name, err)
			}
			return substituteSkillArguments(body, args.Args)
		}
	}
	return fmt.Sprintf("error: unknown skill %q", args.Name)
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

func readFileTool() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        "Read",
				Description: openai.String("Read and return the contents of a file"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "The path to the file to read",
						},
					},
					"required": []string{"file_path"},
				},
			},
		},
	}
}

func writeFileTool() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        "Write",
				Description: openai.String("Write content to a file"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "The path of the file to write to",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The content to write to the file",
						},
					},
					"required": []string{"file_path", "content"},
				},
			},
		},
	}
}

func bashTool() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        "Bash",
				Description: openai.String("Execute a shell command"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"required": []string{
						"command",
					},
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The command to execute",
						},
					},
				},
			},
		},
	}
}

func skillTool() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        "Skill",
				Description: openai.String("Load a skill's instructions into the conversation"),
				Parameters: shared.FunctionParameters{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "The name of the skill to use",
						},
						"args": map[string]interface{}{
							"type":        "string",
							"description": "Optional arguments for the skill",
						},
					},
				},
			},
		},
	}
}
