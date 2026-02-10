package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// --- Entry Point ---

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

// --- Tool Call Handling ---

func handleToolCall(toolCall openai.ChatCompletionMessageToolCallUnion) string {
	switch toolCall.Function.Name {
	case "Read":
		return readFile(toolCall.Function.Arguments)
	default:
		return fmt.Sprintf("error: unknown tool %q", toolCall.Function.Name)
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

// --- CLI & Client Setup ---

func parsePrompt() string {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	return prompt
}

func newClient() *openai.Client {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL))
	return &client
}

// --- LLM Interaction ---

func complete(client *openai.Client, messages []openai.ChatCompletionMessageParamUnion) (*openai.ChatCompletion, error) {
	resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model:    "anthropic/claude-haiku-4.5",
		Messages: messages,
		Tools:    []openai.ChatCompletionToolUnionParam{readFileTool()},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return resp, nil
}

// --- Message & Tool Definitions ---

func userMessage(content string) openai.ChatCompletionMessageParamUnion {
	return openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(content),
			},
		},
	}
}

func assistantMessage(message openai.ChatCompletionMessage) openai.ChatCompletionMessageParamUnion {
	toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(message.ToolCalls))
	for i, tc := range message.ToolCalls {
		toolCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID: tc.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			},
		}
	}

	param := &openai.ChatCompletionAssistantMessageParam{}
	if message.Content != "" {
		param.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(message.Content),
		}
	}
	if len(toolCalls) > 0 {
		param.ToolCalls = toolCalls
	}

	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: param,
	}
}

func toolMessage(toolCallID, content string) openai.ChatCompletionMessageParamUnion {
	return openai.ChatCompletionMessageParamUnion{
		OfTool: &openai.ChatCompletionToolMessageParam{
			ToolCallID: toolCallID,
			Content: openai.ChatCompletionToolMessageParamContentUnion{
				OfString: openai.String(content),
			},
		},
	}
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
