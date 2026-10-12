package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/codecrafters-io/claude-code-starter-go/internal/adapter/outbound/local"
	"github.com/codecrafters-io/claude-code-starter-go/internal/adapter/outbound/openrouter"
	"github.com/codecrafters-io/claude-code-starter-go/internal/application"
	"github.com/codecrafters-io/claude-code-starter-go/internal/port/inbound"
)

func main() {
	prompt := parsePrompt()
	skills, err := local.DiscoverSkills(".claude/skills")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	var agent inbound.Agent = application.NewAgent(openrouter.NewFromEnv(), local.ToolRunner{}, local.SkillStore{}, skills)
	session, err := agent.Start(context.Background(), prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")
	answer, err := session.Run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(answer)
}

func parsePrompt() string {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()
	if prompt == "" {
		panic("Prompt must not be empty")
	}
	return prompt
}
