package cliapp

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"snishaper/app"
	"snishaper/common"
	"snishaper/core"
)

// guiOnlyFirstArgs are launch flags that belong to the desktop lifecycle.
// They must never be interpreted as CLI verbs, so a stray automation launch
// (winget still passing --elevated from an older manifest, an autostart task
// with --startup) cannot turn into "unknown command" and exit 2.
var guiOnlyFirstArgs = map[string]bool{
	"--startup":      true,
	"--elevated":     true,
	"--tray-rebuild": true,
}

// cliVerbs lists every first argument the dispatcher understands, including
// the lifecycle verbs handled here. The desktop binary consults it to decide
// whether a launch is a CLI invocation or a GUI launch.
var cliVerbs = map[string]bool{
	"tui": true, "start": true, "stop": true,
	"status": true, "logs": true, "proxy": true, "sysproxy": true,
	"tun": true, "config": true, "ca": true, "sites": true,
	"upstreams": true, "dns": true, "ech": true, "nat64": true,
	"cf": true, "route": true, "stats": true, "ipv6": true,
	"update": true, "mode": true, "port": true, "socks5": true,
	"migration": true, "evolution": true, "autostart": true,
	"diag": true, "selfcheck": true,
	"version": true, "-v": true, "--version": true,
	"help": true, "-h": true, "--help": true,
}

// IsCLICommand reports whether the first argument selects a CLI verb rather
// than a desktop launch flag or the GUI itself.
func IsCLICommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if guiOnlyFirstArgs[args[0]] {
		return false
	}
	return args[0] == "--cli" || args[0] == "--serve" || cliVerbs[args[0]]
}

// Run executes the CLI dispatch. It backs both the standalone CLI build and
// the desktop binary's bundled CLI mode. headless selects the no-console
// defaults: without a terminal the interactive TUI cannot run, so the
// command help is printed and the process exits 0 instead — this is what an
// automated launcher (winget's post-install executable validation) sees when
// it starts the desktop binary in a headless session.
func Run(args []string, headless bool) int {
	if app.HasLaunchArg("--core") {
		if err := core.RunCoreMain(); err != nil {
			log.Fatal(err)
		}
		return 0
	}

	if app.HasLaunchArg("--serve") {
		runService()
		return 0
	}

	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if !guiOnlyFirstArgs[arg] && arg != "--cli" {
			filtered = append(filtered, arg)
		}
	}
	args = filtered

	if len(args) == 0 {
		if headless {
			out := func(s string) { fmt.Println(s) }
			printHelpText(out)
			return 0
		}
		runTUI()
		return 0
	}

	// Process lifecycle verbs stay at the entry: they either take over the
	// process or hand over to a long running loop, so the interactive panel
	// must not be able to reach them through the shared dispatcher.
	switch args[0] {
	case "tui":
		if headless {
			fmt.Fprintln(os.Stderr, "TUI 需要交互终端; 当前无桌面会话")
			return 1
		}
		runTUI()
		return 0
	case "start":
		cmdStart()
		return 0
	case "stop":
		cmdStop()
		return 0
	}

	out := func(s string) { fmt.Println(s) }
	return dispatchCommand(args, out)
}

// dispatchCommand runs one command and returns its exit code. It covers every
// command that only touches the service or the configuration, which lets the
// TUI expose the very same command set as the shell entry point.
func dispatchCommand(args []string, out cmdOut) int {
	if len(args) == 0 {
		printHelpText(out)
		return 0
	}

	switch args[0] {
	case "status":
		return opStatus(args[1:], out)
	case "logs":
		if len(args) > 1 {
			switch args[1] {
			case "clear", "clean", "files", "show", "capture":
				return opLogsAdmin(args[1:], out)
			}
		}
		n := 100
		if len(args) > 1 {
			if v, err := strconv.Atoi(args[1]); err == nil && v > 0 {
				n = v
			}
		}
		return opLogs(n, out)
	case "proxy":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			out("用法: snishaper proxy on|off")
			return 2
		}
		if args[1] == "on" {
			return opStartProxy(out)
		}
		return opStopProxy(out)
	case "sysproxy":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			out("用法: snishaper sysproxy on|off")
			return 2
		}
		if args[1] == "on" {
			return opEnableSysProxy(out)
		}
		return opDisableSysProxy(out)
	case "tun":
		return opTunCommand(args[1:], out)
	case "config":
		return opConfig(args[1:], out)
	case "ca":
		return opCA(args[1:], out)
	case "sites":
		return opSites(args[1:], out)
	case "upstreams":
		return opUpstreams(args[1:], out)
	case "dns":
		return opDNS(args[1:], out)
	case "ech":
		return opECH(args[1:], out)
	case "nat64":
		return opNAT64(args[1:], out)
	case "cf":
		return opCloudflare(args[1:], out)
	case "route":
		return opRoute(args[1:], out)
	case "stats":
		return opStats(out)
	case "ipv6":
		return opIPv6(out)
	case "update":
		return opUpdate(args[1:], out)
	case "mode":
		return opMode(args[1:], out)
	case "port":
		return opPort(args[1:], out)
	case "socks5":
		return opSocks5(args[1:], out)
	case "migration":
		return opMigration(args[1:], out)
	case "evolution":
		return opEvolution(args[1:], out)
	case "autostart":
		return opAutoStart(args[1:], out)
	case "diag":
		return opDiag(out)
	case "selfcheck":
		return opSelfCheck(out)
	case "version", "-v", "--version":
		out(app.VersionString())
		return 0
	case "help", "-h", "--help":
		printHelpText(out)
		return 0
	default:
		out("未知命令: " + args[0])
		printHelpText(out)
		return 2
	}
}

func runTUI() {
	if coreRunning() {
		fmt.Fprintln(os.Stderr, "服务已在运行中；请先停止: snishaper stop")
		os.Exit(1)
	}
	a := app.NewApp()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[cli] panic recovered, forcing cleanup: %v", r)
			a.ForceCleanup()
		}
	}()
	t := newTUI(a)
	if err := t.run(); err != nil {
		fmt.Fprintln(os.Stderr, "TUI error:", err)
		os.Exit(1)
	}
	a.QuitApp()
}

func runService() {
	a := app.NewApp()
	a.SetCLIMode(true)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[cli] panic recovered, forcing cleanup: %v", r)
			a.ForceCleanup()
		}
	}()
	if err := a.StartupCLI(); err != nil {
		log.Fatal(err)
	}
	writePid()
	defer removePid()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		a.QuitApp()
	}()

	monitorCore(a)

	a.QuitApp()
}

func monitorCore(a *app.App) {
	c := a.GetCore()
	everUp := false
	failStreak := time.Duration(0)
	for {
		time.Sleep(2 * time.Second)
		if a.ShouldQuit() {
			return
		}
		if c.Ping() {
			everUp = true
			failStreak = 0
			continue
		}
		if !everUp {
			continue
		}
		failStreak += 2 * time.Second
		if failStreak >= 5*time.Second {
			return
		}
	}
}

func writePid() {
	if ep, err := os.Executable(); err == nil {
		_ = os.WriteFile(filepath.Join(filepath.Dir(ep), "snishaper.pid"), []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	}
}

func removePid() {
	if ep, err := os.Executable(); err == nil {
		_ = os.Remove(filepath.Join(filepath.Dir(ep), "snishaper.pid"))
	}
}

func coreRunning() bool {
	return core.NewCoreClient().Ping()
}

func execDir() string {
	if ep, err := os.Executable(); err == nil {
		return filepath.Dir(ep)
	}
	return "."
}

func settingsPaths() (string, string) {
	dir := execDir()
	return common.ConfigSettingsPath(dir), common.ConfigRulesPath(dir)
}

func cmdStart() {
	if coreRunning() {
		fmt.Fprintln(os.Stderr, "服务已在运行中")
		os.Exit(1)
	}
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	serveArgs := []string{"--serve"}
	if app.HasLaunchArg("--autoproxy") {
		serveArgs = append(serveArgs, "--autoproxy")
	}
	cmd := exec.Command(exe, serveArgs...)
	applyDetached(cmd)
	dir := filepath.Join(filepath.Dir(exe), "log")
	_ = os.MkdirAll(dir, 0755)
	logFile, err := os.OpenFile(filepath.Join(dir, "service_stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	for i := 0; i < 75; i++ {
		if coreRunning() {
			fmt.Printf("服务已启动 (pid %d)\n", pid)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "服务 15 秒内未就绪；请检查 log/service_stdout.log")
	os.Exit(1)
}

func cmdStop() {
	c := core.NewCoreClient()
	if !c.Ping() {
		fmt.Println("服务未在运行")
		return
	}
	c.ShutdownIfRunning()
	fmt.Println("已发送停止请求，服务正在关闭")
}
