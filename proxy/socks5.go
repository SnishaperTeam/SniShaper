package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/things-go/go-socks5"
	"github.com/things-go/go-socks5/statute"
)

type socks5HijackConn struct {
	net.Conn
	reader io.Reader
	writer io.Writer
}

func (c *socks5HijackConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

func (c *socks5HijackConn) Write(b []byte) (int, error) {
	return c.writer.Write(b)
}

type socks5Hijacker struct {
	conn *socks5HijackConn
}

func (h *socks5Hijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rw := bufio.NewReadWriter(
		bufio.NewReader(h.conn),
		bufio.NewWriter(h.conn),
	)
	return h.conn, rw, nil
}

type socks5ResponseWriter struct {
	hijacker    *socks5Hijacker
	w           io.Writer
	req         *socks5.Request
	header      map[string][]string
	wroteHeader bool
}

func (w *socks5ResponseWriter) Header() map[string][]string {
	return w.header
}

func (w *socks5ResponseWriter) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

func (w *socks5ResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader && statusCode == 200 {
		_, _ = w.w.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		w.wroteHeader = true
	}
}

func (w *socks5ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijacker.Hijack()
}

func (p *ProxyServer) newSocks5Server() *socks5.Server {
	return socks5.NewServer(
		socks5.WithLogger(socks5.NewLogger(log.New(&socks5LogWriter{proxy: p}, "[SOCKS5] ", log.LstdFlags))),
		socks5.WithConnectHandle(p.handleSocks5Connect),
	)
}

func (p *ProxyServer) handleSocks5Connect(ctx context.Context, writer io.Writer, req *socks5.Request) error {
	host := req.RawDestAddr.FQDN
	if host == "" {
		host = req.RawDestAddr.IP.String()
	}
	port := req.RawDestAddr.Port

	p.tracef("[SOCKS5] CMD=CONNECT target=%s:%d", host, port)

	targetAddr := fmt.Sprintf("%s:%d", host, port)

	matchHost := normalizeHost(host)
	mode := p.GetMode()
	rule := p.rules.matchRule(matchHost, mode)
	if rule.SiteID != "" {
		p.rules.incrementRuleHit(rule.SiteID)
	}

	clientConn := p.socks5Tracker.getConn(req.RemoteAddr.String())
	if clientConn == nil {
		p.tracef("[SOCKS5] no underlying connection found")
		socks5.SendReply(writer, statute.RepHostUnreachable, req.LocalAddr)
		return fmt.Errorf("no underlying connection")
	}

	cr := p.prepareConnect(matchHost, targetAddr, rule)

	if cr.effectiveMode == "direct" {
		p.tracef("[SOCKS5] direct mode, connecting directly")
		conn, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
		if err != nil {
			p.tracef("[SOCKS5] direct connect failed: %v", err)
			socks5.SendReply(writer, statute.RepHostUnreachable, req.LocalAddr)
			return err
		}
		defer conn.Close()
		socks5.SendReply(writer, statute.RepSuccess, req.LocalAddr)
		p.directTunnel(clientConn, conn)
		return nil
	}

	if cr.effectiveMode == "server" {
		p.tracef("[SOCKS5] server mode")
		socks5.SendReply(writer, statute.RepSuccess, req.LocalAddr)
		hijackConn := &socks5HijackConn{
			Conn:   clientConn,
			reader: req.Reader,
			writer: writer,
		}
		_ = hijackConn.SetDeadline(time.Time{})
		p.handleServerMITM(hijackConn, cr.targetHost, cr.rule)
		return nil
	}

	if cr.effectiveMode == "quic" {
		p.tracef("[SOCKS5] quic mode")
		socks5.SendReply(writer, statute.RepSuccess, req.LocalAddr)
		hijackConn := &socks5HijackConn{
			Conn:   clientConn,
			reader: req.Reader,
			writer: writer,
		}
		_ = hijackConn.SetDeadline(time.Time{})
		p.handleQUICMITM(hijackConn, cr.targetHost, cr.rule)
		return nil
	}

	if err := p.dialUpstream(cr); err != nil {
		p.tracef("[SOCKS5] upstream connect failed: %v", err)
		socks5.SendReply(writer, statute.RepHostUnreachable, req.LocalAddr)
		return err
	}
	defer cr.conn.Close()

	p.tracef("[SOCKS5] sending success reply")
	socks5.SendReply(writer, statute.RepSuccess, req.LocalAddr)

	p.tracef("[SOCKS5] starting tunnel")

	switch cr.effectiveMode {
	case "mitm":
		hijackConn := &socks5HijackConn{
			Conn:   clientConn,
			reader: req.Reader,
			writer: writer,
		}
		_ = hijackConn.SetDeadline(time.Time{})
		p.handleMITM(hijackConn, cr.targetHost, cr.rule, cr.dialCandidates, cr.dialAddr)
	case "tls-rf":
		hijackConn := &socks5HijackConn{
			Conn:   clientConn,
			reader: req.Reader,
			writer: writer,
		}
		_ = hijackConn.SetDeadline(time.Time{})
		p.handleTLSFragment(hijackConn, cr.conn, cr.targetHost, cr.rule)
	default:
		p.handleTransparent(clientConn, cr.conn, cr.targetHost, cr.rule)
	}

	return nil
}

type socks5LogWriter struct {
	proxy *ProxyServer
}

func (w *socks5LogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		w.proxy.tracef("[SOCKS5-LIB] %s", msg)
	}
	return len(p), nil
}
