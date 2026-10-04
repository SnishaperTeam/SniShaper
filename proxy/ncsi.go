package proxy

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Windows decides whether an adapter has Internet access by probing NCSI
// endpoints and expecting an exact plaintext body. A TUN captures those probes
// like any other traffic; when they fail upstream (MITM rewriting, fake-ip, or
// simply no working egress) the adapter is reported as "no Internet access"
// even though the proxy itself is fine.
//
// Answering the probes locally keeps the OS status honest without sending any
// traffic, and it removes a startup-time dependency on the upstream being
// reachable.
const ncsiBody = "Microsoft NCSI"

// ncsiHosts maps a probe host to the path Windows requests on it.
var ncsiHosts = map[string]string{
	"www.msftconnecttest.com": "connecttest.txt",
	"www.msftncsi.com":        "ncsi.txt",
	"dns.msftncsi.com":        "ncsi.txt",
	"msftncsi.com":            "ncsi.txt",
}

// ncsiEnabled gates the interception. It is on by default because a wrong
// adapter status is worse than one extra locally answered request.
var ncsiEnabled = true

// isNCSIHost reports whether host is one of the Windows connectivity probes.
func isNCSIHost(host string) bool {
	_, ok := ncsiHosts[normalizeHost(host)]
	return ok
}

// handleNCSIProbe answers a probe that arrived as a normal HTTP request and
// reports whether it did.
func handleNCSIProbe(w http.ResponseWriter, req *http.Request, matchHost string) bool {
	if !ncsiEnabled || req.Method != http.MethodGet {
		return false
	}
	expected, ok := ncsiHosts[matchHost]
	if !ok {
		return false
	}
	if !strings.EqualFold(req.URL.Path, "/"+expected) {
		return false
	}
	body := ncsiBody + "\n"
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
	return true
}

// serveNCSIOverConnect answers a probe that arrived as CONNECT to port 80,
// which is how the TUN delivers plain HTTP. It completes the tunnel, replies to
// the probe and closes.
func serveNCSIOverConnect(w http.ResponseWriter, matchHost, authority string) bool {
	if !ncsiEnabled {
		return false
	}
	if _, ok := ncsiHosts[matchHost]; !ok {
		return false
	}
	if _, port, err := net.SplitHostPort(authority); err != nil || port != "80" {
		return false
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return false
	}
	clientConn, rw, err := hijacker.Hijack()
	if err != nil {
		return false
	}
	defer clientConn.Close()

	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return true
	}
	if err := rw.Flush(); err != nil {
		return true
	}

	// Read the probe request line so the client stays in step, then answer.
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := rw.ReadString('\n')
	if err != nil && line == "" {
		return true
	}
	path := "/"
	if fields := strings.Fields(line); len(fields) >= 2 {
		path = fields[1]
	}
	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		path = path[:idx]
	}
	expected := ncsiHosts[matchHost]
	if !strings.EqualFold(path, "/"+expected) {
		return true
	}

	body := ncsiBody + "\n"
	_, _ = fmt.Fprintf(rw, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nCache-Control: no-cache\r\nConnection: close\r\n\r\n%s",
		len(body), body)
	_ = rw.Flush()
	return true
}
