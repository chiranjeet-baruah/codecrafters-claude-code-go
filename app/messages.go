package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/openai/openai-go/v3"
)

func initialMessages(prompt string, skills []skillInfo) ([]openai.ChatCompletionMessageParamUnion, error) {
	prompts := []string{prompt}
	if strings.HasPrefix(prompt, "/") {
		commands := strings.Fields(prompt)
		name := strings.TrimPrefix(commands[0], "/")
		if !validSkillName(name) {
			return nil, fmt.Errorf("invalid skill name %q", name)
		}

		arguments := prompt
		var selected []skillInfo
		for _, command := range commands {
			if !strings.HasPrefix(command, "/") {
				break
			}
			found := false
			for _, skill := range skills {
				if skill.name == strings.TrimPrefix(command, "/") {
					selected = append(selected, skill)
					found = true
					break
				}
			}
			if !found {
				break
			}
			arguments = strings.TrimLeftFunc(arguments[len(command):], unicode.IsSpace)
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("unknown skill %q", name)
		}
		prompts = make([]string, 0, len(selected))
		for _, skill := range selected {
			body, err := readSkillBody(skill.path)
			if err != nil {
				return nil, fmt.Errorf("load skill %q: %w", skill.name, err)
			}
			prompts = append(prompts, substituteSkillArguments(body, arguments))
		}
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(prompts)+1)
	if len(skills) > 0 {
		var description strings.Builder
		description.WriteString("You have access to the following skills:\n\n")
		for i, skill := range skills {
			if i > 0 {
				description.WriteByte('\n')
			}
			fmt.Fprintf(&description, "- %s: %s", skill.name, skill.description)
		}
		description.WriteString("\n\nIf a skill matches the user's request, call the Skill tool with its name and follow the instructions it returns.")
		messages = append(messages, openai.SystemMessage(description.String()))
	}
	for _, content := range prompts {
		messages = append(messages, userMessage(content))
	}
	return messages, nil
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
