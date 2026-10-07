package singtun

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// ---- 测试用 ClientHello 构造工具 ----

type testExt struct {
	typ  uint16
	data []byte
}

func buildServerNameExt(name string) testExt {
	if name == "" {
		// 空的 server_name 列表：list_len = 0
		return testExt{typ: 0x0000, data: []byte{0x00, 0x00}}
	}
	ne := len(name)
	data := []byte{
		byte((ne + 3) >> 8), byte(ne + 3), // server_name_list length
		0x00,                    // name_type = host_name
		byte(ne >> 8), byte(ne), // host_name length
	}
	data = append(data, name...)
	return testExt{typ: 0x0000, data: data}
}

// buildClientHelloBody 构造完整 handshake 消息（含 4 字节 handshake 头）。
func buildClientHelloBody(sni string, extraExts []testExt) []byte {
	var body []byte
	body = append(body, 0x03, 0x03)             // client_version TLS 1.2
	body = append(body, make([]byte, 32)...)    // random
	body = append(body, 0)                      // session_id length
	body = append(body, 0x00, 0x02, 0xc0, 0x2f) // cipher_suites
	body = append(body, 0x01, 0x00)             // compression_methods

	exts := []testExt{buildServerNameExt(sni)}
	exts = append(exts, extraExts...)
	var extBytes []byte
	for _, e := range exts {
		extBytes = append(extBytes, byte(e.typ>>8), byte(e.typ))
		extBytes = append(extBytes, byte(len(e.data)>>8), byte(len(e.data)))
		extBytes = append(extBytes, e.data...)
	}
	body = append(body, byte(len(extBytes)>>8), byte(len(extBytes)))
	body = append(body, extBytes...)

	hs := append([]byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...)
	return hs
}

// wrapRecord 加上 5 字节 TLS record 头。
func wrapRecord(payload []byte) []byte {
	rec := []byte{0x16, 0x03, 0x01, byte(len(payload) >> 8), byte(len(payload))}
	return append(rec, payload...)
}

func buildClientHelloRecord(sni string, extraExts []testExt) []byte {
	return wrapRecord(buildClientHelloBody(sni, extraExts))
}

// ---- ParseClientHelloRecord 纯函数用例（T011 要求的 8 类） ----

// 用例 1：标准 ClientHello。
func TestParseClientHelloStandard(t *testing.T) {
	record := buildClientHelloRecord("example.com", nil)
	info, err := ParseClientHelloRecord(record)
	if err != nil {
		t.Fatalf("standard ClientHello must parse, got %v", err)
	}
	if !info.IsTLS {
		t.Fatal("handshake record must set IsTLS")
	}
	if info.SNI != "example.com" {
		t.Fatalf("SNI must be example.com, got %q", info.SNI)
	}
	if info.HasECH {
		t.Fatal("standard ClientHello must not report ECH")
	}
}

// 用例 3：非握手 record（应用数据）。
func TestParseClientHelloNonHandshakeRecord(t *testing.T) {
	record := []byte{0x17, 0x03, 0x03, 0x00, 0x02, 0x00, 0x00}
	info, err := ParseClientHelloRecord(record)
	if err == nil {
		t.Fatal("non-handshake record must return an error")
	}
	if info.IsTLS {
		t.Fatal("application data record must not set IsTLS")
	}
}

// 用例 4：截断 body。
func TestParseClientHelloTruncatedBody(t *testing.T) {
	record := buildClientHelloRecord("example.com", nil)
	truncated := record[:len(record)-10]
	if _, err := ParseClientHelloRecord(truncated); err == nil {
		t.Fatal("truncated record body must return an error")
	}
}

// 用例 5：长度字段越界（extensions 长度超出 body）。
func TestParseClientHelloLengthOutOfBounds(t *testing.T) {
	body := buildClientHelloBody("example.com", nil)
	// 篡改 handshake 头里的 ClientHello 长度字段，使扩展区长度越界。
	body[1], body[2], body[3] = 0xff, 0xff, 0xff
	record := wrapRecord(body)
	info, err := ParseClientHelloRecord(record)
	if err == nil {
		t.Fatal("out-of-bounds length fields must return an error")
	}
	if info.SNI != "" {
		t.Fatalf("malformed input must not produce an SNI, got %q", info.SNI)
	}
}

// 用例 6：空 server_name 列表（合法结构，无 SNI）。
func TestParseClientHelloEmptyServerNameList(t *testing.T) {
	record := buildClientHelloRecord("", nil)
	info, err := ParseClientHelloRecord(record)
	if err != nil {
		t.Fatalf("empty server_name list is well-formed, got %v", err)
	}
	if info.SNI != "" {
		t.Fatalf("empty server_name list must yield empty SNI, got %q", info.SNI)
	}
}

// 用例 7：带 ECH 扩展——外层 SNI 仍可解析且 HasECH 置位（D4 降级匹配的数据基础）。
func TestParseClientHelloWithECH(t *testing.T) {
	record := buildClientHelloRecord("public.example", []testExt{
		{typ: 0xfe0d, data: []byte{0x01, 0x00, 0xde, 0xad}}, // encrypted_client_hello
	})
	info, err := ParseClientHelloRecord(record)
	if err != nil {
		t.Fatalf("ClientHello with ECH must parse, got %v", err)
	}
	if info.SNI != "public.example" {
		t.Fatalf("outer SNI must be parsed alongside ECH, got %q", info.SNI)
	}
	if !info.HasECH {
		t.Fatal("ECH extension must set HasECH")
	}
}

// ---- SniffClientHello 连接级用例 ----

// 用例 2：record 头与 body 分片到达（模拟 TCP 分段），
// 且已读字节必须经包装连接完整回放（US3 核心验收点）。
func TestSniffClientHelloFragmentedArrival(t *testing.T) {
	record := buildClientHelloRecord("fragmented.example", nil)
	client, server := net.Pipe()
	defer client.Close()

	go func() {
		hdr := record[:5]
		body := record[5:]
		client.Write(hdr)
		time.Sleep(50 * time.Millisecond)
		client.Write(body[:len(body)/2])
		time.Sleep(30 * time.Millisecond)
		client.Write(body[len(body)/2:])
	}()

	info, wrapped := SniffClientHello(server, 3*time.Second)
	if info.Err != nil {
		t.Fatalf("fragmented ClientHello must parse, got %v", info.Err)
	}
	if info.SNI != "fragmented.example" {
		t.Fatalf("SNI must survive fragmentation, got %q", info.SNI)
	}

	// 回放校验：包装连接必须先重放全部已消费字节。
	replayed, err := io.ReadAll(io.LimitReader(wrapped, int64(len(record))))
	if err != nil {
		t.Fatalf("read replay: %v", err)
	}
	if !bytes.Equal(replayed, record) {
		t.Fatalf("consumed bytes must be replayed verbatim, got %d bytes", len(replayed))
	}
}

// 用例 8：非 TLS 首字节（明文 HTTP）——安全返回"非 TLS"，已读字节不丢失。
func TestSniffClientHelloNonTLSFirstByte(t *testing.T) {
	request := "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"
	client, server := net.Pipe()
	defer client.Close()

	go func() {
		client.Write([]byte(request))
	}()

	info, wrapped := SniffClientHello(server, 3*time.Second)
	if info.IsTLS {
		t.Fatal("plaintext HTTP must not be identified as TLS")
	}
	if info.SNI != "" {
		t.Fatalf("non-TLS traffic must yield empty SNI, got %q", info.SNI)
	}

	replayed, err := io.ReadAll(io.LimitReader(wrapped, int64(len(request))))
	if err != nil {
		t.Fatalf("read replay: %v", err)
	}
	if string(replayed) != request {
		t.Fatal("non-TLS bytes must be replayed verbatim (connection must stay usable)")
	}
}

// 边界：ClientHello 横跨多个 TLS record（罕见但合法）——尽力重组解析。
func TestSniffClientHelloAcrossRecords(t *testing.T) {
	payload := buildClientHelloBody("span.example", nil)
	half := len(payload) / 2
	first := wrapRecord(payload[:half])
	second := wrapRecord(payload[half:])
	full := append(append([]byte{}, first...), second...)

	client, server := net.Pipe()
	defer client.Close()
	go func() {
		client.Write(first)
		client.Write(second)
	}()

	info, wrapped := SniffClientHello(server, 3*time.Second)
	if info.Err != nil {
		t.Fatalf("ClientHello spanning records must be reassembled, got %v", info.Err)
	}
	if info.SNI != "span.example" {
		t.Fatalf("reassembled ClientHello must yield SNI, got %q", info.SNI)
	}

	replayed, err := io.ReadAll(io.LimitReader(wrapped, int64(len(full))))
	if err != nil {
		t.Fatalf("read replay: %v", err)
	}
	if !bytes.Equal(replayed, full) {
		t.Fatal("all consumed record bytes must be replayed")
	}
}

// 超时（客户端迟迟不发包）：返回错误且连接仍可用，deadline 已恢复。
func TestSniffClientHelloTimeoutLeavesConnUsable(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	info, wrapped := SniffClientHello(server, 80*time.Millisecond)
	if info.Err == nil {
		t.Fatal("silent client must yield a timeout error")
	}
	if info.IsTLS || info.SNI != "" {
		t.Fatal("timeout must not fabricate TLS/SNI")
	}

	// 超时后客户端才发包：包装连接必须正常透传后续数据。
	go func() {
		time.Sleep(50 * time.Millisecond)
		client.Write([]byte("late bytes"))
	}()
	buf := make([]byte, len("late bytes"))
	_ = wrapped.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(wrapped, buf); err != nil {
		t.Fatalf("connection must stay usable after sniff timeout: %v", err)
	}
	if string(buf) != "late bytes" {
		t.Fatalf("post-timeout bytes must pass through, got %q", string(buf))
	}
}
