package main

import (
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
)

func initialMessages(prompt, skillsDir string) ([]openai.ChatCompletionMessageParamUnion, error) {
	skills, err := discoverSkills(skillsDir)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(prompt, "/") {
		command := strings.Fields(prompt)[0]
		name := strings.TrimPrefix(command, "/")
		arguments := prompt[len(command):]
		if !validSkillName(name) {
			return nil, fmt.Errorf("invalid skill name %q", name)
		}

		found := false
		for _, skill := range skills {
			if skill.name != name {
				continue
			}
			prompt, err = readSkillBody(skill.path)
			if err != nil {
				return nil, fmt.Errorf("load skill %q: %w", name, err)
			}
			prompt = substituteSkillArguments(prompt, arguments)
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("unknown skill %q", name)
		}
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, 0, 2)
	if len(skills) > 0 {
		var description strings.Builder
		description.WriteString("You have access to the following skills:\n\n")
		for i, skill := range skills {
			if i > 0 {
				description.WriteByte('\n')
			}
			fmt.Fprintf(&description, "- %s: %s", skill.name, skill.description)
		}
		messages = append(messages, openai.SystemMessage(description.String()))
	}
	return append(messages, userMessage(prompt)), nil
}

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
	param := &openai.ChatCompletionAssistantMessageParam{}

	if message.Content != "" {
		param.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(message.Content),
		}
	}

	if len(message.ToolCalls) > 0 {
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
