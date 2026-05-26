package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"pr/pkg/ui"
)

type Response struct {
	Provider     string
	FinalMessage string
	RawTransport string
}

type Mode string

const (
	ReadOnly Mode = "read_only"
	Write    Mode = "write"
)

const (
	ansiReset = "\033[0m"
	ansiRed   = "\033[31m"
	ansiBlue  = "\033[34m"
)

func (m Mode) Label() string {
	switch m {
	case Write:
		return "WRITE"
	case ReadOnly:
		fallthrough
	default:
		return "READ"
	}
}

func (m Mode) ColorCode() string {
	switch m {
	case Write:
		return ansiRed
	case ReadOnly:
		fallthrough
	default:
		return ansiBlue
	}
}

func (m Mode) ColoredLabel() string {
	if !ui.IsColorEnabled() {
		return m.Label()
	}
	return m.ColorCode() + m.Label() + ansiReset
}

func (m Mode) StatusText() string {
	switch m {
	case Write:
		return "is applying changes in the worktree..."
	case ReadOnly:
		fallthrough
	default:
		return "is analyzing the worktree..."
	}
}

type commandArtifacts struct {
	provider   string
	outputFile string
	mode       Mode
}

type RunOptions struct {
	Timeout      time.Duration
	ShowProgress bool
	Verbose      bool
}

// Run runs the AI agent in the specific worktree directory and returns its normalized response.
func Run(dir string, prompt string, llm string, mode Mode) (Response, error) {
	return RunWithOptions(dir, prompt, llm, mode, RunOptions{ShowProgress: true, Verbose: true})
}

func RunWithOptions(dir string, prompt string, llm string, mode Mode, opts RunOptions) (Response, error) {
	var ctx context.Context
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	cmd, artifacts, cleanup, err := buildCommand(ctx, prompt, llm, mode, opts)
	if err != nil {
		return Response{}, err
	}
	defer cleanup()
	cmd.Dir = dir

	// Verbose streaming: parse JSONL events so the user sees activity in real-time.
	switch {
	case opts.Verbose && artifacts.provider == "claude":
		return runClaudeStreamJSON(cmd, artifacts)
	case opts.Verbose && artifacts.provider == "codex":
		return runCodexStreamJSON(cmd, artifacts)
	case opts.Verbose && artifacts.provider == "gemini":
		return runGeminiStreamJSON(cmd, artifacts)
	case opts.Verbose && artifacts.provider == "opencode":
		return runOpencodeStreamJSON(cmd, artifacts)
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	showSpinner := opts.ShowProgress || opts.Verbose
	var done chan struct{}
	var finished chan struct{}
	if showSpinner {
		done = make(chan struct{})
		finished = make(chan struct{})
		go func() {
			spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			i := 0
			for {
				select {
				case <-done:
					fmt.Fprint(os.Stderr, "\r\033[K")
					close(finished)
					return
				case <-time.After(100 * time.Millisecond):
					fmt.Fprintf(os.Stderr, "\r\033[K%s AI (%s/%s) %s", spinner[i], artifacts.provider, artifacts.mode.ColoredLabel(), artifacts.mode.StatusText())
					i = (i + 1) % len(spinner)
				}
			}
		}()
	}

	runErr := cmd.Run()
	if showSpinner {
		close(done)
		<-finished
	}

	stdout := outBuf.String()
	stderr := normalizeOutput(errBuf.String())

	if opts.Verbose && normalizeOutput(stdout) != "" {
		fmt.Fprintln(os.Stderr, normalizeOutput(stdout))
	}

	if runErr != nil {
		if stderr != "" {
			return Response{}, fmt.Errorf("AI agent failed: %w\n%s", runErr, stderr)
		}
		return Response{}, fmt.Errorf("AI agent failed: %w", runErr)
	}

	response, err := extractResponse(stdout, artifacts)
	if err != nil {
		return Response{}, fmt.Errorf("failed to parse %s output: %w\nRaw Output:\n%s", artifacts.provider, err, normalizeOutput(stdout))
	}

	return response, nil
}

func buildCommand(ctx context.Context, prompt string, llm string, mode Mode, opts RunOptions) (*exec.Cmd, commandArtifacts, func(), error) {
	switch llm {
	case "gemini":
		format := "json"
		if opts.Verbose {
			format = "stream-json"
		}
		args := []string{"-p", prompt, "--output-format", format}
		if mode == Write {
			args = append(args, "--approval-mode", "auto_edit")
		}
		return exec.CommandContext(ctx, "gemini", args...), commandArtifacts{provider: "gemini", mode: mode}, func() {}, nil
	case "codex":
		outputFile, err := createTempOutputFile()
		if err != nil {
			return nil, commandArtifacts{}, nil, fmt.Errorf("failed to create codex output file: %w", err)
		}

		cleanup := func() {
			_ = os.Remove(outputFile)
		}

		args := []string{"exec", "--color", "never", "--output-last-message", outputFile}
		if opts.Verbose {
			args = append(args, "--json")
		}
		args = append(args, prompt)
		return exec.CommandContext(ctx, "codex", args...), commandArtifacts{
			provider:   "codex",
			outputFile: outputFile,
			mode:       mode,
		}, cleanup, nil
	case "opencode":
		args := []string{"run", "--format", "json"}
		args = append(args, prompt)
		return exec.CommandContext(ctx, "opencode", args...), commandArtifacts{provider: "opencode", mode: mode}, func() {}, nil
	case "claude":
		fallthrough
	default:
		format := "json"
		if opts.Verbose {
			format = "stream-json"
		}
		args := []string{"-p", "--output-format", format}
		if opts.Verbose {
			args = append(args, "--verbose", "--include-partial-messages")
		}
		if mode == Write {
			args = append(args, "--permission-mode", "acceptEdits")
		}
		args = append(args, prompt)
		return exec.CommandContext(ctx, "claude", args...), commandArtifacts{provider: "claude", mode: mode}, func() {}, nil
	}
}

func createTempOutputFile() (string, error) {
	file, err := os.CreateTemp("", "pr-codex-last-message-*.txt")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// runClaudeStreamJSON reads Claude's stream-json output line by line,
// displaying tool use activity to stderr so the user can follow along.
func runClaudeStreamJSON(cmd *exec.Cmd, artifacts commandArtifacts) (Response, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Response{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	// Capture Claude's stderr (--verbose debug output) so it doesn't
	// interfere with our terminal output.
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return Response{}, fmt.Errorf("AI agent failed to start: %w", err)
	}

	var (
		mu               sync.Mutex
		finalMessage     string
		currentToolName  string
		currentToolInput strings.Builder
	)

	// printActivity clears the spinner and prints a tool-use activity line.
	printActivity := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprint(os.Stderr, "\r\033[K") // clear spinner
		fmt.Fprintln(os.Stderr, line)
	}

	// Spinner goroutine — shows a working indicator while Claude is busy.
	// Redraws in-place on the current line; activity lines push it down.
	spinnerDone := make(chan struct{})
	spinnerFinished := make(chan struct{})
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-spinnerDone:
				mu.Lock()
				fmt.Fprint(os.Stderr, "\r\033[K")
				mu.Unlock()
				close(spinnerFinished)
				return
			case <-time.After(100 * time.Millisecond):
				mu.Lock()
				fmt.Fprintf(os.Stderr, "\r\033[K%s AI (%s/%s) %s", frames[i], artifacts.provider, artifacts.mode.ColoredLabel(), artifacts.mode.StatusText())
				i = (i + 1) % len(frames)
				mu.Unlock()
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event struct {
			Type    string `json:"type"`
			Result  string `json:"result,omitempty"`
			IsError bool   `json:"is_error,omitempty"`
			Subtype string `json:"subtype,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "stream_event":
			var se struct {
				Event struct {
					Type         string `json:"type"`
					ContentBlock struct {
						Type string `json:"type"`
						Name string `json:"name"`
					} `json:"content_block"`
					Delta struct {
						Type        string `json:"type"`
						PartialJSON string `json:"partial_json"`
					} `json:"delta"`
				} `json:"event"`
			}
			if err := json.Unmarshal([]byte(line), &se); err != nil {
				continue
			}
			switch se.Event.Type {
			case "content_block_start":
				if se.Event.ContentBlock.Type == "tool_use" {
					currentToolName = se.Event.ContentBlock.Name
					currentToolInput.Reset()
				}
			case "content_block_delta":
				if se.Event.Delta.Type == "input_json_delta" && currentToolName != "" {
					currentToolInput.WriteString(se.Event.Delta.PartialJSON)
				}
			case "content_block_stop":
				if currentToolName != "" {
					label := formatToolUse(currentToolName, currentToolInput.String())
					printActivity(ui.FormatNote(label))
					currentToolName = ""
					currentToolInput.Reset()
				}
			}
		case "result":
			finalMessage = event.Result
			if event.IsError {
				close(spinnerDone)
				<-spinnerFinished
				return Response{}, fmt.Errorf("Claude returned error: %s", event.Result)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		close(spinnerDone)
		<-spinnerFinished
		return Response{}, fmt.Errorf("reading Claude stream: %w", err)
	}

	close(spinnerDone)
	<-spinnerFinished

	if err := cmd.Wait(); err != nil {
		stderr := normalizeOutput(errBuf.String())
		if stderr != "" {
			return Response{}, fmt.Errorf("AI agent failed: %w\n%s", err, stderr)
		}
		return Response{}, fmt.Errorf("AI agent failed: %w", err)
	}

	return Response{
		Provider:     artifacts.provider,
		FinalMessage: normalizeOutput(finalMessage),
		RawTransport: "(streamed)",
	}, nil
}

// runCodexStreamJSON reads Codex's --json JSONL output line by line,
// displaying command executions and file changes as they happen.
func runCodexStreamJSON(cmd *exec.Cmd, artifacts commandArtifacts) (Response, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Response{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return Response{}, fmt.Errorf("AI agent failed to start: %w", err)
	}

	var (
		mu           sync.Mutex
		finalMessage string
	)

	printActivity := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprint(os.Stderr, "\r\033[K")
		fmt.Fprintln(os.Stderr, line)
	}

	spinnerDone := make(chan struct{})
	spinnerFinished := make(chan struct{})
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-spinnerDone:
				mu.Lock()
				fmt.Fprint(os.Stderr, "\r\033[K")
				mu.Unlock()
				close(spinnerFinished)
				return
			case <-time.After(100 * time.Millisecond):
				mu.Lock()
				fmt.Fprintf(os.Stderr, "\r\033[K%s AI (%s/%s) %s", frames[i], artifacts.provider, artifacts.mode.ColoredLabel(), artifacts.mode.StatusText())
				i = (i + 1) % len(frames)
				mu.Unlock()
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event struct {
			Type  string `json:"type"`
			Error struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
			Item struct {
				Type    string `json:"type"`
				Command string `json:"command,omitempty"`
				Text    string `json:"text,omitempty"`
				Server  string `json:"server,omitempty"`
				Tool    string `json:"tool,omitempty"`
				Changes []struct {
					Path string `json:"path"`
					Kind string `json:"kind"`
				} `json:"changes,omitempty"`
			} `json:"item,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "item.started":
			switch event.Item.Type {
			case "command_execution":
				if event.Item.Command != "" {
					cmd := strings.TrimPrefix(event.Item.Command, "/usr/bin/bash -lc ")
					printActivity(ui.FormatNote("$ " + truncate(cmd, 80)))
				}
			case "mcp_tool_call":
				if event.Item.Tool != "" {
					label := event.Item.Tool
					if event.Item.Server != "" {
						label = event.Item.Server + ":" + label
					}
					printActivity(ui.FormatNote(label))
				}
			}
		case "item.completed":
			switch event.Item.Type {
			case "file_change":
				for _, change := range event.Item.Changes {
					kind := strings.ToUpper(change.Kind[:1]) + change.Kind[1:]
					printActivity(ui.FormatNote(kind + " " + shortenPath(change.Path)))
				}
			case "agent_message":
				if event.Item.Text != "" {
					finalMessage = event.Item.Text
				}
			}
		case "turn.failed":
			if event.Error.Message != "" {
				close(spinnerDone)
				<-spinnerFinished
				return Response{}, fmt.Errorf("Codex turn failed: %s", event.Error.Message)
			}
		case "error":
			if event.Error.Message != "" {
				close(spinnerDone)
				<-spinnerFinished
				return Response{}, fmt.Errorf("Codex error: %s", event.Error.Message)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		close(spinnerDone)
		<-spinnerFinished
		return Response{}, fmt.Errorf("reading Codex stream: %w", err)
	}

	close(spinnerDone)
	<-spinnerFinished

	if err := cmd.Wait(); err != nil {
		stderr := normalizeOutput(errBuf.String())
		if stderr != "" {
			return Response{}, fmt.Errorf("AI agent failed: %w\n%s", err, stderr)
		}
		return Response{}, fmt.Errorf("AI agent failed: %w", err)
	}

	// Prefer agent_message from stream; fall back to output file
	if finalMessage == "" && artifacts.outputFile != "" {
		if content, err := os.ReadFile(artifacts.outputFile); err == nil {
			finalMessage = string(content)
		}
	}

	return Response{
		Provider:     artifacts.provider,
		FinalMessage: normalizeOutput(finalMessage),
		RawTransport: "(streamed)",
	}, nil
}

// runGeminiStreamJSON reads Gemini's stream-json JSONL output line by line,
// displaying tool use activity as it happens.
func runGeminiStreamJSON(cmd *exec.Cmd, artifacts commandArtifacts) (Response, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Response{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return Response{}, fmt.Errorf("AI agent failed to start: %w", err)
	}

	var (
		mu        sync.Mutex
		finalText strings.Builder
	)

	printActivity := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprint(os.Stderr, "\r\033[K")
		fmt.Fprintln(os.Stderr, line)
	}

	spinnerDone := make(chan struct{})
	spinnerFinished := make(chan struct{})
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-spinnerDone:
				mu.Lock()
				fmt.Fprint(os.Stderr, "\r\033[K")
				mu.Unlock()
				close(spinnerFinished)
				return
			case <-time.After(100 * time.Millisecond):
				mu.Lock()
				fmt.Fprintf(os.Stderr, "\r\033[K%s AI (%s/%s) %s", frames[i], artifacts.provider, artifacts.mode.ColoredLabel(), artifacts.mode.StatusText())
				i = (i + 1) % len(frames)
				mu.Unlock()
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event struct {
			Type       string                 `json:"type"`
			Role       string                 `json:"role,omitempty"`
			Content    string                 `json:"content,omitempty"`
			Delta      bool                   `json:"delta,omitempty"`
			ToolName   string                 `json:"tool_name,omitempty"`
			Parameters map[string]interface{} `json:"parameters,omitempty"`
			Status     string                 `json:"status,omitempty"`
			Error      struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "tool_use":
			// Tool use marks end of an assistant text segment; reset so we
			// only keep the final response after all tool interactions.
			finalText.Reset()
			if event.ToolName != "" {
				raw, _ := json.Marshal(event.Parameters)
				label := formatToolUse(event.ToolName, string(raw))
				printActivity(ui.FormatNote(label))
			}
		case "message":
			if event.Role == "assistant" {
				finalText.WriteString(event.Content)
			}
		case "result":
			if event.Status == "error" && event.Error.Message != "" {
				close(spinnerDone)
				<-spinnerFinished
				return Response{}, fmt.Errorf("Gemini error: %s", event.Error.Message)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		close(spinnerDone)
		<-spinnerFinished
		return Response{}, fmt.Errorf("reading Gemini stream: %w", err)
	}

	close(spinnerDone)
	<-spinnerFinished

	if err := cmd.Wait(); err != nil {
		stderr := normalizeOutput(errBuf.String())
		if stderr != "" {
			return Response{}, fmt.Errorf("AI agent failed: %w\n%s", err, stderr)
		}
		return Response{}, fmt.Errorf("AI agent failed: %w", err)
	}

	return Response{
		Provider:     artifacts.provider,
		FinalMessage: normalizeOutput(finalText.String()),
		RawTransport: "(streamed)",
	}, nil
}

// runOpencodeStreamJSON reads OpenCode's --format json JSONL output line by line,
// displaying tool use activity as it happens.
func runOpencodeStreamJSON(cmd *exec.Cmd, artifacts commandArtifacts) (Response, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Response{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return Response{}, fmt.Errorf("AI agent failed to start: %w", err)
	}

	var (
		mu           sync.Mutex
		finalMessage string
	)

	printActivity := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprint(os.Stderr, "\r\033[K")
		fmt.Fprintln(os.Stderr, line)
	}

	spinnerDone := make(chan struct{})
	spinnerFinished := make(chan struct{})
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-spinnerDone:
				mu.Lock()
				fmt.Fprint(os.Stderr, "\r\033[K")
				mu.Unlock()
				close(spinnerFinished)
				return
			case <-time.After(100 * time.Millisecond):
				mu.Lock()
				fmt.Fprintf(os.Stderr, "\r\033[K%s AI (%s/%s) %s", frames[i], artifacts.provider, artifacts.mode.ColoredLabel(), artifacts.mode.StatusText())
				i = (i + 1) % len(frames)
				mu.Unlock()
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event struct {
			Type string `json:"type"`
			Part struct {
				Type  string `json:"type"`
				Tool  string `json:"tool"`
				Text  string `json:"text"`
				State struct {
					Status string                 `json:"status"`
					Input  map[string]interface{} `json:"input"`
					Title  string                 `json:"title"`
					Error  string                 `json:"error"`
				} `json:"state"`
			} `json:"part"`
			Error struct {
				Name string `json:"name"`
				Data struct {
					Message string `json:"message"`
				} `json:"data"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "tool_use":
			if event.Part.State.Status == "completed" || event.Part.State.Status == "error" {
				raw, _ := json.Marshal(event.Part.State.Input)
				label := formatToolUse(event.Part.Tool, string(raw))
				printActivity(ui.FormatNote(label))
			}
		case "text":
			if event.Part.Text != "" {
				finalMessage = event.Part.Text
			}
		case "error":
			errMsg := event.Error.Data.Message
			if errMsg == "" {
				errMsg = event.Error.Name
			}
			close(spinnerDone)
			<-spinnerFinished
			return Response{}, fmt.Errorf("OpenCode error: %s", errMsg)
		}
	}
	if err := scanner.Err(); err != nil {
		close(spinnerDone)
		<-spinnerFinished
		return Response{}, fmt.Errorf("reading OpenCode stream: %w", err)
	}

	close(spinnerDone)
	<-spinnerFinished

	if err := cmd.Wait(); err != nil {
		stderr := normalizeOutput(errBuf.String())
		if stderr != "" {
			return Response{}, fmt.Errorf("AI agent failed: %w\n%s", err, stderr)
		}
		return Response{}, fmt.Errorf("AI agent failed: %w", err)
	}

	return Response{
		Provider:     artifacts.provider,
		FinalMessage: normalizeOutput(finalMessage),
		RawTransport: "(streamed)",
	}, nil
}

func parseOpencodeOutput(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty OpenCode response")
	}

	// OpenCode --format json outputs JSONL; extract the last text part.
	var finalText string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event struct {
			Type string `json:"type"`
			Part struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"part"`
			Error struct {
				Name string `json:"name"`
				Data struct {
					Message string `json:"message"`
				} `json:"data"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		switch event.Type {
		case "text":
			if event.Part.Text != "" {
				finalText = event.Part.Text
			}
		case "error":
			errMsg := event.Error.Data.Message
			if errMsg == "" {
				errMsg = event.Error.Name
			}
			return "", fmt.Errorf("OpenCode error: %s", errMsg)
		}
	}

	if finalText == "" {
		return "", fmt.Errorf("OpenCode response missing text content")
	}
	return normalizeOutput(finalText), nil
}

// formatToolUse returns a human-readable description of a tool call.
func formatToolUse(name, rawInput string) string {
	var input map[string]interface{}
	if json.Unmarshal([]byte(rawInput), &input) != nil {
		return name
	}

	switch name {
	case "Read":
		if p, ok := input["file_path"].(string); ok {
			return "Read " + shortenPath(p)
		}
	case "Edit":
		if p, ok := input["file_path"].(string); ok {
			return "Edit " + shortenPath(p)
		}
	case "Write":
		if p, ok := input["file_path"].(string); ok {
			return "Write " + shortenPath(p)
		}
	case "Glob":
		if p, ok := input["pattern"].(string); ok {
			return "Glob " + p
		}
	case "Grep":
		if p, ok := input["pattern"].(string); ok {
			return "Grep " + truncate(p, 60)
		}
	case "Bash":
		if c, ok := input["command"].(string); ok {
			return "Bash: " + truncate(c, 60)
		}
	}

	// Fallback: try common field names
	for _, key := range []string{"file_path", "pattern", "command", "description"} {
		if v, ok := input[key]; ok {
			return name + ": " + truncate(fmt.Sprintf("%v", v), 70)
		}
	}
	return name
}

func shortenPath(p string) string {
	// Show path relative to common worktree prefixes
	for _, prefix := range []string{"/tmp/pr-worktree-"} {
		if idx := strings.Index(p, prefix); idx >= 0 {
			// Find the end of the worktree dir (next /)
			rest := p[idx+len(prefix):]
			if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
				return rest[slashIdx+1:]
			}
		}
	}
	return p
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func extractResponse(stdout string, artifacts commandArtifacts) (Response, error) {
	raw := normalizeOutput(stdout)

	switch artifacts.provider {
	case "claude":
		content, err := parseClaudeOutput(raw)
		if err != nil {
			return Response{}, err
		}
		return Response{Provider: artifacts.provider, FinalMessage: content, RawTransport: raw}, nil
	case "gemini":
		content, err := parseGeminiOutput(raw)
		if err != nil {
			return Response{}, err
		}
		return Response{Provider: artifacts.provider, FinalMessage: content, RawTransport: raw}, nil
	case "codex":
		content, err := parseCodexOutput(raw, artifacts.outputFile)
		if err != nil {
			return Response{}, err
		}
		return Response{Provider: artifacts.provider, FinalMessage: content, RawTransport: raw}, nil
	case "opencode":
		content, err := parseOpencodeOutput(raw)
		if err != nil {
			return Response{}, err
		}
		return Response{Provider: artifacts.provider, FinalMessage: content, RawTransport: raw}, nil
	default:
		return Response{}, fmt.Errorf("unsupported provider %q", artifacts.provider)
	}
}

func parseClaudeOutput(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty Claude response")
	}

	var payload struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", fmt.Errorf("invalid Claude JSON: %w", err)
	}
	if payload.IsError {
		return "", fmt.Errorf("Claude returned error: %s", payload.Result)
	}

	content := normalizeOutput(payload.Result)
	if content == "" {
		return "", fmt.Errorf("Claude response missing result")
	}
	return content, nil
}

func parseGeminiOutput(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty Gemini response")
	}

	var payload struct {
		Response string          `json:"response"`
		Error    json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(payload.Error)) > 0 && string(bytes.TrimSpace(payload.Error)) != "null" {
		return "", fmt.Errorf("Gemini returned error payload: %s", strings.TrimSpace(string(payload.Error)))
	}

	content := normalizeOutput(payload.Response)
	if content == "" {
		return "", fmt.Errorf("Gemini response missing response field")
	}
	return content, nil
}

func parseCodexOutput(rawStdout string, outputFile string) (string, error) {
	if outputFile != "" {
		content, err := os.ReadFile(outputFile)
		if err != nil {
			return "", err
		}

		normalized := normalizeOutput(string(content))
		if normalized != "" {
			return normalized, nil
		}
	}

	if rawStdout != "" {
		return rawStdout, nil
	}

	return "", fmt.Errorf("empty Codex response")
}

func normalizeOutput(raw string) string {
	return strings.TrimSpace(raw)
}
