package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

// The previous oneConnListener returned EOF on the second Accept immediately.
// http.Server.Serve then returned and callers closed the conn before the
// handler could serve the request — which is exactly why QUIC mode broke.
func TestOneConnListenerServesRequestBeforeClose(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()

	var (
		handlerSaw sync.WaitGroup
		gotPath    string
		mu         sync.Mutex
	)
	handlerSaw.Add(1)

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			gotPath = r.URL.Path
			mu.Unlock()
			// Simulate a slow upstream H3 fetch.
			time.Sleep(150 * time.Millisecond)
			w.Header().Set("X-Test", "ok")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "hello-quic")
			handlerSaw.Done()
		}),
	}

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve(newOneConnListener(server))
	}()

	// Give Serve a moment to Accept the connection.
	time.Sleep(20 * time.Millisecond)

	req := "GET /quic-path HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(client, req); err != nil {
		t.Fatalf("client write: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("client read response: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if string(body) != "hello-quic" {
		t.Fatalf("body = %q, want hello-quic", body)
	}
	if resp.Header.Get("X-Test") != "ok" {
		t.Fatalf("missing X-Test header")
	}

	handlerSaw.Wait()
	mu.Lock()
	path := gotPath
	mu.Unlock()
	if path != "/quic-path" {
		t.Fatalf("handler path = %q, want /quic-path", path)
	}

	// Closing the client should let Serve return cleanly.
	_ = client.Close()
	select {
	case err := <-serveDone:
		if err != nil {
			t.Logf("Serve returned: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after client close")
	}
}

func TestIsHopByHopHeader(t *testing.T) {
	cases := map[string]bool{
		"Connection":        true,
		"keep-alive":        true,
		"Transfer-Encoding": true,
		"Proxy-Connection":  true,
		"Content-Type":      false,
		"User-Agent":        false,
	}
	for name, want := range cases {
		if got := isHopByHopHeader(name); got != want {
			t.Fatalf("isHopByHopHeader(%q)=%v want %v", name, got, want)
		}
	}
}
