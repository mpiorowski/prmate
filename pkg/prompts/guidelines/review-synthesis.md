# Review synthesis flow

Produce the final review from two independent reviews and two cross-checks.

## Read first

1. The `<project_guide>` block in the PR context file. It decides which
   findings belong in the final report and what the verdict must be.
2. The `<project_context>` block, the PR title, the description, and the diff
   in the same file.

## Steps

1. Keep the findings the changed code supports and the guide qualifies.
2. Drop the rest, including duplicates that add nothing.
3. Merge overlapping findings into one issue.
4. Where the reviewers disagree, settle it against the changed code and say
   which way you went in the issue description.
5. Set severity and verdict from the final issue list, not from the inputs.

The result is one self-contained report. Every issue must be actionable without
reading the intermediate reviews, and nothing may reference them.

## Required output

Return JSON only. Do not wrap it in prose unless you must place it in a fenced
`json` block.

```json
{
  "summary": "short review summary noting this was cross-checked by two reviewers",
  "verdict": "NEEDS_CHANGES | APPROVE | COMMENT",
  "issues": [
    {
      "severity": "critical | high | medium | low",
      "type": "security | correctness | data_integrity | performance | breaking_change | testing | maintainability",
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
