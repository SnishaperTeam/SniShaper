//go:build linux

package tlsfrag

import (
	"net"

	"golang.org/x/sys/unix"
)

// SendWithOOB writes data followed by an out-of-band byte (MSG_OOB) on a
// single send, the Linux-specific TLS-RF fragmentation trick.
func SendWithOOB(conn net.Conn, data []byte, oob byte) error {
	rawConn, err := GetRawConn(conn)
	if err != nil {
		return Wrap("get raw conn", err)
	}

	toSend := make([]byte, len(data)+1)
	copy(toSend, data)
	toSend[len(data)] = oob

	var innerErr error
	err = rawConn.Write(func(fd uintptr) (done bool) {
		innerErr = unix.Send(int(fd), toSend, unix.MSG_OOB)
		return innerErr != unix.EAGAIN
	})

	if err != nil {
		return Wrap("rawConn.Write", err)
	}
	if innerErr != nil {
		return Wrap("unix.Send (MSG_OOB)", innerErr)
	}
	return nil
}