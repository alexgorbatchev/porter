package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		version     string
		inputIsTTY  bool
		outputIsTTY bool
		wantStdout  string
		wantStderr  string
		wantErrText string
	}{
		{
			name:       "prints help",
			args:       []string{"--help"},
			version:    "dev",
			wantStdout: "Usage:\n  porter\n\nInteractive controls:\n  j/k  Move selection down/up\n  x    Send SIGTERM to selected PID\n  r    Refresh now\n  q    Quit\n\nNotes:\n  - Linux only; listener discovery uses sock_diag plus /proc process metadata.\n  - Matches processes whose current working directory is this cwd or a descendant.\n",
		},
		{
			name:       "prints version",
			args:       []string{"--version"},
			version:    "v1.2.3",
			wantStdout: "porter v1.2.3\n",
		},
		{
			name:        "rejects unknown flag",
			args:        []string{"--wat"},
			version:     "dev",
			wantStderr:  "unknown flag: --wat\n",
			wantErrText: "unknown flag",
		},
		{
			name:        "requires interactive terminal",
			version:     "dev",
			wantStderr:  "porter requires an interactive terminal on stdin and stdout\n",
			wantErrText: "interactive terminal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			err := Run(tt.args, Environment{
				Input:       strings.NewReader(""),
				Output:      stdout,
				ErrorOutput: stderr,
				InputIsTTY:  tt.inputIsTTY,
				OutputIsTTY: tt.outputIsTTY,
				Version:     tt.version,
				WorkDir:     "/work",
				ProcRoot:    "/proc",
			})

			if tt.wantErrText == "" && err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			if tt.wantErrText != "" {
				if err == nil {
					t.Fatal("Run() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("Run() error = %q, want substring %q", err.Error(), tt.wantErrText)
				}
			}

			if got := stdout.String(); got != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, tt.wantStdout)
			}

			if got := stderr.String(); got != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestReadKeysSupportsJKAndX(t *testing.T) {
	t.Parallel()

	keys := make(chan keyEvent)
	go readKeys(strings.NewReader("jkxrq"), keys)

	got := make([]keyEvent, 0, 5)
	for key := range keys {
		got = append(got, key)
	}

	want := []keyEvent{keyDown, keyUp, keyKill, keyRefresh, keyQuit}
	if len(got) != len(want) {
		t.Fatalf("key count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %v, want %v (all=%v)", i, got[i], want[i], got)
		}
	}
}
