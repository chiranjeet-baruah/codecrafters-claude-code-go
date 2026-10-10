package main

import (
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
)

func main() {
	prompt := parsePrompt()
	skills, err := discoverSkills(".claude/skills")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	client := newClient()
	messages, err := initialMessages(prompt, skills, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")
	answer, err := runAgent(client, messages, skills)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(answer)
}

func runAgent(client *openai.Client, messages []openai.ChatCompletionMessageParamUnion, skills []skillInfo) (string, error) {
	for {
		resp, err := complete(client, messages)
		if err != nil {
			return "", err
		}

		message := resp.Choices[0].Message
		messages = append(messages, assistantMessage(message))

		if len(message.ToolCalls) == 0 {
			return message.Content, nil
		}

		for _, toolCall := range message.ToolCalls {
			result := handleToolCall(toolCall, skills, client)
			messages = append(messages, toolMessage(toolCall.ID, result))
		}
	}
}
