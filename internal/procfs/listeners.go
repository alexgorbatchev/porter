package procfs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Entry struct {
	PID     int
	Port    int
	Command string
}

type Scanner struct {
	ProcRoot string
	WorkDir  string
}

func (s Scanner) List() ([]Entry, error) {
	procRoot := s.ProcRoot
	if procRoot == "" {
		procRoot = "/proc"
	}

	listenerPorts, err := readListenerPorts()
	if err != nil {
		return nil, err
	}

	return s.listEntries(procRoot, listenerPorts)
}

func (s Scanner) listEntries(procRoot string, listenerPorts map[string][]int) ([]Entry, error) {

	dirs, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf("reading proc root: %w", err)
	}

	entries := make([]Entry, 0)
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(dir.Name())
		if err != nil {
			continue
		}

		pidEntries, err := s.listProcessEntries(procRoot, pid, listenerPorts)
		if err != nil {
			if shouldIgnoreProcessError(err) {
				continue
			}
			return nil, err
		}

		entries = append(entries, pidEntries...)
	}

	sort.Slice(entries, func(i int, j int) bool {
		if entries[i].PID != entries[j].PID {
			return entries[i].PID < entries[j].PID
		}
		if entries[i].Port != entries[j].Port {
			return entries[i].Port < entries[j].Port
		}
		return entries[i].Command < entries[j].Command
	})

	return entries, nil
}

func (s Scanner) listProcessEntries(procRoot string, pid int, listenerPorts map[string][]int) ([]Entry, error) {
	pidRoot := filepath.Join(procRoot, strconv.Itoa(pid))

	processCWD, err := os.Readlink(filepath.Join(pidRoot, "cwd"))
	if err != nil {
		return nil, fmt.Errorf("reading cwd for pid %d: %w", pid, err)
	}
	if !isWithinRoot(s.WorkDir, processCWD) {
		return nil, nil
	}

	ports, err := readProcessPorts(pidRoot, listenerPorts)
	if err != nil {
		return nil, fmt.Errorf("reading listening ports for pid %d: %w", pid, err)
	}
	if len(ports) == 0 {
		return nil, nil
	}

	command, err := readDisplayCommand(pidRoot, s.WorkDir, processCWD)
	if err != nil {
		return nil, fmt.Errorf("reading command for pid %d: %w", pid, err)
	}

	entries := make([]Entry, 0, len(ports))
	for _, port := range ports {
		entries = append(entries, Entry{
			PID:     pid,
			Port:    port,
			Command: command,
		})
	}

	return entries, nil
}

func readProcessPorts(pidRoot string, listenerPorts map[string][]int) ([]int, error) {
	fdRoot := filepath.Join(pidRoot, "fd")
	fds, err := os.ReadDir(fdRoot)
	if err != nil {
		return nil, err
	}

	portSet := make(map[int]struct{})
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(fdRoot, fd.Name()))
		if err != nil {
			if shouldIgnoreProcessError(err) {
				continue
			}
			return nil, err
		}

		inode, ok := socketInode(target)
		if !ok {
			continue
		}

		for _, port := range listenerPorts[inode] {
			portSet[port] = struct{}{}
		}
	}

	ports := make([]int, 0, len(portSet))
	for port := range portSet {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	return ports, nil
}

func socketInode(target string) (string, bool) {
	if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
		return "", false
	}

	return strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]"), true
}

func readDisplayCommand(pidRoot string, workDir string, processCWD string) (string, error) {
	data, err := os.ReadFile(filepath.Join(pidRoot, "cmdline"))
	if err != nil {
		return "", err
	}

	parts := splitCmdline(data)
	if len(parts) == 0 {
		exe, err := os.Readlink(filepath.Join(pidRoot, "exe"))
		if err != nil {
			return displayPrefix(workDir, processCWD) + "<unknown>", nil
		}
		parts = []string{exe}
	}

	displayParts := make([]string, 0, len(parts))
	for i, part := range parts {
		displayParts = append(displayParts, quoteIfNeeded(formatCommandPart(workDir, part, i == 0)))
	}

	return displayPrefix(workDir, processCWD) + strings.Join(displayParts, " "), nil
}

func formatCommandPart(workDir string, value string, executable bool) string {
	if !executable {
		return relativeToRoot(workDir, value)
	}
	if !filepath.IsAbs(value) {
		return value
	}
	if isWithinRoot(workDir, value) {
		return relativeToRoot(workDir, value)
	}

	base := filepath.Base(value)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return value
	}

	return base
}

func splitCmdline(data []byte) []string {
	trimmed := bytes.TrimRight(data, "\x00")
	if len(trimmed) == 0 {
		return nil
	}

	raw := bytes.Split(trimmed, []byte{0})
	parts := make([]string, 0, len(raw))
	for _, item := range raw {
		if len(item) == 0 {
			continue
		}
		parts = append(parts, string(item))
	}

	return parts
}

func relativeToRoot(root string, value string) string {
	if !filepath.IsAbs(value) {
		return value
	}
	if !isWithinRoot(root, value) {
		return value
	}

	rel, err := filepath.Rel(root, value)
	if err != nil {
		return value
	}

	return rel
}

func displayPrefix(root string, cwd string) string {
	rel, err := filepath.Rel(root, cwd)
	if err != nil || rel == "." {
		return ""
	}

	return rel + " $ "
}

func quoteIfNeeded(value string) string {
	if value == "" {
		return strconv.Quote(value)
	}
	if strings.ContainsAny(value, " \t\n\r\"") {
		return strconv.Quote(value)
	}
	return value
}

func isWithinRoot(root string, candidate string) bool {
	if root == "" {
		return false
	}

	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}

	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "..")
}

func shouldIgnoreProcessError(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission)
}
