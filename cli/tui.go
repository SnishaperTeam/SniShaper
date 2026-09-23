//go:build headless

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

	"snishaper/app"
)

const catalogPage = "catalog"

type tuiApp struct {
	app        *app.App
	tv         *tview.Application
	pages      *tview.Pages
	logView    *tview.TextView
	statusView *tview.TextView
	catalog    *tview.TextView
	input      *tview.InputField

	logMu       sync.Mutex
	pendingLogs []string
	appAnchor   string
	coreAnchor  string

	screenW int
	screenH int
}

func newTUI(a *app.App) *tuiApp {
	t := &tuiApp{app: a, tv: tview.NewApplication()}
	t.logView = tview.NewTextView().
		SetScrollable(true).
		SetMaxLines(1500)
	t.statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	t.catalog = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	t.catalog.SetBorder(true).SetTitle(" 命令目录（Esc 关闭，↑↓ 滚动） ")
	t.pages = tview.NewPages()
	t.input = tview.NewInputField().
		SetLabel("snishaper> ").
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor).
		SetPlaceholder("输入 目录 或 F1 查看命令")
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
		switch event.Key() {
		case tcell.KeyCtrlC:
			t.tv.Stop()
			return nil
		case tcell.KeyF1:
			t.toggleCatalog()
			return nil
		case tcell.KeyEscape:
			if t.pages.HasPage(catalogPage) {
				t.closeCatalog()
				return nil
			}
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
		AddItem(tview.NewTextView().SetText("日志可滚动：滚轮 / PageUp / PageDown；End 到底部；F1 命令目录；Tab 切换焦点"), 1, 0, false).
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
	if err := t.app.StartupCLI(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}
	t.queueLog("TUI 已就绪，输入 help 查看命令")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		t.tv.Stop()
	}()

	t.tv.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		if w, h := screen.Size(); w != t.screenW || h != t.screenH {
			t.screenW, t.screenH = w, h
		}
		t.flushLogs()
		return false
	})

	go t.pollLogs()
	go t.refreshStatus()

	t.pages.AddPage("main", t.build(), true, true)
	if err := t.tv.SetRoot(t.pages, true).EnableMouse(true).Run(); err != nil {
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

// commandAliases maps the Chinese verbs the panel accepts onto the command
// names the shared dispatcher knows.
var commandAliases = map[string]string{
	"状态": "status", "证书": "ca", "配置": "config", "版本": "version",
	"规则": "sites", "站点": "sites", "上游": "upstreams", "节点": "dns",
	"路由": "route", "统计": "stats", "更新": "update", "日志": "logs",
}

func (t *tuiApp) openCatalog() {
	var b strings.Builder
	for _, group := range commandCatalog() {
		fmt.Fprintf(&b, "[yellow::b]%s[-:-:-]\n", group.Title)
		for _, item := range group.Items {
			fmt.Fprintf(&b, "  [white]%s[gray]  %s[-]\n", item.Usage, item.Desc)
		}
		b.WriteString("\n")
	}
	t.catalog.SetText(b.String())
	t.catalog.ScrollToBeginning()
	t.pages.AddPage(catalogPage, t.catalogModal(), true, true)
}

func (t *tuiApp) closeCatalog() {
	t.pages.RemovePage(catalogPage)
}

func (t *tuiApp) toggleCatalog() {
	if t.pages.HasPage(catalogPage) {
		t.closeCatalog()
		return
	}
	t.openCatalog()
}

// catalogModal centres the command catalogue so it reads as a panel instead of
// taking over the log area.
func (t *tuiApp) catalogModal() tview.Primitive {
	width, height := t.screenW, t.screenH
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	panelWidth := width - 4
	if panelWidth > 100 {
		panelWidth = 100
	}
	panelHeight := height - 4
	if panelHeight > 30 {
		panelHeight = 30
	}
	if panelWidth < 20 {
		panelWidth = 20
	}
	if panelHeight < 6 {
		panelHeight = 6
	}
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(t.catalog, panelHeight, 1, true).
			AddItem(nil, 0, 1, false), panelWidth, 1, true).
		AddItem(nil, 0, 1, false)
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
	case "help", "h", "?", "帮助", "目录", "catalog":
		// execCommand already runs on the event loop, so the page change is
		// applied directly: QueueUpdateDraw from inside the loop would wait for
		// the loop to process the queued function and deadlock the panel.
		t.openCatalog()
		return
	case "start", "启动", "on", "proxyon", "代理on":
		go opStartProxy(t.queueLog)
		return
	case "stop", "停止", "off", "proxyoff", "代理off":
		go opStopProxy(t.queueLog)
		return
	case "proxy", "代理":
		if len(args) == 1 && (args[0] == "off" || args[0] == "停止" || args[0] == "关") {
			go opStopProxy(t.queueLog)
			return
		}
		go opStartProxy(t.queueLog)
		return
	case "sysproxy", "sp", "系统代理":
		if len(args) == 0 {
			t.queueLog("用法: sysproxy on|off")
			return
		}
		switch args[0] {
		case "on", "开":
			go opEnableSysProxy(t.queueLog)
		case "off", "关":
			go opDisableSysProxy(t.queueLog)
		default:
			t.queueLog("用法: sysproxy on|off")
		}
		return
	case "clear", "cls", "清屏":
		t.logView.Clear()
		t.logView.ScrollToEnd()
		return
	case "quit", "exit", "q", "退出":
		t.tv.Stop()
		return
	}

	if alias, ok := commandAliases[cmd]; ok {
		cmd = alias
		fields[0] = alias
	}

	// Everything else goes through the same dispatcher the shell entry point
	// uses, so the panel exposes exactly the same command set.
	if !isKnownCommand(cmd) {
		t.queueLog("未知命令: " + cmd + "（按 F1 或输入 目录 查看命令目录）")
		return
	}
	go func() {
		if code := dispatchCommand(fields, t.queueLog); code != 0 {
			t.queueLog(fmt.Sprintf("命令退出码: %d", code))
		}
	}()
}

// isKnownCommand reports whether the verb appears in the command catalogue.
func isKnownCommand(verb string) bool {
	for _, group := range commandCatalog() {
		for _, item := range group.Items {
			if name, _, ok := strings.Cut(item.Usage, " "); ok && name == verb {
				return true
			}
			if item.Usage == verb {
				return true
			}
		}
	}
	return false
}
