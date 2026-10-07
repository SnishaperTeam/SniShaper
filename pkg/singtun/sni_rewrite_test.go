package singtun

import (
	"strings"
	"testing"
)

// ---- RewriteClientHelloSNI 用例（T014：先写测试确认失败，再实现 T013） ----

// 用例 1：等长替换——新旧 SNI 字节数相同，record 结构不变。
func TestRewriteClientHelloSameLength(t *testing.T) {
	orig := buildClientHelloRecord("example.com", nil)
	rewritten, err := RewriteClientHelloSNI(orig, "bbbbbbb.com") // 等长 11 字节
	if err != nil {
		t.Fatalf("same-length rewrite failed: %v", err)
	}
	if len(rewritten) != len(orig) {
		t.Fatalf("same-length rewrite must not change record size: %d -> %d", len(orig), len(rewritten))
	}
	info, err := ParseClientHelloRecord(rewritten)
	if err != nil {
		t.Fatalf("rewritten record must re-parse cleanly: %v", err)
	}
	if info.SNI != "bbbbbbb.com" {
		t.Fatalf("expected rewritten SNI 'bbbbbbb.com', got %q", info.SNI)
	}
	if info.HasECH {
		t.Fatal("rewrite must not introduce ECH")
	}
}

// 用例 2：变长替换——变短与变长各一，所有长度字段必须重算自洽。
// 自洽性由 ParseClientHelloRecord 的全部边界校验间接保证：任何长度字段
// 没有正确 patch，重新解析都会失败。
func TestRewriteClientHelloLengthRecalc(t *testing.T) {
	cases := []struct {
		name    string
		origSNI string
		newSNI  string
	}{
		{"shrink", "www.very-long-example-domain.com", "a.co"},
		{"grow", "a.co", "www.very-long-example-domain.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := buildClientHelloRecord(tc.origSNI, nil)
			rewritten, err := RewriteClientHelloSNI(orig, tc.newSNI)
			if err != nil {
				t.Fatalf("rewrite failed: %v", err)
			}
			wantDelta := len(tc.newSNI) - len(tc.origSNI)
			if len(rewritten) != len(orig)+wantDelta {
				t.Fatalf("record size must change by %d, got %d -> %d",
					wantDelta, len(orig), len(rewritten))
			}
			info, err := ParseClientHelloRecord(rewritten)
			if err != nil {
				t.Fatalf("rewritten record must re-parse cleanly: %v", err)
			}
			if info.SNI != tc.newSNI {
				t.Fatalf("expected rewritten SNI %q, got %q", tc.newSNI, info.SNI)
			}
		})
	}
}

// 用例 3：保留其余扩展与字段——重写只动 server_name，其他扩展原样保留。
func TestRewriteClientHelloPreservesOtherExtensions(t *testing.T) {
	alpn := testExt{typ: 0x0010, data: []byte{0x00, 0x01, 0x02, 'h', '2'}}
	orig := buildClientHelloRecord("example.com", []testExt{alpn})
	rewritten, err := RewriteClientHelloSNI(orig, "other.example.org")
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	info, err := ParseClientHelloRecord(rewritten)
	if err != nil {
		t.Fatalf("rewritten record must re-parse cleanly: %v", err)
	}
	if info.SNI != "other.example.org" {
		t.Fatalf("expected rewritten SNI, got %q", info.SNI)
	}
	if !strings.Contains(string(rewritten), "h2") {
		t.Fatal("ALPN extension payload must survive the rewrite")
	}
}

// 用例 4：含 ECH 扩展的 ClientHello 必须拒绝重写（设计决策 D4）。
func TestRewriteClientHelloRejectsECH(t *testing.T) {
	ech := testExt{typ: 0xfe0d, data: []byte{0x01, 0x00, 0x02, 0x03}}
	orig := buildClientHelloRecord("outer.example.com", []testExt{ech})
	_, err := RewriteClientHelloSNI(orig, "evil.com")
	if err == nil {
		t.Fatal("rewrite of ECH ClientHello must fail (D4: outer SNI participates in matching but rewriting is skipped)")
	}
}

// 用例 5：无 server_name 扩展（空列表）必须报错，不得猜测插入位置。
func TestRewriteClientHelloRejectsMissingServerName(t *testing.T) {
	orig := buildClientHelloRecord("", nil) // 空 server_name 列表
	if _, err := ParseClientHelloRecord(orig); err != nil {
		t.Fatalf("fixture must parse (empty SNI list is legal): %v", err)
	}
	if _, err := RewriteClientHelloSNI(orig, "evil.com"); err == nil {
		t.Fatal("rewrite without server_name extension must fail")
	}
}

// 用例 6：新 SNI 合法性校验——空串 / 超长（>255）/ 含 NUL。
func TestRewriteClientHelloValidatesNewSNI(t *testing.T) {
	orig := buildClientHelloRecord("example.com", nil)
	if _, err := RewriteClientHelloSNI(orig, ""); err == nil {
		t.Fatal("empty new SNI must fail")
	}
	if _, err := RewriteClientHelloSNI(orig, strings.Repeat("a", 256)); err == nil {
		t.Fatal("new SNI over 255 bytes must fail")
	}
	if _, err := RewriteClientHelloSNI(orig, "evil\x00.com"); err == nil {
		t.Fatal("new SNI containing NUL must fail")
	}
}

// 用例 7：横跨多条 record 的 ClientHello（流式嗅探产物）不适用本重写器，
// 必须返回明确错误而非静默改坏。
func TestRewriteClientHelloRejectsMultiRecord(t *testing.T) {
	body := buildClientHelloBody("example.com", nil)
	hsLen := len(body) - 4
	first := body[:hsLen/2]
	second := body[hsLen/2:]
	multi := append(wrapRecord(first), wrapRecord(second)...)
	if _, err := RewriteClientHelloSNI(multi, "evil.com"); err == nil {
		t.Fatal("multi-record ClientHello must be rejected by the rewriter")
	}
}
