# prmate

prmate is a small personal CLI for drafting pull request descriptions and running AI-assisted pull request reviews.

It installs a `prmate` command with two explicit workflows:

- `prmate describe` drafts a PR title/body from a branch diff, then creates or updates a GitHub PR.
- `prmate review` reviews an existing GitHub PR. By default it runs two LLM reviewers on two independent tracks, bugs and architecture, has them cross-check each other within each track, and merges the validated findings into one final report per track.

## Requirements

- `git`
- GitHub CLI: `gh`
- Go, if building from source
- At least one supported LLM CLI available on `PATH`: `claude`, `codex`, `gemini`, or `opencode`
- A `GUIDE.md` at the root of every repository you run `prmate` in

`setup.sh` can install Go and `gh` into your user directory if they are missing.

## GUIDE.md

`prmate` refuses to run without a `GUIDE.md` at the root of the repository you invoke it from:

```
✗ no GUIDE.md at repository root /path/to/repo; symlink the canonical guide there before running prmate
```

That file is the standard every run works to. The embedded prompts in `pkg/prompts/guidelines/` carry structure and flow only: what to read, what is in scope, the rounds, and the output contract. Everything about what makes a finding worth reporting, what to ignore, what blocks a merge, and how a description reads comes from `GUIDE.md`. Change the guide and every review changes with it, with no rebuild.

Keep one canonical copy and symlink it into each project:

```sh
ln -s ../dotfiles/GUIDE.md GUIDE.md
echo GUIDE.md >> .gitignore
```

It is read from the repository you invoke `prmate` in, never from the PR worktree, so a pull request cannot rewrite the standard it is reviewed against.

## Install

```sh
./setup.sh
```

The script builds the CLI and installs it to `~/.local/bin/prmate` by default.

To install somewhere else:

```sh
INSTALL_DIR=/usr/local/bin ./setup.sh
```

Make sure GitHub CLI is authenticated:

```sh
gh auth login
```

## Usage

Let `prmate` pick the workflow for the current branch:

```sh
prmate
```

This also works after checking out a contributor PR with `gh pr checkout <number>`; the CLI asks GitHub CLI for the PR associated with the checked-out branch.

Draft a PR description for the current branch:

```sh
prmate describe
```

Draft a PR description for a specific branch:

```sh
prmate describe --br my-branch
```

Update an existing PR instead of creating a new one:

```sh
prmate describe --pr 123
```

With `--pr`, the diff compares the selected local branch to the PR’s target branch fetched from GitHub, so stacked PR descriptions exclude parent changes. The command stops if that base cannot be fetched or resolved. Without `--pr`, the diff uses `origin/main`.

Review a PR with the default paired flow:

```sh
prmate review --pr 123
```

The default review uses:

```sh
--llm codex:gpt-6-astra,claude:claude-opus-5-5
```

Every default names its model, so a review runs the same models on every machine regardless of what the provider CLI or your account treats as default. The pinned models and the default thinking level live together in `cmd/flags.go`; bump them there when a new model ships.

Use one reviewer for a simpler review:

```sh
prmate review --pr 123 --llm claude
```

Use a different pair:

```sh
prmate review --pr 123 --llm gemini,codex
```

Skip the architecture track and run the bug review only:

```sh
prmate review --pr 123 --no-arch
```

### Picking models

Each reviewer is a provider, optionally with a model: `<provider>` or `<provider>:<model>`. The model is passed straight through to that provider CLI's `--model` flag, so any value the CLI accepts works (aliases or full model ids). Without a model, the provider CLI picks its own default.

Run the same provider twice under two different models:

```sh
prmate review --pr 123 --llm claude:opus,claude:sonnet
prmate review --pr 123 --llm claude:claude-opus-5,claude:claude-sonnet-5
```

The two reviewers stay labelled by their full spec (`claude:opus`, `claude:sonnet`) in progress output and in the cross-check and synthesis prompts. `--llm` on `prmate describe` takes the same spec:

```sh
prmate describe --llm claude:opus
```

Thinking/reasoning effort defaults to `high` where the selected provider supports it:

```sh
prmate describe --think high
prmate review --pr 123 --think xhigh
```

Provider mapping:

- `codex`: passes `--model` only when the spec names one, otherwise Codex uses its default model; passes `model_reasoning_effort` (`none`, `minimal`, `low`, `medium`, `high`, `xhigh`).
- `claude`: passes `--model` only when the spec names one, otherwise Claude Code uses its default model; passes `--effort` (`low`, `medium`, `high`, `xhigh`, `max`).
- `opencode`: passes `--variant` with the provided value.
- `gemini`: the implicit `high` default is ignored; explicitly using `--think` with Gemini returns an error.

## Review Flow

With two LLMs, `prmate review` runs two independent tracks through three rounds:

1. Both LLMs independently review the PR, once for bugs and once for architecture. All four runs go in parallel.
2. Each LLM cross-checks the other review within the same track. A bug reviewer never judges an architecture finding, and the other way round.
3. The first LLM synthesizes one final JSON report per track, which the CLI prints as Markdown, bugs first.

The bug report carries a verdict and can block the merge. The architecture report has no verdict and no severity: every finding is a comment naming what diverges and the sibling or sites it diverges from. `--no-arch` drops the second track.

With one LLM, `prmate review` runs a single bug review. The architecture track needs a second reviewer to cross-check, so it is skipped with a note.

Every round is judged against your `GUIDE.md`, which is embedded in the context file the reviewers read. The prompts decide the shape of the work; the guide decides what counts as a finding on each track and what blocks a merge.

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

`github_reviewers` is used by `prmate describe` when creating a new PR.

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
