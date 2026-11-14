//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Config 描述更新器的安装行为。更新器不下载任何内容，manifest/公钥/
// 版本字段已全部移除，由主程序负责版本判断、下载与镜像选择。
type Config struct {
	Arch               string   `json:"arch"`
	InstallerPattern   string   `json:"installer_pattern"`
	InstallerName      string   `json:"installer_name"`
	ProcessNames       []string `json:"process_names"`
	AppID              string   `json:"app_id"`
	InstallDir         string   `json:"install_dir"`
	VerifyAuthenticode bool     `json:"verify_authenticode"`
	RestartAfter       bool     `json:"restart_after_install"`
	MaxExtractBytes    int64    `json:"max_extract_bytes"`
}

type ResumeState struct {
	ConfigPath string `json:"config_path"`
	BaseDir    string `json:"base_dir"`
	InstallDir string `json:"install_dir"`
	LogPath    string `json:"log_path"`
	WorkDir    string `json:"work_dir"`
}

type installedInfo struct {
	Version string
	Dir     string
	Name    string
}

func loadConfig(path string) (Config, error) {
	c := Config{
		Arch:             "auto",
		InstallerPattern: "*Setup.exe",
		ProcessNames:     []string{"snishaper.exe"},
		AppID:            "{3F2E4DA1-5C8B-4ECD-BDC4-426A5965F8D4}",
		MaxExtractBytes:  1 << 30,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if err := c.validate(); err != nil {
		return c, err
	}
	return c, nil
}

func (c Config) validate() error {
	if strings.TrimSpace(c.AppID) == "" {
		return fmt.Errorf("app_id is required")
	}
	if c.MaxExtractBytes <= 0 {
		return fmt.Errorf("max_extract_bytes must be positive")
	}
	return nil
}

func resolveArch(c Config) string {
	a := strings.ToLower(strings.TrimSpace(c.Arch))
	if a != "" && a != "auto" {
		return a
	}
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	case "arm64":
		return "arm64"
	}
	return runtime.GOARCH
}

func normalizeAppID(id string) string {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, "{") {
		id = "{" + id
	}
	if !strings.HasSuffix(id, "}") {
		id = id + "}"
	}
	return id
}

func readInstalled(c Config) installedInfo {
	sub := `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\` + normalizeAppID(c.AppID) + `_is1`
	views := []struct {
		root  registry.Key
		flags uint32
	}{
		{registry.LOCAL_MACHINE, windows.KEY_WOW64_64KEY},
		{registry.LOCAL_MACHINE, windows.KEY_WOW64_32KEY},
		{registry.CURRENT_USER, windows.KEY_WOW64_64KEY},
		{registry.CURRENT_USER, windows.KEY_WOW64_32KEY},
	}
	for _, v := range views {
		k, err := registry.OpenKey(v.root, sub, registry.QUERY_VALUE|v.flags)
		if err != nil {
			continue
		}
		get := func(name string) string {
			s, _, e := k.GetStringValue(name)
			if e != nil {
				return ""
			}
			return s
		}
		info := installedInfo{
			Version: get("DisplayVersion"),
			Dir:     get("InstallLocation"),
			Name:    get("DisplayName"),
		}
		_ = k.Close()
		if info.Version != "" || info.Dir != "" {
			return info
		}
	}
	return installedInfo{}
}

func loadResumeState(path string) (ResumeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ResumeState{}, err
	}
	var s ResumeState
	if err := json.Unmarshal(data, &s); err != nil {
		return ResumeState{}, err
	}
	return s, nil
}

func writeResumeState(path string, s ResumeState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
