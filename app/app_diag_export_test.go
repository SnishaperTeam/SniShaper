package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactSensitiveText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"url userinfo", "https://user:secret@doh.example/dns-query", "https://" + redactValueMask + "@doh.example/dns-query"},
		{"query api_key", "https://a.io/x?api_key=abc123&b=1", "https://a.io/x?api_key=" + redactValueMask + "&b=1"},
		{"query token", "fetch?token=tok&x=2", "fetch?token=" + redactValueMask + "&x=2"},
		{"bearer", "Authorization: Bearer eyJhbGciOi", "Authorization: Bearer " + redactValueMask},
		{"plain host untouched", "proxy set to 127.0.0.1:8080 ok", "proxy set to 127.0.0.1:8080 ok"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := redactSensitiveText(c.in); got != c.want {
				t.Fatalf("redactSensitiveText(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSanitizeJSONBytes(t *testing.T) {
	raw := `{
		"cloudflare": {"api_key": "cf-secret-123", "auto_update": true, "preferred_ips": ["1.1.1.1"]},
		"ip_pools": [{"name": "cloudfront", "api_key": "pool-secret"}],
		"dns": {"url": "https://user:pass@dns.example/query"},
		"port": 8080,
		"list": ["https://a:b@x.io/p", "plain.example.com"]
	}`
	out := sanitizeJSONBytes([]byte(raw))
	s := string(out)

	if strings.Contains(s, "cf-secret-123") || strings.Contains(s, "pool-secret") {
		t.Fatalf("api keys leaked: %s", s)
	}
	if strings.Contains(s, "user:pass") || strings.Contains(s, "a:b@") {
		t.Fatalf("url credentials leaked: %s", s)
	}
	if !strings.Contains(s, "1.1.1.1") || !strings.Contains(s, "8080") || !strings.Contains(s, "plain.example.com") {
		t.Fatalf("non-sensitive values lost: %s", s)
	}
	// The sanitized output must still be valid JSON.
	var reparsed map[string]interface{}
	if err := json.Unmarshal(out, &reparsed); err != nil {
		t.Fatalf("sanitized output is not valid JSON: %v", err)
	}
}

func TestSanitizeJSONBytesCorruptInput(t *testing.T) {
	raw := `not json at all https://user:pw@evil.io/`
	s := string(sanitizeJSONBytes([]byte(raw)))
	if strings.Contains(s, "user:pw") {
		t.Fatalf("corrupt input not redacted: %s", s)
	}
}

func TestSanitizeJSONBytesRedactsMaskValueKey(t *testing.T) {
	// A key matching "token" with a non-empty string must become the mask,
	// even when nested inside arrays of objects.
	raw := `{"nodes":[{"access_token":"abc","name":"n1"}]}`
	s := string(sanitizeJSONBytes([]byte(raw)))
	if strings.Contains(s, "abc") {
		t.Fatalf("nested token leaked: %s", s)
	}
	if !strings.Contains(s, "n1") {
		t.Fatalf("sibling value lost: %s", s)
	}
}
