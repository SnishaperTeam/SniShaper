//go:build windows

package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	errUserCancelled      = errors.New("user cancelled")
	errElevationCancelled = errors.New("elevation declined")
	errInstallCancelled   = errors.New("installer cancelled")
	errRestartRequired    = errors.New("restart required")
	errIntegrity          = errors.New("integrity")
	errExtract            = errors.New("extract")
	errInstall            = errors.New("install")
	errConfig             = errors.New("config")
)

type options struct {
	configPath  string
	packagePath string
	sha256      string
	resume      string
}

func parseOptions() options {
	var o options
	flag.StringVar(&o.configPath, "config", "", "")
	flag.StringVar(&o.packagePath, "package", "", "update package downloaded by the main app")
	flag.StringVar(&o.sha256, "sha256", "", "expected sha256 of the package")
	flag.StringVar(&o.resume, "resume", "", "")
	flag.Parse()
	return o
}

func resolveBaseDir(o options, state *ResumeState) string {
	if state != nil && state.BaseDir != "" {
		return state.BaseDir
	}
	if o.configPath != "" {
		p, err := filepath.Abs(o.configPath)
		if err == nil {
			return filepath.Dir(p)
		}
	}
	exe, err := os.Executable()
	if err == nil {
		return filepath.Dir(exe)
	}
	wd, err := os.Getwd()
	if err != nil {
		return os.TempDir()
	}
	return wd
}

func resolveConfigPath(o options, state *ResumeState, baseDir string) string {
	if state != nil && state.ConfigPath != "" {
		return state.ConfigPath
	}
	if o.configPath != "" {
		p, err := filepath.Abs(o.configPath)
		if err == nil {
			return p
		}
		return o.configPath
	}
	return filepath.Join(baseDir, "updater.json")
}

func isWritable(dir string) bool {
	p := filepath.Join(dir, ".write-probe")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(p)
	return true
}

func makeWorkDir(baseDir string, state *ResumeState) (string, error) {
	if state != nil && state.WorkDir != "" {
		if err := os.MkdirAll(state.WorkDir, 0755); err == nil {
			return state.WorkDir, nil
		}
	}
	if isWritable(baseDir) {
		if d, err := os.MkdirTemp(baseDir, "updater-work-"); err == nil {
			return d, nil
		}
	}
	return os.MkdirTemp(os.TempDir(), "snishaper-updater-")
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0755)
}

func findByName(root, name string) string {
	if name == "" {
		return ""
	}
	var found string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), name) {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func maybeRelocate(o options, cfg Config, cfgPath, baseDir string, l *Logger) (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	installDir := cfg.InstallDir
	if installDir == "" {
		installDir = readInstalled(cfg).Dir
	}
	if installDir == "" || !pathWithin(exe, installDir) {
		return false, nil
	}
	stage, err := os.MkdirTemp(os.TempDir(), "snishaper-updater")
	if err != nil {
		return false, err
	}
	clone := filepath.Join(stage, filepath.Base(exe))
	if err := copyFile(exe, clone); err != nil {
		return false, err
	}
	workDir := filepath.Join(stage, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return false, err
	}
	statePath := filepath.Join(stage, "state.json")
	s := ResumeState{
		ConfigPath: cfgPath,
		BaseDir:    baseDir,
		InstallDir: installDir,
		LogPath:    l.Path(),
		WorkDir:    workDir,
	}
	if err := writeResumeState(statePath, s); err != nil {
		return false, err
	}
	// 克隆进程必须带上 -package/-sha256：run() 在缺少二者时直接退出，
	// 且状态文件里不保存它们。
	params := `--resume "` + statePath + `"`
	if o.packagePath != "" {
		params += ` -package "` + o.packagePath + `"`
	}
	if o.sha256 != "" {
		params += ` -sha256 "` + o.sha256 + `"`
	}
	if _, err := shellExecute("open", clone, params, stage, false); err != nil {
		return false, err
	}
	l.Logf("relocated updater to %s to avoid locking the install directory", clone)
	return true, nil
}

const stagedUpdaterName = "SniShaperUpdater.exe"

// scheduleUpdaterSelfUpdate 把解压目录里的新版更新器登记为下次重启时替换
// 当前更新器。整包 sha256 已由 -sha256 校验通过，包内文件无需再单独校验。
func scheduleUpdaterSelfUpdate(stage, baseDir string, l *Logger) {
	staged := findByName(stage, stagedUpdaterName)
	if staged == "" {
		return
	}
	currentExe, err := os.Executable()
	if err != nil {
		return
	}
	pending := filepath.Join(baseDir, "updater.pending.exe")
	if err := copyFile(staged, pending); err != nil {
		l.Logf("WARNING: staged updater copy failed: %v", err)
		return
	}
	if err := scheduleReplacement(pending, currentExe); err != nil {
		l.Logf("WARNING: delayed self-replacement not registered: %v", err)
		return
	}
	l.Logf("updater self-replacement scheduled for next reboot")
}

// execute 校验并安装主程序已下载好的更新包。更新器自身不发起任何网络
// 请求：下载、镜像选择、版本判断都由主程序完成。
func execute(o options, state *ResumeState, cfg Config, cfgPath, baseDir string, l *Logger) error {
	if o.resume == "" {
		relocated, err := maybeRelocate(o, cfg, cfgPath, baseDir, l)
		if err != nil {
			return err
		}
		if relocated {
			return nil
		}
	}
	installed := readInstalled(cfg)
	if state != nil && state.InstallDir != "" {
		installed.Dir = state.InstallDir
	}
	l.Logf("arch=%s install_dir=%s package=%s", resolveArch(cfg), installed.Dir, o.packagePath)

	workDir, err := makeWorkDir(baseDir, state)
	if err != nil {
		return fmt.Errorf("%w: cannot create work directory: %v", errConfig, err)
	}
	l.Logf("work directory: %s", workDir)

	if err := verifyFileSHA256(o.packagePath, o.sha256); err != nil {
		return fmt.Errorf("%w: %v", errIntegrity, err)
	}
	l.Logf("package sha256 verified")

	setupPath := o.packagePath
	portableRoot := ""
	if !strings.EqualFold(filepath.Ext(o.packagePath), ".exe") {
		stage := filepath.Join(workDir, "stage")
		if err := extractZip(o.packagePath, stage, cfg.MaxExtractBytes, 10000); err != nil {
			return fmt.Errorf("%w: %v", errExtract, err)
		}
		setupPath, err = findInstaller(stage, cfg)
		if err != nil {
			// 无安装器的扁平便携包：走与主程序 Linux 自更新一致的
			// 结束进程 + 原地覆盖流程。
			portableRoot = findPayloadRoot(stage)
			if portableRoot == "" {
				return fmt.Errorf("%w: %v", errExtract, err)
			}
			l.Logf("portable application payload located: %s", portableRoot)
		} else {
			l.Logf("installer located: %s", setupPath)
			scheduleUpdaterSelfUpdate(stage, baseDir, l)
		}
	}

	if portableRoot != "" {
		if err := installPortable(cfg, portableRoot, baseDir, l); err != nil {
			return err
		}
	} else {
		if err := runInstallerFlow(cfg, setupPath, installed, l); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(workDir); err != nil {
		l.Logf("WARNING: cleanup failed for %s: %v", workDir, err)
	} else {
		l.Logf("temporary files removed")
	}
	if state != nil {
		if exe, err := os.Executable(); err == nil {
			if err := scheduleDeletion(exe); err != nil {
				l.Logf("WARNING: cannot schedule updater clone cleanup: %v", err)
			} else {
				l.Logf("updater clone scheduled for deletion on next reboot")
			}
		}
		if o.resume != "" {
			_ = os.Remove(o.resume)
		}
	}
	return nil
}

// runInstallerFlow 是带安装器（Inno Setup）的安装路径。
func runInstallerFlow(cfg Config, setupPath string, installed installedInfo, l *Logger) error {
	if cfg.VerifyAuthenticode {
		if err := verifyAuthenticode(setupPath); err != nil {
			return fmt.Errorf("%w: %v", errIntegrity, err)
		}
		l.Logf("installer Authenticode signature verified")
	}

	if err := closeProcesses(cfg, installed.Dir, l); err != nil {
		return err
	}

	l.Logf("launching installer with runas, normal UI, no silent switches: %s", setupPath)
	code, err := shellExecute("runas", setupPath, "", filepath.Dir(setupPath), true)
	if err != nil {
		if errors.Is(err, errElevationCancelled) {
			return err
		}
		return fmt.Errorf("%w: %v", errInstall, err)
	}
	l.Logf("installer exit code %d: %s", code, innoExitText(code))
	switch code {
	case 0:
	case 2, 5:
		return errInstallCancelled
	case 8:
		return errRestartRequired
	default:
		return fmt.Errorf("%w: exit code %d (%s)", errInstall, code, innoExitText(code))
	}

	post := readInstalled(cfg)
	l.Logf("installed version %s (dir %s)", post.Version, post.Dir)
	if cfg.RestartAfter && post.Dir != "" {
		exePath := filepath.Join(post.Dir, appExeName)
		if _, err := os.Stat(exePath); err == nil {
			if _, err := shellExecute("open", exePath, "", post.Dir, false); err != nil {
				l.Logf("WARNING: relaunch failed: %v", err)
			} else {
				l.Logf("relaunched %s", exePath)
			}
		}
	}
	return nil
}

const appExeName = "snishaper.exe"

// findPayloadRoot 返回包内应用主程序所在目录；扁平便携包即解压根目录。
func findPayloadRoot(root string) string {
	if p := findByName(root, appExeName); p != "" {
		return filepath.Dir(p)
	}
	return ""
}

// installPortable 与主程序 Linux 自更新同款流程：结束应用进程后把已
// 通过整包 sha256 校验的负载原样覆盖到应用目录，再重新拉起主程序。
func installPortable(cfg Config, payloadRoot, baseDir string, l *Logger) error {
	if err := closeProcesses(cfg, "", l); err != nil {
		return err
	}
	if err := overlayTree(payloadRoot, baseDir, l); err != nil {
		return fmt.Errorf("%w: %v", errInstall, err)
	}
	l.Logf("application directory updated: %s", baseDir)
	exePath := filepath.Join(baseDir, appExeName)
	if _, err := os.Stat(exePath); err == nil {
		if _, err := shellExecute("open", exePath, "", baseDir, false); err != nil {
			l.Logf("WARNING: relaunch failed: %v", err)
		} else {
			l.Logf("relaunched %s", exePath)
		}
	}
	return nil
}

// overlayTree 把 src 下所有文件覆盖到 dst，保留 dst 中 src 未包含的文件。
// 目标被占用时短暂重试；覆盖到正在运行的更新器自身时先改名让位（运行
// 中的 exe 可改名不可写），旧文件登记为重启后删除。
func overlayTree(src, dst string, l *Logger) error {
	self, _ := os.Executable()
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if self != "" && strings.EqualFold(target, self) {
			if err := os.Rename(self, self+".old"); err != nil {
				return fmt.Errorf("rename running updater: %w", err)
			}
			if err := scheduleDeletion(self + ".old"); err != nil {
				l.Logf("WARNING: cannot schedule removal of %s: %v", self+".old", err)
			}
			l.Logf("renamed running updater for self-replacement")
		}
		var cerr error
		for i := 0; i < 5; i++ {
			cerr = copyFile(p, target)
			if cerr == nil {
				return nil
			}
			time.Sleep(300 * time.Millisecond)
		}
		return fmt.Errorf("copy %s: %w", rel, cerr)
	})
}

func exitCode(err error) int {
	switch {
	case errors.Is(err, errUserCancelled), errors.Is(err, errInstallCancelled):
		return 16
	case errors.Is(err, errElevationCancelled):
		return 14
	case errors.Is(err, errRestartRequired):
		return 17
	case errors.Is(err, errIntegrity):
		return 11
	case errors.Is(err, errExtract):
		return 12
	case errors.Is(err, errInstall):
		return 15
	case errors.Is(err, errConfig):
		return 2
	default:
		return 1
	}
}

func run() int {
	o := parseOptions()
	if o.packagePath == "" || o.sha256 == "" {
		showInfo("更新器不负责下载更新。\n请在 SniShaper 主程序里检查并下载更新，再由主程序发起安装。")
		return 2
	}
	var state *ResumeState
	if o.resume != "" {
		s, err := loadResumeState(o.resume)
		if err != nil {
			showError(fmt.Errorf("读取续跑状态失败：%v", err), "")
			return 2
		}
		state = &s
	}
	baseDir := resolveBaseDir(o, state)
	logDir := baseDir
	if state != nil && state.LogPath != "" {
		logDir = filepath.Dir(state.LogPath)
	}
	l := NewLogger(logDir)
	defer l.Close()
	l.Logf("=== updater start pid=%d resume=%t package=%s ===", os.Getpid(), o.resume != "", o.packagePath)
	cfgPath := resolveConfigPath(o, state, baseDir)
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		l.Logf("config error: %v", err)
		showError(fmt.Errorf("读取配置失败：%v", err), l.Path())
		return 2
	}
	err = execute(o, state, cfg, cfgPath, baseDir, l)
	if err == nil {
		l.Logf("=== updater finished ===")
		return 0
	}
	l.Logf("FAILED: %v (work directory retained)", err)
	if errors.Is(err, errRestartRequired) {
		showInfo("安装程序要求重启系统后才能继续。请重启后再次运行更新器。\n日志：" + l.Path())
		return exitCode(err)
	}
	if !errors.Is(err, errUserCancelled) && !errors.Is(err, errInstallCancelled) {
		showError(err, l.Path())
	}
	return exitCode(err)
}

func main() {
	os.Exit(run())
}
