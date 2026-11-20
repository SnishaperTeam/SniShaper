//go:build linux || darwin

package common

import (
	"os"
	"path/filepath"
)

// Linux/macOS 与 Windows 一致采用便携式布局：settings/rules/proxy marker
// 都在二进制旁目录（execDir），编辑即生效。旧版的 XDG 数据
// （~/.config/snishaper 下的 config/ 与 rules/）在首次解析路径时一次性
// 迁移到 execDir（XDG 为权威源，覆盖发行包种子文件）；证书目录不迁移。
// execDir 不可写时跳过迁移并回退 XDG 路径（旧行为，优雅降级）。
// 迁移以 XDG 目录是否存在为触发条件，迁移完成后删除该目录，天然幂等。

// ConfigSettingsPath resolves the settings file path for the current platform.
func ConfigSettingsPath(execDir string) string {
	if dir, ok := portableConfigDir(execDir, "config"); ok {
		return filepath.Join(dir, "settings.json")
	}
	p, _ := EnsureUserConfig(execDir, filepath.Join("config", "settings.json"))
	return p
}

// ConfigRulesPath resolves the rules config file path for the current platform.
func ConfigRulesPath(execDir string) string {
	if dir, ok := portableConfigDir(execDir, "rules"); ok {
		return filepath.Join(dir, "config.json")
	}
	p, _ := EnsureUserConfig(execDir, filepath.Join("rules", "config.json"))
	return p
}

// ConfigCertDir resolves the certificate directory for the current platform.
// 证书保留在用户配置目录（对齐 Windows 的"配置便携、证书在用户目录"模式）。
func ConfigCertDir(_ string) string {
	return UserConfigPath("cert")
}

// ConfigProxyMarker resolves the managed system proxy marker path.
func ConfigProxyMarker(execDir string) string {
	if dir, ok := portableConfigDir(execDir, "config"); ok {
		return filepath.Join(dir, "system_proxy_owner.json")
	}
	return UserConfigPath(filepath.Join("config", "system_proxy_owner.json"))
}

// portableConfigDir 返回子目录（config/rules）的生效目录：XDG 侧存在旧
// 数据时先整体迁移到 execDir 下，迁移成功后返回 execDir 路径；execDir
// 不可写（复制失败）时回退返回 XDG 路径，保留旧行为。
func portableConfigDir(execDir, sub string) (string, bool) {
	xdgDir := filepath.Join(UserConfigDir(), sub)
	localDir := filepath.Join(execDir, sub)
	if _, err := os.Stat(xdgDir); err != nil {
		// 全新解压（无 XDG 旧数据）：直接使用便携式目录。
		return localDir, true
	}
	if copyDirContents(xdgDir, localDir) == nil {
		// 本地已是权威副本；XDG 残留（删除失败时）下次启动重试清理。
		_ = os.RemoveAll(xdgDir)
		return localDir, true
	}
	return xdgDir, false
}

// copyDirContents 把 src 的全部内容复制到 dst（已存在的文件被覆盖）。
// 任一文件失败即中止并返回错误，此时 dst 只可能多出部分已复制文件，
// src 完整无损，下次迁移可安全重试。
func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDirContents(s, d); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			return err
		}
		info, err := os.Stat(s)
		if err != nil {
			return err
		}
		if err := os.WriteFile(d, data, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}
