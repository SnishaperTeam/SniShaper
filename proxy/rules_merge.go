package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type RulesMergeSummary struct {
	AddedGroups      int      `json:"added_groups"`
	UpdatedGroups    int      `json:"updated_groups"`
	KeptGroups       int      `json:"kept_groups"`
	AddedUpstreams   int      `json:"added_upstreams"`
	UpdatedUpstreams int      `json:"updated_upstreams"`
	KeptUpstreams    int      `json:"kept_upstreams"`
	AddedDNSNodes    int      `json:"added_dns_nodes"`
	AddedECHProfiles int      `json:"added_ech_profiles"`
	AddedNAT64       int      `json:"added_nat64_profiles"`
	Details          []string `json:"details,omitempty"`
}

func (s RulesMergeSummary) Changed() bool {
	return s.AddedGroups+s.UpdatedGroups+s.AddedUpstreams+s.UpdatedUpstreams+
		s.AddedDNSNodes+s.AddedECHProfiles+s.AddedNAT64 > 0
}

type rulesSyncState struct {
	RemoteHash string            `json:"remote_hash"`
	Groups     map[string]string `json:"groups,omitempty"`
	Upstreams  map[string]string `json:"upstreams,omitempty"`
}

func sameJSON(a, b interface{}) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

func jsonFingerprint(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return dataHash(data)
}

func (rm *RuleManager) syncStatePath() string {
	return filepath.Join(filepath.Dir(rm.rulesPath), "rules_remote_sync.json")
}

func (rm *RuleManager) loadSyncState() rulesSyncState {
	state := rulesSyncState{}
	data, err := os.ReadFile(rm.syncStatePath())
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return rulesSyncState{}
	}
	if state.Groups == nil {
		state.Groups = map[string]string{}
	}
	if state.Upstreams == nil {
		state.Upstreams = map[string]string{}
	}
	return state
}

func (rm *RuleManager) saveSyncState(state rulesSyncState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rm.syncStatePath(), data, 0644)
}

// RemoteRulesHash reports the hash of the rules file revision that was applied
// last, so callers can skip the whole merge when nothing changed upstream.
func (rm *RuleManager) RemoteRulesHash() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.loadSyncState().RemoteHash
}

// MergeRemoteRules folds a remote rules file into the local set three-way: the
// last applied upstream revision is the baseline, so a rule is only replaced
// when the local copy is untouched, upstream changed it and local edits win.
// Rules that only exist locally are never touched and nothing is ever deleted.
func (rm *RuleManager) MergeRemoteRules(remote RulesConfig, remoteHash string) (RulesMergeSummary, error) {
	var sum RulesMergeSummary

	rm.mu.Lock()
	defer rm.mu.Unlock()

	baseline := rm.loadSyncState()
	next := rulesSyncState{
		RemoteHash: remoteHash,
		Groups:     make(map[string]string, len(remote.SiteGroups)),
		Upstreams:  make(map[string]string, len(remote.Upstreams)),
	}

	groupIndex := make(map[string]int, len(rm.siteGroups))
	for i := range rm.siteGroups {
		groupIndex[rm.siteGroups[i].ID] = i
	}
	for _, g := range remote.SiteGroups {
		if g.ID == "" {
			continue
		}
		fingerprint := jsonFingerprint(g)
		next.Groups[g.ID] = fingerprint

		i, ok := groupIndex[g.ID]
		if !ok {
			rm.siteGroups = append(rm.siteGroups, g)
			groupIndex[g.ID] = len(rm.siteGroups) - 1
			sum.AddedGroups++
			sum.Details = append(sum.Details, "added site group "+g.ID+" ("+g.Name+")")
			continue
		}
		if sameJSON(rm.siteGroups[i], g) {
			continue
		}
		previous, known := baseline.Groups[g.ID]
		if known && previous != jsonFingerprint(rm.siteGroups[i]) {
			sum.KeptGroups++
			sum.Details = append(sum.Details, "kept local site group "+g.ID+" ("+g.Name+")")
			continue
		}
		rm.siteGroups[i] = g
		sum.UpdatedGroups++
		sum.Details = append(sum.Details, "updated site group "+g.ID+" ("+g.Name+")")
	}

	upstreamIndex := make(map[string]int, len(rm.upstreams))
	for i := range rm.upstreams {
		upstreamIndex[rm.upstreams[i].ID] = i
	}
	for _, u := range remote.Upstreams {
		if u.ID == "" {
			continue
		}
		fingerprint := jsonFingerprint(u)
		next.Upstreams[u.ID] = fingerprint

		i, ok := upstreamIndex[u.ID]
		if !ok {
			rm.upstreams = append(rm.upstreams, u)
			upstreamIndex[u.ID] = len(rm.upstreams) - 1
			sum.AddedUpstreams++
			sum.Details = append(sum.Details, "added upstream "+u.ID)
			continue
		}
		if sameJSON(rm.upstreams[i], u) {
			continue
		}
		previous, known := baseline.Upstreams[u.ID]
		if known && previous != jsonFingerprint(rm.upstreams[i]) {
			sum.KeptUpstreams++
			sum.Details = append(sum.Details, "kept local upstream "+u.ID)
			continue
		}
		rm.upstreams[i] = u
		sum.UpdatedUpstreams++
		sum.Details = append(sum.Details, "updated upstream "+u.ID)
	}

	dnsSeen := make(map[string]bool, len(rm.dnsNodes))
	for i := range rm.dnsNodes {
		dnsSeen[rm.dnsNodes[i].ID] = true
	}
	for _, n := range remote.DNSNodes {
		if n.ID == "" || dnsSeen[n.ID] {
			continue
		}
		rm.dnsNodes = append(rm.dnsNodes, n)
		sum.AddedDNSNodes++
		sum.Details = append(sum.Details, "added DNS node "+n.ID)
	}

	echSeen := make(map[string]bool, len(rm.echProfiles))
	for i := range rm.echProfiles {
		echSeen[rm.echProfiles[i].ID] = true
	}
	for _, p := range remote.ECHProfiles {
		if p.ID == "" || echSeen[p.ID] {
			continue
		}
		rm.echProfiles = append(rm.echProfiles, p)
		sum.AddedECHProfiles++
		sum.Details = append(sum.Details, "added ECH profile "+p.ID)
	}

	natSeen := make(map[string]bool, len(rm.nat64Profiles))
	for i := range rm.nat64Profiles {
		natSeen[rm.nat64Profiles[i].ID] = true
	}
	for _, p := range remote.NAT64Profiles {
		if p.ID == "" || natSeen[p.ID] {
			continue
		}
		rm.nat64Profiles = append(rm.nat64Profiles, p)
		sum.AddedNAT64++
		sum.Details = append(sum.Details, "added NAT64 profile "+p.ID)
	}

	if sum.Changed() {
		rm.buildRules()
		if err := rm.saveRulesConfig(); err != nil {
			return sum, err
		}
	}
	return sum, rm.saveSyncState(next)
}
