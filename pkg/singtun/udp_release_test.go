package singtun

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// idlePacketConn 模拟真实 gVisor packet conn 的空闲行为：
// 它不会因为「上游无数据」而报错，只按 read deadline 返回超时。
// 因此中继循环只会走 "Timeout -> continue" 这条路，不会自然退出
// —— 这正是线上 908 个 UDP socket 永不释放的形态。
type idlePacketConn struct {
	closeCount atomic.Int32
	closed     chan struct{}
}

func newIdlePacketConn() *idlePacketConn {
	return &idlePacketConn{closed: make(chan struct{})}
}

// ReadPacket 在 deadline 到期时返回超时错误（不是关闭错误）。
func (c *idlePacketConn) ReadPacket(*buf.Buffer) (M.Socksaddr, error) {
	select {
	case <-c.closed:
		return M.Socksaddr{}, net.ErrClosed
	case <-time.After(15 * time.Millisecond):
		return M.Socksaddr{}, timeoutError{}
	}
}

func (c *idlePacketConn) WritePacket(b *buf.Buffer, _ M.Socksaddr) error {
	b.Release()
	return nil
}

func (c *idlePacketConn) Close() error {
	c.closeCount.Add(1)
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return nil
}

func (c *idlePacketConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (c *idlePacketConn) SetDeadline(time.Time) error      { return nil }
func (c *idlePacketConn) SetReadDeadline(time.Time) error  { return nil }
func (c *idlePacketConn) SetWriteDeadline(time.Time) error { return nil }

var _ N.PacketConn = (*idlePacketConn)(nil)

// timeoutError 实现 net.Error 且 Timeout()==true，
// 与真实 UDP / gVisor 的 deadline 行为一致。
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// idlePeer 是上游侧 socket 的假实现：同样只超时、不报错。
type idlePeer struct{ closed atomic.Int32 }

func (p *idlePeer) ReadFrom([]byte) (int, net.Addr, error) {
	time.Sleep(15 * time.Millisecond)
	return 0, nil, timeoutError{}
}

func (p *idlePeer) WriteTo(b []byte, _ net.Addr) (int, error) { return len(b), nil }

func (p *idlePeer) SetReadDeadline(time.Time) error { return nil }

var _ udpRelayPeer = (*idlePeer)(nil)

// TestRelayUDPReleasesIdleSession 断言：两侧都长期静默时，中继必须在
// 空闲 TTL 后自行返回（从而让调用方的 defer 关闭两个 socket）。
//
// 这是针对线上「908 个 UDP socket 永不释放」的回归测试。
// 未加空闲看门狗前，relayUDP 会永久阻塞，本测试必然超时失败。
func TestRelayUDPReleasesIdleSession(t *testing.T) {
	// 调小 TTL，避免测试等待真实的 120s。
	prev := udpIdleSessionTTL.Load()
	udpIdleSessionTTL.Store(int64(2 * time.Second))
	defer udpIdleSessionTTL.Store(prev)

	h := NewHandler("127.0.0.1:1", nil, func(string) {})
	conn := newIdlePacketConn()
	peer := &idlePeer{}

	ctx := context.Background() // 复刻 manager.go 的 Background ctx（线上永不 cancel）
	dest := M.SocksaddrFrom(netip.MustParseAddr("1.1.1.1"), 443)
	destAddr := &net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 443}

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.relayUDP(ctx, conn, peer, dest, destAddr)
	}()

	select {
	case <-done:
		// 中继已自行退出 —— 预期行为
	case <-time.After(20 * time.Second):
		t.Fatal("relayUDP 未在空闲 TTL 后退出：两个转发 goroutine 永久滞留，" +
			"其本地 UDP socket 永不释放（线上 908 个 socket 泄漏的根因）")
	}
}

// TestRelayUDPExitsOnContextCancel 断言 ctx 取消仍是有效出口，
// 避免新增看门狗后破坏原有的取消语义。
func TestRelayUDPExitsOnContextCancel(t *testing.T) {
	h := NewHandler("127.0.0.1:1", nil, func(string) {})
	conn := newIdlePacketConn()
	peer := &idlePeer{}

	ctx, cancel := context.WithCancel(context.Background())
	dest := M.SocksaddrFrom(netip.MustParseAddr("1.1.1.1"), 443)
	destAddr := &net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 443}

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.relayUDP(ctx, conn, peer, dest, destAddr)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("ctx 取消后 relayUDP 未退出")
	}
}
