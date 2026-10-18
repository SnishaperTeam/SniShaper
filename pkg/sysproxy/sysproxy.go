// Package sysproxy 是 snishaper-core/sysproxy 的应用侧门面。
//
// 系统代理的实现在库里（Windows 注册表 / macOS networksetup / Linux
// gsettings），库中是可实例化的 Manager；本包只保留一个进程内共享的
// Manager 实例，维持原有的包级函数调用方式，让调用点无需改动。
package sysproxy

import (
	"os"

	coresysproxy "github.com/SnishaperTeam/snishaper-core/sysproxy"
)

// SystemProxyStatus 是系统代理状态，等同于库中的类型。
type SystemProxyStatus = coresysproxy.SystemProxyStatus

// manager 是本进程共享的系统代理管理器。
//
// 通过 sudo 启动时，gsettings 需要切回原用户执行；库本身不读环境变量，
// 因此由应用侧读取 SUDO_USER 注入。
var manager = coresysproxy.New(coresysproxy.WithRunAsUser(os.Getenv("SUDO_USER")))

// GetSystemProxyStatus 返回当前系统代理状态（带 2 秒缓存）。
func GetSystemProxyStatus() SystemProxyStatus { return manager.Status() }

// SetSystemProxy 启用或关闭系统代理；启用时 server 形如 127.0.0.1:7890。
func SetSystemProxy(enable bool, server string) error { return manager.Set(enable, server) }

// EnableSystemProxy 把系统代理指向本机指定端口。
func EnableSystemProxy(port int) error { return manager.Enable(port) }

// DisableSystemProxy 关闭系统代理。
func DisableSystemProxy() error { return manager.Disable() }

// SaveOriginalProxySettings 记录当前系统代理设置，供退出时恢复。
func SaveOriginalProxySettings() error { return manager.SaveOriginal() }

// RestoreOriginalProxySettings 恢复 SaveOriginalProxySettings 记录的原设置。
func RestoreOriginalProxySettings() error { return manager.RestoreOriginal() }

// SetSystemProxyManual 打开系统自带的代理设置界面。
func SetSystemProxyManual() error { return manager.Manual() }
