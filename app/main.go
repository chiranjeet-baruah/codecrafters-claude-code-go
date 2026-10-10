package main

import (
	"fmt"
	"os"
)

func main() {
	prompt := parsePrompt()
	skills, err := discoverSkills(".claude/skills")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	messages, err := initialMessages(prompt, skills)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	client := newClient()

	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	for {
		resp, err := complete(client, messages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		message := resp.Choices[0].Message
		messages = append(messages, assistantMessage(message))

		if len(message.ToolCalls) == 0 {
			fmt.Print(message.Content)
			return
		}

		for _, toolCall := range message.ToolCalls {
			result := handleToolCall(toolCall, skills)
			messages = append(messages, toolMessage(toolCall.ID, result))
		}
	}
}
