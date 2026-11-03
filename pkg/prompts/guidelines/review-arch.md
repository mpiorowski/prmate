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
