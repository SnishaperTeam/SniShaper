package proxy

import (
	"net"

	"snishaper/pkg/tlsfrag"
)

func (p *ProxyServer) handleTLSFragment(clientConn, upstreamConn net.Conn, host string, rule Rule) {
	p.tracef("[TLS-RF] Handling %s via upstream %s", host, rule.Upstream)

	record, err := tlsfrag.ReadInitialTLSRecord(clientConn)
	if err != nil {
		p.tracef("[TLS-RF] Failed to read initial TLS record for %s: %v", host, err)
		clientConn.Close()
		upstreamConn.Close()
		return
	}

	_, sniPos, sniLen, _, err := tlsfrag.ParseClientHello(record)
	if err != nil {
		p.tracef("[TLS-RF] Parse ClientHello failed for %s: %v", host, err)
		clientConn.Close()
		upstreamConn.Close()
		return
	}

	if sniPos <= 0 || sniLen <= 0 {
		if _, err := upstreamConn.Write(record); err != nil {
			p.tracef("[TLS-RF] Initial passthrough write failed for %s: %v", host, err)
			clientConn.Close()
			upstreamConn.Close()
			return
		}
		p.tracef("[TLS-RF] No SNI in ClientHello for %s, forwarded directly", host)
		p.directTunnel(clientConn, upstreamConn)
		return
	}

	err = tlsfrag.SendRecords(
		upstreamConn,
		record,
		sniPos,
		sniLen,
		tlsfrag.DefaultTLSRFNumRecords,
		tlsfrag.DefaultTLSRFNumSegments,
		tlsfrag.DefaultTLSRFOOB,
		tlsfrag.DefaultTLSRFOOBEx,
		tlsfrag.DefaultTLSRFModMinorVer,
		tlsfrag.DefaultTLSRFSendInterval,
	)
	if err != nil {
		p.tracef("[TLS-RF] Fragmented send failed for %s: %v", host, err)
		upstreamConn.Close()
		clientConn.Close()
		return
	}

	p.tracef("[TLS-RF] ClientHello sent in original-style fragments for %s", host)
	p.directTunnel(clientConn, upstreamConn)
}
