package procfs

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

func TestListEntries(t *testing.T) {
	t.Parallel()

	procRoot := t.TempDir()
	workDir := filepath.Join(procRoot, "workspace")
	if err := os.MkdirAll(filepath.Join(workDir, "apps", "web"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	createProcess(t, processFixture{
		procRoot: procRoot,
		pid:      "1234",
		cwd:      filepath.Join(workDir, "apps", "web"),
		cmdline:  []string{filepath.Join(workDir, "node_modules", ".bin", "bun"), "dev"},
		sockets:  []string{"12345", "12346"},
	})
	createProcess(t, processFixture{
		procRoot: procRoot,
		pid:      "9999",
		cwd:      filepath.Join(procRoot, "elsewhere"),
		cmdline:  []string{"python", "-m", "http.server"},
		sockets:  []string{"54321"},
	})

	scanner := Scanner{ProcRoot: procRoot, WorkDir: workDir}
	got, err := scanner.listEntries(procRoot, map[string][]int{
		"12345": {3000},
		"12346": {3000},
		"54321": {4444},
	})
	if err != nil {
		t.Fatalf("listEntries() error = %v", err)
	}

	want := []Entry{{
		PID:     1234,
		Port:    3000,
		Command: "apps/web $ node_modules/.bin/bun dev",
	}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listEntries() = %#v, want %#v", got, want)
	}
}

func TestListEntriesFormatsExternalExecutableAsBasename(t *testing.T) {
	t.Parallel()

	procRoot := t.TempDir()
	workDir := filepath.Join(procRoot, "workspace")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	createProcess(t, processFixture{
		procRoot: procRoot,
		pid:      "2345",
		cwd:      workDir,
		cmdline:  []string{"/home/alex/.dotfiles/.generated/binaries/mdserve/current/mdserve", "."},
		sockets:  []string{"22222"},
	})

	scanner := Scanner{ProcRoot: procRoot, WorkDir: workDir}
	got, err := scanner.listEntries(procRoot, map[string][]int{
		"22222": {3000},
	})
	if err != nil {
		t.Fatalf("listEntries() error = %v", err)
	}

	want := []Entry{{
		PID:     2345,
		Port:    3000,
		Command: "mdserve .",
	}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listEntries() = %#v, want %#v", got, want)
	}
}

func TestShouldIgnoreProcessError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "wrapped permission denied",
			err:  fmt.Errorf("reading cwd for pid 1: %w", &os.PathError{Op: "readlink", Path: "/proc/1/cwd", Err: syscall.EACCES}),
			want: true,
		},
		{
			name: "wrapped not exist",
			err:  fmt.Errorf("reading fd for pid 9: %w", &os.PathError{Op: "readlink", Path: "/proc/9/fd/0", Err: syscall.ENOENT}),
			want: true,
		},
		{
			name: "other wrapped error",
			err:  fmt.Errorf("reading proc root: %w", syscall.EIO),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldIgnoreProcessError(tt.err); got != tt.want {
				t.Fatalf("shouldIgnoreProcessError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

type processFixture struct {
	procRoot string
	pid      string
	cwd      string
	cmdline  []string
	sockets  []string
}

func createProcess(t *testing.T, fixture processFixture) {
	t.Helper()

	pidRoot := filepath.Join(fixture.procRoot, fixture.pid)
	if err := os.MkdirAll(filepath.Join(pidRoot, "fd"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(fixture.cwd, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidRoot, "cmdline"), []byte(joinCmdline(fixture.cmdline)), 0o644); err != nil {
		t.Fatalf("WriteFile(cmdline) error = %v", err)
	}
	if err := os.Symlink(fixture.cwd, filepath.Join(pidRoot, "cwd")); err != nil {
		t.Fatalf("Symlink(cwd) error = %v", err)
	}

	for i, socket := range fixture.sockets {
		fdPath := filepath.Join(pidRoot, "fd", string(rune('0'+i)))
		if err := os.Symlink("socket:["+socket+"]", fdPath); err != nil {
			t.Fatalf("Symlink(fd) error = %v", err)
		}
	}
}

func joinCmdline(parts []string) string {
	if len(parts) == 0 {
		return ""
	}

	joined := ""
	for _, part := range parts {
		joined += part + "\x00"
	}

	return joined
}
