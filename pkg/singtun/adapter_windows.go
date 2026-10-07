//go:build windows

package singtun

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var netClassGUID = windows.GUID{
	Data1: 0x4d36e972,
	Data2: 0xe325,
	Data3: 0x11ce,
	Data4: [8]byte{0xbf, 0xc1, 0x08, 0x00, 0x2b, 0xe1, 0x03, 0x18},
}

var (
	setupapiDLL                      = windows.NewLazySystemDLL("setupapi.dll")
	procSetupDiSetClassInstallParams = setupapiDLL.NewProc("SetupDiSetClassInstallParamsW")
	procSetupDiCallClassInstaller    = setupapiDLL.NewProc("SetupDiCallClassInstaller")
)

// matchesSniShaperAdapter 判断某个网卡设备是否由 SniShaper 自己创建。
//
// 必须严格限定在本项目创建的适配器上：sing-tun 与 sing-box / mihomo 共用同一套
// Wintun 驱动，设备描述同样含 "sing-tun"。若仅凭描述匹配，用户同时运行
// sing-box 时，SniShaper 每次启停都会把对方的网卡删掉。
// 判定依据是本项目独有的名称（SniShaper 网卡名）或 wintun 描述 + SniShaper 友好名。
func matchesSniShaperAdapter(desc string, friendly string) bool {
	d := strings.ToLower(desc)
	f := strings.ToLower(friendly)
	// 本项目自有命名（网卡片 / 友好名 / 描述）
	if strings.Contains(d, "snishaper") || strings.Contains(f, "snishaper") {
		return true
	}
	// Wintun 驱动 + 本项目友好名：驱动描述已漂移为 "sing-tun Tunnel"，
	// 因此必须同时要求友好名为 SniShaper，避免误删其他 sing-tun 用户的网卡。
	return strings.Contains(d, "wintun") && f == "snishaper"
}

// cleanupStaleAdapters 移除本项目遗留的虚拟网卡，返回移除数量。
// 返回值供启动清理阶段记入日志（US1：用户需在启动日志中看到"清了几个"）。
func cleanupStaleAdapters(logf func(string)) int {
	devInfo, err := windows.SetupDiGetClassDevsEx(&netClassGUID, "", 0, windows.DIGCF_PRESENT, 0, "")
	if err != nil {
		logf("[sing-tun] stale adapter scan unavailable: " + err.Error())
		return 0
	}
	defer devInfo.Close()
	removed := 0
	for i := 0; ; i++ {
		data, err := windows.SetupDiEnumDeviceInfo(devInfo, i)
		if err != nil {
			break
		}
		desc := deviceStringProperty(devInfo, data, windows.SPDRP_DEVICEDESC)
		friendly := deviceStringProperty(devInfo, data, windows.SPDRP_FRIENDLYNAME)
		if !matchesSniShaperAdapter(desc, friendly) {
			continue
		}
		label := friendly
		if label == "" {
			label = desc
		}
		if removeNetDevice(devInfo, data) {
			removed++
			logf("[sing-tun] removed stale adapter: " + label)
		} else {
			logf("[sing-tun] failed to remove stale adapter: " + label)
		}
	}
	if removed > 0 {
		logf("[sing-tun] stale adapter cleanup removed " + fmt.Sprint(removed) + " device(s)")
	}
	return removed
}

func deviceStringProperty(devInfo windows.DevInfo, data *windows.DevInfoData, property windows.SPDRP) string {
	value, err := windows.SetupDiGetDeviceRegistryProperty(devInfo, data, property)
	if err != nil {
		return ""
	}
	s, _ := value.(string)
	return s
}

func removeNetDevice(devInfo windows.DevInfo, data *windows.DevInfoData) bool {
	params := &windows.RemoveDeviceParams{
		ClassInstallHeader: *windows.MakeClassInstallHeader(windows.DIF_REMOVE),
		Scope:              windows.DI_REMOVEDEVICE_GLOBAL,
	}
	r1, _, _ := procSetupDiSetClassInstallParams.Call(
		uintptr(devInfo),
		uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(params)),
		unsafe.Sizeof(*params),
	)
	if r1 == 0 {
		return false
	}
	r1, _, _ = procSetupDiCallClassInstaller.Call(
		uintptr(windows.DIF_REMOVE),
		uintptr(devInfo),
		uintptr(unsafe.Pointer(data)),
	)
	return r1 != 0
}
