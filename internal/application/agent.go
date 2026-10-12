package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/codecrafters-io/claude-code-starter-go/internal/domain"
	"github.com/codecrafters-io/claude-code-starter-go/internal/port/inbound"
	"github.com/codecrafters-io/claude-code-starter-go/internal/port/outbound"
)

type Agent struct {
	model   outbound.Model
	tools   outbound.ToolRunner
	skills  outbound.SkillStore
	catalog []domain.Skill
}

func NewAgent(model outbound.Model, tools outbound.ToolRunner, skills outbound.SkillStore, catalog []domain.Skill) *Agent {
	return &Agent{model: model, tools: tools, skills: skills, catalog: catalog}
}

func (a *Agent) Start(ctx context.Context, prompt string) (inbound.Session, error) {
	messages, err := a.initialMessages(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return &session{agent: a, messages: messages}, nil
}

type session struct {
	agent    *Agent
	messages []domain.Message
}

func (s *session) Run(ctx context.Context) (string, error) {
	for {
		message, err := s.agent.model.Complete(ctx, s.messages)
		if err != nil {
			return "", err
		}
		s.messages = append(s.messages, message)
		if len(message.ToolCalls) == 0 {
			return message.Content, nil
		}
		for _, call := range message.ToolCalls {
			result := s.agent.handleToolCall(ctx, call)
			s.messages = append(s.messages, domain.Message{Role: "tool", ToolCallID: call.ID, Content: result})
		}
	}
}

func (a *Agent) handleToolCall(ctx context.Context, call domain.ToolCall) string {
	if call.Name != "Skill" {
		return a.tools.Execute(call.Name, call.Arguments)
	}
	var args struct {
		Name string `json:"name"`
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}
	if !domain.ValidSkillName(args.Name) {
		return fmt.Sprintf("error: invalid skill name %q", args.Name)
	}
	for _, skill := range a.catalog {
		if skill.Name == args.Name {
			content, err := a.executeSkill(ctx, skill, args.Args)
			if err != nil {
				return fmt.Sprintf("error %v", err)
			}
			return content
		}
	}
	return fmt.Sprintf("error: unknown skill %q", args.Name)
}

func (a *Agent) executeSkill(ctx context.Context, skill domain.Skill, arguments string) (string, error) {
	body, err := a.skills.ReadBody(skill.Path)
	if err != nil {
		return "", fmt.Errorf("loading skill %q: %w", skill.Name, err)
	}
	body = domain.SubstituteSkillArguments(body, arguments)
	if skill.Context != "fork" {
		return body, nil
	}
	answer, err := (&session{agent: a, messages: []domain.Message{{Role: "user", Content: body}}}).Run(ctx)
	if err != nil {
		return "", fmt.Errorf("running skill %q in separate context: %w", skill.Name, err)
	}
	return fmt.Sprintf("Skill %s ran in a separate context and returned: %s", skill.Name, answer), nil
}

func (a *Agent) initialMessages(ctx context.Context, prompt string) ([]domain.Message, error) {
	prompts := []string{prompt}
	if strings.HasPrefix(prompt, "/") {
		commands := strings.Fields(prompt)
		name := strings.TrimPrefix(commands[0], "/")
		if !domain.ValidSkillName(name) {
			return nil, fmt.Errorf("invalid skill name %q", name)
		}

		arguments := prompt
		var selected []domain.Skill
		for _, command := range commands {
			if !strings.HasPrefix(command, "/") {
				break
			}
			found := false
			for _, skill := range a.catalog {
				if skill.Name == strings.TrimPrefix(command, "/") {
					if skill.Context == "fork" && len(selected) > 0 {
						break
					}
					selected = append(selected, skill)
					found = true
					break
				}
			}
			if !found {
				break
			}
			arguments = strings.TrimLeftFunc(arguments[len(command):], unicode.IsSpace)
			if selected[len(selected)-1].Context == "fork" {
				break
			}
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("unknown skill %q", name)
		}
		prompts = make([]string, 0, len(selected))
		for _, skill := range selected {
			content, err := a.executeSkill(ctx, skill, arguments)
			if err != nil {
				return nil, err
			}
			prompts = append(prompts, content)
		}
	}

	messages := make([]domain.Message, 0, len(prompts)+1)
	if len(a.catalog) > 0 {
		var description strings.Builder
		description.WriteString("You have access to the following skills:\n\n")
		for i, skill := range a.catalog {
			if i > 0 {
				description.WriteByte('\n')
			}
			fmt.Fprintf(&description, "- %s: %s", skill.Name, skill.Description)
		}
		description.WriteString("\n\nIf a skill matches the user's request, call the Skill tool with its name and follow the instructions it returns.")
		messages = append(messages, domain.Message{Role: "system", Content: description.String()})
	}
	for _, content := range prompts {
		messages = append(messages, domain.Message{Role: "user", Content: content})
	}
	return messages, nil
}
