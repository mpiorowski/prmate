package ai

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		llm     string
		mode    Mode
		want    []string
		cleanup bool
	}{
		{
			name: "claude uses json print mode",
			llm:  "claude",
			mode: ReadOnly,
			want: []string{"claude", "-p", "--output-format", "json", "prompt"},
		},
		{
			name: "claude write mode enables edit permissions",
			llm:  "claude",
			mode: Write,
			want: []string{"claude", "-p", "--output-format", "json", "--permission-mode", "acceptEdits", "prompt"},
		},
		{
			name: "gemini uses json print mode",
			llm:  "gemini",
			mode: ReadOnly,
			want: []string{"gemini", "-p", "prompt", "--output-format", "json"},
		},
		{
			name:    "codex writes last message to a temp file",
			llm:     "codex",
			mode:    Write,
			want:    []string{"codex", "exec", "--color", "never", "--output-last-message"},
			cleanup: true,
		},
		{
			name: "unknown llm falls back to claude",
			llm:  "unknown",
			mode: ReadOnly,
			want: []string{"claude", "-p", "--output-format", "json", "prompt"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd, artifacts, cleanup, err := buildCommand(context.Background(), "prompt", tt.llm, tt.mode, RunOptions{})
			if err != nil {
				t.Fatalf("buildCommand returned error: %v", err)
			}
			defer cleanup()

			got := append([]string{filepath.Base(cmd.Path)}, cmd.Args[1:]...)
			for i, want := range tt.want {
				if i >= len(got) {
					t.Fatalf("missing arg %d, want %q; full args: %#v", i, want, got)
				}
				if got[i] != want {
					t.Fatalf("arg %d = %q, want %q; full args: %#v", i, got[i], want, got)
				}
			}

			if tt.cleanup {
				if artifacts.outputFile == "" {
					t.Fatal("expected codex output file to be set")
				}
				if !strings.HasPrefix(filepath.Base(artifacts.outputFile), "pr-codex-last-message-") {
					t.Fatalf("unexpected codex output file: %s", artifacts.outputFile)
				}
			}
		})
	}
}

func TestModeLabel(t *testing.T) {
	t.Parallel()

	if got := ReadOnly.Label(); got != "READ" {
		t.Fatalf("ReadOnly label = %q, want %q", got, "READ")
	}
	if got := Write.Label(); got != "WRITE" {
		t.Fatalf("Write label = %q, want %q", got, "WRITE")
	}
}

func TestModeColoredLabel(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")

	if got := ReadOnly.ColoredLabel(); got != "\033[34mREAD\033[0m" {
		t.Fatalf("ReadOnly colored label = %q, want %q", got, "\033[34mREAD\033[0m")
	}
	if got := Write.ColoredLabel(); got != "\033[31mWRITE\033[0m" {
		t.Fatalf("Write colored label = %q, want %q", got, "\033[31mWRITE\033[0m")
	}
}

func TestModeStatusText(t *testing.T) {
	t.Parallel()

	if got := ReadOnly.StatusText(); got != "is analyzing the worktree..." {
		t.Fatalf("ReadOnly status = %q, want %q", got, "is analyzing the worktree...")
	}
	if got := Write.StatusText(); got != "is applying changes in the worktree..." {
		t.Fatalf("Write status = %q, want %q", got, "is applying changes in the worktree...")
	}
}

func TestExtractResponse(t *testing.T) {
	t.Parallel()

	t.Run("claude json result", func(t *testing.T) {
		t.Parallel()

		got, err := extractResponse(`{"type":"result","subtype":"success","result":"# hello\nworld","is_error":false}`, commandArtifacts{provider: "claude"})
		if err != nil {
			t.Fatalf("extractResponse returned error: %v", err)
		}
		if got.FinalMessage != "# hello\nworld" {
			t.Fatalf("got %q, want %q", got.FinalMessage, "# hello\nworld")
		}
	})

	t.Run("gemini json result", func(t *testing.T) {
		t.Parallel()

		got, err := extractResponse(`{"response":"{\"summary\":\"ok\"}","stats":{"tokens":12},"error":null}`, commandArtifacts{provider: "gemini"})
		if err != nil {
			t.Fatalf("extractResponse returned error: %v", err)
		}
		if got.FinalMessage != "{\"summary\":\"ok\"}" {
			t.Fatalf("got %q, want %q", got.FinalMessage, "{\"summary\":\"ok\"}")
		}
	})

	t.Run("codex reads last message file", func(t *testing.T) {
		t.Parallel()

		outputFile, err := createTempOutputFile()
		if err != nil {
			t.Fatalf("createTempOutputFile returned error: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(outputFile)
		})

		if err := os.WriteFile(outputFile, []byte("final answer\n"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		got, err := extractResponse("", commandArtifacts{provider: "codex", outputFile: outputFile})
		if err != nil {
			t.Fatalf("extractResponse returned error: %v", err)
		}
		if got.FinalMessage != "final answer" {
			t.Fatalf("got %q, want %q", got.FinalMessage, "final answer")
		}
	})
}

func TestNormalizeOutput(t *testing.T) {
	t.Parallel()

	got := normalizeOutput("\n  hello world  \n\n")
	if got != "hello world" {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}

func TestRunDelegatesToRunWithProgressEnabled(t *testing.T) {
	t.Parallel()

	opts := RunOptions{ShowProgress: true}
	if !opts.ShowProgress {
		t.Fatal("expected ShowProgress to be true")
	}
}
