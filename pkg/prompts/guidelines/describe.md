# PR description flow

Draft the title and description for this branch.

## Read first

1. The `<project_guide>` block in the diff context file. Its PR descriptions
   section is the standard for what a description says and how it says it,
   including the length budget. Apply it as written.
2. The `<project_context>` block in the same file, plus any domain context
   files it points to that are relevant to the diff. Use them to name the
   domains and boundaries the diff touches in the project's own terms; file
   paths alone are not enough.
3. The branch diff in the same file.

## Required output

Markdown, with the PR title as the first H1:

```markdown
# Clear, specific PR title

## Summary
At most three sentences: what was wrong, what changes, what it costs or
unlocks. No list of what the screen or the code now contains.

## Footprint
- Containment: inside one domain, or which domains and through what public
  surface. Whether a public surface changed, and what callers see differently.
- Boundaries crossed, one line. Boundaries left alone that a reviewer would
  ask about, one line.
- Gating and revert, one line: flags, rollout, what undoing it takes.

## Review focus
- At most three decisions a reviewer has to judge rather than check. At most
  two sentences each, ending with a `path/to/file` pointer. Drop the section
  when the diff holds none.

## Screenshots / Video
- Only for visible UI changes. When assets are not available, write "Not
  included in this draft; attach before review."

## Testing
- Automated: the behaviors now covered, in one or two lines, and whether the
  tests ran.
- Manual: steps and outcomes, or "Not run" with the reason.
```

`Summary`, `Footprint`, and `Testing` are always present. `Review focus` and
`Screenshots / Video` appear only when the diff gives them content. Do not add
sections beyond these. A fact appears in one section only; when it is a
decision it lives in `Review focus` and is not restated elsewhere. The only
code in the body is a file path pointer; module and function names stay in
the diff. Everything before `Testing` fits in 250 words.
