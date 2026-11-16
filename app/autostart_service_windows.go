//go:build windows

package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	// autoStartServiceName is the SCM name of the auto-start helper service.
	autoStartServiceName = "SniShaperAutostart"
	// serviceHostLaunchFlag marks the process as the SCM service host; the
	// host relaunches the app (minus this flag) in the user session.
	serviceHostLaunchFlag = "--service-host"
)

var (
	userenvDLL                  = windows.NewLazySystemDLL("userenv.dll")
	procCreateEnvironmentBlock  = userenvDLL.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock = userenvDLL.NewProc("DestroyEnvironmentBlock")
)

// RunAutoStartServiceHost runs the auto-start helper as a managed Windows
// service. It is the process entry for a `--service-host` launch (main.go):
// the SCM starts it at boot, and once a user logs on it launches the desktop
// app in that session with the user's elevated token — the "elevation
// service" pattern, replacing a HighestAvailable scheduled task for users
// who pick the Windows-service autostart method.
func RunAutoStartServiceHost() error {
	return svc.Run(autoStartServiceName, &autoStartService{})
}

type autoStartService struct{}

func (s *autoStartService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	stop := make(chan struct{})
	go s.launchLoop(stop)

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				close(stop)
				changes <- svc.Status{State: svc.StopPending}
				return false, 0
			}
		}
	}
}

// launchLoop waits for an interactive console session, then starts the app
// there once per logon. WTSQueryUserToken fails while nobody is logged on,
// which also resets the dedupe marker so a logoff/logon cycle relaunches.
func (s *autoStartService) launchLoop(stop <-chan struct{}) {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("[autostart-service] cannot resolve executable: %v", err)
		return
	}
	args := serviceSpawnArgs()
	launched := ^uint32(0)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		session := windows.WTSGetActiveConsoleSessionId()
		token, err := querySessionToken(session)
		if err != nil {
			launched = ^uint32(0)
			continue
		}
		token.Close()
		if session == launched {
			continue
		}
		if err := launchAppInSession(exe, args, session); err != nil {
			log.Printf("[autostart-service] launch in session %d failed: %v", session, err)
			continue // retry on the next tick
		}
		launched = session
	}
}

// serviceSpawnArgs returns the launch args recorded at service-install time
// (everything after the executable) minus the host flag itself.
func serviceSpawnArgs() []string {
	var args []string
	for _, arg := range os.Args[1:] {
		if arg == serviceHostLaunchFlag {
			continue
		}
		args = append(args, arg)
	}
	return args
}

func querySessionToken(session uint32) (windows.Token, error) {
	if session == ^uint32(0) {
		return 0, fmt.Errorf("no active console session")
	}
	var token windows.Token
	if err := windows.WTSQueryUserToken(session, &token); err != nil {
		return 0, err
	}
	return token, nil
}

// launchAppInSession starts the app in the given session with the user's
// elevated (linked) token when available, so an administrator user gets the
// full token without a UAC prompt. Standard users have no linked token; the
// app then goes through its usual self-elevation, same as the task method.
func launchAppInSession(exe string, args []string, session uint32) error {
	userToken, err := querySessionToken(session)
	if err != nil {
		return err
	}
	defer userToken.Close()

	runToken := userToken
	if linked, ltErr := userToken.GetLinkedToken(); ltErr == nil && linked != 0 {
		runToken = linked
		defer linked.Close()
	}

	env, err := buildUserEnvironmentBlock(runToken)
	if err != nil {
		return fmt.Errorf("build environment block: %w", err)
	}
	defer destroyEnvironmentBlock(env)

	cmdLine := windows.EscapeArg(exe)
	for _, arg := range args {
		cmdLine += " " + windows.EscapeArg(arg)
	}
	exe16, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	cmd16, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return err
	}
	dir16, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}
	desktop16, err := windows.UTF16PtrFromString(`winsta0\default`)
	if err != nil {
		return err
	}
	si := &windows.StartupInfo{
		Desktop: desktop16,
		Cb:      uint32(unsafe.Sizeof(windows.StartupInfo{})),
	}
	var pi windows.ProcessInformation
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)
	// The service's own environment belongs to SYSTEM; the user's block plus
	// CREATE_UNICODE_ENVIRONMENT keeps the child resolving config, logs and
	// WebView2 state against the real user profile.
	if err := windows.CreateProcessAsUser(runToken, exe16, cmd16, nil, nil, false,
		windows.CREATE_UNICODE_ENVIRONMENT, env, dir16, si, &pi); err != nil {
		return err
	}
	return nil
}

// buildUserEnvironmentBlock creates the environment of the token's user
// (CreateEnvironmentBlock, userenv.dll — not exposed by x/sys).
func buildUserEnvironmentBlock(token windows.Token) (*uint16, error) {
	var env *uint16
	r1, _, err := procCreateEnvironmentBlock.Call(
		uintptr(unsafe.Pointer(&env)), uintptr(token), 0)
	if r1 == 0 {
		return nil, err
	}
	return env, nil
}

func destroyEnvironmentBlock(env *uint16) {
	if env != nil {
		_, _, _ = procDestroyEnvironmentBlock.Call(uintptr(unsafe.Pointer(env)))
	}
}

// installAutoStartService registers (or reconfigures) the helper service so
// it starts at boot. command is the desktop autostart command line built by
// buildAutoStartCommand; the service host relaunches it in the user session.
func installAutoStartService(command string) error {
	exe, args, err := splitCommandLine(command)
	if err != nil {
		return err
	}
	// args comes back as one verbatim string from splitCommandLine; the
	// autostart flags are plain unquoted tokens, so Fields is exact here.
	hostArgs := append([]string{serviceHostLaunchFlag}, strings.Fields(args)...)

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer m.Disconnect()

	if s, err := m.OpenService(autoStartServiceName); err == nil {
		defer s.Close()
		// Reconfigure in place: deleting a running service only marks it for
		// deletion, and re-creating under the same name fails until the old
		// instance stops.
		cfg, err := s.Config()
		if err != nil {
			return fmt.Errorf("read service config: %w", err)
		}
		cfg.StartType = mgr.StartAutomatic
		cfg.BinaryPathName = buildServiceBinaryPath(exe, hostArgs)
		if err := s.UpdateConfig(cfg); err != nil {
			return fmt.Errorf("update service config: %w", err)
		}
		return nil
	}
	s, err := m.CreateService(autoStartServiceName, exe, mgr.Config{
		StartType:   mgr.StartAutomatic,
		DisplayName: "SniShaper Autostart",
		Description: "Starts SniShaper in the user session at logon without a UAC prompt.",
	}, hostArgs...)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	return s.Close()
}

// removeAutoStartService unregisters the helper service; missing is success.
func removeAutoStartService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(autoStartServiceName)
	if err != nil {
		return nil
	}
	defer s.Close()
	return s.Delete()
}

func buildServiceBinaryPath(exe string, args []string) string {
	path := windows.EscapeArg(exe)
	for _, arg := range args {
		path += " " + windows.EscapeArg(arg)
	}
	return path
}
