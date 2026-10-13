package singtun

import (
	"fmt"
	"io"
	"net"
	"time"
)

// SNIInfo 是一次 ClientHello 嗅探的结果（US3 数据模型）。
//
// 设计决策 D2：本模块是 singtun 内自包含的 TLS 解析器，不扩展 pkg/tlsfrag
// （tlsfrag 面向分片偏移，本模块面向结构化 SNI/ECH 信息，消费者不同）。
type SNIInfo struct {
	IsTLS  bool   // 首字节是否 TLS handshake record
	SNI    string // 外层 server_name（可空；ECH 场景即外层 SNI，D4 降级用）
	HasECH bool   // 是否存在 ECH（encrypted_client_hello）扩展
	Record []byte // 已消费的全部 TLS record 字节（重写注入与回放用）
	Err    error  // 畸形/读取失败原因（任何输入不 panic）
}

const (
	tlsRecordHeaderLen          = 5
	tlsHandshakeTypeClientHello = 0x01
	extServerName               = 0x0000
	extEncryptedClientHello     = 0xfe0d  // ECH draft-13+；存在时 inner SNI 加密不可见
	maxClientHelloRecord        = 1 << 14 // 单条 TLS record 明文上限
	maxHandshakeTotal           = 1 << 16 // ClientHello 跨 record 重组的总字节上限
)

// ParseClientHelloRecord 解析单条完整 TLS record（含 5 字节 record 头），纯函数。
// 畸形输入（非握手 record / 截断 / 长度越界 / 空 server_name 列表）返回错误
// 而非 panic（US3 验收场景 3）。ClientHello 横跨多条 record 时返回明确错误，
// 由 SniffClientHello 的流式路径负责重组。
func ParseClientHelloRecord(record []byte) (SNIInfo, error) {
	var info SNIInfo
	if len(record) < tlsRecordHeaderLen {
		return info, fmt.Errorf("record too short: %d bytes", len(record))
	}
	if record[0] != 0x16 {
		return info, fmt.Errorf("not a handshake record: type 0x%02x", record[0])
	}
	info.IsTLS = true
	bodyLen := int(record[3])<<8 | int(record[4])
	if bodyLen < 4 {
		return info, fmt.Errorf("record body too short: %d bytes", bodyLen)
	}
	if len(record) < tlsRecordHeaderLen+bodyLen {
		return info, fmt.Errorf("record truncated: header declares %d body bytes, got %d",
			bodyLen, len(record)-tlsRecordHeaderLen)
	}
	body := record[tlsRecordHeaderLen : tlsRecordHeaderLen+bodyLen]
	if body[0] != tlsHandshakeTypeClientHello {
		return info, fmt.Errorf("not a ClientHello: handshake type 0x%02x", body[0])
	}
	hsLen := int(body[1])<<16 | int(body[2])<<8 | int(body[3])
	if hsLen > bodyLen-4 {
		return info, fmt.Errorf("handshake message spans multiple records (%d > %d), use the streaming sniffer", hsLen, bodyLen-4)
	}
	sni, hasECH, err := parseClientHelloBody(body[4 : 4+hsLen])
	info.SNI = sni
	info.HasECH = hasECH
	return info, err
}

// SniffClientHello 从连接读取首个 ClientHello 并解析 SNI（US3）。
//
// 关键保证：已消费的字节通过返回的 prefixConn 完整回放——嗅探对连接数据
// 零丢失（旧内联实现经 bufio 消费后不回放，ClientHello 会丢给上游）。
// 超时 / 非 TLS / 畸形：返回对应 SNIInfo（IsTLS=false 或 Err 非空），
// 返回的连接仍然可用；读超时在返回前恢复（清零 deadline）。
// ClientHello 横跨多条 record 时继续读取重组（尽力解析，总长度有上限）。
func SniffClientHello(conn net.Conn, timeout time.Duration) (SNIInfo, net.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	info, consumed := readClientHelloStream(conn)
	info.Record = consumed
	return info, &prefixConn{Conn: conn, prefix: consumed}
}

// readClientHelloStream 从连接流式读取并解析 ClientHello，返回结果与
// 全部已消费字节（即使中途失败，已读部分也用于回放，不丢失客户端数据）。
func readClientHelloStream(conn net.Conn) (SNIInfo, []byte) {
	var info SNIInfo
	consumed := make([]byte, 0, 1024)

	// readFull 在读取的同时记录已消费字节，供回放。
	readFull := func(dst []byte) error {
		n, err := io.ReadFull(conn, dst)
		consumed = append(consumed, dst[:n]...)
		return err
	}

	hdr := make([]byte, tlsRecordHeaderLen)
	if err := readFull(hdr); err != nil {
		info.Err = fmt.Errorf("read record header: %w", err)
		return info, consumed
	}
	if hdr[0] != 0x16 {
		// 非 TLS handshake record（明文 HTTP 等）：安全返回"非 TLS"。
		info.Err = fmt.Errorf("not a handshake record: type 0x%02x", hdr[0])
		return info, consumed
	}
	info.IsTLS = true

	bodyLen := int(hdr[3])<<8 | int(hdr[4])
	if bodyLen < 4 || bodyLen > maxClientHelloRecord {
		info.Err = fmt.Errorf("record body length out of range: %d", bodyLen)
		return info, consumed
	}
	body := make([]byte, bodyLen)
	if err := readFull(body); err != nil {
		info.Err = fmt.Errorf("read record body: %w", err)
		return info, consumed
	}
	if body[0] != tlsHandshakeTypeClientHello {
		info.Err = fmt.Errorf("not a ClientHello: handshake type 0x%02x", body[0])
		return info, consumed
	}

	hsLen := int(body[1])<<16 | int(body[2])<<8 | int(body[3])
	if hsLen > bodyLen-4 {
		// ClientHello 横跨多条 TLS record（罕见但合法，spec 边界条款）：
		// 继续读取后续 record 重组完整 handshake 消息，尽力解析。
		if hsLen > maxHandshakeTotal {
			info.Err = fmt.Errorf("handshake message too large: %d bytes", hsLen)
			return info, consumed
		}
		remaining := hsLen - (bodyLen - 4)
		for remaining > 0 {
			rh := make([]byte, tlsRecordHeaderLen)
			if err := readFull(rh); err != nil {
				info.Err = fmt.Errorf("read continuation record header: %w", err)
				return info, consumed
			}
			if rh[0] != 0x16 {
				info.Err = fmt.Errorf("expected continuation handshake record, got type 0x%02x", rh[0])
				return info, consumed
			}
			rl := int(rh[3])<<8 | int(rh[4])
			if rl <= 0 || rl > maxClientHelloRecord {
				info.Err = fmt.Errorf("continuation record length out of range: %d", rl)
				return info, consumed
			}
			frag := make([]byte, rl)
			if err := readFull(frag); err != nil {
				info.Err = fmt.Errorf("read continuation record body: %w", err)
				return info, consumed
			}
			body = append(body, frag...)
			remaining -= rl
		}
	}

	sni, hasECH, err := parseClientHelloBody(body[4 : 4+hsLen])
	info.SNI = sni
	info.HasECH = hasECH
	info.Err = err
	return info, consumed
}

// parseClientHelloBody 解析 ClientHello 消息体（handshake 头之后），
// 提取 server_name 与 ECH 存在标记。全部边界长度校验，畸形返回错误不 panic。
func parseClientHelloBody(ch []byte) (sni string, hasECH bool, err error) {
	// client_version(2) + random(32)
	p := 2 + 32
	if p >= len(ch) {
		return "", false, fmt.Errorf("client hello truncated before session id")
	}
	sidLen := int(ch[p])
	p++
	if p+sidLen > len(ch) {
		return "", false, fmt.Errorf("client hello truncated in session id")
	}
	p += sidLen
	if p+2 > len(ch) {
		return "", false, fmt.Errorf("client hello truncated before cipher suites")
	}
	cipherLen := int(ch[p])<<8 | int(ch[p+1])
	p += 2
	if cipherLen == 0 || p+cipherLen > len(ch) {
		return "", false, fmt.Errorf("invalid cipher suites length: %d", cipherLen)
	}
	p += cipherLen
	if p >= len(ch) {
		return "", false, fmt.Errorf("client hello truncated before compression methods")
	}
	compLen := int(ch[p])
	p++
	if p+compLen > len(ch) {
		return "", false, fmt.Errorf("client hello truncated in compression methods")
	}
	p += compLen
	if p+2 > len(ch) {
		// 无扩展区：合法的最小 ClientHello，无 SNI。
		return "", false, nil
	}
	extLen := int(ch[p])<<8 | int(ch[p+1])
	p += 2
	if p+extLen > len(ch) {
		return "", false, fmt.Errorf("client hello extensions length out of bounds: %d", extLen)
	}
	end := p + extLen
	for p+4 <= end {
		extType := int(ch[p])<<8 | int(ch[p+1])
		extDataLen := int(ch[p+2])<<8 | int(ch[p+3])
		p += 4
		if p+extDataLen > end {
			return "", false, fmt.Errorf("extension 0x%04x length out of bounds: %d", extType, extDataLen)
		}
		data := ch[p : p+extDataLen]
		switch extType {
		case extServerName:
			s, perr := parseServerNameExt(data)
			if perr != nil {
				return "", false, perr
			}
			if sni == "" {
				sni = s
			}
		case extEncryptedClientHello:
			hasECH = true
		}
		p += extDataLen
	}
	return sni, hasECH, nil
}

// parseServerNameExt 解析 server_name 扩展数据，返回首个 host_name 条目。
// 空列表或非 host_name 类型返回空串（合法输入）；结构截断/越界返回错误。
func parseServerNameExt(data []byte) (string, error) {
	if len(data) < 2 {
		return "", fmt.Errorf("server_name extension too short")
	}
	listLen := int(data[0])<<8 | int(data[1])
	if listLen == 0 {
		return "", nil
	}
	if len(data) < 3 {
		return "", fmt.Errorf("server_name list truncated")
	}
	if data[2] != 0 {
		return "", nil // 非 host_name 类型：忽略
	}
	if len(data) < 5 {
		return "", fmt.Errorf("server_name entry truncated")
	}
	nameLen := int(data[3])<<8 | int(data[4])
	if 5+nameLen > len(data) {
		return "", fmt.Errorf("server_name length out of bounds: %d", nameLen)
	}
	return string(data[5 : 5+nameLen]), nil
}

// prefixConn 先回放注入的前缀字节，耗尽后透传底层连接（US3/US4）。
// 嗅探场景 prefix 为已消费的原始字节（零丢失回放）；重写场景 prefix 为
// 重写后的 ClientHello record——原始字节已被嗅探消费，不会重复到达上游。
type prefixConn struct {
	net.Conn
	prefix []byte
}

func (c *prefixConn) Read(p []byte) (int, error) {
	if len(c.prefix) > 0 {
		n := copy(p, c.prefix)
		c.prefix = c.prefix[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}
