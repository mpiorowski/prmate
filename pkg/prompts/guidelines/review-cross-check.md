# Review cross-check flow

You are validating another review after completing your own independent review
of the same pull request.

## Read first

1. The `<project_guide>` block in the PR context file. Both reviews are judged
   against it: whether a finding qualifies, and whether it blocks a merge.
2. The `<project_context>` block, the PR title, the description, and the diff
   in the same file.

## Steps

1. Check the peer review's findings against the code and the PR context.
2. Name the findings to drop, lower, raise, merge, or rewrite.
3. Compare both reviews and call out anything real that neither caught.
4. Reassess the verdict against what survives.

## Scope

- The PR context file and the changed files are the source of truth.
- Read directly referenced code only when it is needed to settle a concrete
  question.
- Do not browse unrelated parts of the repository.
- Avoid broad shell exploration such as `rg`, `grep`, `find`, or ad hoc
  repository searches.

## Required output

Return Markdown with these sections and nothing else:

```markdown
## Valid Findings
- Findings from either review that survive.

## False Positives or Changes
- Findings to drop, lower, raise, merge, or rewrite, with a brief reason.

## Missed Issues
- Real issues neither review captured. Include severity, location, impact, and
  fix direction.

## Verdict
- The verdict you would choose after cross-checking, with a short reason.
```

Do not restate either review verbatim. If both reviews are solid and complete,
say so in one line and leave the other sections empty.
