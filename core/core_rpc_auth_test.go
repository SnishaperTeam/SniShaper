package core

import (
	"errors"
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
