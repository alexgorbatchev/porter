package cli

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"porter/internal/procfs"
)

const (
	defaultRefreshInterval = 750 * time.Millisecond
	followUpRefreshDelay   = 200 * time.Millisecond
	helpText               = `Usage:
  porter

Interactive controls:
  j/k  Move selection down/up
  x    Send SIGTERM to selected PID
  r    Refresh now
  q    Quit

Notes:
  - Linux only; listener discovery uses sock_diag plus /proc process metadata.
  - Matches processes whose current working directory is this cwd or a descendant.
`
)

type Environment struct {
	Input           io.Reader
	Output          io.Writer
	ErrorOutput     io.Writer
	InputFD         int
	OutputFD        int
	InputIsTTY      bool
	OutputIsTTY     bool
	Version         string
	WorkDir         string
	ProcRoot        string
	RefreshInterval time.Duration
}

type keyEvent int

const (
	keyUnknown keyEvent = iota
	keyUp
	keyDown
	keyKill
	keyRefresh
	keyQuit
)

func Run(args []string, env Environment) error {
	version := env.Version
	if version == "" {
		version = "dev"
	}

	switch len(args) {
	case 0:
		return runInteractive(env)
	case 1:
		switch args[0] {
		case "--help":
			_, err := io.WriteString(env.Output, helpText)
			return err
		case "--version":
			_, err := fmt.Fprintf(env.Output, "porter %s\n", version)
			return err
		default:
			err := fmt.Errorf("unknown flag: %s", args[0])
			_, _ = fmt.Fprintf(env.ErrorOutput, "%v\n", err)
			return err
		}
	default:
		err := fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
		_, _ = fmt.Fprintf(env.ErrorOutput, "%v\n", err)
		return err
	}
}

func runInteractive(env Environment) error {
	if runtime.GOOS != "linux" {
		err := fmt.Errorf("porter currently supports Linux only because it relies on Linux socket diagnostics and /proc")
		_, _ = fmt.Fprintf(env.ErrorOutput, "%v\n", err)
		return err
	}

	if !env.InputIsTTY || !env.OutputIsTTY {
		err := fmt.Errorf("porter requires an interactive terminal on stdin and stdout")
		_, _ = fmt.Fprintf(env.ErrorOutput, "%v\n", err)
		return err
	}

	refreshInterval := env.RefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = defaultRefreshInterval
	}

	scanner := procfs.Scanner{
		ProcRoot: env.ProcRoot,
		WorkDir:  env.WorkDir,
	}

	state := dashboardState{}
	state.refresh(scanner)

	rawState, err := term.MakeRaw(env.InputFD)
	if err != nil {
		return fmt.Errorf("enabling raw terminal mode: %w", err)
	}
	defer func() {
		_ = term.Restore(env.InputFD, rawState)
	}()

	keys := make(chan keyEvent)
	go readKeys(env.Input, keys)

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	var followUp <-chan time.Time
	state.render(env.Output, env.WorkDir, currentTerminalSize(env.OutputFD))

	for {
		select {
		case key, ok := <-keys:
			if !ok {
				return nil
			}

			switch key {
			case keyQuit:
				return nil
			case keyUp:
				state.moveSelection(-1)
			case keyDown:
				state.moveSelection(1)
			case keyRefresh:
				state.refresh(scanner)
			case keyKill:
				if err := state.killSelected(); err != nil {
					state.status = err.Error()
				} else {
					state.refresh(scanner)
					followUp = time.After(followUpRefreshDelay)
				}
			}
		case <-ticker.C:
			state.refresh(scanner)
		case <-followUp:
			state.refresh(scanner)
			followUp = nil
		}

		state.render(env.Output, env.WorkDir, currentTerminalSize(env.OutputFD))
	}
}

type terminalSize struct {
	Width  int
	Height int
}

func currentTerminalSize(fd int) terminalSize {
	size := terminalSize{Width: 80, Height: 24}
	if fd <= 0 {
		return size
	}

	width, height, err := term.GetSize(fd)
	if err != nil {
		return size
	}
	if width > 0 {
		size.Width = width
	}
	if height > 0 {
		size.Height = height
	}

	return size
}

type dashboardState struct {
	entries     []procfs.Entry
	selected    int
	status      string
	lastUpdated time.Time
}

func (s *dashboardState) refresh(scanner procfs.Scanner) {
	entries, err := scanner.List()
	if err != nil {
		s.status = fmt.Sprintf("refresh failed: %v", err)
		return
	}

	s.entries = entries
	s.lastUpdated = time.Now()
	if len(entries) == 0 {
		s.selected = 0
		s.status = "No matching listening TCP ports under cwd"
		return
	}

	if s.selected >= len(entries) {
		s.selected = len(entries) - 1
	}
	if s.selected < 0 {
		s.selected = 0
	}

	s.status = fmt.Sprintf("Tracking %d listening process entries", len(entries))
}

func (s *dashboardState) moveSelection(delta int) {
	if len(s.entries) == 0 {
		return
	}

	s.selected += delta
	if s.selected < 0 {
		s.selected = 0
	}
	if s.selected >= len(s.entries) {
		s.selected = len(s.entries) - 1
	}
}

func (s *dashboardState) killSelected() error {
	if len(s.entries) == 0 {
		return fmt.Errorf("no process selected")
	}

	entry := s.entries[s.selected]
	if err := syscall.Kill(entry.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("sending SIGTERM to %d: %w", entry.PID, err)
	}

	s.status = fmt.Sprintf("Sent SIGTERM to PID %d on port %d", entry.PID, entry.Port)
	return nil
}

func readKeys(r io.Reader, ch chan<- keyEvent) {
	defer close(ch)

	buf := make([]byte, 1)
	for {
		_, err := r.Read(buf)
		if err != nil {
			return
		}

		switch buf[0] {
		case 0x03, 'q':
			ch <- keyQuit
		case 'j':
			ch <- keyDown
		case 'k':
			ch <- keyUp
		case 'x':
			ch <- keyKill
		case 'r':
			ch <- keyRefresh
		case 0x1b:
			key, ok := readEscapeSequence(r)
			if ok {
				ch <- key
			}
		}
	}
}

func readEscapeSequence(r io.Reader) (keyEvent, bool) {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(r, buf[:1]); err != nil {
		return keyUnknown, false
	}
	if buf[0] != '[' {
		return keyUnknown, false
	}
	if _, err := io.ReadFull(r, buf[1:]); err != nil {
		return keyUnknown, false
	}

	switch buf[1] {
	case 'A':
		return keyUp, true
	case 'B':
		return keyDown, true
	default:
		return keyUnknown, false
	}
}
