//go:build !linux

package procfs

import "fmt"

func readListenerPorts() (map[string][]int, error) {
	return nil, fmt.Errorf("socket diagnostics are available on Linux only")
}
