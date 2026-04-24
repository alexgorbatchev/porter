package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"porter/internal/procfs"
)

const (
	minScreenWidth  = 40
	minScreenHeight = 3
	headerLineCount = 2
)

func (s *dashboardState) render(w io.Writer, workDir string, size terminalSize) {
	_, _ = io.WriteString(w, "\x1b[H\x1b[2J")
	_, _ = io.WriteString(w, s.screen(workDir, size))
}

func (s *dashboardState) screen(workDir string, size terminalSize) string {
	width := maxInt(size.Width, minScreenWidth)
	height := maxInt(size.Height, minScreenHeight)

	rowLimit := maxInt(height-headerLineCount, 1)
	start, end := s.visibleRange(rowLimit)
	lines := make([]string, 0, height)
	lines = append(lines, s.headerLines(workDir, width, start, end)...)
	lines = append(lines, s.entryLines(width, start, end)...)

	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\r\n") + "\r\n"
}

func (s *dashboardState) headerLines(workDir string, width int, start int, end int) []string {
	return []string{
		truncateText("cwd "+workDir, width),
		truncateText("  PID    PORT  COMMAND", width),
	}
}

func (s *dashboardState) entryLines(width int, start int, end int) []string {
	if len(s.entries) == 0 {
		return []string{truncateText("  (no listening TCP ports under cwd)", width)}
	}

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		lines = append(lines, formatEntryRow(s.entries[i], i == s.selected, width))
	}
	return lines
}

func (s *dashboardState) visibleRange(limit int) (int, int) {
	if len(s.entries) == 0 {
		return 0, 0
	}
	if len(s.entries) <= limit {
		return 0, len(s.entries)
	}

	start := s.selected - limit/2
	if start < 0 {
		start = 0
	}
	maxStart := len(s.entries) - limit
	if start > maxStart {
		start = maxStart
	}

	return start, start + limit
}

func formatEntryRow(entry procfs.Entry, selected bool, width int) string {
	marker := ' '
	if selected {
		marker = '>'
	}

	prefix := fmt.Sprintf("%c %-6d %-5d ", marker, entry.PID, entry.Port)
	commandWidth := maxInt(width-utf8.RuneCountInString(prefix), 0)
	if commandWidth == 0 {
		return truncateText(prefix, width)
	}

	return prefix + truncateText(entry.Command, commandWidth)
}

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}

	return string(runes[:width-3]) + "..."
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
