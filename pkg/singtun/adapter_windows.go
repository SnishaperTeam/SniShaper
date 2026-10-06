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

func matchesSniShaperAdapter(desc string, friendly string) bool {
	d := strings.ToLower(desc)
	f := strings.ToLower(friendly)
	if strings.Contains(d, "sing-tun") || strings.Contains(d, "snishaper") || strings.Contains(f, "snishaper") {
		return true
	}
	return strings.Contains(d, "wintun") && f == "snishaper"
}

func cleanupStaleAdapters(logf func(string)) {
	devInfo, err := windows.SetupDiGetClassDevsEx(&netClassGUID, "", 0, windows.DIGCF_PRESENT, 0, "")
	if err != nil {
		logf("[sing-tun] stale adapter scan unavailable: " + err.Error())
		return
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
