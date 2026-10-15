# PR description flow

Draft the title and description for this branch.

## Read first

1. The `<project_guide>` block in the diff context file. Its PR descriptions
   section is the standard for what a description says and how it says it.
   Apply it as written.
2. The `<project_context>` block in the same file, plus any domain context
   files it points to that are relevant to the diff. Use them to name the
   domains and boundaries the diff touches in the project's own terms; file
   paths alone are not enough.
3. The branch diff in the same file.

## Required output

Markdown, with the PR title as the first H1:

```markdown
# Clear, specific PR title

One short paragraph, no heading: what a user or operator experiences before
and after this change, and why it is being made now.

## Footprint
- First line: whether the diff stays inside one domain. When it crosses,
  name each domain as the project context names it and the public surface
  the crossing goes through.
- One line per domain touched: what changed there, and whether its public
  surface changed.
- One line for the boundaries the diff crosses and one for those it leaves
  alone. The project guide lists the boundaries.

## Review focus
- The decisions in the diff a reviewer has to judge rather than check, each
  with a `path/to/file` pointer. Drop the section when the diff holds none.

## Blast radius
- Who is affected, how it is gated or rolled out, what downstream consumers
  change, and how it is reverted. A single line is fine when the answer is
  "the touched screen only".

## Screenshots / Video
- Only for visible UI changes. When assets are not available, write "Not
  included in this draft; attach before review."

## Testing
- Automated: tests added or updated, or "Not run" with the reason.
- Manual: verification steps and observed outcomes, or "Not run" with the
  reason.
```

The lead paragraph, `Footprint`, `Blast radius`, and `Testing` are always
present. `Review focus` and `Screenshots / Video` appear only when the diff
gives them content. Do not add sections beyond these.
