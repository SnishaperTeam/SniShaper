package proxy

import (
	"bytes"
	"io"
	"net"
	"time"

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

	// Save original ClientHello for potential fallback (sendRecords modifies in-place)
	hasFallback := rule.FallbackMode != ""
	var savedRecord []byte
	if hasFallback {
		savedRecord = make([]byte, len(record))
		copy(savedRecord, record)
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

		if hasFallback {
			p.handleTLSRFFallback(clientConn, host, rule, savedRecord)
			return
		}
		clientConn.Close()
		return
	}

	// If fallback is configured, probe that upstream is alive before tunneling.
	// GFW typically RSTs the connection after seeing fragmented ClientHello.
	if hasFallback {
		_ = upstreamConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		probe := make([]byte, 1)
		_, probeErr := upstreamConn.Read(probe)
		_ = upstreamConn.SetReadDeadline(time.Time{})

		if probeErr != nil {
			p.tracef("[TLS-RF] Upstream probe failed for %s: %v; trying fallback %s", host, probeErr, rule.FallbackMode)
			upstreamConn.Close()
			p.handleTLSRFFallback(clientConn, host, rule, savedRecord)
			return
		}
		// Prepend the probed byte back into the read stream
		wrappedUp := &bufferedReadConn{
			Conn:   upstreamConn,
			reader: io.MultiReader(bytes.NewReader(probe), upstreamConn),
		}
		p.tracef("[TLS-RF] ClientHello OK for %s", host)
		p.directTunnel(clientConn, wrappedUp)
		return
	}

	p.tracef("[TLS-RF] ClientHello sent in original-style fragments for %s", host)
	p.directTunnel(clientConn, upstreamConn)
}

func (p *ProxyServer) handleTLSRFFallback(clientConn net.Conn, host string, rule Rule, originalRecord []byte) {
	p.tracef("[TLS-RF] Fallback via %s for %s", rule.FallbackMode, host)

	targetAddr := net.JoinHostPort(host, "443")

	newConn, err := DialFallback(rule.FallbackMode, targetAddr, "")
	if err != nil {
		p.tracef("[TLS-RF] Fallback %s dial failed for %s: %v", rule.FallbackMode, host, err)
		clientConn.Close()
		return
	}

	// Send original ClientHello un-fragmented through the protected transport
	if _, err := newConn.Write(originalRecord); err != nil {
		p.tracef("[TLS-RF] Fallback write ClientHello failed for %s: %v", host, err)
		newConn.Close()
		clientConn.Close()
		return
	}

	p.tracef("[TLS-RF] Fallback %s succeeded for %s", rule.FallbackMode, host)
	p.directTunnel(clientConn, newConn)
}
