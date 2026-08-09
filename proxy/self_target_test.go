package proxy

import "testing"

func TestIsSelfTarget(t *testing.T) {
	p := &ProxyServer{listenAddr: "127.0.0.1:8080"}
	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"localhost:8080", true},
		{"http://127.0.0.1:8080/", true},
		{"127.0.0.1:8081", false},
		{"example.com:8080", false},
		{"example.com", false},
		{"127.0.0.1", false},
		{"[::1]:8080", false},
		{"127.0.0.1.:8080", true},
		{"localhost.:8080", true},
		{"LOCALHOST:8080", true},
		{"LocalHost:8080", true},
		{"127.0.0.1:8080/", true},
		{"http://localhost:8080/path?q=1", true},
		{"0.0.0.0:8080", true},
		{"127.0.0.1:80", false},
		{"localhost:443", false},
		{"[::1]", false},
	}
	for _, c := range cases {
		if got := p.isSelfTarget(c.host); got != c.want {
			t.Errorf("isSelfTarget(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestIsSelfTargetWildcardListen(t *testing.T) {
	p := &ProxyServer{listenAddr: "0.0.0.0:8080"}
	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"localhost:8080", true},
		{"0.0.0.0:8080", true},
		{"[::1]:8080", true},
		{"example.com:8080", false},
	}
	for _, c := range cases {
		if got := p.isSelfTarget(c.host); got != c.want {
			t.Errorf("isSelfTarget(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}
