package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleNCSIProbe pins the exact reply Windows expects. The body must be
// "Microsoft NCSI" followed by a newline; anything else and the adapter keeps
// showing "no Internet access".
func TestHandleNCSIProbe(t *testing.T) {
	cases := []struct {
		host string
		path string
		want bool
	}{
		{"www.msftconnecttest.com", "/connecttest.txt", true},
		{"dns.msftncsi.com", "/ncsi.txt", true},
		{"www.msftncsi.com", "/ncsi.txt", true},
		{"www.msftconnecttest.com", "/connecttest.txt?x=1", true},
		{"www.msftconnecttest.com", "/other.txt", false},
		{"example.com", "/connecttest.txt", false},
		{"msftncsi.com", "/", false},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "http://"+tc.host+tc.path, nil)
		rec := httptest.NewRecorder()
		handled := handleNCSIProbe(rec, req, normalizeHost(tc.host))
		if handled != tc.want {
			t.Errorf("handleNCSIProbe(%s%s) = %v, want %v", tc.host, tc.path, handled, tc.want)
			continue
		}
		if !tc.want {
			continue
		}
		if rec.Code != http.StatusOK {
			t.Errorf("handleNCSIProbe(%s%s) status = %d, want 200", tc.host, tc.path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.HasPrefix(body, "Microsoft NCSI") {
			t.Errorf("handleNCSIProbe(%s%s) body = %q, want it to start with %q",
				tc.host, tc.path, body, "Microsoft NCSI")
		}
	}
}

// TestHandleNCSIProbeIgnoresNonGET makes sure the interception cannot swallow
// real traffic to those hosts.
func TestHandleNCSIProbeIgnoresNonGET(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodHead, http.MethodPut} {
		req := httptest.NewRequest(method, "http://www.msftconnecttest.com/connecttest.txt", nil)
		rec := httptest.NewRecorder()
		if handleNCSIProbe(rec, req, "www.msftconnecttest.com") {
			t.Errorf("handleNCSIProbe(%s) = true, want false", method)
		}
	}
}

func TestIsNCSIHost(t *testing.T) {
	for _, host := range []string{"www.msftconnecttest.com", "DNS.MSFTNCSI.COM", "www.msftncsi.com"} {
		if !isNCSIHost(host) {
			t.Errorf("isNCSIHost(%q) = false, want true", host)
		}
	}
	for _, host := range []string{"example.com", "microsoft.com", ""} {
		if isNCSIHost(host) {
			t.Errorf("isNCSIHost(%q) = true, want false", host)
		}
	}
}
