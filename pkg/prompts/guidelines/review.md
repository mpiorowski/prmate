# Review flow

Review the pull request diff against its base and return one JSON report.

## Read first

1. The `<project_guide>` block in the PR context file. It is the standard this
   review is judged against: what counts as a finding, what to leave out, and
   what blocks a merge. Apply it as written.
2. The `<project_context>` block in the same file, plus any domain context
   files it points to that are relevant to the diff.
3. The PR title, description, and diff in the same file.

The checked out worktree is the source of truth. When the title or description
disagrees with the code, the code wins.

## Scope

- Review the files this PR changes.
- Read directly referenced code only when it is needed to confirm behavior.
- Do not browse unrelated parts of the repository.
- Avoid broad shell exploration such as `rg`, `grep`, `find`, or ad hoc
  repository searches.

## Required output

Return JSON only. Do not wrap it in prose unless you must place it in a fenced
`json` block.

```json
{
  "summary": "short review summary",
  "verdict": "NEEDS_CHANGES | APPROVE | COMMENT",
  "issues": [
    {
      "severity": "critical | high | medium | low",
      "type": "security | correctness | data_integrity | performance | breaking_change | testing",
      "location": "path/to/file:line",
      "description": "specific impact and why it matters",
      "suggestion": "concrete fix or mitigation"
    }
  ],
  "positives": [
    "brief thing that looks good"
  ]
}
```

## Field rules

- `verdict`, `severity`, and `type` accept only the values listed above. The
  project guide decides which value a given finding warrants.
- `location` is `path/to/file:line`.
- `issues` is an empty array when there is nothing to report.
- Keep `summary` and each `positives` entry to one short line.
