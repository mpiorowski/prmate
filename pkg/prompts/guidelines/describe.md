# PR description guidelines

Write for a reviewer who needs to understand the change quickly and decide whether it is safe to merge.

## Output format

The final answer must be Markdown with the PR title as the first H1:

```markdown
# Clear, specific PR title

## Summary
- What changed and why it matters.

## Changes
- Reviewer-facing behavior changes.

## Behavior notes
- Important caveats, migrations, rollout details, fallbacks, or edge cases.

## Feature flags
- Required flags, configuration, or rollout gates. Use "None" when there are none.

## Screenshots / Video
- Required for visible UI changes. If assets are not available, say "Not included in this draft; attach before review."

## Testing
- Automated: tests added or updated, or "Not run" with the reason.
- Manual: manual verification steps and observed outcomes, or "Not run" with the reason.
```

## Writing rules

- Start from user-visible or operational impact, then cover implementation details only when they help review.
- Keep the title short, concrete, and written in normal words. Do not use branch names, ticket IDs alone, or vague titles.
- Prefer a short paragraph or a few high-signal bullets over exhaustive change logs.
- Do not invent screenshots, videos, feature flags, test results, links, metrics, or rollout details.
- Keep `Behavior notes` only when there is something material to call out.
- If the diff is small, keep the body small; do not pad it with generic sections beyond the required ones.
- Regression fixes should explain the broken behavior and mention regression coverage when present.
