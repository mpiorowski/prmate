# Architecture synthesis flow

Produce the final architecture review from two independent reviews and two
cross-checks.

## Read first

1. The `<project_guide>` block in the PR context file. Its architecture review
   section decides which findings belong in the final report.
2. The `<project_context>` block, the PR title, the description, and the diff
   in the same file.

## Steps

1. Keep the findings whose reference the cross-checks confirmed in the code.
2. Drop the rest, including duplicates that add nothing.
3. Merge overlapping findings into one.
4. Where the reviewers disagree, settle it against the worktree and say which
   way you went in the finding description.

The result is one self-contained report. Every finding must be actionable
without reading the intermediate reviews, and nothing may reference them.

## Required output

Return JSON only. Do not wrap it in prose unless you must place it in a fenced
`json` block.

```json
{
  "summary": "short review summary noting this was cross-checked by two reviewers",
  "findings": [
    {
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

- There is no verdict and no severity. Every finding is a comment.
- `location` is `path/to/file:line`.
- `diverges_from` is required. It names a real path in the worktree, or lists
  real sites. A finding you cannot anchor this way is not reported.
- `findings` is an empty array when there is nothing to report.
- Keep `summary` and each `positives` entry to one short line.
