# Architecture cross-check flow

You are validating another architecture review after completing your own
independent architecture review of the same pull request.

## Read first

1. The `<project_guide>` block in the PR context file. Both reviews are judged
   against its architecture review section and the guide sections it names.
2. The `<project_context>` block, the PR title, the description, and the diff
   in the same file.

## Steps

1. For each peer finding, open the path or sites named in `diverges_from` and
   confirm the divergence is real in the worktree.
2. Name the findings to drop, re-cost, merge, or rewrite.
3. Compare both reviews and call out anything real that neither caught.

## Scope

- The PR context file, the changed files, and the references the findings
  name are the source of truth.
- Read the touched domain and one sibling domain when a finding depends on
  how the rest of the codebase does something.
- Do not tour the repository beyond that.

## Required output

Return Markdown with these sections and nothing else:

```markdown
## Valid Findings
- Findings from either review that survive, each with its confirmed reference.

## False Positives or Changes
- Findings to drop, re-cost, merge, or rewrite, with a brief reason.

## Missed Findings
- Real divergence neither review captured. Include cost, location, reference,
  and fix direction.
```

Do not restate either review verbatim. If both reviews are solid and complete,
say so in one line and leave the other sections empty.
