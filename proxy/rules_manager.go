package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"snishaper/pkg/dohresolver"
)

type RuleManager struct {
	rules                      []Rule
	siteGroups                 []SiteGroup
	upstreams                  []Upstream
	dnsNodes                   []DNSNode
	settingsPath               string
	rulesPath                  string
	cloudflareConfig           CloudflareConfig
	tunConfig                  TUNConfig
	closeToTray                bool
	autoStart                  bool
	showMainOnAutoStart        bool
	autoEnableProxyOnAutoStart bool
	socks5Enabled              bool
	socks5Port                 string
	serverHost                 string
	serverAuth                 string
	listenPort                 string
	echProfiles                []ECHProfile
	autoRouter                 *AutoRouter
	autoRoutingConfig          AutoRoutingConfig
	mu                         sync.RWMutex
	routeEventCallback         func(domain, mode string)
	onConfigSaved              func()
	language                   string
	theme                      string
}

func (r *RuleManager) SetRouteEventCallback(cb func(domain, mode string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routeEventCallback = cb
}

func (r *RuleManager) emitRouteEvent(domain, mode string) {
	r.mu.RLock()
	cb := r.routeEventCallback
	r.mu.RUnlock()
	if cb != nil {
		cb(domain, mode)
	}
}

func (r *RuleManager) SetOnConfigSaved(cb func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onConfigSaved = cb
}

// triggerConfigSaved fires the config-saved callback asynchronously.
// It is always called from methods that already hold rm.mu, so no extra locking is needed.
func (r *RuleManager) triggerConfigSaved() {
	cb := r.onConfigSaved
	if cb != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Config] panic in onConfigSaved callback: %v", r)
				}
			}()
			cb()
		}()
	}
}

type RulesConfig struct {
	SiteGroups  []SiteGroup  `json:"site_groups"`
	Upstreams   []Upstream   `json:"upstreams"`
	DNSNodes    []DNSNode    `json:"dns_nodes,omitempty"`
	ECHProfiles []ECHProfile `json:"ech_profiles,omitempty"`
}

// DNSNode defines a DoH upstream with optional SNI obfuscation.
// It reuses the same dial-level concepts as proxy rules (SNI spoofing, ECH, QUIC, static IPs).
func (r *RuleManager) SetRules(rules []Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = rules
}

func (r *RuleManager) matchRule(host, mode string) Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	host = normalizeHost(host)
	mode = strings.ToLower(strings.TrimSpace(mode))

	best := Rule{}
	bestScore := -1
	for _, rule := range r.rules {
		if !rule.Enabled {
			continue
		}

		score := domainMatchScore(host, rule.Domain)
		if score >= 0 && score > bestScore {
			best = rule
			bestScore = score
		}
	}

	// 如果命中了特定规则
	if bestScore >= 0 {
		if mode == "transparent" && best.Mode == "mitm" {
			log.Printf("[RuleMatch] Global Transparent detected: Downgrading MITM rule (%s) to DIRECT to avoid cert errors.", host)
			best.Mode = "direct"
		}
		log.Printf("[Router] %s -> %s", host, best.Mode)
		r.emitRouteEvent(host, best.Mode)
		return best
	}

	// 自动分流层：手动规则未命中时，查询 AutoRouter
	if r.autoRouter != nil && r.autoRoutingConfig.Mode != "" {
		autoRule := r.autoRouter.Decide(host)
		if autoRule.Mode != "direct" {
			log.Printf("[Router] %s -> %s (AutoRoute)", host, autoRule.Mode)
			r.emitRouteEvent(host, autoRule.Mode)
			return autoRule
		}
	}

	// 未命中任何规则，走直连
	log.Printf("[Router] %s -> direct (Default)", host)
	r.emitRouteEvent(host, "direct")
	return Rule{
		Mode:    "direct",
		Enabled: true,
	}
}

func NewRuleManager(settingsPath, rulesPath string) *RuleManager {
	return &RuleManager{
		settingsPath:        settingsPath,
		rulesPath:           rulesPath,
		rules:               []Rule{},
		closeToTray:         true,
		showMainOnAutoStart: true,
		language:            "zh",
		theme:               "dark",
	}
}

func findECHProfileByID(profiles []ECHProfile, id string) *ECHProfile {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for i := range profiles {
		if profiles[i].ID == id {
			return &profiles[i]
		}
	}
	return nil
}

func ensureLegacyCloudflareProfile(profiles *[]ECHProfile) string {
	const profileID = "legacy-cloudflare"
	if existing := findECHProfileByID(*profiles, profileID); existing != nil {
		normalizeECHProfile(existing)
		if existing.Name == "" {
			existing.Name = "Legacy Cloudflare"
		}
		if existing.DiscoveryDomain == "" {
			existing.DiscoveryDomain = "crypto.cloudflare.com"
		}
		return existing.ID
	}

	*profiles = append(*profiles, ECHProfile{
		ID:              profileID,
		Name:            "Legacy Cloudflare",
		DiscoveryDomain: "crypto.cloudflare.com",
		AutoUpdate:      true,
	})
	return profileID
}

func migrateLegacyECHRules(siteGroups []SiteGroup, profiles *[]ECHProfile) bool {
	migrated := false
	for i := range siteGroups {
		siteGroups[i].ECHProfileID = strings.TrimSpace(siteGroups[i].ECHProfileID)
		siteGroups[i].ECHDomain = strings.TrimSpace(siteGroups[i].ECHDomain)
		if siteGroups[i].ECHEnabled && siteGroups[i].ECHProfileID == "" &&
			strings.EqualFold(siteGroups[i].ECHDomain, "crypto.cloudflare.com") {
			siteGroups[i].ECHProfileID = ensureLegacyCloudflareProfile(profiles)
			siteGroups[i].ECHDomain = ""
			migrated = true
		}
	}
	return migrated
}

func (rm *RuleManager) LoadConfig() error {
	if err := rm.loadSettingsConfig(); err != nil {
		return err
	}
	if err := rm.loadRulesConfig(); err != nil {
		return err
	}

	for i := range rm.siteGroups {
		rm.siteGroups[i].DNSMode = normalizeDNSMode(rm.siteGroups[i].DNSMode)
	}
	if rm.upstreams == nil {
		rm.upstreams = []Upstream{}
	}
	if rm.echProfiles == nil {
		rm.echProfiles = []ECHProfile{}
	}
	for i := range rm.echProfiles {
		normalizeECHProfile(&rm.echProfiles[i])
	}
	rm.applySettingsDefaults()

	// Sync Cloudflare Config if ProxyServer is linked
	// Note: In current architecture, RuleManager doesn't have a back-pointer to ProxyServer.
	// ProxyServer.SetRuleManager is used. We might need to update ProxyServer's pool elsewhere.
	// But actually, ProxyServer holds the pool, so when LoadConfig is called via the RuleManager
	// inside ProxyServer, it should be updated.
	// Wait, ProxyServer has a pointer to RuleManager.

	migrated := false
	for i := range rm.siteGroups {
		rm.siteGroups[i].Website = strings.TrimSpace(rm.siteGroups[i].Website)
		if rm.siteGroups[i].Website == "" {
			rm.siteGroups[i].Website = inferWebsiteFromSiteGroup(rm.siteGroups[i])
			migrated = true
		}
	}
	if migrateLegacyECHRules(rm.siteGroups, &rm.echProfiles) {
		migrated = true
	}

	rm.buildRules()
	if migrated {
		if err := rm.saveRulesConfig(); err != nil {
			log.Printf("[Config] migrate website field failed: %v", err)
		} else {
			log.Printf("[Config] migrated website field for existing site groups")
		}
	}
	return nil
}

func (rm *RuleManager) applySettingsDefaults() {
	if rm.listenPort == "" {
		rm.listenPort = "8080"
	}
	rm.tunConfig = normalizeTUNConfig(rm.tunConfig)
}

func (rm *RuleManager) loadSettingsConfig() error {
	data, err := os.ReadFile(rm.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return rm.saveDefaultSettingsConfig()
		}
		return err
	}

	var config SettingsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	// 1. Set internal defaults first
	rm.closeToTray = true
	rm.autoStart = false
	rm.showMainOnAutoStart = true
	rm.autoEnableProxyOnAutoStart = false

	// 2. Override with JSON values if they exist
	rm.cloudflareConfig = config.CloudflareConfig
	rm.tunConfig = config.TUN
	rm.serverHost = config.ServerHost
	rm.serverAuth = config.ServerAuth
	if config.ListenPort != "" {
		rm.listenPort = config.ListenPort
	}
	if config.Socks5Port != "" {
		rm.socks5Port = config.Socks5Port
	}
	rm.autoRoutingConfig = config.AutoRouting
	if config.Language != "" {
		rm.language = config.Language
	}
	if config.Theme != "" {
		rm.theme = config.Theme
	}

	if config.CloseToTray != nil {
		rm.closeToTray = *config.CloseToTray
	}
	if config.AutoStart != nil {
		rm.autoStart = *config.AutoStart
	}
	if config.ShowMainWindowOnAutoStart != nil {
		rm.showMainOnAutoStart = *config.ShowMainWindowOnAutoStart
	}
	if config.AutoEnableProxyOnAutoStart != nil {
		rm.autoEnableProxyOnAutoStart = *config.AutoEnableProxyOnAutoStart
	}
	if config.Socks5Enabled != nil {
		rm.socks5Enabled = *config.Socks5Enabled
	}
	rm.applySettingsDefaults()
	return nil
}

func (rm *RuleManager) loadRulesConfig() error {
	data, err := os.ReadFile(rm.rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return rm.saveDefaultRulesConfig()
		}
		return err
	}

	var config RulesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	rm.siteGroups = config.SiteGroups
	rm.upstreams = config.Upstreams
	rm.dnsNodes = config.DNSNodes
	rm.echProfiles = config.ECHProfiles
	// Ensure at least the default Ali DoH bootstrap node exists
	if len(rm.dnsNodes) == 0 {
		rm.dnsNodes = defaultDNSNodes()
	}
	return nil
}

func (rm *RuleManager) saveDefaultSettingsConfig() error {
	rm.closeToTray = true
	rm.autoStart = false
	rm.showMainOnAutoStart = true
	rm.autoEnableProxyOnAutoStart = false
	rm.applySettingsDefaults()
	return rm.saveSettingsConfig()
}

func (rm *RuleManager) saveDefaultRulesConfig() error {
	rm.siteGroups = []SiteGroup{}
	rm.upstreams = []Upstream{}
	rm.dnsNodes = defaultDNSNodes()
	rm.echProfiles = []ECHProfile{}
	rm.buildRules()
	return rm.saveRulesConfig()
}

func (rm *RuleManager) buildRules() {
	rm.rules = []Rule{}
	upstreamMap := make(map[string]string)
	for _, up := range rm.upstreams {
		if up.Enabled && up.Address != "" {
			upstreamMap[up.ID] = up.Address
		}
	}

	echProfileMap := make(map[string]ECHProfile)
	for _, profile := range rm.echProfiles {
		echProfileMap[profile.ID] = profile
	}

	for _, sg := range rm.siteGroups {
		if !sg.Enabled {
			continue
		}

		// Resolve upstream ID to actual address
		resolvedUpstream := sg.Upstream
		if addr, ok := upstreamMap[sg.Upstream]; ok {
			resolvedUpstream = addr
		}

		resolvedUpstreams := make([]string, 0, len(sg.Upstreams))
		for _, upId := range sg.Upstreams {
			if addr, ok := upstreamMap[upId]; ok {
				resolvedUpstreams = append(resolvedUpstreams, addr)
			} else {
				resolvedUpstreams = append(resolvedUpstreams, upId)
			}
		}

		var echConfigBytes []byte
		var echProfile ECHProfile
		if sg.ECHProfileID != "" {
			if profile, ok := echProfileMap[sg.ECHProfileID]; ok {
				echProfile = profile
				if configStr := strings.TrimSpace(profile.Config); configStr != "" {
					if decoded, err := base64.StdEncoding.DecodeString(configStr); err == nil {
						echConfigBytes = decoded
						log.Printf("[BuildRules] Successfully loaded ECH Config for SiteGroup %s (%d bytes)", sg.ID, len(echConfigBytes))
					} else {
						log.Printf("[BuildRules] ERROR: Failed to decode ECH Config for SiteGroup %s: %v", sg.ID, err)
					}
				}
			} else {
				log.Printf("[BuildRules] WARNING: ECHProfileID %s linked to SiteGroup %s but profile not found", sg.ECHProfileID, sg.ID)
			}
		}

		for _, domain := range sg.Domains {
			rule := Rule{
				Domain:             domain,
				Mode:               sg.Mode,
				Upstream:           resolvedUpstream,
				Upstreams:          resolvedUpstreams,
				DNSMode:            normalizeDNSMode(sg.DNSMode),
				SniFake:            sg.SniFake,
				ConnectPolicy:      strings.TrimSpace(sg.ConnectPolicy),
				SniPolicy:          strings.TrimSpace(sg.SniPolicy),
				Enabled:            true,
				SiteID:             sg.ID,
				ECHEnabled:         sg.ECHEnabled,
				ECHProfileID:       sg.ECHProfileID,
				UseCFPool:          sg.UseCFPool,
				ECHDiscoveryDomain: echProfile.DiscoveryDomain,
				ECHDoHUpstream:     echProfile.DoHUpstream,
				ECHAutoUpdate:      echProfile.AutoUpdate,
				CertVerify:         sg.CertVerify,
			}
			rm.rules = append(rm.rules, rule)
		}
	}
}

func (rm *RuleManager) incrementRuleHit(siteID string) {
	// No-op after stats removal
}

func (rm *RuleManager) GetRuleHitCounts() map[string]int64 {
	return map[string]int64{}
}

func (rm *RuleManager) GetSiteGroups() []SiteGroup {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.siteGroups
}

func (rm *RuleManager) GetServerHost() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.serverHost
}

func (rm *RuleManager) GetServerAuth() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.serverAuth
}

func (rm *RuleManager) GetListenPort() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.listenPort
}

func (rm *RuleManager) SetListenPort(port string) {
	rm.mu.Lock()
	rm.listenPort = port
	rm.mu.Unlock()
}

func (rm *RuleManager) GetSocks5Enabled() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.socks5Enabled
}

func (rm *RuleManager) SetSocks5Enabled(enabled bool) {
	rm.mu.Lock()
	rm.socks5Enabled = enabled
	rm.mu.Unlock()
}

func (rm *RuleManager) GetSocks5Port() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.socks5Port
}

func (rm *RuleManager) SetSocks5Port(port string) {
	rm.mu.Lock()
	rm.socks5Port = port
	rm.mu.Unlock()
}

func (rm *RuleManager) SaveConfig() error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	if err := rm.saveSettingsConfig(); err != nil {
		return err
	}
	return rm.saveRulesConfig()
}

func (rm *RuleManager) UpdateServerConfig(host, auth string) error {
	rm.mu.Lock()
	rm.serverHost = host
	rm.serverAuth = auth
	rm.mu.Unlock()
	return rm.saveSettingsConfig()
}

func (rm *RuleManager) GetCloudflareConfig() CloudflareConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.cloudflareConfig
}

func (rm *RuleManager) GetTUNConfig() TUNConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return normalizeTUNConfig(rm.tunConfig)
}

func (rm *RuleManager) GetCloseToTray() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.closeToTray
}

func (rm *RuleManager) SetCloseToTray(enabled bool) error {
	rm.mu.Lock()
	rm.closeToTray = enabled
	rm.mu.Unlock()
	return rm.saveSettingsConfig()
}

func (rm *RuleManager) GetAutoStart() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.autoStart
}

func (rm *RuleManager) SetAutoStart(enabled bool) error {
	rm.mu.Lock()
	rm.autoStart = enabled
	rm.mu.Unlock()
	return rm.saveSettingsConfig()
}

func (rm *RuleManager) GetShowMainWindowOnAutoStart() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.showMainOnAutoStart
}

func (rm *RuleManager) SetShowMainWindowOnAutoStart(enabled bool) error {
	rm.mu.Lock()
	rm.showMainOnAutoStart = enabled
	rm.mu.Unlock()
	return rm.saveSettingsConfig()
}

func (rm *RuleManager) GetAutoEnableProxyOnAutoStart() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.autoEnableProxyOnAutoStart
}

func (rm *RuleManager) SetAutoEnableProxyOnAutoStart(enabled bool) error {
	rm.mu.Lock()
	rm.autoEnableProxyOnAutoStart = enabled
	rm.mu.Unlock()
	return rm.saveSettingsConfig()
}

func (r *RuleManager) GetLanguage() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.language
}

func (r *RuleManager) SetLanguage(lang string) error {
	r.mu.Lock()
	r.language = lang
	r.mu.Unlock()
	return r.saveSettingsConfig()
}

func (r *RuleManager) GetTheme() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.theme == "" {
		return "dark" // Default to dark
	}
	return r.theme
}

func (r *RuleManager) SetTheme(theme string) error {
	r.mu.Lock()
	r.theme = theme
	r.mu.Unlock()
	return r.saveSettingsConfig()
}

func (rm *RuleManager) UpdateCloudflareConfig(cfg CloudflareConfig) error {
	rm.mu.Lock()
	rm.cloudflareConfig = cfg
	rm.mu.Unlock()
	return rm.saveSettingsConfig()
}

func (rm *RuleManager) UpdateTUNConfig(cfg TUNConfig) error {
	rm.mu.Lock()
	rm.tunConfig = normalizeTUNConfig(cfg)
	rm.mu.Unlock()
	return rm.saveSettingsConfig()
}

func (rm *RuleManager) AddSiteGroup(sg SiteGroup) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	sg.ID = generateID()
	sg.Website = strings.TrimSpace(sg.Website)
	rm.siteGroups = append(rm.siteGroups, sg)
	rm.buildRules()
	return rm.saveRulesConfig()
}

func (rm *RuleManager) UpdateSiteGroup(sg SiteGroup) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	sg.Website = strings.TrimSpace(sg.Website)
	for i, s := range rm.siteGroups {
		if s.ID == sg.ID {
			rm.siteGroups[i] = sg
			break
		}
	}
	rm.buildRules()
	return rm.saveRulesConfig()
}

func (rm *RuleManager) DeleteSiteGroup(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for i, s := range rm.siteGroups {
		if s.ID == id {
			rm.siteGroups = append(rm.siteGroups[:i], rm.siteGroups[i+1:]...)
			break
		}
	}
	rm.buildRules()
	return rm.saveRulesConfig()
}

func (rm *RuleManager) GetUpstreams() []Upstream {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.upstreams
}

func (rm *RuleManager) AddUpstream(u Upstream) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	u.ID = generateID()
	rm.upstreams = append(rm.upstreams, u)
	return rm.saveRulesConfig()
}

func (rm *RuleManager) UpdateUpstream(u Upstream) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for i, up := range rm.upstreams {
		if up.ID == u.ID {
			rm.upstreams[i] = u
			break
		}
	}
	return rm.saveRulesConfig()
}

func (rm *RuleManager) DeleteUpstream(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for i, up := range rm.upstreams {
		if up.ID == id {
			rm.upstreams = append(rm.upstreams[:i], rm.upstreams[i+1:]...)
			break
		}
	}
	return rm.saveRulesConfig()
}

// --- DNS Node CRUD ---

func (rm *RuleManager) GetDNSNodes() []DNSNode {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	if rm.dnsNodes == nil {
		return []DNSNode{}
	}
	out := make([]DNSNode, len(rm.dnsNodes))
	copy(out, rm.dnsNodes)
	return out
}

func (rm *RuleManager) AddDNSNode(n DNSNode) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	n.ID = generateID()
	rm.dnsNodes = append(rm.dnsNodes, n)
	return rm.saveRulesConfig()
}

func (rm *RuleManager) UpdateDNSNode(n DNSNode) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for i, node := range rm.dnsNodes {
		if node.ID == n.ID {
			rm.dnsNodes[i] = n
			break
		}
	}
	return rm.saveRulesConfig()
}

func (rm *RuleManager) DeleteDNSNode(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for i, node := range rm.dnsNodes {
		if node.ID == id {
			rm.dnsNodes = append(rm.dnsNodes[:i], rm.dnsNodes[i+1:]...)
			break
		}
	}
	return rm.saveRulesConfig()
}

// SetDNSNodePriority reorders DNS nodes by moving the node with the given ID
// to the specified target index (0-based). Nodes are queried in list order.
func (rm *RuleManager) SetDNSNodePriority(id string, targetIndex int) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	srcIdx := -1
	for i, node := range rm.dnsNodes {
		if node.ID == id {
			srcIdx = i
			break
		}
	}
	if srcIdx < 0 {
		return fmt.Errorf("dns node %s not found", id)
	}
	if targetIndex < 0 {
		targetIndex = 0
	}
	if targetIndex >= len(rm.dnsNodes) {
		targetIndex = len(rm.dnsNodes) - 1
	}
	if srcIdx == targetIndex {
		return nil
	}

	node := rm.dnsNodes[srcIdx]
	rm.dnsNodes = append(rm.dnsNodes[:srcIdx], rm.dnsNodes[srcIdx+1:]...)
	tail := append([]DNSNode{}, rm.dnsNodes[targetIndex:]...)
	rm.dnsNodes = append(rm.dnsNodes[:targetIndex], node)
	rm.dnsNodes = append(rm.dnsNodes, tail...)
	return rm.saveRulesConfig()
}

func (rm *RuleManager) saveSettingsConfig() error {
	listenPort := rm.listenPort
	if listenPort == "" {
		listenPort = "8080"
	}
	socks5Port := rm.socks5Port
	if socks5Port == "" {
		socks5Port = "8081"
	}
	closeToTray := rm.closeToTray
	autoStart := rm.autoStart
	showMainOnAutoStart := rm.showMainOnAutoStart
	autoEnableProxyOnAutoStart := rm.autoEnableProxyOnAutoStart
	socks5Enabled := rm.socks5Enabled
	cloudflareConfig := rm.cloudflareConfig
	tunConfig := normalizeTUNConfig(rm.tunConfig)
	settings := SettingsConfig{
		ListenPort:                 listenPort,
		Socks5Port:                 socks5Port,
		ServerHost:                 rm.serverHost,
		ServerAuth:                 rm.serverAuth,
		CloseToTray:                &closeToTray,
		AutoStart:                  &autoStart,
		ShowMainWindowOnAutoStart:  &showMainOnAutoStart,
		AutoEnableProxyOnAutoStart: &autoEnableProxyOnAutoStart,
		CloudflareConfig:           cloudflareConfig,
		AutoRouting:                rm.autoRoutingConfig,
		TUN:                        tunConfig,
		Language:                   rm.language,
		Theme:                      rm.theme,
		Socks5Enabled:              &socks5Enabled,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(rm.settingsPath), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(rm.settingsPath, data, 0644); err != nil {
		return err
	}
	rm.triggerConfigSaved()
	return nil
}

func (rm *RuleManager) saveRulesConfig() error {
	config := RulesConfig{
		SiteGroups:  rm.siteGroups,
		Upstreams:   rm.upstreams,
		DNSNodes:    rm.dnsNodes,
		ECHProfiles: rm.echProfiles,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(rm.rulesPath), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(rm.rulesPath, data, 0644); err != nil {
		return err
	}
	rm.triggerConfigSaved()
	return nil
}

func (rm *RuleManager) GetECHProfiles() []ECHProfile {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	if rm.echProfiles == nil {
		return []ECHProfile{}
	}
	return rm.echProfiles
}

func (rm *RuleManager) UpsertECHProfile(p ECHProfile) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	normalizeECHProfile(&p)
	if p.ID == "" {
		p.ID = generateID()
		rm.echProfiles = append(rm.echProfiles, p)
	} else {
		found := false
		for i, x := range rm.echProfiles {
			if x.ID == p.ID {
				rm.echProfiles[i] = p
				found = true
				break
			}
		}
		if !found {
			rm.echProfiles = append(rm.echProfiles, p)
		}
	}
	return rm.saveRulesConfig()
}

func (rm *RuleManager) DeleteECHProfile(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for i, x := range rm.echProfiles {
		if x.ID == id {
			rm.echProfiles = append(rm.echProfiles[:i], rm.echProfiles[i+1:]...)
			break
		}
	}
	return rm.saveRulesConfig()
}

func (r *RuleManager) GetBinaryECHConfig(id string) []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.echProfiles {
		if p.ID == id {
			data, err := base64.StdEncoding.DecodeString(p.Config)
			if err == nil && len(data) > 0 {
				return data
			}
			break
		}
	}
	return nil
}

func (r *RuleManager) UpdateECHProfileConfig(profileID string, configBytes []byte) error {
	if profileID == "" || len(configBytes) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	found := false
	configBase64 := base64.StdEncoding.EncodeToString(configBytes)
	for i := range r.echProfiles {
		if r.echProfiles[i].ID == profileID {
			if r.echProfiles[i].Config == configBase64 {
				return nil // No change
			}
			r.echProfiles[i].Config = configBase64
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("profile %s not found", profileID)
	}

	log.Printf("[RuleManager] ECH Profile %s updated via sync", profileID)
	return r.saveRulesConfig()
}

func (rm *RuleManager) GetAutoRoutingConfig() AutoRoutingConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.autoRoutingConfig
}

func (rm *RuleManager) UpdateAutoRoutingConfig(cfg AutoRoutingConfig) error {
	rm.mu.Lock()
	rm.autoRoutingConfig = cfg
	if rm.autoRouter != nil {
		rm.autoRouter.UpdateConfig(cfg)
	}
	rm.mu.Unlock()
	return rm.saveSettingsConfig()
}

func (rm *RuleManager) GetAutoRouter() *AutoRouter {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.autoRouter
}

func (rm *RuleManager) InitAutoRouter(resolver *dohresolver.FailoverResolver) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.autoRouter = NewAutoRouter(rm.autoRoutingConfig, resolver)

	// Try loading cached GFW list
	cachePath := gfwListCachePath(rm.rulesPath)
	if count, err := rm.autoRouter.GetGFWList().LoadFromFile(cachePath); err == nil {
		log.Printf("[AutoRoute] Loaded %d domains from cache: %s", count, cachePath)
	} else {
		log.Printf("[AutoRoute] No cached GFW list at %s: %v", cachePath, err)
	}
}

func (rm *RuleManager) RefreshGFWList() (int, error) {
	rm.mu.RLock()
	ar := rm.autoRouter
	cfg := rm.autoRoutingConfig
	rulesPath := rm.rulesPath
	rm.mu.RUnlock()

	if ar == nil {
		return 0, fmt.Errorf("auto router not initialized")
	}

	url := cfg.GFWListURL
	if url == "" {
		url = defaultGFWListURL
	}

	count, err := ar.GetGFWList().LoadFromURL(url)
	if err != nil {
		return 0, err
	}

	// Save to local cache
	cachePath := gfwListCachePath(rulesPath)
	if saveErr := ar.GetGFWList().SaveToFile(cachePath); saveErr != nil {
		log.Printf("[AutoRoute] Failed to save GFW list cache: %v", saveErr)
	}

	// Update last update time
	rm.mu.Lock()
	rm.autoRoutingConfig.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
	cfg = rm.autoRoutingConfig
	if rm.autoRouter != nil {
		rm.autoRouter.UpdateConfig(cfg)
	}
	rm.mu.Unlock()
	_ = rm.saveSettingsConfig()

	return count, nil
}

func (rm *RuleManager) GetAutoRoutingStatus() GFWListStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	if rm.autoRouter != nil {
		return rm.autoRouter.GetStatus()
	}
	return GFWListStatus{
		Enabled: false,
		Mode:    string(rm.autoRoutingConfig.Mode),
	}
}
