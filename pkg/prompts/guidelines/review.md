# Review guidelines

Review the pull request diff against its base. Prioritize issues that could break production behavior, expose data, corrupt state, or surprise downstream callers.

## Scope

- Use the PR context file first. It contains the PR title, description, and diff.
- Review files changed by the PR. Read directly referenced code only when needed to confirm behavior.
- Do not browse unrelated parts of the repository.
- Avoid broad shell exploration such as `rg`, `grep`, `find`, or ad hoc repository searches.

## Priorities

1. Security: auth bypasses, permission mistakes, data leaks, unsafe input handling, exposed secrets.
2. Correctness: logic bugs, broken control flow, missing error handling, race conditions, regressions.
3. Data integrity: wrong scoping, unsafe writes, broken migrations, missing invariants.
4. Breaking changes: APIs, schemas, contracts, or behavior that callers rely on.
5. Performance: only when the impact is concrete and material.
6. Tests: missing coverage when the change is risky or fixes a regression.

## Ignore

- Formatting and style issues that normal linters or formatters should catch.
- Low-value nits without user, operational, or maintenance impact.
- Speculative concerns not supported by the diff.

## Required output

Return JSON only. Do not wrap it in prose unless you must place it in a fenced `json` block.

```json
{
  "summary": "short review summary",
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

- Prefer a few high-confidence findings over many weak ones.
- If there are no material issues, set `verdict` to `APPROVE` or `COMMENT` and leave `issues` empty.
- Every issue must identify impact, affected behavior, and a concrete fix direction.
- Use `NEEDS_CHANGES` for blocking correctness, security, data integrity, or breaking-change issues.
- Keep the summary concise.
