# prmate

prmate is a small personal CLI for drafting pull request descriptions and running AI-assisted pull request reviews.

It installs a short `pr` command with a smart default plus two explicit workflows:

- `pr` inspects the current branch. If it has no open GitHub PR, it runs `pr describe` with `codex`; if it already has an open PR, it runs `pr review` with `codex,claude`.
- `pr describe` drafts a PR title/body from a branch diff, then creates or updates a GitHub PR.
- `pr review` reviews an existing GitHub PR. By default it runs two LLM reviewers, has them cross-check each other, and merges the validated findings into one final review.

## Requirements

- `git`
- GitHub CLI: `gh`
- Go, if building from source
- At least one supported LLM CLI available on `PATH`: `claude`, `codex`, `gemini`, or `opencode`
- A `GUIDE.md` at the root of every repository you run `pr` in

`setup.sh` can install Go and `gh` into your user directory if they are missing.

## GUIDE.md

`pr` refuses to run without a `GUIDE.md` at the root of the repository you invoke it from:

```
✗ no GUIDE.md at repository root /path/to/repo; symlink the canonical guide there before running pr
```

That file is the standard every run works to. The embedded prompts in `pkg/prompts/guidelines/` carry structure and flow only: what to read, what is in scope, the rounds, and the output contract. Everything about what makes a finding worth reporting, what to ignore, what blocks a merge, and how a description reads comes from `GUIDE.md`. Change the guide and every review changes with it, with no rebuild.

Keep one canonical copy and symlink it into each project:

```sh
ln -s ../dotfiles/GUIDE.md GUIDE.md
echo GUIDE.md >> .gitignore
```

It is read from the repository you invoke `pr` in, never from the PR worktree, so a pull request cannot rewrite the standard it is reviewed against.

## Install

```sh
./setup.sh
```

The script builds the CLI and installs it to `~/.local/bin/pr` by default.

To install somewhere else:

```sh
INSTALL_DIR=/usr/local/bin ./setup.sh
```

Make sure GitHub CLI is authenticated:

```sh
gh auth login
```

## Usage

Let `pr` pick the workflow for the current branch:

```sh
pr
```

This also works after checking out a contributor PR with `gh pr checkout <number>`; the CLI asks GitHub CLI for the PR associated with the checked-out branch.

Draft a PR description for the current branch:

```sh
pr describe
```

Draft a PR description for a specific branch:

```sh
pr describe --br my-branch
```

Update an existing PR instead of creating a new one:

```sh
pr describe --pr 123
```

Review a PR with the default paired flow:

```sh
pr review --pr 123
```

The default review uses:

```sh
--llm codex,claude
```

Use one reviewer for a simpler review:

```sh
pr review --pr 123 --llm claude
```

Use a different pair:

```sh
pr review --pr 123 --llm gemini,codex
```

### Picking models

Each reviewer is a provider, optionally with a model: `<provider>` or `<provider>:<model>`. The model is passed straight through to that provider CLI's `--model` flag, so any value the CLI accepts works (aliases or full model ids). Without a model, the provider CLI picks its own default.

Run the same provider twice under two different models:

```sh
pr review --pr 123 --llm claude:opus,claude:sonnet
pr review --pr 123 --llm claude:claude-opus-5,claude:claude-sonnet-5
```

The two reviewers stay labelled by their full spec (`claude:opus`, `claude:sonnet`) in progress output and in the cross-check and synthesis prompts. `--llm` on `pr describe` takes the same spec:

```sh
pr describe --llm claude:opus
```

Thinking/reasoning effort defaults to `high` where the selected provider supports it:

```sh
pr --think high
pr describe --think high
pr review --pr 123 --think xhigh
```

Provider mapping:

- `codex`: passes `--model` only when the spec names one, otherwise Codex uses its default model; passes `model_reasoning_effort` (`none`, `minimal`, `low`, `medium`, `high`, `xhigh`).
- `claude`: passes `--model` only when the spec names one, otherwise Claude Code uses its default model; passes `--effort` (`low`, `medium`, `high`, `xhigh`, `max`).
- `opencode`: passes `--variant` with the provided value.
- `gemini`: the implicit `high` default is ignored; explicitly using `--think` with Gemini returns an error.

## Review Flow

With two LLMs, `pr review` runs three rounds:

1. Both LLMs independently review the PR.
2. Each LLM cross-checks the other review.
3. The first LLM synthesizes one final JSON review report, which the CLI prints as Markdown.

With one LLM, `pr review` runs a single standard review.

Every round is judged against your `GUIDE.md`, which is embedded in the context file the reviewers read. The prompts decide the shape of the work; the guide decides the verdict.

## Configuration

Optional config lives at:

```sh
~/.config/pr/config.json
```

Supported config:

```json
{
  "github_reviewers": "github-handle-1,github-handle-2"
}
```

`github_reviewers` is used by `pr describe` when creating a new PR.

## Privacy

prmate writes PR context files into temporary or current worktrees and passes root `CONTEXT.md`, PR title, description, and diff context to the selected LLM CLI. Do not use it on code you are not allowed to send to those providers.

## Development

Run tests:

```sh
go test ./...
```

Run from source:

```sh
go run . --help
```
