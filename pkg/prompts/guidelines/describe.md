# PR description flow

Draft the title and description for this branch.

## Read first

1. The `<project_guide>` block in the diff context file. It is the standard for
   what a description says and how it says it. Apply it as written.
2. The `<project_context>` block in the same file, plus any domain context
   files it points to that are relevant to the diff.
3. The branch diff in the same file.

## Required output

Markdown, with the PR title as the first H1:

```markdown
# Clear, specific PR title

## Summary
- What changed and why it matters.

## Changes
- Reviewer-facing behavior changes.

## Behavior notes
- Caveats, migrations, rollout details, fallbacks, or edge cases. Drop this
  section when there is nothing material to call out.

## Feature flags
- Required flags, configuration, or rollout gates. Use "None" when there are
  none.

## Screenshots / Video
- Required for visible UI changes. When assets are not available, write "Not
  included in this draft; attach before review."

## Testing
- Automated: tests added or updated, or "Not run" with the reason.
- Manual: verification steps and observed outcomes, or "Not run" with the
  reason.
```

Every section above is required except `Behavior notes`. Do not add sections
beyond these.
