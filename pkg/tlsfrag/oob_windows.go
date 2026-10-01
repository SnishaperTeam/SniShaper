//go:build windows

package tlsfrag

import (
	"fmt"
	"net"

	"golang.org/x/sys/windows"
)

func SendWithOOB(conn net.Conn, data []byte, oob byte) error {
	rawConn, err := GetRawConn(conn)
	if err != nil {
		return fmt.Errorf("get raw conn: %w", err)
	}

	var toSend []byte
	if data == nil {
		toSend = []byte{oob}
	} else {
		toSend = make([]byte, len(data)+1)
		copy(toSend, data)
		toSend[len(data)] = oob
	}
	wsabuf := windows.WSABuf{
		Len: uint32(len(toSend)),
		Buf: &toSend[0],
	}
	var n uint32
	var innerErr error
	err = rawConn.Write(func(fd uintptr) (done bool) {
		innerErr = windows.WSASend(
			windows.Handle(fd),
			&wsabuf,
			1,
			&n,
			windows.MSG_OOB,
			nil,
			nil,
		)
		return innerErr != windows.WSAEWOULDBLOCK
	})
	if err != nil {
		return fmt.Errorf("rawConn.Write: %w", err)
	}
	if innerErr != nil && innerErr != windows.NOERROR {
		return fmt.Errorf("WSASend (MSG_OOB): %w", innerErr)
	}
	return nil
}
