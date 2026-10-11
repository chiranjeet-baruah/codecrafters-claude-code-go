# Build Your Own Claude Code (Go)

[![CodeCrafters progress](https://backend.codecrafters.io/progress/claude-code/c8efd480-6450-4ded-abaa-a0767e048eb2)](https://codecrafters.io/challenges/claude-code)

A command-line coding agent built for the [CodeCrafters challenge](https://codecrafters.io/challenges/claude-code). It takes one prompt, calls a model through OpenRouter, runs requested tools, and prints the final response.

## About the project

- **Built with:** Go 1.25 and [`openai-go/v3`](https://github.com/openai/openai-go).
- **Model:** `anthropic/claude-haiku-4.5` (currently fixed in the code).
- **Tools:** Read files, write files, run Bash commands, and load local skills.
- **Agent loop:** Keeps calling the model while it requests tools; exits when the model returns a response without tool calls.

## Getting started

1. Install [Go 1.25](https://go.dev/dl/) and obtain an OpenRouter API key with access to the configured model.
2. From the repository root, set your key and run a prompt:

   ```sh
   export OPENROUTER_API_KEY='your-key'
   ./your_program.sh -p 'Summarize app/main.go'
   ```

`OPENROUTER_BASE_URL` is optional; it defaults to `https://openrouter.ai/api/v1`. The `-p` prompt is required. `your_program.sh` builds the Go program before running it.

## Usage

The agent exposes four tools to the model:

| Tool | Action |
| --- | --- |
| `Read` | Return a file's contents. |
| `Write` | Write content to a file. |
| `Bash` | Run a Bash command. |
| `Skill` | Load instructions from a local skill. |

### Local skills

At startup, the program looks for `.claude/skills/<name>/SKILL.md` relative to the **current working directory**. This repository does not include any skills. Each skill needs a matching `name` and a `description` in its frontmatter. For example, create `.claude/skills/review/SKILL.md`:

```md
---
name: review
description: Review code changes for bugs.
---

Review these changes: $ARGUMENTS
```

Run it with `./your_program.sh -p '/review app/main.go'`, or use an ordinary prompt and let the model choose a skill. Skills can use `$ARGUMENTS` for all supplied text and `$0`, `$1`, etc. for individual words. An optional `context: fork` frontmatter field runs the skill in a separate agent context and returns its answer to the main agent.

**Safety:** `Write` and `Bash` execute without a confirmation step or sandbox in this program. Run it only where you trust the prompt and can tolerate file changes or shell commands. File contents and tool results used by the agent are sent to the configured API.

## Development

```sh
go test ./...
```

The [CodeCrafters CLI](https://docs.codecrafters.io/challenges/program-interface) can submit this solution with `codecrafters submit`. CodeCrafters uses `.codecrafters/compile.sh` and `.codecrafters/run.sh`; `your_program.sh` is the local entry point.
