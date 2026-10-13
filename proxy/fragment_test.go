package proxy

import (
	"encoding/json"
	"testing"
	"time"

	"snishaper/pkg/tlsfrag"
)

func jsonMarshalSettingsFragment(v any) ([]byte, error) { return json.Marshal(v) }
func jsonUnmarshalSettingsFragment(b []byte) (FragmentConfig, error) {
	var out FragmentConfig
	err := json.Unmarshal(b, &out)
	return out, err
}

func boolPtr(v bool) *bool { return &v }

func intPtr(v int) *int { return &v }

func fragEqual(a, b FragmentConfig) bool {
	return a.NumRecords == b.NumRecords &&
		a.NumSegments == b.NumSegments &&
		a.IntervalMillis() == b.IntervalMillis() &&
		a.ModMinorVerEnabled() == b.ModMinorVerEnabled() &&
		a.OOBEnabled() == b.OOBEnabled() &&
		a.OOBExEnabled() == b.OOBExEnabled()
}

func TestNormalizeFragmentConfig_MissingSectionKeepsLegacyPresets(t *testing.T) {
	got := normalizeFragmentConfig(FragmentConfig{})
	want := FragmentConfig{
		NumRecords:  tlsfrag.DefaultTLSRFNumRecords,  // 4
		NumSegments: tlsfrag.DefaultTLSRFNumSegments, // 1
		IntervalMs:  intPtr(int(tlsfrag.DefaultTLSRFSendInterval / time.Millisecond)),
		ModMinorVer: boolPtr(tlsfrag.DefaultTLSRFModMinorVer),
		OOB:         boolPtr(tlsfrag.DefaultTLSRFOOB),
		OOBEx:       boolPtr(tlsfrag.DefaultTLSRFOOBEx),
	}
	if !fragEqual(got, want) {
		t.Fatalf("missing-section mismatch:\n got records=%d segments=%d interval=%d minorVer=%t oob=%t oobex=%t",
			got.NumRecords, got.NumSegments, got.IntervalMillis(),
			got.ModMinorVerEnabled(), got.OOBEnabled(), got.OOBExEnabled())
	}
	if got.NumRecords != 4 || got.NumSegments != 1 || got.IntervalMillis() != 50 {
		t.Errorf("legacy preset drifted: records=%d segments=%d interval=%d", got.NumRecords, got.NumSegments, got.IntervalMs)
	}
	if !got.ModMinorVerEnabled() {
		t.Error("modMinorVer must default to true (legacy behaviour)")
	}
}

func TestNormalizeFragmentConfig_ClampsOutOfRange(t *testing.T) {
	cases := []struct {
		name       string
		in         FragmentConfig
		wantRecord int
		wantSeg    int
		wantInt    int
	}{
		{"records too high", FragmentConfig{NumRecords: 999, NumSegments: 2, IntervalMs: intPtr(50)}, fragmentRecordsMax, 2, 50},
		{"records negative falls back to legacy 4", FragmentConfig{NumRecords: -5, NumSegments: 2, IntervalMs: intPtr(50)}, tlsfrag.DefaultTLSRFNumRecords, 2, 50},
		{"segments too high", FragmentConfig{NumRecords: 1, NumSegments: 999, IntervalMs: intPtr(50)}, 1, fragmentSegmentsMax, 50},
		{"segments negative falls back to legacy 1", FragmentConfig{NumRecords: 1, NumSegments: -1, IntervalMs: intPtr(50)}, 1, tlsfrag.DefaultTLSRFNumSegments, 50},
		{"interval negative clamps to 0", FragmentConfig{NumRecords: 1, NumSegments: 2, IntervalMs: intPtr(-10)}, 1, 2, 0},
		{"interval too high", FragmentConfig{NumRecords: 1, NumSegments: 2, IntervalMs: intPtr(100000)}, 1, 2, fragmentIntervalMax},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeFragmentConfig(c.in)
			if got.NumRecords != c.wantRecord || got.NumSegments != c.wantSeg || got.IntervalMillis() != c.wantInt {
				t.Fatalf("\n got records=%d segments=%d interval=%d\nwant records=%d segments=%d interval=%d",
					got.NumRecords, got.NumSegments, got.IntervalMillis(), c.wantRecord, c.wantSeg, c.wantInt)
			}
		})
	}
}

func TestNormalizeFragmentConfig_DistinguishesMissingFromFalse(t *testing.T) {
	missing := normalizeFragmentConfig(FragmentConfig{NumRecords: 1, NumSegments: 2})
	if !missing.ModMinorVerEnabled() {
		t.Error("missing mod_minor_ver must fall back to default true")
	}

	explicitOff := normalizeFragmentConfig(FragmentConfig{
		NumRecords: 1, NumSegments: 2, ModMinorVer: boolPtr(false),
	})
	if explicitOff.ModMinorVerEnabled() {
		t.Error("explicit mod_minor_ver=false must be honoured")
	}

	explicitOn := normalizeFragmentConfig(FragmentConfig{
		NumRecords: 1, NumSegments: 2, ModMinorVer: boolPtr(true),
	})
	if !explicitOn.ModMinorVerEnabled() {
		t.Error("explicit mod_minor_ver=true must be honoured")
	}
}

func TestNormalizeFragmentConfig_FillsNilBooleans(t *testing.T) {
	got := normalizeFragmentConfig(FragmentConfig{NumRecords: 1, NumSegments: 2})
	if got.ModMinorVer == nil || got.OOB == nil || got.OOBEx == nil {
		t.Fatalf("normalize left nil booleans: minorVer=%v oob=%v oobex=%v", got.ModMinorVer, got.OOB, got.OOBEx)
	}
	if got.OOBEnabled() || got.OOBExEnabled() {
		t.Error("OOB/OOBEx must default to false")
	}
}

func TestDefaultFragmentConfig(t *testing.T) {
	got := DefaultFragmentConfig()
	if got.NumRecords != 4 || got.NumSegments != 1 || got.IntervalMillis() != 50 || !got.ModMinorVerEnabled() {
		t.Fatalf("unexpected factory defaults: records=%d segments=%d interval=%d minorVer=%t",
			got.NumRecords, got.NumSegments, got.IntervalMillis(), got.ModMinorVerEnabled())
	}
}

func TestFragmentConfig_JSONTags(t *testing.T) {
	raw := `{"num_records":1,"num_segments":2,"interval_ms":80,"mod_minor_ver":false,"oob":true,"oob_ex":true}`
	var cfg FragmentConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg = normalizeFragmentConfig(cfg)
	if cfg.NumRecords != 1 || cfg.NumSegments != 2 || cfg.IntervalMillis() != 80 {
		t.Fatalf("numeric fields mis-decoded: %+v", cfg)
	}
	if cfg.ModMinorVerEnabled() {
		t.Error("mod_minor_ver=false must survive decode")
	}
	if !cfg.OOBEnabled() || !cfg.OOBExEnabled() {
		t.Error("oob / oob_ex true must survive decode")
	}
}

func TestFragmentConfig_RoundTripPreservesValues(t *testing.T) {
	in := normalizeFragmentConfig(FragmentConfig{
		NumRecords:  1,
		NumSegments: 2,
		IntervalMs:  intPtr(80),
		ModMinorVer: boolPtr(false),
	})
	data, err := jsonMarshalSettingsFragment(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := jsonUnmarshalSettingsFragment(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out = normalizeFragmentConfig(out)
	if !fragEqual(in, out) {
		t.Fatalf("round trip lost values: in=%+v out=%+v", in, out)
	}
}
