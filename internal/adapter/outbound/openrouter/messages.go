package openrouter

import (
	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
	"github.com/openai/openai-go/v3"
)

func messageParam(message domain.Message) openai.ChatCompletionMessageParamUnion {
	switch message.Role {
	case "system":
		return openai.SystemMessage(message.Content)
	case "user":
		return openai.ChatCompletionMessageParamUnion{
			OfUser: &openai.ChatCompletionUserMessageParam{
				Content: openai.ChatCompletionUserMessageParamContentUnion{
					OfString: openai.String(message.Content),
				},
			},
		}
	case "tool":
		return openai.ChatCompletionMessageParamUnion{
			OfTool: &openai.ChatCompletionToolMessageParam{
				ToolCallID: message.ToolCallID,
				Content: openai.ChatCompletionToolMessageParamContentUnion{
					OfString: openai.String(message.Content),
				},
			},
		}
	case "assistant":
		param := &openai.ChatCompletionAssistantMessageParam{}
		if message.Content != "" {
			param.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: openai.String(message.Content),
			}
		}
		if len(message.ToolCalls) > 0 {
			toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(message.ToolCalls))
			for i, call := range message.ToolCalls {
				toolCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: call.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name: call.Name, Arguments: call.Arguments,
						},
					},
				}
			}
			param.ToolCalls = toolCalls
		}
		return openai.ChatCompletionMessageParamUnion{OfAssistant: param}
	}
	panic("unexpected message role: " + message.Role)
}
