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

	"snishaper/cli/app"
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

	out := func(s string) { fmt.Println(s) }
	switch args[0] {
	case "tui":
		runTUI()
	case "start":
		cmdStart()
	case "stop":
		cmdStop()
	case "status":
		os.Exit(opStatus(out))
	case "logs":
		n := 100
		if len(args) > 1 {
			if v, err := strconv.Atoi(args[1]); err == nil && v > 0 {
				n = v
			}
		}
		os.Exit(opLogs(n, out))
	case "proxy":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			fmt.Fprintln(os.Stderr, "用法: snishaper proxy on|off")
			os.Exit(2)
		}
		if args[1] == "on" {
			os.Exit(opStartProxy(out))
		}
		os.Exit(opStopProxy(out))
	case "sysproxy":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			fmt.Fprintln(os.Stderr, "用法: snishaper sysproxy on|off")
			os.Exit(2)
		}
		if args[1] == "on" {
			os.Exit(opEnableSysProxy(out))
		}
		os.Exit(opDisableSysProxy(out))
	case "tun":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			fmt.Fprintln(os.Stderr, "用法: snishaper tun on|off")
			os.Exit(2)
		}
		os.Exit(opTun(args[1] == "on", out))
	case "config":
		os.Exit(opConfig(args[1:], out))
	case "ca":
		os.Exit(opCA(args[1:], out))
	case "version", "-v", "--version":
		fmt.Println(app.VersionString())
	case "help", "-h", "--help":
		printHelpText(out)
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", args[0])
		printHelpText(out)
		os.Exit(2)
	}
}

func runTUI() {
	if coreRunning() {
		fmt.Fprintln(os.Stderr, "服务已在运行中；请先停止: snishaper stop")
		os.Exit(1)
	}
	a := app.NewApp()
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
	if err := a.Startup(); err != nil {
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
