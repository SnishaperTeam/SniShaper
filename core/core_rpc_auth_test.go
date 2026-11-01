package core

import (
	"errors"
	"net"
	"testing"
)

// Ping/Shutdown 的 token 隔离：旧实例（stale token）不得把新实例的 core
// 判定为存活，更不得将其关闭。
func TestCoreRPCRequiresToken(t *testing.T) {
	stopped := false
	s := &coreService{stop: func() { stopped = true }}

	// 错误 token：Ping 返回 false（不是 error——客户端靠 Value 判定）
	var pong BoolReply
	if err := s.Ping(AuthArgs{Token: "stale-token"}, &pong); err != nil {
		t.Fatalf("Ping should not return transport error: %v", err)
	}
	if pong.Value {
		t.Fatal("Ping must reject a stale token")
	}

	// 错误 token：Shutdown 必须拒绝
	var empty EmptyArgs
	if err := s.Shutdown(AuthArgs{Token: "stale-token"}, &empty); err == nil {
		t.Fatal("Shutdown must reject a stale token")
	} else if !errors.Is(err, err) { // 拉进 errors 包并断言非 nil
		t.Fatal("unreachable")
	}
	if stopped {
		t.Fatal("Shutdown with stale token must not stop the core")
	}

	// 正确 token：Ping 通过
	if err := s.Ping(AuthArgs{Token: coreRPCToken}, &pong); err != nil || !pong.Value {
		t.Fatalf("Ping must accept the current token, err=%v value=%v", err, pong.Value)
	}

	// 正确 token：Shutdown 通过并触发 stop
	if err := s.Shutdown(AuthArgs{Token: coreRPCToken}, &empty); err != nil {
		t.Fatalf("Shutdown must accept the current token: %v", err)
	}
	if !stopped {
		t.Fatal("Shutdown with current token must stop the core")
	}
}

// handshake 在 ServeConn 之前把关所有 RPC 方法：未持有 token 的本地进程
// 连上固定端口后必须被拒。否则任何本地进程都能以 core 的管理员权限
// 拉起/关闭 TUN、停止代理、清空日志。
func runHandshake(t *testing.T, clientWrite func(net.Conn) error) bool {
	t.Helper()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	errCh := make(chan error, 1)
	go func() { errCh <- clientWrite(client) }()
	ok := verifyHandshake(server)
	if err := <-errCh; err != nil {
		t.Fatalf("client write failed: %v", err)
	}
	return ok
}

func TestCoreRPCHandshake(t *testing.T) {
	// 正确 token：announce + verify 通过
	if ok := runHandshake(t, func(c net.Conn) error { return announceToken(c, coreRPCToken) }); !ok {
		t.Fatal("handshake must accept the current token")
	}

	// 错误 token：拒绝
	if ok := runHandshake(t, func(c net.Conn) error { return announceToken(c, "stale-token") }); ok {
		t.Fatal("handshake must reject a stale token")
	}

	// 空/超长 token：announceToken 在写连接之前就拒绝
	client, _ := net.Pipe()
	defer client.Close()
	if err := announceToken(client, ""); err == nil {
		t.Fatal("announceToken must reject an empty token")
	}
	long := make([]byte, maxRPCTokenLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if err := announceToken(client, string(long)); err == nil {
		t.Fatal("announceToken must reject an oversized token")
	}

	// 畸形输入：长度字节超过上限 → 服务端拒绝
	if ok := runHandshake(t, func(c net.Conn) error {
		_, err := c.Write([]byte{byte(maxRPCTokenLen + 1)})
		return err
	}); ok {
		t.Fatal("handshake must reject an oversized length prefix")
	}

	// 畸形输入：长度字节后无内容（EOF）→ 服务端拒绝
	if ok := runHandshake(t, func(c net.Conn) error {
		if _, err := c.Write([]byte{16}); err != nil {
			return err
		}
		return c.Close() // 不再写 body：verify 侧读到 EOF，而非等到 deadline
	}); ok {
		t.Fatal("handshake must reject a truncated announcement")
	}
}
