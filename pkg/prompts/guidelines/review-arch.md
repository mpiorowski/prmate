# Architecture review flow

Review how the pull request diff, and the domains it lands in, fit the
codebase, and return one JSON report. This pass does not look for bugs; a separate review covers those.

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

- Review the files this PR changes and the rest of every domain they live
  in. The whole touched domain is under review, not only the diff.
- Read one sibling domain as the reference for how this codebase does
  things. The sibling is a reference, not a target.
- A targeted search is allowed for two things: counting and locating the
  existing sites of a copy the diff adds, and confirming that a symbol in a
  touched domain has no caller. Do not tour the repository beyond that.

## Required output

Return JSON only. Do not wrap it in prose unless you must place it in a fenced
`json` block.

```json
{
  "summary": "short review summary",
  "findings": [
    {
      "cost": "high | medium",
      "impact": "one sentence: what drifts and what the next change to this area pays",
      "location": "path/to/file:line",
      "diverges_from": "path/to/reference/file:line",
      "description": "the evidence: the sites involved and how they differ",
      "suggestion": "concrete move, rename, split, or extraction"
    }
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
- `diverges_from` is a single `path/to/file:line`: the reference the finding
  is judged against, in the worktree. No prose and no backticks. It is empty
  only when the finding is code nothing calls, which has no reference; any
  other finding you cannot anchor this way is not reported.
- `description` is the evidence for `impact`, at most three sentences. Other
  sites a duplication spans go here, with their paths. It states the problem
  only: never what cannot happen, and never how the finding was reached.
- One finding per entry. A second problem is its own entry or is left out,
  and `suggestion` fixes only the finding it belongs to.
- `findings` is an empty array when there is nothing to report.
- Keep `summary` to one short line.
