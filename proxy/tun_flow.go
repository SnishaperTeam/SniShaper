package proxy

import (
	"context"
	"fmt"
	"net"
	"strings"
)

type TUNFlow struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host,omitempty"`
	SNI      string `json:"sni,omitempty"`
	DestAddr string `json:"dest_addr,omitempty"`
	DestPort int    `json:"dest_port,omitempty"`
}

type TUNFlowPlan struct {
	Protocol       string   `json:"protocol"`
	Host           string   `json:"host,omitempty"`
	MatchHost      string   `json:"match_host,omitempty"`
	RuleMode       string   `json:"rule_mode"`
	EffectiveMode  string   `json:"effective_mode"`
	Upstream       string   `json:"upstream,omitempty"`
	DialCandidates []string `json:"dial_candidates,omitempty"`
	UDPStrategy    string   `json:"udp_strategy,omitempty"`
	CanMITM        bool     `json:"can_mitm"`
	NeedsSniffing  bool     `json:"needs_sniffing"`
	Notes          []string `json:"notes,omitempty"`
}

func (p *ProxyServer) PlanTUNFlow(flow TUNFlow) TUNFlowPlan {
	protocol := strings.ToLower(strings.TrimSpace(flow.Protocol))
	if protocol == "" {
		protocol = "tcp"
	}

	host := normalizeHost(flow.Host)
	if host == "" {
		host = normalizeHost(flow.SNI)
	}
	if host == "" {
		host = normalizeHost(flow.DestAddr)
	}

	port := flow.DestPort
	if port <= 0 || port > 65535 {
		switch protocol {
		case "udp":
			port = 443
		default:
			port = 443
		}
	}

	matchMode := "mitm"
	if protocol == "udp" {
		matchMode = "transparent"
	}

	rule := p.rules.matchRule(host, matchMode)
	effectiveMode := strings.ToLower(strings.TrimSpace(rule.Mode))
	if effectiveMode == "" {
		effectiveMode = "direct"
	}

	notes := make([]string, 0, 4)
	canMITM := protocol == "tcp"
	udpStrategy := ""

	switch protocol {
	case "udp":
		canMITM = false
		switch effectiveMode {
		case "mitm", "tls-rf", "server":
			notes = append(notes, fmt.Sprintf("%s is TCP-oriented in the current TUN path; downgrade UDP handling to transparent passthrough", effectiveMode))
			effectiveMode = "transparent"
		case "quic":
			udpStrategy = "native-quic"
			notes = append(notes, "QUIC-specific UDP handling can be attached here later")
		case "warp":
			udpStrategy = "warp"
			notes = append(notes, "Warp-selected UDP will need a dedicated UDP associate path")
		default:
			udpStrategy = "passthrough"
			if effectiveMode == "direct" {
				notes = append(notes, "Direct UDP flow will bypass HTTP proxy semantics and use transparent forwarding")
			}
		}
	case "tcp":
		switch effectiveMode {
		case "quic":
			notes = append(notes, "QUIC mode selected for TCP flow; preserve rule but dataplane may remap it later")
		case "transparent":
			canMITM = false
			notes = append(notes, "Transparent TCP flow can reuse existing upstream dial logic without TLS termination")
		case "direct":
			canMITM = false
		default:
			notes = append(notes, "TCP flow can reuse existing rule matching and upstream candidate selection")
		}
	default:
		notes = append(notes, "Unknown protocol; treating as direct")
		effectiveMode = "direct"
	}

	targetHost := host
	if targetHost == "" {
		notes = append(notes, "No host/SNI available yet; dataplane should sniff TLS or correlate DNS before advanced routing")
	}
	targetAddr := flow.DestAddr
	if strings.TrimSpace(targetAddr) == "" {
		if targetHost != "" {
			targetAddr = net.JoinHostPort(targetHost, fmt.Sprintf("%d", port))
		}
	}
	if targetAddr == "" {
		targetAddr = net.JoinHostPort("0.0.0.0", fmt.Sprintf("%d", port))
	}

	dialCandidates := p.buildDialCandidates(context.Background(), targetHost, ensureAddrWithPort(targetAddr, fmt.Sprintf("%d", port)), rule, effectiveMode)
	if protocol == "udp" && len(dialCandidates) == 0 && targetAddr != "" {
		dialCandidates = []string{ensureAddrWithPort(targetAddr, fmt.Sprintf("%d", port))}
	}

	return TUNFlowPlan{
		Protocol:       protocol,
		Host:           host,
		MatchHost:      host,
		RuleMode:       rule.Mode,
		EffectiveMode:  effectiveMode,
		Upstream:       resolveRuleUpstream(targetHost, rule),
		DialCandidates: dialCandidates,
		UDPStrategy:    udpStrategy,
		CanMITM:        canMITM,
		NeedsSniffing:  host == "",
		Notes:          notes,
	}
}

// SNIRewriteDecisionForTUN 返回 TUN 流的 SNI 重写决策（US4，供
// singtun Handler.SetSNIDecider 注入）：
//   - matched=true：host 命中了启用的规则（~ 正则 / 后缀域名，非法正则安全跳过）；
//     rewriteTo 为该规则 TrimSpace 后的 sni_fake，空串表示只匹配不重写。
//   - matched=false：未命中 / 规则被禁用 / 规则管理器未初始化。
//
// 匹配模式传 "tun"：matchRule 的 mode 参数仅影响 transparent 降级 MITM 的
// 逻辑，"tun" 不触发该降级；SNI 重写与规则出站模式正交。
// 未命中时 matchRule 返回零值 Rule（Domain 为空）——domainMatchScore 对空
// Domain 恒返回 -1，故真实命中规则的 Domain 必非空，以 rule.Domain 区分。
func (p *ProxyServer) SNIRewriteDecisionForTUN(host string) (rewriteTo string, matched bool) {
	if p.rules == nil {
		return "", false
	}
	rule := p.rules.matchRule(host, "tun")
	if rule.Domain == "" {
		return "", false
	}
	return strings.TrimSpace(rule.SniFake), true
}
