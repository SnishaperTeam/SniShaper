//go:build headless

package main

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

func main() {
	if app.HasLaunchArg("--core") {
		if err := core.RunCoreMain(); err != nil {
			log.Fatal(err)
		}
		return
	}

	if app.HasLaunchArg("--serve") {
		runService()
		return
	}

	args := os.Args[1:]
	if len(args) == 0 {
		runTUI()
		return
	}

	// Process lifecycle verbs stay in main: they either take over the process
	// or hand over to a long running loop, so the interactive panel must not be
	// able to reach them through the shared dispatcher.
	switch args[0] {
	case "tui":
		runTUI()
		return
	case "start":
		cmdStart()
		return
	case "stop":
		cmdStop()
		return
	}

	out := func(s string) { fmt.Println(s) }
	os.Exit(dispatchCommand(args, out))
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
		return opStatus(out)
	case "logs":
		if len(args) > 1 && (args[1] == "clear" || args[1] == "clean") {
			return opLogsAdmin(args[1:], out)
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
	cmd := exec.Command(exe, "--serve")
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
