package main

import (
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
)

func main() {
	prompt := parsePrompt()
	client := newClient()

	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	messages := []openai.ChatCompletionMessageParamUnion{
		userMessage(prompt),
	}

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
			result := handleToolCall(toolCall)
			messages = append(messages, toolMessage(toolCall.ID, result))
		}
	}
}
