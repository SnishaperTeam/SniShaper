package tlsfrag

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	TlsRecordHeaderLen       = 5
	TlsHandshakeRecord       = 22
	TlsFragmentReadWait      = 10 * time.Second
	DefaultTLSRFNumRecords   = 4
	DefaultTLSRFNumSegments  = 1
	DefaultTLSRFSendInterval = 50 * time.Millisecond
	DefaultTLSRFModMinorVer  = true
	DefaultTLSRFOOB          = false
	DefaultTLSRFOOBEx        = false
)

func ReadInitialTLSRecord(conn net.Conn) ([]byte, error) {
	header := make([]byte, TlsRecordHeaderLen)
	_ = conn.SetReadDeadline(time.Now().Add(TlsFragmentReadWait))
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	recordLen := int(binary.BigEndian.Uint16(header[3:5]))
	record := make([]byte, TlsRecordHeaderLen+recordLen)
	copy(record, header)
	if recordLen > 0 {
		if _, err := io.ReadFull(conn, record[TlsRecordHeaderLen:]); err != nil {
			return nil, err
		}
	}
	_ = conn.SetReadDeadline(time.Time{})
	return record, nil
}

func FindLastDot(data []byte, sniPos, sniLen int) (offset int, found bool) {
	for i := sniPos + sniLen; i >= sniPos; i-- {
		if data[i] == '.' {
			return i, true
		}
	}
	return sniLen/2 + sniPos, false
}

func ParseClientHello(data []byte) (prtVer []byte, sniPos int, sniLen int, hasKeyShare bool, err error) {
	const (
		handshakeHeaderLen       = 4
		handshakeTypeClientHello = 0x01
		extTypeSNI               = 0x0000
		extTypeKeyShare          = 0x0033
	)

	prtVer = nil
	sniPos = -1
	sniLen = 0

	if len(data) < TlsRecordHeaderLen {
		return prtVer, sniPos, sniLen, false, errors.New("TLS record too short")
	}

	recordLen := int(binary.BigEndian.Uint16(data[3:5]))
	if len(data) < TlsRecordHeaderLen+recordLen {
		return prtVer, sniPos, sniLen, false, errors.New("record length exceeds data size")
	}
	offset := TlsRecordHeaderLen

	if recordLen < handshakeHeaderLen {
		return prtVer, sniPos, sniLen, false, errors.New("handshake message too short")
	}
	if data[offset] != handshakeTypeClientHello {
		return prtVer, sniPos, sniLen, false, fmt.Errorf("not a ClientHello handshake (type=%d)", data[offset])
	}
	handshakeLen := int(uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3]))
	if handshakeLen+handshakeHeaderLen > recordLen {
		return prtVer, sniPos, sniLen, false, errors.New("handshake length exceeds record length")
	}
	offset += handshakeHeaderLen

	if handshakeLen < 2+32+1 {
		return prtVer, sniPos, sniLen, false, errors.New("ClientHello too short for mandatory fields")
	}
	prtVer = data[offset : offset+2]
	offset += 2
	offset += 32
	if offset >= len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("unexpected end after Random")
	}
	sessionIDLen := int(data[offset])
	offset++
	if offset+sessionIDLen > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("session_id length exceeds data")
	}
	offset += sessionIDLen

	if offset+2 > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("cannot read cipher_suites length")
	}
	csLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+csLen > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("cipher_suites exceed data")
	}
	offset += csLen

	if offset >= len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("cannot read compression_methods length")
	}
	compMethodsLen := int(data[offset])
	offset++
	if offset+compMethodsLen > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("compression_methods exceed data")
	}
	offset += compMethodsLen

	if offset+2 > len(data) {
		return prtVer, sniPos, sniLen, false, nil
	}
	extTotalLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+extTotalLen > len(data) {
		return prtVer, sniPos, sniLen, false, errors.New("extensions length exceeds data")
	}
	extensionsEnd := offset + extTotalLen

	for offset+4 <= extensionsEnd {
		extType := binary.BigEndian.Uint16(data[offset : offset+2])
		extLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		extDataStart := offset + 4
		extDataEnd := extDataStart + extLen

		if extDataEnd > extensionsEnd {
			return prtVer, sniPos, sniLen, false, errors.New("extension length exceeds extensions block")
		}

		if extType == extTypeKeyShare {
			hasKeyShare = true
			if sniPos != -1 {
				return prtVer, sniPos, sniLen, hasKeyShare, nil
			}
		}

		if sniPos == -1 && extType == extTypeSNI {
			if extLen < 2 {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("malformed SNI extension")
			}
			listLen := int(binary.BigEndian.Uint16(data[extDataStart : extDataStart+2]))
			if listLen+2 != extLen {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("SNI list length field mismatch")
			}
			cursor := extDataStart + 2
			if cursor+3 > extDataEnd {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("SNI entry too short")
			}
			if data[cursor] != 0 {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("unsupported SNI name type")
			}
			nameLen := int(binary.BigEndian.Uint16(data[cursor+1 : cursor+3]))
			nameStart := cursor + 3
			nameEnd := nameStart + nameLen
			if nameEnd > extDataEnd {
				return prtVer, sniPos, sniLen, hasKeyShare, errors.New("SNI name length exceeds extension")
			}
			sniPos = nameStart
			sniLen = nameLen
			if hasKeyShare {
				return prtVer, sniPos, sniLen, hasKeyShare, nil
			}
		}
		offset = extDataEnd
	}
	return prtVer, sniPos, sniLen, hasKeyShare, nil
}

func SendRecords(conn net.Conn, clientHello []byte, offset, length, records, segments int, oob, oobex, modMinorVer bool, interval time.Duration) error {
	if modMinorVer && len(clientHello) >= 3 {
		clientHello[2] = 0x04
	}

	if records == 1 {
		if oobex {
			if err := SendWithOOB(conn, clientHello[:15], clientHello[15]); err != nil {
				return Wrap("oob 1", err)
			}
			if err := SendWithOOB(conn, clientHello[16:20], 0x0); err != nil {
				return Wrap("oob 2", err)
			}
			if interval > 0 {
				time.Sleep(interval)
			}
			clientHello = clientHello[20:]
		}
		if segments == 1 {
			if _, err := conn.Write(clientHello); err != nil {
				return Wrap("send remaining data", err)
			}
			return nil
		}
		leftSegments := segments / 2
		rightSegments := segments - leftSegments
		packets := make([][]byte, 0, segments)
		cut, _ := FindLastDot(clientHello, offset-20, length)
		splitAndAppend(clientHello[:cut], nil, leftSegments, &packets)
		splitAndAppend(clientHello[cut:], nil, rightSegments, &packets)
		for i, packet := range packets {
			if i == 0 && oob {
				if err := SendWithOOB(conn, packet, 0x0); err != nil {
					return Wrap("oob", err)
				}
			} else {
				if _, err := conn.Write(packet); err != nil {
					return Wrap("write packet "+strconv.Itoa(i+1), err)
				}
			}
			if interval > 0 {
				time.Sleep(interval)
			}
		}
		return nil
	}

	leftChunks := records / 2
	rightChunks := records - leftChunks
	chunks := make([][]byte, 0, records)
	cut, _ := FindLastDot(clientHello, offset, length)
	header := clientHello[:3]
	splitAndAppend(clientHello[5:cut], header, leftChunks, &chunks)
	splitAndAppend(clientHello[cut:], header, rightChunks, &chunks)

	if segments == -1 {
		for i, chunk := range chunks {
			if i == 0 {
				if oob {
					if err := SendWithOOB(conn, chunk, 0x0); err != nil {
						return Wrap("oob", err)
					}
					if interval > 0 {
						time.Sleep(interval)
					}
				} else if oobex {
					l := len(chunk)
					if err := SendWithOOB(conn, chunk[:l-1], chunk[l-1]); err != nil {
						return Wrap("oob 1", err)
					}
				}
			} else if i == 1 && oobex {
				if err := SendWithOOB(conn, chunk, 0x0); err != nil {
					return Wrap("oob 2", err)
				}
			} else {
				if _, err := conn.Write(chunk); err != nil {
					return Wrap("write record "+strconv.Itoa(i+1), err)
				}
				if interval > 0 {
					time.Sleep(interval)
				}
			}
		}
		return nil
	}

	merged := make([]byte, 0, records*5+len(clientHello))
	for _, c := range chunks {
		merged = append(merged, c...)
	}

	if oobex {
		if err := SendWithOOB(conn, merged[:15], merged[15]); err != nil {
			return Wrap("oob 1", err)
		}
		if err := SendWithOOB(conn, merged[16:20], 0x0); err != nil {
			return Wrap("oob 2", err)
		}
		if interval > 0 {
			time.Sleep(interval)
		}
		merged = merged[20:]
	}
	if segments == 1 || len(merged) <= segments {
		_, err := conn.Write(merged)
		return err
	}

	base := len(merged) / segments
	for i := 0; i < segments; i++ {
		start := i * base
		end := start + base
		if i == segments-1 {
			end = len(merged)
		}
		if i == 0 && oob {
			if err := SendWithOOB(conn, merged[start:end], 0x0); err != nil {
				return Wrap("oob", err)
			}
		} else {
			if _, err := conn.Write(merged[start:end]); err != nil {
				return Wrap("write segment "+strconv.Itoa(i+1), err)
			}
		}
		if interval > 0 {
			time.Sleep(interval)
		}
	}
	return nil
}

func splitAndAppend(data, header []byte, n int, result *[][]byte) {
	if n <= 0 {
		return
	}
	addHeader := header != nil
	if n == 1 || len(data) < n {
		if addHeader {
			*result = append(*result, makeRecord(header, data))
		} else {
			*result = append(*result, data)
		}
		return
	}
	base := len(data) / n
	for i := 0; i < n; i++ {
		var part []byte
		if i == n-1 {
			part = data[i*base:]
		} else {
			part = data[i*base : (i+1)*base]
		}
		if addHeader {
			*result = append(*result, makeRecord(header, part))
		} else {
			*result = append(*result, part)
		}
	}
}

func makeRecord(header, payload []byte) []byte {
	rec := make([]byte, 5+len(payload))
	copy(rec[:3], header)
	binary.BigEndian.PutUint16(rec[3:5], uint16(len(payload)))
	copy(rec[5:], payload)
	return rec
}

func GetRawConn(conn net.Conn) (syscall.RawConn, error) {
	rawConnProvider, ok := conn.(syscall.Conn)
	if !ok {
		return nil, errors.New("connection does not support raw access")
	}
	return rawConnProvider.SyscallConn()
}

type wrappedError struct {
	msg   string
	cause error
}

func (e *wrappedError) Error() string {
	return e.msg + ": " + e.cause.Error()
}

func (e *wrappedError) Unwrap() error {
	return e.cause
}

func Wrap(msg string, cause error) error {
	return &wrappedError{
		msg:   msg,
		cause: cause,
	}
}

func IsUseOfClosedConn(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "use of closed")
}
