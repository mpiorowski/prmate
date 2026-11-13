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
  "issues": [
    {
      "severity": "blocking | comment",
      "type": "security | correctness | data_integrity | performance | breaking_change | testing | docs",
      "impact": "one sentence: what is wrong and what it does to the app",
      "location": "path/to/file:line",
      "description": "the evidence: the code path that produces the impact",
      "suggestion": "concrete fix or mitigation"
    }
  ]
}
```

## Field rules

- `severity` and `type` accept only the values listed above. The project
  guide decides which value a given issue warrants. There is no verdict
  field: the verdict is derived from the issues after parsing.
- `impact` is the headline. It is one sentence that a reader with no file
  open understands: what is wrong and what it does to the app. No paths, no
  function names, no reasoning. It is required; an issue without one is not
  reported.
- `location` is `path/to/file:line`.
- `description` is the evidence for `impact`, at most three sentences. The
  paths and names go here. It states the problem only: never what cannot
  happen, and never how the finding was reached.
- One issue per entry. A second problem is its own entry or is left out, and
  `suggestion` fixes only the issue it belongs to.
- `issues` is an empty array when there is nothing to report.
- Keep `summary` to one short line.
