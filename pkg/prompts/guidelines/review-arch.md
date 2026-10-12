# Architecture review flow

Review how the pull request diff fits the codebase it lands in, and return one
JSON report. This pass does not look for bugs; a separate review covers those.

## Read first

1. The `<project_guide>` block in the PR context file. Its architecture review
   section says what counts as a finding here and which sections of the guide
   are the standard. Apply it as written.
2. The `<project_context>` block in the same file, plus any domain context
   files it points to that are relevant to the diff.
3. The PR title, description, and diff in the same file.

The checked out worktree is the source of truth. When the title or description
disagrees with the code, the code wins.

## Scope

- Review the files this PR changes against the domain they live in.
- Read the rest of the touched domain, and one sibling domain, as the
  reference for how this codebase does things.
- When the diff adds another copy of something that already exists, a targeted
  search to count and locate the existing sites is allowed. Do not tour the
  repository beyond that.

## Required output

Return JSON only. Do not wrap it in prose unless you must place it in a fenced
`json` block.

```json
{
  "summary": "short review summary",
  "findings": [
    {
      "cost": "high | medium | low",
      "location": "path/to/file:line",
      "diverges_from": "path/to/reference/file, or the sites a duplication now spans",
      "description": "what drifts and what it costs over time",
      "suggestion": "concrete move, rename, split, or extraction"
    }
  ],
  "positives": [
    "brief thing that fits the codebase well"
  ]
}
```

## Field rules

- There is no verdict. Every finding is a comment.
- `cost` accepts only the values listed above. The project guide defines them.
- `location` is `path/to/file:line`.
- `diverges_from` is required. It names a real path in the worktree, or lists
  real sites. A finding you cannot anchor this way is not reported.
- `findings` is an empty array when there is nothing to report.
- Keep `summary` and each `positives` entry to one short line.
