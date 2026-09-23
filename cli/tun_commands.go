//go:build headless

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"snishaper/core"
	"snishaper/pkg/sysproxy"
	"snishaper/proxy"
)

// TUN commands mirror the current app level TUN flow one to one:
//
//	start: disable the managed system proxy, then ask the core to bring TUN up
//	stop:  stop TUN, then restore the managed system proxy that start disabled
//
// The app keeps that restore flag in memory because it stays alive; a one shot
// CLI process cannot, so the flag is persisted next to the managed system proxy
// marker instead. When the TUN implementation changes, revisit this file: it is
// the only CLI place that knows about the TUN/system proxy coupling.

func tunRestoreMarkerPath() string {
	return filepath.Join(filepath.Dir(proxyMarkerPath()), "tun_restore_sysproxy.marker")
}

func proxyMarkerPath() string {
	settingsPath, _ := settingsPaths()
	if settingsPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(settingsPath), "system_proxy_owner.json")
}

func markTunRestorePending() error {
	path := tunRestoreMarkerPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().Format(time.RFC3339)+"\n"), 0644)
}

func clearTunRestorePending() {
	_ = os.Remove(tunRestoreMarkerPath())
}

func tunRestorePending() bool {
	_, err := os.Stat(tunRestoreMarkerPath())
	return err == nil
}

// waitForSystemProxy waits until the system proxy reaches the wanted state. The
// app applies those changes on a background goroutine, which a short lived CLI
// process has to wait for before it exits.
func waitForSystemProxy(wantEnabled bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if sysproxy.GetSystemProxyStatus().Enabled == wantEnabled {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func opTunCommand(args []string, out cmdOut) int {
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch args[0] {
	case "on":
		return tunStart(out)
	case "off":
		return tunStop(out)
	case "status":
		return tunStatus(out)
	case "config":
		return tunConfig(args[1:], out)
	default:
		out("用法: tun on|off|status|config [json]")
		return 2
	}
}

func tunStart(out cmdOut) int {
	c := opRequireService(out)
	if c == nil {
		return 1
	}
	a := configApp(out)
	if a == nil {
		return 1
	}

	// TUN and a system proxy on the same port fight over the traffic path, so
	// the managed proxy goes down first and is remembered for tun off.
	if a.IsManagedSystemProxy() {
		out("检测到本程序设置的系统代理，先关闭它（TUN 与系统代理互斥）")
		if err := a.DisableSystemProxy(); err != nil {
			out("关闭系统代理失败: " + err.Error())
			return 1
		}
		if !waitForSystemProxy(false, 3*time.Second) {
			out("系统代理仍未关闭，已中止以免 TUN 启动后流量异常")
			return 1
		}
		if err := markTunRestorePending(); err != nil {
			out("记录系统代理恢复状态失败: " + err.Error())
		}
	}

	if err := c.StartTUN(); err != nil {
		out("启动 TUN 失败: " + err.Error())
		return 1
	}
	return tunStatus(out)
}

func tunStop(out cmdOut) int {
	c := opRequireService(out)
	if c == nil {
		return 1
	}

	if err := c.StopTUN(); err != nil {
		out("停止 TUN 失败: " + err.Error())
		return 1
	}
	out("TUN 已停止")

	if !tunRestorePending() {
		return 0
	}

	a := configApp(out)
	if a == nil {
		return 1
	}
	if err := a.EnableSystemProxy(); err != nil {
		out("恢复系统代理失败: " + err.Error())
		out("可手动执行: snishaper sysproxy on")
		return 1
	}
	if waitForSystemProxy(true, 3*time.Second) {
		out("已恢复之前关闭的系统代理")
		clearTunRestorePending()
		return 0
	}
	out("系统代理已提交但尚未生效，可执行 snishaper sysproxy on 重试")
	return 1
}

func tunStatus(out cmdOut) int {
	status := coreTUNStatus()
	out(fmt.Sprintf("支持: %v   运行: %v   驱动: %s", status.Supported, status.Running, status.Driver))
	if status.Message != "" {
		out("说明: " + status.Message)
	}
	return 0
}

func coreTUNStatus() proxy.TUNStatus {
	c := core.NewCoreClient()
	if !c.Ping() {
		return proxy.TUNStatus{Message: "服务未在运行"}
	}
	return c.GetTUNStatus()
}

func tunConfig(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		printJSON(out, a.GetTUNConfig())
		return 0
	}

	var payload tunConfigPayload
	if !decodeJSONArg(out, `tun config '{"mtu":9000,"dns_hijack":true,"auto_route":true}'`, args[0], &payload) {
		return 2
	}

	current := a.GetTUNConfig()
	if payload.Enabled != nil {
		current.Enabled = *payload.Enabled
	}
	if payload.MTU != nil {
		current.MTU = *payload.MTU
	}
	if payload.DNSHijack != nil {
		current.DNSHijack = *payload.DNSHijack
	}
	if payload.AutoRoute != nil {
		current.AutoRoute = *payload.AutoRoute
	}
	if payload.StrictRoute != nil {
		current.StrictRoute = *payload.StrictRoute
	}

	if err := a.UpdateTUNConfig(current); err != nil {
		out("保存 TUN 配置失败: " + err.Error())
		return 1
	}
	out("已保存 TUN 配置")
	printJSON(out, current)
	reloadService(out)
	return 0
}

// tunConfigPayload uses pointers so a partial update cannot silently switch
// unset options off.
type tunConfigPayload struct {
	Enabled     *bool `json:"enabled,omitempty"`
	MTU         *int  `json:"mtu,omitempty"`
	DNSHijack   *bool `json:"dns_hijack,omitempty"`
	AutoRoute   *bool `json:"auto_route,omitempty"`
	StrictRoute *bool `json:"strict_route,omitempty"`
}
