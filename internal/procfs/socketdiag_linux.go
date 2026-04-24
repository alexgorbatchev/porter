//go:build linux

package procfs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	inetDiagStateListen = 10
	inetDiagListenMask  = 1 << inetDiagStateListen
	inetDiagCookieAny   = ^uint32(0)
	inetDiagFamilyV4    = unix.AF_INET
	inetDiagFamilyV6    = unix.AF_INET6
)

type inetDiagSockID struct {
	Sport     [2]byte
	Dport     [2]byte
	Src       [16]byte
	Dst       [16]byte
	Interface uint32
	Cookie    [2]uint32
}

type inetDiagReqV2 struct {
	Family   uint8
	Protocol uint8
	Ext      uint8
	Pad      uint8
	States   uint32
	ID       inetDiagSockID
}

type inetDiagMsg struct {
	Family  uint8
	State   uint8
	Timer   uint8
	Retrans uint8
	ID      inetDiagSockID
	Expires uint32
	RQueue  uint32
	WQueue  uint32
	UID     uint32
	Inode   uint32
}

func readListenerPorts() (map[string][]int, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		return nil, fmt.Errorf("opening socket diagnostics netlink socket: %w", err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, fmt.Errorf("binding socket diagnostics netlink socket: %w", err)
	}

	ports := make(map[string][]int)
	for i, family := range []uint8{inetDiagFamilyV4, inetDiagFamilyV6} {
		seq := uint32(i + 1)
		if err := dumpListenerPorts(fd, family, seq, ports); err != nil {
			return nil, err
		}
	}

	return ports, nil
}

func dumpListenerPorts(fd int, family uint8, seq uint32, ports map[string][]int) error {
	if err := sendInetDiagRequest(fd, family, seq); err != nil {
		return err
	}

	buf := make([]byte, 64*1024)
	for {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return fmt.Errorf("receiving socket diagnostics response: %w", err)
		}

		messages, err := syscall.ParseNetlinkMessage(buf[:n])
		if err != nil {
			return fmt.Errorf("parsing socket diagnostics netlink message: %w", err)
		}

		done, err := applyNetlinkMessages(messages, seq, ports)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func sendInetDiagRequest(fd int, family uint8, seq uint32) error {
	req := inetDiagReqV2{
		Family:   family,
		Protocol: unix.IPPROTO_TCP,
		States:   inetDiagListenMask,
		ID: inetDiagSockID{
			Cookie: [2]uint32{inetDiagCookieAny, inetDiagCookieAny},
		},
	}

	header := unix.NlMsghdr{
		Len:   uint32(unix.SizeofNlMsghdr + binary.Size(req)),
		Type:  unix.SOCK_DIAG_BY_FAMILY,
		Flags: unix.NLM_F_REQUEST | unix.NLM_F_DUMP,
		Seq:   seq,
	}

	var payload bytes.Buffer
	if err := binary.Write(&payload, binary.NativeEndian, header); err != nil {
		return fmt.Errorf("encoding socket diagnostics header: %w", err)
	}
	if err := binary.Write(&payload, binary.NativeEndian, req); err != nil {
		return fmt.Errorf("encoding socket diagnostics request: %w", err)
	}

	if err := unix.Sendto(fd, payload.Bytes(), 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return fmt.Errorf("sending socket diagnostics request: %w", err)
	}

	return nil
}

func applyNetlinkMessages(messages []syscall.NetlinkMessage, seq uint32, ports map[string][]int) (bool, error) {
	for _, message := range messages {
		if message.Header.Seq != seq {
			continue
		}

		switch message.Header.Type {
		case syscall.NLMSG_DONE:
			return true, nil
		case syscall.NLMSG_ERROR:
			errno, err := parseNetlinkError(message.Data)
			if err != nil {
				return false, fmt.Errorf("parsing socket diagnostics error: %w", err)
			}
			if errno == 0 {
				continue
			}
			return false, fmt.Errorf("socket diagnostics request failed: %w", syscall.Errno(-errno))
		case unix.SOCK_DIAG_BY_FAMILY:
			inode, port, ok, err := parseInetDiagMessage(message.Data)
			if err != nil {
				return false, fmt.Errorf("parsing socket diagnostics response: %w", err)
			}
			if !ok {
				continue
			}
			ports[inode] = append(ports[inode], port)
		}
	}

	return false, nil
}

func parseNetlinkError(data []byte) (int32, error) {
	if len(data) < 4 {
		return 0, fmt.Errorf("short netlink error payload: got %d bytes", len(data))
	}

	return int32(binary.NativeEndian.Uint32(data[:4])), nil
}

func parseInetDiagMessage(data []byte) (string, int, bool, error) {
	var msg inetDiagMsg
	if len(data) < binary.Size(msg) {
		return "", 0, false, fmt.Errorf("short inet diag payload: got %d bytes", len(data))
	}

	if err := binary.Read(bytes.NewReader(data[:binary.Size(msg)]), binary.NativeEndian, &msg); err != nil {
		return "", 0, false, fmt.Errorf("decoding inet diag payload: %w", err)
	}
	if msg.Family != inetDiagFamilyV4 && msg.Family != inetDiagFamilyV6 {
		return "", 0, false, nil
	}
	if msg.State != inetDiagStateListen {
		return "", 0, false, nil
	}

	port := int(binary.BigEndian.Uint16(msg.ID.Sport[:]))
	inode := strconv.FormatUint(uint64(msg.Inode), 10)
	return inode, port, true, nil
}
