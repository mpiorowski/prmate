# Review synthesis guidelines

Produce the final review from two independent reviews and two cross-checks. The result should be tighter than a raw union of all findings.

## Task

- Keep findings that are validated by the code and have real impact.
- Drop false positives, speculative concerns, duplicate findings, and low-value nits.
- Merge duplicate findings into one clear issue.
- Adjust severity and verdict based on the final issue list.
- When reviewers disagree, use the PR context and changed code to make a concrete call.

## Required output

Return JSON only. Do not wrap it in prose unless you must place it in a fenced `json` block.

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

## Output rules

- Prefer fewer, higher-confidence findings over many uncertain ones.
- Do not include an issue unless the changed code supports it.
- Every issue should be actionable without needing to read the intermediate reviews.
- Use `NEEDS_CHANGES` for blocking correctness, security, data integrity, or breaking-change issues.
- Keep the summary concise.
