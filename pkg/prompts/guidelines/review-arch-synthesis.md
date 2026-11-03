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
4. Where the reviewers disagree, settle it against the worktree. The finding
   states only the outcome: it never names a reviewer, a disagreement, or a
   claim that turned out false.
5. Set each finding's cost from the final finding, not from the inputs, and
   list findings highest cost first.

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
      "cost": "high | medium | low",
      "impact": "one sentence: what drifts and what the next change to this area pays",
      "location": "path/to/file:line",
      "diverges_from": "path/to/reference/file:line",
      "description": "the evidence: the sites involved and how they differ",
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
- `impact` is the headline. It is one sentence that a reader with no file
  open understands: what drifts and what the next change to this area pays
  for it. No paths, no function names, no reasoning. It is required; a
  finding without one is not reported.
- `location` is `path/to/file:line`.
- `diverges_from` is required and is a single `path/to/file:line`: the
  reference the finding is judged against, in the worktree. No prose and no
  backticks. A finding you cannot anchor this way is not reported.
- `description` is the evidence for `impact`, kept to a few sentences. Other
  sites a duplication spans go here, with their paths. It states the problem
  only: never what cannot happen, and never how the finding was reached.
- One finding per entry. A second problem is its own entry or is left out,
  and `suggestion` fixes only the finding it belongs to.
- `findings` is an empty array when there is nothing to report.
- Keep `summary` and each `positives` entry to one short line.
