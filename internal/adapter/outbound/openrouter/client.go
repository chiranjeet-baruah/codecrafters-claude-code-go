package openrouter

import (
	"context"
	"fmt"
	"os"

	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Client struct {
	sdk *openai.Client
}

func NewFromEnv() *Client {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL))
	return &Client{sdk: &client}
}

func (c *Client) Complete(ctx context.Context, messages []domain.Message) (domain.Message, error) {
	params := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, message := range messages {
		params[i] = messageParam(message)
	}
	resp, err := c.sdk.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    "anthropic/claude-haiku-4.5",
		Messages: params,
		Tools:    []openai.ChatCompletionToolUnionParam{readFileTool(), writeFileTool(), bashTool(), skillTool()},
	})
	if err != nil {
		return domain.Message{}, err
	}
	if len(resp.Choices) == 0 {
		return domain.Message{}, fmt.Errorf("no choices in response")
	}
	answer := resp.Choices[0].Message
	message := domain.Message{Role: "assistant", Content: answer.Content}
	for _, call := range answer.ToolCalls {
		message.ToolCalls = append(message.ToolCalls, domain.ToolCall{
			ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments,
		})
	}
	return message, nil
}
