//go:build linux

package procfs

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseInetDiagMessage(t *testing.T) {
	t.Parallel()

	msg := inetDiagMsg{
		Family: inetDiagFamilyV4,
		State:  inetDiagStateListen,
		ID: inetDiagSockID{
			Sport: [2]byte{0x0b, 0xb8},
		},
		Inode: 12345,
	}

	var payload bytes.Buffer
	if err := binary.Write(&payload, binary.NativeEndian, msg); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}

	inode, port, ok, err := parseInetDiagMessage(payload.Bytes())
	if err != nil {
		t.Fatalf("parseInetDiagMessage() error = %v", err)
	}
	if !ok {
		t.Fatal("parseInetDiagMessage() ok = false, want true")
	}
	if inode != "12345" {
		t.Fatalf("inode = %q, want %q", inode, "12345")
	}
	if port != 3000 {
		t.Fatalf("port = %d, want %d", port, 3000)
	}
}

func TestParseInetDiagMessageRejectsShortPayload(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseInetDiagMessage([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Fatal("parseInetDiagMessage() error = nil, want error")
	}
}
