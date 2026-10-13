package tlsfrag

import (
	"bytes"
	"net"
	"testing"
	"time"
)

type recorderConn struct {
	writes [][]byte
}

func (c *recorderConn) Write(b []byte) (int, error) {
	cp := make([]byte, len(b))
	copy(cp, b)
	c.writes = append(c.writes, cp)
	return len(b), nil
}

func (c *recorderConn) Read(b []byte) (int, error)         { return 0, nil }
func (c *recorderConn) Close() error                       { return nil }
func (c *recorderConn) LocalAddr() net.Addr                { return nil }
func (c *recorderConn) RemoteAddr() net.Addr               { return nil }
func (c *recorderConn) SetDeadline(t time.Time) error      { return nil }
func (c *recorderConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *recorderConn) SetWriteDeadline(t time.Time) error { return nil }

func buildClientHelloForTest(host string) []byte {
	name := []byte(host)

	var list []byte
	list = append(list, 0x00)
	list = append(list, byte(len(name)>>8), byte(len(name)))
	list = append(list, name...)

	var inner []byte
	inner = append(inner, byte(len(list)>>8), byte(len(list)))
	inner = append(inner, list...)

	var sniExt []byte
	sniExt = append(sniExt, 0x00, 0x00)
	sniExt = append(sniExt, byte(len(inner)>>8), byte(len(inner)))
	sniExt = append(sniExt, inner...)

	var ext []byte
	ext = append(ext, sniExt...)
	ext = append(ext, 0x00, 0x0a, 0x00, 0x04, 0x00, 0x02, 0x00, 0x1d)
	ext = append(ext, 0x00, 0x0b, 0x00, 0x02, 0x01, 0x00)

	var hs []byte
	hs = append(hs, 0x03, 0x03)
	hs = append(hs, make([]byte, 32)...) // random
	hs = append(hs, 0x00)                // session id
	hs = append(hs, 0x00, 0x02, 0xc0, 0x2f)
	hs = append(hs, 0x01, 0x00)
	hs = append(hs, byte(len(ext)>>8), byte(len(ext)))
	hs = append(hs, ext...)

	var hf []byte
	hf = append(hf, 0x01, byte(len(hs)>>16), byte(len(hs)>>8), byte(len(hs)))
	hf = append(hf, hs...)

	var rec []byte
	rec = append(rec, 0x16, 0x03, 0x01, byte(len(hf)>>8), byte(len(hf)))
	rec = append(rec, hf...)
	return rec
}

func TestSendRecords_TCPOnly_MovesSNIOutOfFirstSegment(t *testing.T) {
	for _, host := range []string{"github.com", "gist.github.com", "raw.githubusercontent.com", "a.io"} {
		t.Run(host, func(t *testing.T) {
			ch := buildClientHelloForTest(host)
			_, sniPos, sniLen, _, err := ParseClientHello(ch)
			if err != nil {
				t.Fatalf("ParseClientHello: %v", err)
			}
			if sniPos <= 0 || sniLen <= 0 {
				t.Fatalf("SNI not parsed: pos=%d len=%d", sniPos, sniLen)
			}

			rec := &recorderConn{}
			if err := SendRecords(rec, ch, sniPos, sniLen, 1, 2, false, false, false, 0); err != nil {
				t.Fatalf("SendRecords: %v", err)
			}

			if len(rec.writes) < 2 {
				t.Fatalf("expected >=2 TCP writes, got %d", len(rec.writes))
			}
			if bytes.Contains(rec.writes[0], []byte(host)) {
				t.Errorf("SNI %q leaked into first TCP segment (%d bytes)", host, len(rec.writes[0]))
			}
			var joined []byte
			for _, w := range rec.writes {
				joined = append(joined, w...)
			}
			if !bytes.Equal(joined, ch) {
				t.Errorf("reassembled bytes differ from original:\n got %d bytes, want %d", len(joined), len(ch))
			}
		})
	}
}

func TestSendRecords_TCPOnly_RespectsSegmentCount(t *testing.T) {
	host := "github.com"
	ch := buildClientHelloForTest(host)
	_, sniPos, sniLen, _, err := ParseClientHello(ch)
	if err != nil {
		t.Fatalf("ParseClientHello: %v", err)
	}

	for _, want := range []int{2, 3, 4, 8} {
		rec := &recorderConn{}
		if err := SendRecords(rec, ch, sniPos, sniLen, 1, want, false, false, false, 0); err != nil {
			t.Fatalf("SendRecords(segments=%d): %v", want, err)
		}
		if len(rec.writes) != want {
			t.Errorf("segments=%d: got %d writes", want, len(rec.writes))
		}
		if bytes.Contains(rec.writes[0], []byte(host)) {
			t.Errorf("segments=%d: SNI leaked into first write", want)
		}
	}
}

func TestSendRecords_ModMinorVer_Toggle(t *testing.T) {
	host := "github.com"

	build := func() []byte {
		ch := buildClientHelloForTest(host)
		_, sniPos, sniLen, _, err := ParseClientHello(ch)
		if err != nil {
			t.Fatalf("ParseClientHello: %v", err)
		}
		rec := &recorderConn{}
		if err := SendRecords(rec, ch, sniPos, sniLen, 1, 2, false, false, true, 0); err != nil {
			t.Fatalf("SendRecords: %v", err)
		}
		return rec.writes[0]
	}

	on := build()
	if on[2] != 0x04 {
		t.Errorf("modMinorVer=true: record minor ver = 0x%02x, want 0x04", on[2])
	}

	ch := buildClientHelloForTest(host)
	_, sniPos, sniLen, _, err := ParseClientHello(ch)
	if err != nil {
		t.Fatalf("ParseClientHello: %v", err)
	}
	rec := &recorderConn{}
	if err := SendRecords(rec, ch, sniPos, sniLen, 1, 2, false, false, false, 0); err != nil {
		t.Fatalf("SendRecords: %v", err)
	}
	if rec.writes[0][2] != 0x01 {
		t.Errorf("modMinorVer=false: record minor ver = 0x%02x, want 0x01", rec.writes[0][2])
	}
}

func TestSendRecords_SingleSegment_SendsAllAtOnce(t *testing.T) {
	host := "github.com"
	ch := buildClientHelloForTest(host)
	_, sniPos, sniLen, _, err := ParseClientHello(ch)
	if err != nil {
		t.Fatalf("ParseClientHello: %v", err)
	}
	rec := &recorderConn{}
	if err := SendRecords(rec, ch, sniPos, sniLen, 1, 1, false, false, false, 0); err != nil {
		t.Fatalf("SendRecords: %v", err)
	}
	if len(rec.writes) != 1 {
		t.Fatalf("segments=1: got %d writes, want 1", len(rec.writes))
	}
	if !bytes.Equal(rec.writes[0], ch) {
		t.Error("segments=1: payload differs from original")
	}
}
