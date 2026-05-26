package ui

import "testing"

func TestFormatLineNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := formatLine("›", cyan, "hello")
	if got != "› hello" {
		t.Fatalf("got %q, want %q", got, "› hello")
	}
}

func TestFormatLineWithColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")

	got := formatLine("✓", green, "done")
	want := "\033[32m\033[1m✓\033[0m done"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
