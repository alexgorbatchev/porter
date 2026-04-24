package cli

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"porter/internal/procfs"
)

func TestDashboardScreenFitsViewport(t *testing.T) {
	t.Parallel()

	state := dashboardState{
		entries: []procfs.Entry{{
			PID:     1234,
			Port:    3000,
			Command: "apps/web $ node_modules/.bin/bun dev --host 0.0.0.0",
		}},
		status:      "Tracking 1 listening process entry",
		lastUpdated: time.Date(2026, time.April, 24, 12, 34, 56, 0, time.UTC),
	}

	screen := state.screen("/home/alex/development/github/porter", terminalSize{Width: 40, Height: 8})
	normalized := strings.ReplaceAll(screen, "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(normalized, "\n"), "\n")
	if len(lines) > 8 {
		t.Fatalf("line count = %d, want <= 8\n%s", len(lines), screen)
	}
	for _, line := range lines {
		if utf8.RuneCountInString(line) > 40 {
			t.Fatalf("line width = %d, want <= 40\n%s", utf8.RuneCountInString(line), screen)
		}
	}
	if strings.Contains(normalized, "porter |") {
		t.Fatalf("screen still shows porter header:\n%s", normalized)
	}
	if strings.Contains(normalized, "showing ") {
		t.Fatalf("screen still shows range summary:\n%s", normalized)
	}
	if !strings.Contains(screen, "> 1234") {
		t.Fatalf("screen missing selected row:\n%s", screen)
	}
	if strings.Contains(normalized, "selected ") {
		t.Fatalf("screen still shows selected footer:\n%s", normalized)
	}
	if strings.Contains(normalized, "command  ") {
		t.Fatalf("screen still shows command footer:\n%s", normalized)
	}
	if strings.Contains(normalized, "status   ") {
		t.Fatalf("screen still shows status footer:\n%s", normalized)
	}
	if !strings.Contains(screen, "\r\n  PID") {
		t.Fatalf("screen missing carriage return between lines:\n%q", screen)
	}
	if strings.Contains(strings.ReplaceAll(screen, "\r\n", ""), "\n") {
		t.Fatalf("screen still contains bare newline after removing CRLF pairs:\n%q", screen)
	}
}

func TestDashboardScreenShowsScrollableListAndSelectedDetails(t *testing.T) {
	t.Parallel()

	entries := make([]procfs.Entry, 0, 6)
	for i := range 6 {
		entries = append(entries, procfs.Entry{
			PID:     1200 + i,
			Port:    3000 + i,
			Command: "apps/service-" + string(rune('a'+i)) + " $ bun dev --hot --port dynamic",
		})
	}

	state := dashboardState{
		entries:     entries,
		selected:    4,
		status:      "Tracking 6 listening process entries",
		lastUpdated: time.Date(2026, time.April, 24, 12, 34, 56, 0, time.UTC),
	}

	screen := state.screen("/home/alex/development/github/porter", terminalSize{Width: 60, Height: 8})
	normalized := strings.ReplaceAll(screen, "\r\n", "\n")
	if strings.Contains(normalized, "showing ") {
		t.Fatalf("screen still includes removed range summary:\n%s", normalized)
	}
	if !strings.Contains(normalized, "apps/service-e") {
		t.Fatalf("screen missing selected row content:\n%s", normalized)
	}
	if strings.Contains(normalized, "selected 5/6") || strings.Contains(normalized, "command  ") || strings.Contains(normalized, "status   ") {
		t.Fatalf("screen still includes removed footer lines:\n%s", normalized)
	}
}
