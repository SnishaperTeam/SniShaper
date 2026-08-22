package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"snishaper/cli/app"
)

type tuiApp struct {
	app        *app.App
	tv         *tview.Application
	logView    *tview.TextView
	statusView *tview.TextView
	input      *tview.InputField

	logMu       sync.Mutex
	pendingLogs []string
	appAnchor   string
	coreAnchor  string
}

func newTUI(a *app.App) *tuiApp {
	t := &tuiApp{app: a, tv: tview.NewApplication()}
	t.logView = tview.NewTextView().
		SetScrollable(true).
		SetMaxLines(1500)
	t.statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	t.input = tview.NewInputField().
		SetLabel("snishaper> ").
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor).
		SetPlaceholder("输入 help 查看命令")
	t.input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			cmdline := t.input.GetText()
			t.input.SetText("")
			if strings.TrimSpace(cmdline) == "" {
				return
			}
			t.execCommand(strings.TrimSpace(cmdline))
		}
	})
	t.tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			t.tv.Stop()
			return nil
		}
		return event
	})
	return t
}

func (t *tuiApp) build() tview.Primitive {
	status := tview.NewFlex().
		AddItem(t.statusView, 1, 0, false)
	inputBox := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewTextView().SetText("日志可滚动：鼠标滚轮 / PageUp / PageDown；End 回到底部；Tab 切换焦点"), 1, 0, false).
		AddItem(t.input, 1, 0, true)
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(status, 1, 0, false).
		AddItem(t.logView, 0, 1, false).
		AddItem(inputBox, 2, 0, true)
}

func (t *tuiApp) run() error {
	t.app.SetCLIMode(true)
	t.app.SetSilentStdout(true)
	if err := t.app.Startup(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}
	t.queueLog("TUI 已就绪，输入 help 查看命令")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		t.tv.Stop()
	}()

	t.tv.SetBeforeDrawFunc(func(tcell.Screen) bool {
		t.flushLogs()
		return false
	})

	go t.pollLogs()
	go t.refreshStatus()

	if err := t.tv.SetRoot(t.build(), true).EnableMouse(true).Run(); err != nil {
		return err
	}
	return nil
}

// queueLog appends a line to the pending buffer; it is flushed into the
// log view on the next draw (UI thread only), so heavy log streams never
// flood the tview event queue or touch the screen from other goroutines.
func (t *tuiApp) queueLog(line string) {
	t.logMu.Lock()
	t.pendingLogs = append(t.pendingLogs, line)
	t.logMu.Unlock()
}

func (t *tuiApp) flushLogs() {
	t.logMu.Lock()
	lines := t.pendingLogs
	t.pendingLogs = nil
	t.logMu.Unlock()
	if len(lines) == 0 {
		return
	}
	for _, l := range lines {
		fmt.Fprintln(t.logView, l)
	}
	// Auto-follow the bottom only when the user has not scrolled up to
	// inspect older lines (offset stays small near the top); scrolling up
	// pauses the follow, pressing End (or the wheel to the bottom) resumes.
	if row, _ := t.logView.GetScrollOffset(); row < 5 {
		t.logView.ScrollToEnd()
	}
}

func (t *tuiApp) pollLogs() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		if t.app == nil {
			continue
		}
		appLines := nonEmptyLines(t.app.GetAppRecentLogs(200))
		coreLines := nonEmptyLines(t.app.GetCoreRecentLogs(400))
		newLines := append(dedupLines(appLines, &t.appAnchor), dedupLines(coreLines, &t.coreAnchor)...)
		if len(newLines) == 0 {
			continue
		}
		if len(newLines) > 800 {
			newLines = newLines[len(newLines)-800:]
		}
		t.logMu.Lock()
		t.pendingLogs = append(t.pendingLogs, newLines...)
		t.logMu.Unlock()
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func dedupLines(lines []string, anchor *string) []string {
	if len(lines) == 0 {
		return nil
	}
	last := lines[len(lines)-1]
	if *anchor == "" {
		*anchor = last
		return lines
	}
	idx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == *anchor {
			idx = i
		}
	}
	if idx >= 0 {
		lines = lines[idx+1:]
	}
	*anchor = last
	return lines
}

func (t *tuiApp) refreshStatus() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		status := t.statusText()
		t.tv.QueueUpdateDraw(func() {
			t.statusView.SetText(status)
		})
	}
}

func (t *tuiApp) statusText() string {
	if t.app == nil {
		return "SniShaper"
	}
	proxyState := "关"
	if t.app.IsProxyRunning() {
		proxyState = "开"
	}
	sysProxyState := "关"
	if t.app.GetSystemProxyStatus().Enabled {
		sysProxyState = "开"
	}
	tunState := "关"
	if t.app.GetTUNStatus().Running {
		tunState = "开"
	}
	port := t.app.GetListenPort()
	mode := t.app.GetProxyMode()
	return fmt.Sprintf(" SniShaper   代理[%s] 系统代理[%s] TUN[%s]   HTTP:%d  模式:%s",
		statusColor(proxyState == "开"), statusColor(sysProxyState == "开"), statusColor(tunState == "开"), port, mode)
}

func statusColor(on bool) string {
	if on {
		return "green:开:white"
	}
	return "red:关:white"
}

func (t *tuiApp) execCommand(cmdline string) {
	fields := strings.Fields(cmdline)
	if len(fields) == 0 {
		return
	}
	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	t.queueLog("> " + cmdline)

	switch cmd {
	case "help", "h", "?", "帮助":
		t.queueLog(t.cmdHelp())
	case "start", "启动", "on", "proxyon", "代理on":
		go opStartProxy(t.queueLog)
	case "stop", "停止", "off", "proxyoff", "代理off":
		go opStopProxy(t.queueLog)
	case "proxy", "代理":
		if len(args) == 1 && (args[0] == "off" || args[0] == "停止" || args[0] == "关") {
			go opStopProxy(t.queueLog)
		} else {
			go opStartProxy(t.queueLog)
		}
	case "sysproxy", "sp", "系统代理":
		if len(args) == 0 {
			t.queueLog("用法: 系统代理 on|off 或 sysproxy on|off")
			return
		}
		switch args[0] {
		case "on", "开":
			go opEnableSysProxy(t.queueLog)
		case "off", "关":
			go opDisableSysProxy(t.queueLog)
		default:
			t.queueLog("用法: 系统代理 on|off 或 sysproxy on|off")
		}
	case "tun":
		if len(args) == 0 {
			t.queueLog("用法: tun on|off（需要管理员/root 权限）")
			return
		}
		switch args[0] {
		case "on", "开":
			go opTun(true, t.queueLog)
		case "off", "关":
			go opTun(false, t.queueLog)
		default:
			t.queueLog("用法: tun on|off（需要管理员/root 权限）")
		}
	case "ca":
		go opCA(args, t.queueLog)
	case "status", "s", "状态":
		go opStatus(t.queueLog)
	case "version", "v", "版本":
		t.queueLog("SniShaper CLI " + app.VersionString())
	case "clear", "cls", "清屏":
		t.logView.Clear()
		t.logView.ScrollToEnd()
	case "quit", "exit", "q", "退出":
		t.tv.Stop()
	default:
		t.queueLog("未知命令: " + cmd)
		t.queueLog(t.cmdHelp())
	}
}

func (t *tuiApp) cmdHelp() string {
	return `命令列表:
  start / proxy on / 启动        启动代理 (HTTP + SOCKS5)
  stop  / proxy off / 停止       停止代理
  sysproxy on|off / 系统代理     开启/关闭系统代理
  tun on|off                    切换 TUN 模式 (需要管理员/root)
  ca status                     查看根证书安装状态
  ca install                    安装根证书 (需要管理员)
  ca uninstall                  卸载根证书
  ca export                     导出 CA 证书到 ca.crt
  ca path                       显示 CA 证书路径
  ca regenerate                 重新生成根证书
  status / 状态                  查看当前状态
  clear / 清屏                   清空日志面板
  quit / exit / 退出             停止服务并退出
  help / 帮助                    显示本帮助`
}
