package proxy

import (
	"net"

	"snishaper/pkg/tlsfrag"
)

func (p *ProxyServer) handleTLSFragment(clientConn, upstreamConn net.Conn, host string, rule Rule) {
	cfg := p.effectiveFragmentConfig(rule)
	scope := "global"
	if rule.Frag != nil {
		scope = "rule"
	}
	p.tracef("[TLS-RF] Handling %s via upstream %s (%s scope, records=%d segments=%d interval=%dms modMinorVer=%t)",
		host, rule.Upstream, scope, cfg.NumRecords, cfg.NumSegments, cfg.IntervalMillis(), cfg.ModMinorVerEnabled())

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

	interval := cfg.Interval()

	err = tlsfrag.SendRecords(
		upstreamConn,
		record,
		sniPos,
		sniLen,
		cfg.NumRecords,
		cfg.NumSegments,
		cfg.OOBEnabled(),
		cfg.OOBExEnabled(),
		cfg.ModMinorVerEnabled(),
		interval,
	)
	if err != nil {
		p.tracef("[TLS-RF] Fragmented send failed for %s: %v", host, err)
		upstreamConn.Close()
		clientConn.Close()
		return
	}

	if cfg.NumRecords <= 1 {
		p.tracef("[TLS-RF] TCP-only fragmentation: %d segment(s), SNI moved to packet 2 for %s",
			cfg.NumSegments, host)
	} else {
		p.tracef("[TLS-RF] ClientHello split into %d TLS records / %d TCP segments for %s",
			cfg.NumRecords, cfg.NumSegments, host)
	}
	p.directTunnel(clientConn, upstreamConn)
}

func (p *ProxyServer) effectiveFragmentConfig(rule Rule) FragmentConfig {
	if rule.Frag != nil {
		return normalizeFragmentConfig(*rule.Frag)
	}
	return p.rules.GetFragmentConfig()
}
