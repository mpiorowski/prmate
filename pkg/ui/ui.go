package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

func Confirm(prompt string) bool {
	Bell()
	fmt.Fprint(os.Stderr, formatLine("?", yellow, prompt+" "))
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		text := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return text == "y" || text == "yes"
	}
	return false
}

func Step(format string, args ...any) {
	printLine("›", cyan, format, args...)
}

func Success(format string, args ...any) {
	printLine("✓", green, format, args...)
}

func Warn(format string, args ...any) {
	printLine("!", yellow, format, args...)
}

func Error(format string, args ...any) {
	printLine("✗", red, format, args...)
}

func Note(format string, args ...any) {
	printLine("•", dim, format, args...)
}

// FormatNote returns a formatted note line without printing it.
func FormatNote(message string) string {
	return formatLine("•", dim, message)
}

func Bell() {
	fmt.Fprint(os.Stderr, "\a")
}

func Emphasize(value string) string {
	return style(bold, value)
}

func IsColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return os.Getenv("TERM") != "dumb"
}

func FormatCategory(name string) string {
	return style(magenta+bold, name)
}

func FormatReadUsage(usage string) string {
	return style(blue, "[READ] ") + usage
}

func FormatWriteUsage(usage string) string {
	return style(red, "[WRITE]") + " " + usage
}
func printLine(prefix string, color string, format string, args ...any) {
	fmt.Fprintln(os.Stderr, formatLine(prefix, color, fmt.Sprintf(format, args...)))
}

func formatLine(prefix string, color string, message string) string {
	if !IsColorEnabled() {
		return prefix + " " + message
	}

	return strings.Join([]string{
		style(color+bold, prefix),
		message,
	}, " ")
}

func style(code string, text string) string {
	if !IsColorEnabled() {
		return text
	}
	return code + text + reset
}
