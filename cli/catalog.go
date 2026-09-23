//go:build headless

package main

// commandEntry describes one entry of the CLI command catalogue. The catalogue
// is the single source of truth for the plain help output and for the TUI
// command panel, so a new command only has to be added here once.
type commandEntry struct {
	Usage string
	Desc  string
}

type commandGroup struct {
	Title string
	Items []commandEntry
}

func commandCatalog() []commandGroup {
	return []commandGroup{
		{
			Title: "服务状态",
			Items: []commandEntry{
				{"status", "查看服务 / 代理 / 系统代理 / TUN 状态"},
				{"stats", "查看上下行流量统计"},
				{"ipv6", "检测 IPv6 可用性"},
				{"diag", "输出诊断信息（版本/端口/系统代理/TUN/路径，JSON）"},
				{"selfcheck", "端到端自检：服务、监听端口、经代理请求、系统代理与 TUN"},
				{"logs [N]", "打印最近 N 行日志（默认 100）"},
				{"logs clear", "清空内存日志缓冲"},
				{"logs clean", "删除历史日志文件（保留当前）"},
				{"logs files", "列出历史日志文件"},
				{"logs show <文件名>", "查看某个日志文件的内容"},
				{"logs capture [on|off]", "查看 / 切换核心日志捕获"},
			},
		},
		{
			Title: "代理",
			Items: []commandEntry{
				{"proxy on|off", "启动 / 停止代理（HTTP + SOCKS5）"},
				{"sysproxy on|off", "开启 / 关闭系统代理"},
				{"mode get|set <模式>", "查看 / 切换代理模式（需服务在运行）"},
				{"port get|set <端口>", "查看 / 修改 HTTP 监听端口"},
				{"port occupant [端口]", "查看端口被哪个进程占用"},
				{"port kill <pid>", "结束占用端口的进程"},
				{"socks5 status|on|off|port <端口>", "SOCKS5 开关与端口"},
				{"migration status|on|off|server|test", "迁移模式与迁移服务器"},
			},
		},
		{
			Title: "TUN",
			Items: []commandEntry{
				{"tun on", "启动 TUN（先关闭本程序设置的系统代理）"},
				{"tun off", "停止 TUN（恢复之前关闭的系统代理）"},
				{"tun status", "查看 TUN 状态"},
				{"tun config [json]", "查看 / 修改 TUN 配置（mtu / dns_hijack / auto_route）"},
			},
		},
		{
			Title: "证书",
			Items: []commandEntry{
				{"ca status", "查看根证书安装状态"},
				{"ca install", "安装根证书到系统信任库（需要管理员 / root）"},
				{"ca uninstall", "卸载已安装的根证书"},
				{"ca list", "列出系统中本程序安装的根证书"},
				{"ca pem", "输出 CA 证书 PEM 内容"},
				{"ca export", "导出 CA 证书到 ca.crt"},
				{"ca path", "显示 CA 证书文件路径"},
				{"ca regenerate", "重新生成根证书（之后需重新安装）"},
			},
		},
		{
			Title: "规则与配置",
			Items: []commandEntry{
				{"config get [key]", "读取 settings.json（不带 key 时整体输出）"},
				{"config set <key> <value>", "修改 settings.json"},
				{"config export [path]", "导出规则与设置（缺省输出到标准输出）"},
				{"config import <path>", "导入规则与设置"},
				{"sites list|show|add|update|delete", "站点组规则（add / update 接受 JSON）"},
				{"upstreams list|show|add|update|delete", "上游配置（add / update 接受 JSON）"},
				{"dns list|show|add|update|delete|priority|test", "DNS 节点"},
				{"ech list|upsert|fetch|delete", "ECH 配置（fetch 从域名拉取新配置）"},
				{"nat64 list|add|update|delete|test", "NAT64 配置"},
				{"cf status|config|refresh|fetch|prune|health", "Cloudflare IP 池与配置"},
				{"evolution status|start|stop|apply", "规则进化测试（start 会等到测试结束）"},
				{"route get|set|status", "自动路由配置"},
			},
		},
		{
			Title: "更新",
			Items: []commandEntry{
				{"update check", "检查新版本并列出可用资产"},
				{"update download [序号|名称]", "下载更新（缺省选当前平台适配的资产）"},
				{"update install [序号|名称|路径]", "安装更新（进程可能被替换并重启）"},
				{"update channel [名称]", "查看 / 切换更新通道"},
				{"update source [名称] [前缀]", "查看 / 切换下载源与自定义前缀"},
				{"update measure", "测试各下载源延迟"},
			},
		},
		{
			Title: "其他",
			Items: []commandEntry{
				{"tui", "进入交互面板"},
				{"start [--autoproxy] | stop", "后台启动 / 停止常驻服务（--autoproxy 启动后自动开代理）"},
				{"autostart status|on [--with-proxy]|off", "命令行服务的开机自启（独立条目，不影响桌面端）"},
				{"version", "打印版本号"},
				{"help", "显示本目录"},
			},
		},
	}
}

func printHelpText(out cmdOut) {
	out("用法:")
	out("  snishaper                启动交互面板（TUI）")
	out("  snishaper <命令> [参数]  执行单条命令，见下方命令目录")
	out("")
	out("命令目录:")
	for _, group := range commandCatalog() {
		out("【" + group.Title + "】")
		for _, item := range group.Items {
			out("  " + item.Usage)
			out("      " + item.Desc)
		}
	}
	out("")
	out("写操作大多接受 JSON 参数，例如:")
	out(`  snishaper sites add '{"name":"示例","mode":"direct","domains":["a.com"]}'`)
	out(`  snishaper dns add '{"name":"CF","url":"https://1.1.1.1/dns-query","enabled":true}'`)
}
