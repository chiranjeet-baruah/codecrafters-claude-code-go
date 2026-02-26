# Claude Code Clone (Go)

[![progress-banner](https://backend.codecrafters.io/progress/claude-code/c8efd480-6450-4ded-abaa-a0767e048eb2)](https://app.codecrafters.io/users/codecrafters-bot?r=2qF)

A minimal AI coding assistant built in Go, inspired by [Claude Code](https://docs.anthropic.com/en/docs/claude-code). This project is a solution to the [CodeCrafters "Build Your Own Claude Code" Challenge](https://codecrafters.io/challenges/claude-code).

The assistant connects to an LLM via the OpenRouter API, sends user prompts, and autonomously executes tool calls (reading files, writing files, running shell commands) in an agentic loop until a final text response is produced.

## Architecture

```
User prompt ──► main.go (agent loop)
                  │
                  ├──► client.go   – OpenRouter API client & chat completion
                  ├──► messages.go – Message type helpers (user/assistant/tool)
                  └──► tools.go    – Tool definitions & handlers (Read, Write, Bash)
```

### How the Agent Loop Works

1. The user supplies a prompt via the `-p` flag.
2. The prompt is sent as a user message to the **Anthropic Claude Haiku 4.5** model through the OpenRouter API (OpenAI-compatible).
3. If the model responds with **tool calls**, each call is executed locally and the results are appended as tool messages.
4. Steps 2–3 repeat until the model returns a plain text response (no tool calls), which is printed to stdout.

## Tools

The assistant exposes three tools to the LLM:

| Tool | Description | Parameters |
|------|-------------|------------|
| **Read** | Reads and returns the contents of a file | `file_path` (string) |
| **Write** | Writes content to a file (creates or overwrites) | `file_path` (string), `content` (string) |
| **Bash** | Executes an arbitrary shell command and returns combined stdout/stderr | `command` (string) |

## Project Structure

| File | Purpose |
|------|---------|
| `app/main.go` | Entry point — parses the prompt, runs the agent loop |
| `app/client.go` | Configures the OpenRouter client, exposes `parsePrompt()`, `newClient()`, and `complete()` |
| `app/messages.go` | Helpers to build OpenAI-compatible user, assistant, and tool messages |
| `app/tools.go` | Tool definitions (JSON schemas) and handler implementations |
| `your_program.sh` | Convenience script to compile and run locally |
| `codecrafters.yml` | CodeCrafters build configuration (Go 1.25) |

## Prerequisites

- **Go 1.25+**
- An **OpenRouter API key** (set as `OPENROUTER_API_KEY` environment variable)

## Getting Started

### 1. Set your API key

```sh
export OPENROUTER_API_KEY="your-key-here"
```

Optionally override the base URL (defaults to `https://openrouter.ai/api/v1`):

```sh
export OPENROUTER_BASE_URL="https://custom-endpoint/v1"
```

### 2. Run locally

```sh
./your_program.sh -p "Explain what this project does"
```

This compiles all files under `app/` and executes the resulting binary.

### 3. Test on CodeCrafters testers

```sh
codecrafters test
```

### 4. Submit to CodeCrafters

```sh
codecrafters submit
```

## Dependencies

| Package | Purpose |
|---------|---------|
| [`github.com/openai/openai-go/v3`](https://github.com/openai/openai-go) | Official OpenAI Go SDK (used with OpenRouter's OpenAI-compatible API) |
| `github.com/tidwall/gjson` / `sjson` | Transitive JSON handling dependencies |

## License

This project was scaffolded by [CodeCrafters](https://codecrafters.io).
