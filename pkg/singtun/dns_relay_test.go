package singtun

import (
	"context"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"snishaper/pkg/netiface"
)

func TestShouldHijackDNS(t *testing.T) {
	cases := []struct {
		addr string
		port uint16
		want bool
	}{
		{"198.18.0.2", 53, true},
		{"fd65:198:18::2", 53, true},
		{"8.8.8.8", 53, true},
		{"223.5.5.5", 53, true},
		{"1.1.1.1", 53, true},
		{"198.18.0.2", 80, false},
		{"23.53.12.5", 443, false},
		{"23.53.12.5", 3478, false},
	}
	for _, c := range cases {
		destination := M.SocksaddrFrom(netip.MustParseAddr(c.addr), c.port)
		if got := shouldHijackDNS(destination); got != c.want {
			t.Fatalf("shouldHijackDNS(%s:%d) = %v, want %v", c.addr, c.port, got, c.want)
		}
	}
}

func TestHandlerTrackRefusesAfterClose(t *testing.T) {
	handler := NewHandler("127.0.0.1:1", nil, func(string) {})
	handler.Close()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	if handler.track(client) {
		t.Fatal("track must fail once the handler is closed")
	}
}

func TestHandlerTrackPacketRefusesAfterClose(t *testing.T) {
	handler := NewHandler("127.0.0.1:1", nil, func(string) {})
	handler.Close()

	if handler.trackPacketConn(&blockingPacketConn{closed: make(chan struct{})}) {
		t.Fatal("trackPacketConn must fail once the handler is closed")
	}
}

func TestHandlerCloseUnblocksTrackedConn(t *testing.T) {
	handler := NewHandler("127.0.0.1:1", nil, func(string) {})
	defer handler.Close()

	client, server := net.Pipe()
	defer server.Close()
	if !handler.track(client) {
		t.Fatal("track must succeed before Close")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.Close()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return; a tracked conn kept it blocked")
	}
}

func TestServeDNSOverPacketConnExitsOnClose(t *testing.T) {
	handler := NewHandler("127.0.0.1:1", nil, func(string) {})
	defer handler.Close()

	conn := newBlockingPacketConn()
	if !handler.trackPacketConn(conn) {
		t.Fatal("trackPacketConn must succeed before the relay starts")
	}

	destination := M.SocksaddrFrom(netip.MustParseAddr("198.18.0.2"), 53)
	source := M.SocksaddrFrom(netip.MustParseAddr("198.18.0.1"), 40000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler.serveDNSOverPacketConn(ctx, conn, source, destination, nil)
	settleGoroutines()
	before := runtime.NumGoroutine()

	_ = conn.Close()
	settleGoroutines()
	after := runtime.NumGoroutine()

	if after > before {
		t.Fatalf("DNS relay goroutine did not exit: %d -> %d", before, after)
	}
}

func TestServeDNSOverPacketConnCycleDoesNotLeak(t *testing.T) {
	handler := NewHandler("127.0.0.1:1", nil, func(string) {})
	defer handler.Close()

	destination := M.SocksaddrFrom(netip.MustParseAddr("198.18.0.2"), 53)
	source := M.SocksaddrFrom(netip.MustParseAddr("198.18.0.1"), 40000)

	settleGoroutines()
	before := runtime.NumGoroutine()

	for i := 0; i < 50; i++ {
		conn := newBlockingPacketConn()
		ctx, cancel := context.WithCancel(context.Background())
		handler.serveDNSOverPacketConn(ctx, conn, source, destination, nil)
		_ = conn.Close()
		cancel()
	}

	settleGoroutines()
	after := runtime.NumGoroutine()

	if after > before+6 {
		t.Fatalf("goroutines grew from %d to %d across 50 DNS relay cycles", before, after)
	}
}

func TestInterfaceConfigIsRaceFree(t *testing.T) {
	handler := NewHandler("127.0.0.1:1", nil, func(string) {})
	defer handler.Close()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				handler.SetInterfaceConfig(netifaceTestConfig())
				_ = handler.interfaceConfig()
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// blockingPacketConn blocks in ReadPacket until Close, mirroring a real gVisor
// packet conn sitting idle inside a session.
type blockingPacketConn struct {
	closed   chan struct{}
	closeOne sync.Once
}

func newBlockingPacketConn() *blockingPacketConn {
	return &blockingPacketConn{closed: make(chan struct{})}
}

func (c *blockingPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	<-c.closed
	return M.Socksaddr{}, net.ErrClosed
}

func (c *blockingPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	buffer.Release()
	return nil
}

func (c *blockingPacketConn) Close() error {
	c.closeOne.Do(func() { close(c.closed) })
	return nil
}

func (c *blockingPacketConn) LocalAddr() net.Addr { return &net.UDPAddr{} }

func (c *blockingPacketConn) SetDeadline(t time.Time) error { return nil }

func (c *blockingPacketConn) SetReadDeadline(t time.Time) error { return nil }

func (c *blockingPacketConn) SetWriteDeadline(t time.Time) error { return nil }

var _ N.PacketConn = (*blockingPacketConn)(nil)

func settleGoroutines() {
	for i := 0; i < 5; i++ {
		runtime.Gosched()
		time.Sleep(20 * time.Millisecond)
	}
}

func netifaceTestConfig() netiface.Config {
	return netiface.Config{ForceInterface: "test-nic"}
}
