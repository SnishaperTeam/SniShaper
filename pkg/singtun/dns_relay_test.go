package singtun

import (
	"context"
	"io"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
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

	go func() {
		buf := make([]byte, 64)
		for {
			if _, err := client.Read(buf); err != nil {
				handler.untrack(client)
				return
			}
		}
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

func TestServeDNSOverStreamAnswersQuery(t *testing.T) {
	handler := NewHandler("127.0.0.1:1", nil, func(string) {})
	defer handler.Close()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	destination := M.SocksaddrFrom(netip.MustParseAddr("198.18.0.2"), 53)
	source := M.SocksaddrFrom(netip.MustParseAddr("198.18.0.1"), 40000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler.serveDNSOverStream(ctx, server, source, destination, nil)

	msg := new(dns.Msg)
	msg.Id = 4321
	msg.RecursionDesired = true
	msg.Question = []dns.Question{{Name: "stream.example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}
	query, err := msg.Pack()
	if err != nil {
		t.Fatal(err)
	}

	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := client.Write([]byte{0, byte(len(query))}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Write(query); err != nil {
		t.Fatal(err)
	}

	hdr := make([]byte, 2)
	if _, err := io.ReadFull(client, hdr); err != nil {
		t.Fatal(err)
	}
	respLen := int(hdr[0])<<8 | int(hdr[1])
	if respLen == 0 {
		t.Fatal("empty DNS over TCP response")
	}
	respPayload := make([]byte, respLen)
	if _, err := io.ReadFull(client, respPayload); err != nil {
		t.Fatal(err)
	}

	resp := new(dns.Msg)
	if err := resp.Unpack(respPayload); err != nil {
		t.Fatal(err)
	}
	if !resp.Response {
		t.Fatal("QR bit not set in DNS over TCP response")
	}
	if resp.Id != msg.Id {
		t.Fatalf("response id mismatch: %d != %d", resp.Id, msg.Id)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected exactly one fake-ip answer, got %d", len(resp.Answer))
	}
	if resp.Answer[0].Header().Name != "stream.example.com." {
		t.Fatalf("unexpected answer name: %s", resp.Answer[0].Header().Name)
	}
	answer, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A answer, got type %d", resp.Answer[0].Header().Rrtype)
	}
	addr, valid := netip.AddrFromSlice(answer.A)
	if !valid {
		t.Fatalf("answer %s is not a valid address", answer.A)
	}
	if !handler.fakeIP.Contains(addr) {
		t.Fatalf("answer %s is outside the fake-ip range", addr)
	}
	if answer.A.Equal(net.ParseIP("198.18.0.2")) {
		t.Fatal("fake-ip must not collide with the TUN DNS address")
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
