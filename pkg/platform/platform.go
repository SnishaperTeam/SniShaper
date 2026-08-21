//go:build linux

// Package platform provides Linux-native system primitives that the TUN,
// routing and DNS layers of SniShaper are built on. It is the single
// platform layer of the project: everything here targets Linux only.
//
// The TUN data plane used by the app is sing-tun, which internally creates
// the device exactly the way OpenTun below does (open /dev/net/tun, ioctl
// TUNSETIFF) and manages routes/DNS through netlink and resolvectl. The
// helpers in this package are kept as the standalone building blocks for the
// operations the app performs directly on the system (elevated commands,
// opening files, owned DNS configuration).
package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func init() {
	// Can only ever build and run on Linux.
	if runtime.GOOS != "linux" {
		panic("snishaper/pkg/platform must only be built for linux")
	}
}

// OpenTun opens /dev/net/tun and attaches a TUN device with the given name,
// returning the file descriptor wrapped in *os.File. Callers must be root
// (CAP_NET_ADMIN) for TUNSETIFF to succeed.
func OpenTun(name string) (*os.File, error) {
	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/net/tun: %w", err)
	}
	ifr, err := unix.NewIfreq(name)
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	ifr.SetUint16(uint16(unix.IFF_TUN | unix.IFF_NO_PI))
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TUNSETIFF), uintptr(unsafe.Pointer(ifr)))
	if errno != 0 {
		unix.Close(fd)
		return nil, os.NewSyscallError("ioctl TUNSETIFF", errno)
	}
	return os.NewFile(uintptr(fd), name), nil
}

// AddRoute installs a route to dst (e.g. "0.0.0.0/0") via gateway through
// device. Needs root.
func AddRoute(dst, gateway, device string) error {
	link, err := netlink.LinkByName(device)
	if err != nil {
		return fmt.Errorf("find link %s: %w", device, err)
	}
	_, dstNet, err := net.ParseCIDR(dst)
	if err != nil {
		return fmt.Errorf("parse dst %s: %w", dst, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dstNet,
		Gw:        net.ParseIP(gateway).To4(),
	}
	if route.Gw == nil {
		route.Gw = net.ParseIP(gateway)
	}
	if err := netlink.RouteAdd(route); err != nil && !strings.Contains(err.Error(), "exists") {
		return fmt.Errorf("netlink.RouteAdd: %w", err)
	}
	return nil
}

// DelRoute removes the route previously installed by AddRoute. Needs root.
func DelRoute(dst, gateway, device string) error {
	link, err := netlink.LinkByName(device)
	if err != nil {
		return fmt.Errorf("find link %s: %w", device, err)
	}
	_, dstNet, err := net.ParseCIDR(dst)
	if err != nil {
		return fmt.Errorf("parse dst %s: %w", dst, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dstNet,
		Gw:        net.ParseIP(gateway).To4(),
	}
	if route.Gw == nil {
		route.Gw = net.ParseIP(gateway)
	}
	return netlink.RouteDel(route)
}

// RunElevated runs a command with root privileges. When the current process
// is already root it runs directly; otherwise it falls back to pkexec
// (polkit GUI prompt) and then sudo.
func RunElevated(name string, args ...string) error {
	if os.Geteuid() == 0 {
		return exec.Command(name, args...).Run()
	}
	for _, elevated := range []struct{ name string; args []string }{
		{"pkexec", append([]string{name}, args...)},
		{"sudo", append([]string{name}, args...)},
	} {
		cmd := exec.Command(elevated.name, elevated.args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no way to escalate privileges (tried pkexec and sudo)")
}

// RunElevatedOutput runs a privileged command and returns its combined output.
func RunElevatedOutput(name string, args ...string) ([]byte, error) {
	if os.Geteuid() == 0 {
		return exec.Command(name, args...).CombinedOutput()
	}
	cmd := exec.Command("pkexec", append([]string{name}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.Command("sudo", append([]string{name}, args...)...)
		out, err = cmd.CombinedOutput()
	}
	return out, err
}

// OpenWithDefaultApp opens a path (file, directory, URL) with the user's
// default application via xdg-open.
func OpenWithDefaultApp(target string) error {
	bin, err := exec.LookPath("xdg-open")
	if err != nil {
		return fmt.Errorf("xdg-open not found: %w", err)
	}
	cmd := exec.Command(bin, target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}