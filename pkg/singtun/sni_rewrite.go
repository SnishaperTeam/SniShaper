package singtun

import (
	"errors"
	"fmt"
)

// RewriteClientHelloSNI 将单条完整 TLS record 中的 server_name 替换为
// newSNI，并重算所有受影响的长度字段（US4）。
//
// 设计决策 D4：含 ECH 扩展的 ClientHello 拒绝重写——outer SNI 仅用于规则
// 匹配，改写外层 SNI 会破坏 ECH 的 HPKE 封装一致性，直接跳过。
// 输入必须是单条完整 record（SniffClientHello 的多 record 重组产物不适用，
// 调用方据错误跳过重写）。
func RewriteClientHelloSNI(record []byte, newSNI string) ([]byte, error) {
	if newSNI == "" {
		return nil, errors.New("rewrite refused: new SNI is empty")
	}
	if len(newSNI) > 255 {
		return nil, fmt.Errorf("rewrite refused: new SNI too long (%d > 255)", len(newSNI))
	}
	for i := 0; i < len(newSNI); i++ {
		if newSNI[i] == 0 {
			return nil, errors.New("rewrite refused: new SNI contains NUL byte")
		}
	}

	// 先经完整解析校验结构（截断/越界/非 ClientHello/多 record 在此拦截）。
	info, err := ParseClientHelloRecord(record)
	if err != nil {
		return nil, fmt.Errorf("rewrite refused: %w", err)
	}
	if info.HasECH {
		return nil, errors.New("rewrite refused: ECH extension present (D4: match on outer SNI, skip rewrite)")
	}

	// 定位 server_name 条目的各长度字段偏移（record 已通过解析校验，
	// 此处按 TLS 结构遍历，无需重复边界检查）。
	hsLen := int(record[6])<<16 | int(record[7])<<8 | int(record[8])
	ch := record[9 : 9+hsLen]
	p := 2 + 32 // client_version + random
	p += 1 + int(ch[p])
	p += 2 + (int(ch[p])<<8 | int(ch[p+1])) // cipher_suites
	p += 1 + int(ch[p])                   // compression_methods
	extBlockLenOff := 9 + p               // extensions 总长字段（2 字节）的绝对偏移
	extLen := int(ch[p])<<8 | int(ch[p+1])
	p += 2
	end := p + extLen

	var nameOff, nameLen, extDataOff int
	found := false
	for p+4 <= end {
		extType := int(ch[p])<<8 | int(ch[p+1])
		extDataLen := int(ch[p+2])<<8 | int(ch[p+3])
		p += 4
		if extType == extServerName {
			data := ch[p : p+extDataLen]
			if len(data) < 5 || data[2] != 0x00 || int(data[0])<<8|int(data[1]) == 0 {
				return nil, errors.New("rewrite refused: no host_name entry in server_name list")
			}
			extDataOff = 9 + p
			nameOff = extDataOff + 5
			nameLen = int(data[3])<<8 | int(data[4])
			found = true
		}
		p += extDataLen
	}
	if !found {
		return nil, errors.New("rewrite refused: no server_name extension")
	}

	// splice：替换 name 字节。所有长度字段（record len 2B / handshake len 3B /
	// extensions len 2B / ext data len 2B / list len 2B / name len 2B）都在
	// name 之前，统一按 delta 补偿即可自洽。
	delta := len(newSNI) - nameLen
	out := make([]byte, 0, len(record)+delta)
	out = append(out, record[:nameOff]...)
	out = append(out, newSNI...)
	out = append(out, record[nameOff+nameLen:]...)

	patch16(out, 3, delta)              // record body length
	patch24(out, 6, delta)              // handshake message length
	patch16(out, extBlockLenOff, delta) // extensions block length
	patch16(out, extDataOff-2, delta)   // server_name ext data length
	patch16(out, extDataOff, delta)     // server_name_list length
	patch16(out, extDataOff+3, delta)   // host_name length
	return out, nil
}

// patch16 给 out[off:off+2] 的 16 位大端长度字段加上 delta（就地修改）。
func patch16(out []byte, off int, delta int) {
	v := (int(out[off])<<8 | int(out[off+1])) + delta
	out[off] = byte(v >> 8)
	out[off+1] = byte(v)
}

// patch24 给 out[off:off+3] 的 24 位大端长度字段加上 delta（就地修改）。
func patch24(out []byte, off int, delta int) {
	v := (int(out[off])<<16 | int(out[off+1])<<8 | int(out[off+2])) + delta
	out[off] = byte(v >> 16)
	out[off+1] = byte(v >> 8)
	out[off+2] = byte(v)
}
