package proxy

import (
	"encoding/json"
	"testing"
)

func TestEffectiveFragmentConfig_RuleOverridesGlobal(t *testing.T) {
	global := normalizeFragmentConfig(FragmentConfig{
		NumRecords:  4,
		NumSegments: 1,
		IntervalMs:  intPtr(50),
		ModMinorVer: boolPtr(true),
	})

	ruleLevel := normalizeFragmentConfig(FragmentConfig{
		NumRecords:  1,
		NumSegments: 2,
		IntervalMs:  intPtr(80),
		ModMinorVer: boolPtr(false),
	})

	if ruleLevel.NumRecords != 1 || ruleLevel.NumSegments != 2 || ruleLevel.IntervalMillis() != 80 {
		t.Fatalf("rule scope not applied: %+v", ruleLevel)
	}
	if ruleLevel.ModMinorVerEnabled() {
		t.Error("rule scope mod_minor_ver=false must win over global true")
	}
	if fragEqual(ruleLevel, global) {
		t.Fatal("rule scope unexpectedly equals global")
	}
}

func TestSiteGroupFrag_JSONRoundTrip(t *testing.T) {
	raw := `{"id":"sg-1","name":"github","mode":"tls-rf","domains":["github.com"],
	          "frag":{"num_records":1,"num_segments":2,"interval_ms":80,"mod_minor_ver":false,"oob":false,"oob_ex":false}}`
	var sg SiteGroup
	if err := json.Unmarshal([]byte(raw), &sg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sg.Frag == nil {
		t.Fatal("frag must be decoded")
	}
	cfg := normalizeFragmentConfig(*sg.Frag)
	if cfg.NumRecords != 1 || cfg.NumSegments != 2 || cfg.IntervalMillis() != 80 || cfg.ModMinorVerEnabled() {
		t.Fatalf("frag lost values: %+v", cfg)
	}

	raw2 := `{"id":"sg-2","name":"gh2","mode":"tls-rf","domains":["gist.github.com"]}`
	var sg2 SiteGroup
	if err := json.Unmarshal([]byte(raw2), &sg2); err != nil {
		t.Fatalf("unmarshal 2: %v", err)
	}
	if sg2.Frag != nil {
		t.Fatalf("absent frag must stay nil, got %+v", sg2.Frag)
	}

	out, err := json.Marshal(sg2)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if contains(string(out), `"frag"`) {
		t.Errorf("nil frag must be omitted, got %s", string(out))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
