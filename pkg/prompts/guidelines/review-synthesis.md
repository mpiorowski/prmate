# Review synthesis flow

Produce the final review from two independent reviews and two cross-checks.

## Read first

1. The `<project_guide>` block in the PR context file. It decides which
   findings belong in the final report and which of them block.
2. The `<project_context>` block, the PR title, the description, and the diff
   in the same file.

## Steps

1. Keep the findings the changed code supports and the guide qualifies.
2. Drop the rest, including duplicates that add nothing.
3. Merge overlapping findings into one issue.
4. Where the reviewers disagree, settle it against the changed code. The
   issue states only the outcome: it never names a reviewer, a disagreement,
   or a claim that turned out false.
5. Set each issue's severity from the final issue, not from the inputs.

The result is one self-contained report. Every issue must be actionable without
reading the intermediate reviews, and nothing may reference them.

## Required output

Return JSON only. Do not wrap it in prose unless you must place it in a fenced
`json` block.

```json
{
  "summary": "short review summary noting this was cross-checked by two reviewers",
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
