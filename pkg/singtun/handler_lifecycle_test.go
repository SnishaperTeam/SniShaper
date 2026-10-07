package singtun

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/sagernet/sing/common/buf"

	M "github.com/sagernet/sing/common/metadata"

	"snishaper/pkg/netiface"
	"snishaper/proxy"
)

func TestIPv6EgressProbeIsCachedAndInvalidatable(t *testing.T) {
	invalidateIPv6Egress()
	if ipv6EgressDone {
		t.Fatal("invalidateIPv6Egress must clear the cached probe result")
	}

	first := ipv6EgressAvailable(netiface.Config{}, nil)
	if !ipv6EgressDone {
		t.Fatal("probe result must be cached after the first call")
	}
	second := ipv6EgressAvailable(netiface.Config{}, nil)
	if first != second {
		t.Fatalf("cached probe must be stable across calls: %t vs %t", first, second)
	}

	invalidateIPv6Egress()
	if ipv6EgressDone {
		t.Fatal("invalidateIPv6Egress must clear the cached probe result")
	}
}

func TestPackEmptyReplySetsQRBit(t *testing.T) {
	query := new(dns.Msg)
	query.SetQuestion(dns.Fqdn("example.com"), dns.TypeAAAA)

	packed, err := packEmptyReply(query)
	if err != nil {
		t.Fatalf("pack empty reply: %v", err)
	}

	parsed := new(dns.Msg)
	if err := parsed.Unpack(packed); err != nil {
		t.Fatalf("unpack empty reply: %v", err)
	}
	if !parsed.Response {
		t.Fatal("empty reply must set the QR bit, otherwise clients discard it")
	}
	if parsed.Rcode != dns.RcodeSuccess {
		t.Fatalf("empty reply must be NOERROR, got rcode %d", parsed.Rcode)
	}
	if len(parsed.Answer) != 0 {
		t.Fatalf("empty reply must carry no answers, got %d", len(parsed.Answer))
	}
}

// trackAndServe 模拟真实的转发 goroutine：阻塞在 conn 上，conn 被关闭后 untrack。
// track 增加的 WaitGroup 计数必须由它成对回收，否则 Release 内的 wg.Wait 永不返回。
func trackAndServe(t *testing.T, h *Handler, conn net.Conn) {
	t.Helper()
	if !h.track(conn) {
		return
	}
	go func() {
		defer h.untrack(conn)
		buf := make([]byte, 64)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()
}

func TestHandlerReleaseIsReusableAndCloseIsTerminal(t *testing.T) {
	h := NewHandler("127.0.0.1:1", nil, func(string) {})

	c1, p1 := net.Pipe()
	c2, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	trackAndServe(t, h, c1)
	trackAndServe(t, h, c2)

	h.Release()

	if h.closed {
		t.Fatal("Release must not mark the handler closed")
	}
	if len(h.live) != 0 {
		t.Fatalf("Release must drain live conns, got %d", len(h.live))
	}

	// Release 是非终态：回收存量连接后必须仍能接收新连接。
	c3, p3 := net.Pipe()
	defer p3.Close()
	trackAndServe(t, h, c3)

	h.Close()
	if !h.closed {
		t.Fatal("Close must mark the handler closed")
	}
	if h.track(c3) {
		t.Fatal("track must be refused after Close")
	}
}

func TestHandlerCloseDrainsConcurrentUntrack(t *testing.T) {
	h := NewHandler("127.0.0.1:1", nil, func(string) {})

	_, server := net.Pipe()
	if !h.track(server) {
		t.Fatal("track must succeed before Close")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.untrack(server)
	}()

	h.Close()
	wg.Wait()

	if len(h.live) != 0 || len(h.livePacket) != 0 {
		t.Fatalf("Close must drain all entries, live=%d packet=%d", len(h.live), len(h.livePacket))
	}
	_ = server.Close()
}

func TestHandlerInterfaceConfigConcurrentAccess(t *testing.T) {
	h := NewHandler("127.0.0.1:1", nil, func(string) {})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				h.SetInterfaceConfig(netiface.Config{})
				_ = h.interfaceConfig()
			}
		}()
	}
	wg.Wait()
}

// fakeIPTargetPacketConn 是一个不会被读取的 PacketConn 桩：
// fake-ip 目标在 forwardUDPDirect 开头就被拒绝，测试只需要一个合法实例。
type fakeIPTargetPacketConn struct {
	net.Conn
}

func (fakeIPTargetPacketConn) ReadPacket(*buf.Buffer) (M.Socksaddr, error) {
	return M.Socksaddr{}, net.ErrClosed
}
func (fakeIPTargetPacketConn) WritePacket(*buf.Buffer, M.Socksaddr) error {
	return nil
}

func TestForwardUDPDirectClosesSessionOnFakeIP(t *testing.T) {
	h := NewHandler("127.0.0.1:1", nil, func(string) {})

	destination := M.SocksaddrFrom(netip.MustParseAddr("198.18.0.9"), 443)
	if !h.fakeIP.Contains(destination.Addr) {
		t.Fatal("address inside the fake-ip range must be recognised as fake")
	}

	raw, peer := net.Pipe()
	defer peer.Close()
	conn := fakeIPTargetPacketConn{Conn: raw}

	closed := make(chan struct{})
	var once sync.Once
	h.forwardUDPDirect(context.Background(), conn,
		M.SocksaddrFrom(netip.MustParseAddr("198.18.0.1"), 1234),
		destination, func(error) {
			once.Do(func() { close(closed) })
		})

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("forwardUDPDirect must invoke onClose for fake-ip targets")
	}
}

func TestRouteExcludePrefixesDropsSelfOverlap(t *testing.T) {
	cfg := proxy.TUNConfig{
		RouteExcludeAddresses: []string{"198.18.0.0/16", "10.0.0.0/8"},
	}

	got := routeExcludePrefixes(cfg, false, nil)
	self := netip.MustParsePrefix(fakeIPv4Prefix)
	for _, prefix := range got {
		if prefix.Overlaps(self) {
			t.Fatalf("route exclude %s overlaps the TUN prefix and must be dropped", prefix)
		}
	}

	var kept bool
	for _, prefix := range got {
		if prefix.String() == "10.0.0.0/8" {
			kept = true
		}
	}
	if !kept {
		t.Fatalf("unrelated route excludes must be preserved, got %v", got)
	}
}

func TestRouteExcludePrefixesDropsSelfOverlapIPv6(t *testing.T) {
	cfg := proxy.TUNConfig{
		RouteExcludeAddresses: []string{"fd65:198:18::/64", "2001:db8::/32"},
	}

	got := routeExcludePrefixes(cfg, true, nil)
	self := netip.MustParsePrefix(fakeIPv6Prefix)
	for _, prefix := range got {
		if prefix.Overlaps(self) {
			t.Fatalf("route exclude %s overlaps the TUN prefix and must be dropped", prefix)
		}
	}
	if len(got) != 1 || got[0].String() != "2001:db8::/32" {
		t.Fatalf("unrelated IPv6 route excludes must be preserved, got %v", got)
	}
}

func TestCloseWithTimeoutWaitsForShutdownToFinish(t *testing.T) {
	var finished atomic.Bool
	release := make(chan struct{})
	returned := make(chan closeOutcome, 1)

	go func() {
		returned <- closeWithTimeout("test", func() error {
			<-release
			finished.Store(true)
			return nil
		}, 50*time.Millisecond, 5*time.Second, func(string) {})
	}()

	select {
	case <-returned:
		t.Fatal("closeWithTimeout returned before shutdown completed")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)

	select {
	case outcome := <-returned:
		if outcome != closeLate {
			t.Fatal("closeWithTimeout must report the timeout it observed")
		}
		if !finished.Load() {
			t.Fatal("closeWithTimeout must wait for the in-flight shutdown to finish")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("closeWithTimeout did not return after shutdown finished")
	}
}

func TestCloseWithTimeoutReturnsImmediatelyOnFastShutdown(t *testing.T) {
	done := make(chan closeOutcome, 1)
	go func() {
		done <- closeWithTimeout("test", func() error { return nil }, 5*time.Second, 30*time.Second, func(string) {})
	}()

	select {
	case outcome := <-done:
		if outcome != closeClean {
			t.Fatal("a shutdown that finishes within the timeout must not be reported as timed out")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("closeWithTimeout did not return for a fast shutdown")
	}
}

// TestCloseWithTimeoutGivesUpAtHardCap 验证 D6：shutdown 挂死时，
// closeWithTimeout 在 timeout+hardCap 到顶后必须返回 closeLeaked，
// 不再无限期等待（这是旧实现整机冻结的根因）。
func TestCloseWithTimeoutGivesUpAtHardCap(t *testing.T) {
	started := make(chan struct{})
	done := make(chan closeOutcome, 1)
	go func() {
		done <- closeWithTimeout("test", func() error {
			close(started)
			select {} // 永久挂死，模拟 wintun Close 卡住
		}, 30*time.Millisecond, 100*time.Millisecond, func(string) {})
	}()
	<-started

	select {
	case outcome := <-done:
		if outcome != closeLeaked {
			t.Fatalf("expected closeLeaked when shutdown never returns, got %v", outcome)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("closeWithTimeout must give up at the hard cap instead of blocking forever")
	}
}
