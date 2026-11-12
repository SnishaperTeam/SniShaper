//go:build windows

package main

import (
	"context"
	"encoding/json"
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
	errNoUpdate           = errors.New("no update")
	errNetwork            = errors.New("network")
	errIntegrity          = errors.New("integrity")
	errExtract            = errors.New("extract")
	errInstall            = errors.New("install")
	errConfig             = errors.New("config")
)

type options struct {
	configPath string
	resume     string
	checkOnly  bool
	force      bool
}

func parseOptions() options {
	var o options
	flag.StringVar(&o.configPath, "config", "", "")
	flag.StringVar(&o.resume, "resume", "", "")
	flag.BoolVar(&o.checkOnly, "check-only", false, "")
	flag.BoolVar(&o.force, "force", false, "")
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

func resolvePackageURL(manifestURL, u string) string {
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return u
	}
	base := manifestURL
	if i := strings.Index(base, "?"); i >= 0 {
		base = base[:i]
	}
	if i := strings.LastIndex(base, "/"); i >= 0 {
		return base[:i+1] + strings.TrimLeft(u, "/")
	}
	return u
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

func maybeRelocate(cfg Config, cfgPath, baseDir string, l *Logger) (bool, error) {
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
	stage, err := os.MkdirTemp(os.TempDir(), "snishaper-updater-")
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
	if _, err := shellExecute("open", clone, `--resume "`+statePath+`"`, stage, false); err != nil {
		return false, err
	}
	l.Logf("relocated updater to %s to avoid locking the install directory", clone)
	return true, nil
}

func execute(o options, state *ResumeState, cfg Config, cfgPath, baseDir string, l *Logger) error {
	if o.resume == "" {
		relocated, err := maybeRelocate(cfg, cfgPath, baseDir, l)
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
	local := localVersion(cfg, baseDir, installed)
	l.Logf("arch=%s channel=%s local=%s install_dir=%s", resolveArch(cfg), cfg.Channel, local, installed.Dir)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.HTTPTimeoutMinutes*2)*time.Minute)
	defer cancel()
	h := newHTTPClient(cfg, l)
	body, err := h.get(ctx, cfg.ManifestURL, 4<<20)
	if err != nil {
		return fmt.Errorf("%w: manifest download: %v", errNetwork, err)
	}
	if cfg.PublicKey != "" {
		sig, sigErr := h.get(ctx, cfg.ManifestURL+".sig", 4096)
		if sigErr != nil {
			return fmt.Errorf("%w: manifest signature download: %v", errIntegrity, sigErr)
		}
		if err := verifyManifestSignature(body, string(sig), cfg.PublicKey); err != nil {
			return fmt.Errorf("%w: %v", errIntegrity, err)
		}
		l.Logf("manifest Ed25519 signature verified")
	} else {
		l.Logf("WARNING: public_key empty, manifest signature check skipped")
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return fmt.Errorf("%w: invalid manifest: %v", errConfig, err)
	}
	if m.Version == "" || m.Package.URL == "" || m.Package.SHA256 == "" {
		return fmt.Errorf("%w: manifest missing version/package url/sha256", errConfig)
	}
	if cfg.Channel != "" && m.Channel != "" && !strings.EqualFold(cfg.Channel, m.Channel) {
		l.Logf("manifest channel %s does not match configured %s", m.Channel, cfg.Channel)
		return errNoUpdate
	}
	if compareVersions(local, m.Version) >= 0 && !o.force {
		l.Logf("already up to date: local %s >= remote %s", local, m.Version)
		return errNoUpdate
	}
	l.Logf("update available: %s -> %s", local, m.Version)
	if o.checkOnly {
		showInfo("发现新版本 " + m.Version + "。运行更新器（不带 -check-only）即可安装。")
		return nil
	}

	workDir, err := makeWorkDir(baseDir, state)
	if err != nil {
		return fmt.Errorf("%w: cannot create work directory: %v", errConfig, err)
	}
	l.Logf("work directory: %s", workDir)

	pkgPath := filepath.Join(workDir, safeFileName(m.Package.Name))
	if err := h.download(ctx, resolvePackageURL(cfg.ManifestURL, m.Package.URL), pkgPath, m.Package.Size); err != nil {
		return fmt.Errorf("%w: package download: %v", errNetwork, err)
	}
	if err := verifyFileSHA256(pkgPath, m.Package.SHA256); err != nil {
		return fmt.Errorf("%w: %v", errIntegrity, err)
	}
	l.Logf("package sha256 verified")

	stage := filepath.Join(workDir, "stage")
	if err := extractZip(pkgPath, stage, cfg.MaxExtractBytes, 10000); err != nil {
		return fmt.Errorf("%w: %v", errExtract, err)
	}
	if m.Installer.Name != "" {
		cfg.InstallerName = m.Installer.Name
	}
	setupPath, err := findInstaller(stage, cfg)
	if err != nil {
		return fmt.Errorf("%w: %v", errExtract, err)
	}
	l.Logf("installer located: %s", setupPath)
	if m.Installer.SHA256 != "" {
		if err := verifyFileSHA256(setupPath, m.Installer.SHA256); err != nil {
			return fmt.Errorf("%w: %v", errIntegrity, err)
		}
		l.Logf("installer sha256 verified")
	}
	if cfg.VerifyAuthenticode {
		if err := verifyAuthenticode(setupPath); err != nil {
			return fmt.Errorf("%w: %v", errIntegrity, err)
		}
		l.Logf("installer Authenticode signature verified")
	}

	if m.Updater.Name != "" {
		stagedUpdater := findByName(stage, m.Updater.Name)
		if stagedUpdater == "" {
			l.Logf("WARNING: staged updater %q not found, self-update skipped", m.Updater.Name)
		} else if m.Updater.SHA256 == "" {
			l.Logf("WARNING: staged updater has no sha256, self-update skipped")
		} else if err := verifyFileSHA256(stagedUpdater, m.Updater.SHA256); err != nil {
			return fmt.Errorf("%w: %v", errIntegrity, err)
		} else {
			currentExe, err := os.Executable()
			if err != nil {
				return err
			}
			pending := filepath.Join(baseDir, "updater.pending.exe")
			if err := copyFile(stagedUpdater, pending); err != nil {
				return err
			}
			if err := scheduleReplacement(pending, currentExe); err != nil {
				l.Logf("WARNING: delayed self-replacement not registered: %v", err)
			} else {
				l.Logf("updater self-replacement scheduled for next reboot")
			}
		}
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
	if post.Version != "" && !versionEqual(post.Version, m.Version) {
		l.Logf("WARNING: registry DisplayVersion=%s, manifest version=%s", post.Version, m.Version)
	} else {
		l.Logf("registry version verified: %s (dir %s)", post.Version, post.Dir)
	}
	if cfg.RestartAfter && post.Dir != "" {
		exePath := filepath.Join(post.Dir, "snishaper.exe")
		if _, err := os.Stat(exePath); err == nil {
			if _, err := shellExecute("open", exePath, "", post.Dir, false); err != nil {
				l.Logf("WARNING: relaunch failed: %v", err)
			} else {
				l.Logf("relaunched %s", exePath)
			}
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

func exitCode(err error) int {
	switch {
	case errors.Is(err, errUserCancelled), errors.Is(err, errInstallCancelled):
		return 16
	case errors.Is(err, errElevationCancelled):
		return 14
	case errors.Is(err, errRestartRequired):
		return 17
	case errors.Is(err, errNetwork):
		return 10
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
	l.Logf("=== updater start pid=%d resume=%t ===", os.Getpid(), o.resume != "")
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
	if errors.Is(err, errNoUpdate) {
		l.Logf("=== no update ===")
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
