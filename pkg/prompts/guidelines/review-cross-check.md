# Review cross-check guidelines

You are validating another review after completing your own independent review of the same pull request.

## Task

1. Validate the peer review's findings against the code and PR context.
2. Identify false positives, overstated severity, incomplete suggestions, or duplicated findings.
3. Compare the peer review with your review and call out real issues either reviewer missed.
4. Reassess the verdict based on the issues that remain after validation.

## Scope

- Use the PR context file and changed files as the source of truth.
- Read directly referenced code only when necessary to resolve a concrete question.
- Do not browse unrelated parts of the repository.
- Avoid broad shell exploration such as `rg`, `grep`, `find`, or ad hoc repository searches.

## Output format

Return Markdown with these sections:

```markdown
## Valid Findings
- Findings from either review that are real and materially important.

## False Positives or Changes
- Findings to drop, lower, raise, merge, or rewrite, with a brief reason.

## Missed Issues
- Real issues neither review captured clearly enough. Include severity, location, impact, and fix direction.

## Verdict
- The verdict you would choose after cross-checking, with a short reason.
```

## Rules

- Do not repeat either review verbatim.
- Keep the bar high; do not add issues just to be different.
- Be specific about why a finding should stay, change, or be removed.
- If both reviews are solid and complete, say so briefly.
