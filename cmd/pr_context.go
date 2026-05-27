package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pr/pkg/git"
)

const projectContextFilename = "CONTEXT.md"

type projectContext struct {
	source  string
	content string
	found   bool
}

func writeBranchDiffContextFile(dir string, baseRef string, diff string) (string, error) {
	filename := filepath.Join(dir, ".pr-diff-context.md")
	ctx, err := loadWorktreeProjectContext(dir)
	if err != nil {
		return "", err
	}

	content := buildBranchDiffContextContent(ctx, baseRef, diff)
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write branch diff context file: %w", err)
	}

	return filepath.Base(filename), nil
}

func writePullRequestContextFileAs(dir string, name string, pr git.PullRequest, diff string) (string, error) {
	filename := filepath.Join(dir, name)
	content, err := buildPullRequestContextContent(dir, pr, diff, true)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write PR context file: %w", err)
	}

	return filepath.Base(filename), nil
}

func writePullRequestContextFile(dir string, pr git.PullRequest, diff string) (string, error) {
	filename := filepath.Join(dir, ".pr-context.md")
	content, err := buildPullRequestContextContent(dir, pr, diff, true)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write PR context file: %w", err)
	}

	return filepath.Base(filename), nil
}

func writePullRequestContextFileWithoutDiff(dir string, pr git.PullRequest) (string, error) {
	filename := filepath.Join(dir, ".pr-context.md")
	content, err := buildPullRequestContextContent(dir, pr, "", false)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write PR context file: %w", err)
	}

	return filepath.Base(filename), nil
}

func buildBranchDiffContextContent(ctx projectContext, baseRef string, diff string) string {
	var sb strings.Builder
	sb.WriteString("# Branch Diff Context\n\n")
	sb.WriteString(renderProjectContextSection(ctx))
	sb.WriteString(fmt.Sprintf("## Branch Diff (against %s)\n\n", baseRef))
	sb.WriteString("```diff\n")
	sb.WriteString(diff)
	sb.WriteString("\n```\n")
	return sb.String()
}

func buildPullRequestContextContent(dir string, pr git.PullRequest, diff string, includeDiff bool) (string, error) {
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
	sb.WriteString(renderProjectContextSection(ctx))
	sb.WriteString("## Title\n\n")
	sb.WriteString(pr.Title)
	sb.WriteString("\n\n")
	sb.WriteString("## Body\n\n")
	sb.WriteString(strings.TrimSpace(pr.Body))
	sb.WriteString("\n")

	if includeDiff {
		sb.WriteString("\n## Diff\n\n")
		sb.WriteString("```diff\n")
		sb.WriteString(diff)
		sb.WriteString("\n```\n")
	}

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
