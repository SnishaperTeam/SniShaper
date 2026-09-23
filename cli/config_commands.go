//go:build headless

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"runtime"
	"strings"

	"snishaper/app"
	"snishaper/core"
	"snishaper/proxy"
)

// configApp builds an App that only loads settings and rules. Commands that
// inspect or edit configuration use it, so they neither start the core process
// nor touch the running service.
func configApp(out cmdOut) *app.App {
	// NewApp loads the rules, which logs one line per site group. A one-shot
	// command must not print that chatter, so the library logger stays muted
	// until the app installs its own sink.
	log.SetOutput(io.Discard)
	a := app.NewApp()
	if err := a.StartupConfigOnly(); err != nil {
		out("加载配置失败: " + err.Error())
		return nil
	}
	return a
}

// reloadService asks a running service to pick up the new configuration.
func reloadService(out cmdOut) {
	c := core.NewCoreClient()
	if c.Ping() {
		c.ReloadIfRunning()
		out("已通知运行中的服务重载配置")
	}
}

func decodeJSONArg(out cmdOut, usage, raw string, target interface{}) bool {
	if strings.TrimSpace(raw) == "" {
		out("缺少 JSON 参数: " + usage)
		return false
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		out("解析 JSON 失败: " + err.Error())
		return false
	}
	return true
}

func printJSON(out cmdOut, v interface{}) {
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		out("序列化失败: " + err.Error())
		return
	}
	out(string(pretty))
}

// addedID returns a lookup for the entry that appeared after an insert. The
// stores generate ids themselves, so the requested id cannot be echoed back.
func addedID(before []string) func(after []string) string {
	return func(after []string) string {
		seen := make(map[string]bool, len(before))
		for _, id := range before {
			seen[id] = true
		}
		for _, id := range after {
			if !seen[id] {
				return id
			}
		}
		return ""
	}
}

func siteIDs(items []proxy.SiteGroup) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func upstreamIDs(items []proxy.Upstream) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func dnsIDs(items []proxy.DNSNode) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func echIDs(items []proxy.ECHProfile) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func nat64IDs(items []proxy.NAT64Profile) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func hasID(list []string, want string) bool {
	for _, id := range list {
		if id == want {
			return true
		}
	}
	return false
}

func opSites(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		groups := a.GetSiteGroups()
		if len(groups) == 0 {
			out("没有站点组")
			return 0
		}
		for _, g := range groups {
			out(fmt.Sprintf("%-28s %-10s domains=%d ech=%v upstreams=%d", g.ID, g.Mode, len(g.Domains), g.ECHEnabled, len(g.Upstreams)))
		}
		return 0
	case "show":
		if len(args) < 2 {
			out("用法: sites show <id>")
			return 2
		}
		for _, g := range a.GetSiteGroups() {
			if g.ID == args[1] {
				printJSON(out, g)
				return 0
			}
		}
		out("未找到站点组: " + args[1])
		return 1
	case "add":
		var sg proxy.SiteGroup
		if !decodeJSONArg(out, `sites add '{"name":"...","mode":"direct","domains":["a.com"]}'`, argAt(args, 1), &sg) {
			return 2
		}
		sg.ID = ""
		track := addedID(siteIDs(a.GetSiteGroups()))
		if err := a.AddSiteGroup(sg); err != nil {
			out("保存站点组失败: " + err.Error())
			return 1
		}
		out("已新增站点组: " + track(siteIDs(a.GetSiteGroups())))
		reloadService(out)
		return 0
	case "update":
		var sg proxy.SiteGroup
		if !decodeJSONArg(out, `sites update '{"id":"...","name":"...","domains":["a.com"]}'`, argAt(args, 1), &sg) {
			return 2
		}
		if !hasID(siteIDs(a.GetSiteGroups()), sg.ID) {
			out("未找到站点组: " + sg.ID)
			return 1
		}
		if err := a.UpdateSiteGroup(sg); err != nil {
			out("保存站点组失败: " + err.Error())
			return 1
		}
		out("已更新站点组: " + sg.ID)
		reloadService(out)
		return 0
	case "delete":
		if len(args) < 2 {
			out("用法: sites delete <id>")
			return 2
		}
		if !hasID(siteIDs(a.GetSiteGroups()), args[1]) {
			out("未找到站点组: " + args[1])
			return 1
		}
		if err := a.DeleteSiteGroup(args[1]); err != nil {
			out("删除站点组失败: " + err.Error())
			return 1
		}
		out("已删除站点组: " + args[1])
		reloadService(out)
		return 0
	default:
		out("用法: sites list|show <id>|add <json>|update <json>|delete <id>")
		return 2
	}
}

func opUpstreams(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		items := a.GetUpstreams()
		if len(items) == 0 {
			out("没有上游配置")
			return 0
		}
		for _, u := range items {
			out(fmt.Sprintf("%-28s %-12s enabled=%v addresses=%d", u.ID, u.Type, u.Enabled, len(u.Addresses)))
		}
		return 0
	case "show":
		if len(args) < 2 {
			out("用法: upstreams show <id>")
			return 2
		}
		for _, u := range a.GetUpstreams() {
			if u.ID == args[1] {
				printJSON(out, u)
				return 0
			}
		}
		out("未找到上游: " + args[1])
		return 1
	case "add":
		var u proxy.Upstream
		if !decodeJSONArg(out, `upstreams add '{"name":"...","type":"cf_preferred","addresses":["1.1.1.1:443"]}'`, argAt(args, 1), &u) {
			return 2
		}
		u.ID = ""
		track := addedID(upstreamIDs(a.GetUpstreams()))
		if err := a.AddUpstream(u); err != nil {
			out("保存上游失败: " + err.Error())
			return 1
		}
		out("已新增上游: " + track(upstreamIDs(a.GetUpstreams())))
		reloadService(out)
		return 0
	case "update":
		var u proxy.Upstream
		if !decodeJSONArg(out, `upstreams update '{"id":"...","name":"..."}'`, argAt(args, 1), &u) {
			return 2
		}
		if !hasID(upstreamIDs(a.GetUpstreams()), u.ID) {
			out("未找到上游: " + u.ID)
			return 1
		}
		if err := a.UpdateUpstream(u); err != nil {
			out("保存上游失败: " + err.Error())
			return 1
		}
		out("已更新上游: " + u.ID)
		reloadService(out)
		return 0
	case "delete":
		if len(args) < 2 {
			out("用法: upstreams delete <id>")
			return 2
		}
		if !hasID(upstreamIDs(a.GetUpstreams()), args[1]) {
			out("未找到上游: " + args[1])
			return 1
		}
		if err := a.DeleteUpstream(args[1]); err != nil {
			out("删除上游失败: " + err.Error())
			return 1
		}
		out("已删除上游: " + args[1])
		reloadService(out)
		return 0
	default:
		out("用法: upstreams list|show <id>|add <json>|update <json>|delete <id>")
		return 2
	}
}

func opDNS(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		nodes := a.GetDNSNodes()
		if len(nodes) == 0 {
			out("没有 DNS 节点")
			return 0
		}
		for i, n := range nodes {
			out(fmt.Sprintf("%d  %-28s %-38s enabled=%v ech=%v", i, n.ID, n.URL, n.Enabled, n.ECHEnabled))
		}
		return 0
	case "show":
		if len(args) < 2 {
			out("用法: dns show <id>")
			return 2
		}
		for _, n := range a.GetDNSNodes() {
			if n.ID == args[1] {
				printJSON(out, n)
				return 0
			}
		}
		out("未找到 DNS 节点: " + args[1])
		return 1
	case "add":
		var n proxy.DNSNode
		if !decodeJSONArg(out, `dns add '{"name":"...","url":"https://1.1.1.1/dns-query","enabled":true}'`, argAt(args, 1), &n) {
			return 2
		}
		n.ID = ""
		track := addedID(dnsIDs(a.GetDNSNodes()))
		if err := a.AddDNSNode(n); err != nil {
			out("保存 DNS 节点失败: " + err.Error())
			return 1
		}
		out("已新增 DNS 节点: " + track(dnsIDs(a.GetDNSNodes())))
		reloadService(out)
		return 0
	case "update":
		var n proxy.DNSNode
		if !decodeJSONArg(out, `dns update '{"id":"...","name":"...","url":"..."}'`, argAt(args, 1), &n) {
			return 2
		}
		if !hasID(dnsIDs(a.GetDNSNodes()), n.ID) {
			out("未找到 DNS 节点: " + n.ID)
			return 1
		}
		if err := a.UpdateDNSNode(n); err != nil {
			out("保存 DNS 节点失败: " + err.Error())
			return 1
		}
		out("已更新 DNS 节点: " + n.ID)
		reloadService(out)
		return 0
	case "delete":
		if len(args) < 2 {
			out("用法: dns delete <id>")
			return 2
		}
		if !hasID(dnsIDs(a.GetDNSNodes()), args[1]) {
			out("未找到 DNS 节点: " + args[1])
			return 1
		}
		if err := a.DeleteDNSNode(args[1]); err != nil {
			out("删除 DNS 节点失败: " + err.Error())
			return 1
		}
		out("已删除 DNS 节点: " + args[1])
		reloadService(out)
		return 0
	case "priority":
		if len(args) < 3 {
			out("用法: dns priority <id> <index>")
			return 2
		}
		index := 0
		if _, err := fmt.Sscanf(args[2], "%d", &index); err != nil {
			out("索引必须是整数: " + args[2])
			return 2
		}
		if !hasID(dnsIDs(a.GetDNSNodes()), args[1]) {
			out("未找到 DNS 节点: " + args[1])
			return 1
		}
		if err := a.SetDNSNodePriority(args[1], index); err != nil {
			out("调整优先级失败: " + err.Error())
			return 1
		}
		out("已调整优先级: " + args[1])
		reloadService(out)
		return 0
	case "test":
		if len(args) < 2 {
			out("用法: dns test <id>")
			return 2
		}
		res, err := a.TestDNSNode(args[1])
		if err != nil {
			out("测试失败: " + err.Error())
			return 1
		}
		out(fmt.Sprintf("成功: %v  延迟: %d ms  IP: %s", res.Success, res.Latency, strings.Join(res.IPs, ", ")))
		if res.Error != "" {
			out("错误: " + res.Error)
		}
		if res.Success {
			return 0
		}
		return 1
	default:
		out("用法: dns list|show <id>|add <json>|update <json>|delete <id>|priority <id> <index>|test <id>")
		return 2
	}
}

func opECH(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		profiles := a.GetECHProfiles()
		if len(profiles) == 0 {
			out("没有 ECH 配置")
			return 0
		}
		for _, p := range profiles {
			out(fmt.Sprintf("%-28s %-24s auto=%v config=%dB", p.ID, p.Name, p.AutoUpdate, len(p.Config)))
		}
		return 0
	case "upsert":
		var p proxy.ECHProfile
		if !decodeJSONArg(out, `ech upsert '{"id":"...","name":"...","config":"base64","discovery_domain":"example.com"}'`, argAt(args, 1), &p) {
			return 2
		}
		track := addedID(echIDs(a.GetECHProfiles()))
		if err := a.UpsertECHProfile(p); err != nil {
			out("保存 ECH 配置失败: " + err.Error())
			return 1
		}
		if p.ID == "" {
			p.ID = track(echIDs(a.GetECHProfiles()))
		}
		out("已保存 ECH 配置: " + p.ID)
		reloadService(out)
		return 0
	case "fetch":
		if len(args) < 2 {
			out("用法: ech fetch <域名> [DoH 地址]")
			return 2
		}
		config, err := a.FetchECHConfig(args[1], argAt(args, 2))
		if err != nil {
			out("获取 ECH 配置失败: " + err.Error())
			return 1
		}
		out("ECH 配置（base64）:")
		out(config)
		out("保存: ech upsert '{\"name\":\"" + args[1] + "\",\"config\":\"" + config + "\"}'")
		return 0
	case "delete":
		if len(args) < 2 {
			out("用法: ech delete <id>")
			return 2
		}
		if !hasID(echIDs(a.GetECHProfiles()), args[1]) {
			out("未找到 ECH 配置: " + args[1])
			return 1
		}
		if err := a.DeleteECHProfile(args[1]); err != nil {
			out("删除 ECH 配置失败: " + err.Error())
			return 1
		}
		out("已删除 ECH 配置: " + args[1])
		reloadService(out)
		return 0
	default:
		out("用法: ech list|upsert <json>|fetch <域名> [DoH 地址]|delete <id>")
		return 2
	}
}

func opNAT64(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		items := a.GetNAT64Profiles()
		if len(items) == 0 {
			out("没有 NAT64 配置")
			return 0
		}
		for _, p := range items {
			out(fmt.Sprintf("%-28s %-24s %s", p.ID, p.Name, p.Prefix))
		}
		return 0
	case "add":
		var p proxy.NAT64Profile
		if !decodeJSONArg(out, `nat64 add '{"name":"...","prefix":"64:ff9b::/96"}'`, argAt(args, 1), &p) {
			return 2
		}
		p.ID = ""
		track := addedID(nat64IDs(a.GetNAT64Profiles()))
		if err := a.AddNAT64Profile(p); err != nil {
			out("保存 NAT64 配置失败: " + err.Error())
			return 1
		}
		out("已新增 NAT64 配置: " + track(nat64IDs(a.GetNAT64Profiles())))
		reloadService(out)
		return 0
	case "update":
		var p proxy.NAT64Profile
		if !decodeJSONArg(out, `nat64 update '{"id":"...","name":"...","prefix":"..."}'`, argAt(args, 1), &p) {
			return 2
		}
		if !hasID(nat64IDs(a.GetNAT64Profiles()), p.ID) {
			out("未找到 NAT64 配置: " + p.ID)
			return 1
		}
		if err := a.UpdateNAT64Profile(p); err != nil {
			out("保存 NAT64 配置失败: " + err.Error())
			return 1
		}
		out("已更新 NAT64 配置: " + p.ID)
		reloadService(out)
		return 0
	case "delete":
		if len(args) < 2 {
			out("用法: nat64 delete <id>")
			return 2
		}
		if !hasID(nat64IDs(a.GetNAT64Profiles()), args[1]) {
			out("未找到 NAT64 配置: " + args[1])
			return 1
		}
		if err := a.DeleteNAT64Profile(args[1]); err != nil {
			out("删除 NAT64 配置失败: " + err.Error())
			return 1
		}
		out("已删除 NAT64 配置: " + args[1])
		reloadService(out)
		return 0
	case "test":
		if len(args) < 2 {
			out("用法: nat64 test <prefix>")
			return 2
		}
		latency, err := a.TestNAT64Profile(args[1])
		if err != nil {
			out("测试失败: " + err.Error())
			return 1
		}
		out(fmt.Sprintf("延迟: %d ms", latency))
		return 0
	default:
		out("用法: nat64 list|add <json>|update <json>|delete <id>|test <prefix>")
		return 2
	}
}

func opCloudflare(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch args[0] {
	case "status":
		stats := a.GetCloudflareIPStats()
		if len(stats) == 0 {
			out("CF IP 池为空")
			return 0
		}
		for _, s := range stats {
			out(fmt.Sprintf("%-18s 延迟=%-10s 失败=%d 最近=%s", s.IP, s.Latency, s.Failures, s.LastCheck))
		}
		out(fmt.Sprintf("共 %d 个 IP", len(stats)))
		return 0
	case "refresh":
		a.RefreshCloudflareIPPool()
		out("已触发 CF IP 池刷新")
		return 0
	case "fetch":
		if err := a.ForceFetchCloudflareIPs(); err != nil {
			out("拉取 CF IP 失败: " + err.Error())
			return 1
		}
		out("已重新拉取 CF IP")
		return 0
	case "prune":
		if err := a.RemoveInvalidCFIPs(); err != nil {
			out("清理无效 IP 失败: " + err.Error())
			return 1
		}
		out("已清理无效 IP")
		return 0
	case "health":
		if err := a.TriggerCFHealthCheck(); err != nil {
			out("健康检查失败: " + err.Error())
			return 1
		}
		out("已触发健康检查")
		return 0
	case "config":
		if len(args) == 1 {
			printJSON(out, a.GetCloudflareConfig())
			return 0
		}
		var cfg proxy.CloudflareConfig
		if !decodeJSONArg(out, `cf config '{"enabled":true,"preferred_ips":["1.1.1.1"]}'`, args[1], &cfg) {
			return 2
		}
		if err := a.UpdateCloudflareConfig(cfg); err != nil {
			out("保存 CF 配置失败: " + err.Error())
			return 1
		}
		out("已保存 CF 配置")
		reloadService(out)
		return 0
	default:
		out("用法: cf status|config [json]|refresh|fetch|prune|health")
		return 2
	}
}

func opRoute(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"get"}
	}

	switch args[0] {
	case "get":
		printJSON(out, a.GetAutoRoutingConfig())
		return 0
	case "set":
		var cfg proxy.AutoRoutingConfig
		if !decodeJSONArg(out, `route set '{"mode":"default"}'`, argAt(args, 1), &cfg) {
			return 2
		}
		if err := a.UpdateAutoRoutingConfig(cfg); err != nil {
			out("保存自动路由配置失败: " + err.Error())
			return 1
		}
		out("已保存自动路由配置")
		reloadService(out)
		return 0
	case "status":
		printJSON(out, a.GetAutoRoutingStatus())
		return 0
	default:
		out("用法: route get|set <json>|status")
		return 2
	}
}

func opStats(out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	stats := a.GetStats()
	out(fmt.Sprintf("下行: %s  上行: %s  其它: %s", humanSize(stats.Down), humanSize(stats.Up), humanSize(stats.Etc)))
	if c := core.NewCoreClient(); !c.Ping() {
		out("提示: 服务未运行，以上为本地统计")
	}
	return 0
}

func opIPv6(out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if a.RefreshIPv6Check() {
		out("IPv6 可用")
		return 0
	}
	out("IPv6 不可用")
	return 0
}

func opUpdate(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"check"}
	}

	switch args[0] {
	case "check":
		res := a.CheckUpdate()
		out(fmt.Sprintf("当前版本: %s（工具版本 %s，通道 %s）", a.GetCurrentVersionFull(), a.GetAppVersion(), a.GetReleaseChannel()))
		out(fmt.Sprintf("最新版本: %s", res.LatestVersion))
		if res.Message != "" {
			out("说明: " + res.Message)
		}
		if res.ErrorDetail != "" {
			out("错误: " + res.ErrorDetail)
		}
		if len(res.Assets) > 0 {
			out("可用资产:")
			for _, asset := range res.Assets {
				out(fmt.Sprintf("  %-40s %-8s %s", asset.Name, asset.Kind, humanSize(asset.Size)))
			}
		}
		if res.HasUpdate {
			out("有可用更新，执行 snishaper update download")
			return 0
		}
		out("已是最新版本")
		return 0
	case "download":
		asset, ok := resolveUpdateAsset(out, a, argAt(args, 1))
		if !ok {
			return 1
		}
		out("正在下载: " + asset.Name)
		result, err := a.DownloadUpdateAsset(asset.DownloadURL)
		if err != nil {
			out("下载失败: " + err.Error())
			return 1
		}
		out("已下载: " + result.LocalPath)
		out("安装: snishaper update install")
		return 0
	case "install":
		path := argAt(args, 1)
		if path == "" {
			path = a.GetPendingUpdate()
		}
		if path == "" {
			asset, ok := resolveUpdateAsset(out, a, "")
			if !ok {
				return 1
			}
			out("正在下载: " + asset.Name)
			result, err := a.DownloadUpdateAsset(asset.DownloadURL)
			if err != nil {
				out("下载失败: " + err.Error())
				return 1
			}
			path = result.LocalPath
			out("已下载: " + path)
		}
		out("开始安装，进程可能被替换并重启")
		if err := a.InstallUpdateAsset(path); err != nil {
			out("安装失败: " + err.Error())
			return 1
		}
		out("安装完成")
		return 0
	case "channel":
		if len(args) == 1 {
			out("当前更新通道: " + a.GetUpdateChannel())
			out("可选: " + strings.Join(app.UpdateChannels(), " / "))
			return 0
		}
		if err := a.SetUpdateChannel(args[1]); err != nil {
			out("设置更新通道失败: " + err.Error())
			out("可选: " + strings.Join(app.UpdateChannels(), " / "))
			return 1
		}
		out("更新通道已设为 " + a.GetUpdateChannel())
		return 0
	case "source":
		if len(args) == 1 {
			out("当前下载源: " + a.GetDownloadSource())
			if custom := a.GetCustomDownloadSource(); custom != "" {
				out("自定义前缀: " + custom)
			}
			out("可选: " + strings.Join(app.DownloadSources(), " / "))
			return 0
		}
		if err := a.SetDownloadSource(args[1]); err != nil {
			out("设置下载源失败: " + err.Error())
			out("可选: " + strings.Join(app.DownloadSources(), " / "))
			return 1
		}
		out("下载源已设为 " + a.GetDownloadSource())
		if len(args) >= 3 {
			if err := a.SetCustomDownloadSource(args[2]); err != nil {
				out("设置自定义前缀失败: " + err.Error())
				return 1
			}
			out("自定义前缀已设为 " + args[2])
		}
		return 0
	case "measure":
		results := a.MeasureDownloadSources()
		if len(results) == 0 {
			out("没有可测速的下载源")
			return 1
		}
		for _, r := range results {
			if r.OK {
				out(fmt.Sprintf("%-20s %6d ms  %s", r.Name, r.LatencyMS, r.URL))
				continue
			}
			out(fmt.Sprintf("%-20s %10s  %s (%s)", r.Name, "失败", r.URL, r.Error))
		}
		return 0
	default:
		out("用法: update check|download [序号或名称]|install [序号或名称或路径]|channel [名称]|source [名称] [前缀]|measure")
		return 2
	}
}

func opLogsAdmin(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		out("用法: logs [N]|clear|clean|files|show <文件名>|capture [on|off]")
		return 2
	}

	switch args[0] {
	case "clear":
		if err := a.ClearLogs(); err != nil {
			out("清空日志失败: " + err.Error())
			return 1
		}
		out("已清空内存日志缓冲")
		return 0
	case "clean":
		removed, err := a.CleanOldLogs()
		if err != nil {
			out("清理日志文件失败: " + err.Error())
			return 1
		}
		out(fmt.Sprintf("已删除 %d 个历史日志文件", removed))
		return 0
	case "files":
		files := a.GetLogFiles()
		if len(files) == 0 {
			out("没有日志文件")
			return 0
		}
		for _, file := range files {
			out(fmt.Sprintf("%-32s %s", file.Name, humanSize(file.Size)))
		}
		return 0
	case "show":
		if len(args) < 2 {
			out("用法: logs show <文件名>")
			return 2
		}
		content := a.GetLogFileContent(args[1])
		if strings.TrimSpace(content) == "" {
			out("日志文件不存在或为空: " + args[1])
			return 1
		}
		out(content)
		return 0
	case "capture":
		if len(args) < 2 {
			state := "关"
			if c := core.NewCoreClient(); c.Ping() && c.IsLogCaptureEnabled() {
				state = "开"
			}
			out("日志捕获: " + state)
			return 0
		}
		switch args[1] {
		case "on":
			if err := a.StartLogCapture(); err != nil {
				out("开启日志捕获失败: " + err.Error())
				return 1
			}
			out("已开启核心日志捕获")
			return 0
		case "off":
			if err := a.StopLogCapture(); err != nil {
				out("关闭日志捕获失败: " + err.Error())
				return 1
			}
			out("已关闭核心日志捕获")
			return 0
		default:
			out("用法: logs capture [on|off]")
			return 2
		}
	default:
		out("用法: logs [N]|clear|clean|files|show <文件名>|capture [on|off]")
		return 2
	}
}

// resolveUpdateAsset picks the asset to fetch: an explicit name or index wins,
// otherwise the best match for this platform is chosen.
func resolveUpdateAsset(out cmdOut, a *app.App, selector string) (app.ReleaseAsset, bool) {
	res := a.CheckUpdate()
	if len(res.Assets) == 0 {
		if res.ErrorDetail != "" {
			out("检查更新失败: " + res.ErrorDetail)
		} else {
			out("当前版本没有可用资产")
		}
		return app.ReleaseAsset{}, false
	}

	if selector != "" {
		if index, err := indexOf(selector); err == nil {
			if index >= 0 && index < len(res.Assets) {
				return res.Assets[index], true
			}
			out("资产序号超出范围")
			return app.ReleaseAsset{}, false
		}
		for _, asset := range res.Assets {
			if strings.EqualFold(asset.Name, selector) {
				return asset, true
			}
		}
		for _, asset := range res.Assets {
			if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(selector)) {
				return asset, true
			}
		}
		out("未找到资产: " + selector)
		return app.ReleaseAsset{}, false
	}

	preferred := "exe"
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		preferred = "tar.gz"
	}
	for _, asset := range res.Assets {
		if asset.Kind == preferred {
			return asset, true
		}
	}
	for _, asset := range res.Assets {
		if strings.Contains(strings.ToLower(asset.Name), runtime.GOOS) {
			return asset, true
		}
	}
	out("没有适配当前平台的资产，请从以下列表指定名称:")
	for _, asset := range res.Assets {
		out("  " + asset.Name)
	}
	return app.ReleaseAsset{}, false
}

func argAt(args []string, index int) string {
	if index >= len(args) {
		return ""
	}
	return args[index]
}

func indexOf(s string) (int, error) {
	var index int
	if _, err := fmt.Sscanf(s, "%d", &index); err != nil {
		return 0, err
	}
	return index, nil
}

func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
