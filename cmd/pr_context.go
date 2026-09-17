package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pr/pkg/git"
)

const (
	projectContextFilename = "CONTEXT.md"
	projectGuideFilename   = "GUIDE.md"
)

type projectContext struct {
	source  string
	content string
	found   bool
}

// projectGuide is the canonical engineering guide for the repository `pr` was
// invoked from. It is required: every prompt is written against it, so a run
// without it would silently review or describe against no standard at all.
type projectGuide struct {
	source  string
	content string
}

// requireProjectGuide reads GUIDE.md from the root of the repository the
// command was invoked in, following the symlink projects keep there. It never
// reads the guide from a PR worktree: the symlink is gitignored, so a worktree
// copy would be absent, and a checked-in one would let a PR rewrite the
// standard it is reviewed against.
func requireProjectGuide() (projectGuide, error) {
	root, err := git.GetRepositoryRoot()
	if err != nil {
		return projectGuide{}, fmt.Errorf("failed to get repo root: %w", err)
	}

	path := filepath.Join(root, projectGuideFilename)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return projectGuide{}, fmt.Errorf("no %s at repository root %s; symlink the canonical guide there before running prmate", projectGuideFilename, root)
		}
		return projectGuide{}, fmt.Errorf("read %s: %w", path, err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return projectGuide{}, fmt.Errorf("%s is empty", path)
	}

	return projectGuide{source: projectGuideFilename, content: string(content)}, nil
}

func renderProjectGuideSection(guide projectGuide) string {
	var sb strings.Builder
	sb.WriteString("## Project Guide\n\n")
	sb.WriteString("Source: `")
	sb.WriteString(guide.source)
	sb.WriteString("`.\n\n")
	sb.WriteString("This guide is the standard for this task. Read it before anything else and apply it as written.\n\n")
	sb.WriteString("<project_guide>\n")
	sb.WriteString(strings.TrimSpace(guide.content))
	sb.WriteString("\n</project_guide>\n\n")
	return sb.String()
}

func writeBranchDiffContextFile(dir string, guide projectGuide, baseRef string, diff string) (string, error) {
	filename := filepath.Join(dir, ".pr-diff-context.md")
	ctx, err := loadWorktreeProjectContext(dir)
	if err != nil {
		return "", err
	}

	content := buildBranchDiffContextContent(guide, ctx, baseRef, diff)
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write branch diff context file: %w", err)
	}

	return filepath.Base(filename), nil
}

func writePullRequestContextFile(dir string, guide projectGuide, pr git.PullRequest, diff string) (string, error) {
	filename := filepath.Join(dir, ".pr-context.md")
	content, err := buildPullRequestContextContent(dir, guide, pr, diff)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write PR context file: %w", err)
	}

	return filepath.Base(filename), nil
}

func buildBranchDiffContextContent(guide projectGuide, ctx projectContext, baseRef string, diff string) string {
	var sb strings.Builder
	sb.WriteString("# Branch Diff Context\n\n")
	sb.WriteString(renderProjectGuideSection(guide))
	sb.WriteString(renderProjectContextSection(ctx))
	sb.WriteString(fmt.Sprintf("## Branch Diff (against %s)\n\n", baseRef))
	sb.WriteString("```diff\n")
	sb.WriteString(diff)
	sb.WriteString("\n```\n")
	return sb.String()
}

func buildPullRequestContextContent(dir string, guide projectGuide, pr git.PullRequest, diff string) (string, error) {
	ctx, err := loadPullRequestProjectContext(dir, pr.BaseRefName)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# Pull Request Context\n\n")
	sb.WriteString("- Number: ")
	sb.WriteString(fmt.Sprintf("%d\n", pr.Number))
	sb.WriteString("- URL: ")
	sb.WriteString(pr.URL)
	sb.WriteString("\n")
	sb.WriteString("- State: ")
	sb.WriteString(pr.State)
	sb.WriteString("\n")
	sb.WriteString("- Base branch: ")
	sb.WriteString(pr.BaseRefName)
	sb.WriteString("\n")
	sb.WriteString("- Head branch: ")
	sb.WriteString(pr.HeadRefName)
	sb.WriteString("\n")
	sb.WriteString("- Merged at: ")
	sb.WriteString(pr.MergedAt)
	sb.WriteString("\n\n")
	sb.WriteString(renderProjectGuideSection(guide))
	sb.WriteString(renderProjectContextSection(ctx))
	sb.WriteString("## Title\n\n")
	sb.WriteString(pr.Title)
	sb.WriteString("\n\n")
	sb.WriteString("## Body\n\n")
	sb.WriteString(strings.TrimSpace(pr.Body))
	sb.WriteString("\n")

	sb.WriteString("\n## Diff\n\n")
	sb.WriteString("```diff\n")
	sb.WriteString(diff)
	sb.WriteString("\n```\n")

	return sb.String(), nil
}

func renderProjectContextSection(ctx projectContext) string {
	var sb strings.Builder
	sb.WriteString("## Project Context\n\n")
	if !ctx.found {
		sb.WriteString("No root `CONTEXT.md` was found for this worktree.\n\n")
		return sb.String()
	}

	sb.WriteString("Source: `")
	sb.WriteString(ctx.source)
	sb.WriteString("`.\n\n")
	sb.WriteString("If this root context points to domain-specific context files relevant to the diff, read those referenced files before producing the final answer.\n\n")
	sb.WriteString("<project_context>\n")
	sb.WriteString(strings.TrimSpace(ctx.content))
	sb.WriteString("\n</project_context>\n\n")
	return sb.String()
}

func loadPullRequestProjectContext(dir string, baseRef string) (projectContext, error) {
	baseRef = strings.TrimSpace(baseRef)
	for _, ref := range []string{"origin/" + baseRef, baseRef} {
		if strings.TrimSpace(ref) == "" || strings.TrimSpace(ref) == "origin/" {
			continue
		}
		content, ok, err := git.ReadFileAtRef(dir, ref, projectContextFilename)
		if err != nil {
			return projectContext{}, err
		}
		if ok && strings.TrimSpace(content) != "" {
			return projectContext{
				source:  fmt.Sprintf("%s from PR base ref %s", projectContextFilename, ref),
				content: content,
				found:   true,
			}, nil
		}
	}

	return loadWorktreeProjectContext(dir)
}

func loadWorktreeProjectContext(dir string) (projectContext, error) {
	path := filepath.Join(dir, projectContextFilename)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return projectContext{}, nil
		}
		return projectContext{}, fmt.Errorf("read %s: %w", projectContextFilename, err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return projectContext{}, nil
	}

	return projectContext{
		source:  projectContextFilename,
		content: string(content),
		found:   true,
	}, nil
}
