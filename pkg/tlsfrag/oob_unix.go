//go:build !windows && !linux

package tlsfrag

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

func SendWithOOB(conn net.Conn, data []byte, oob byte) error {
	rawConn, err := GetRawConn(conn)
	if err != nil {
		return fmt.Errorf("get raw conn: %w", err)
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
		return fmt.Errorf("rawConn.Write: %w", err)
	}
	if innerErr != nil {
		return fmt.Errorf("unix.Send (MSG_OOB): %w", innerErr)
	}
	return nil
}
