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

`setup.sh` can install Go and `gh` into your user directory if they are missing.

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
--llm claude,codex
```

Use one reviewer for a simpler review:

```sh
pr review --pr 123 --llm claude
```

Use a different pair:

```sh
pr review --pr 123 --llm gemini,codex
```

## Review Flow

With two LLMs, `pr review` runs three rounds:

1. Both LLMs independently review the PR.
2. Each LLM cross-checks the other review.
3. The first LLM synthesizes one final JSON review report, which the CLI prints as Markdown.

With one LLM, `pr review` runs a single standard review.

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
