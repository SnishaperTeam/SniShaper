package core

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"snishaper/proxy"
)

// CoreClient is an RPC client that communicates with the core process.
type CoreClient struct {
	token string
}

// NewCoreClient creates a new core RPC client.
func NewCoreClient() *CoreClient {
	return &CoreClient{}
}

func (c *CoreClient) readToken() string {
	if c.token != "" {
		return c.token
	}
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	tokenPath := filepath.Join(filepath.Dir(execPath), "core_rpc_token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return ""
	}
	c.token = strings.TrimSpace(string(data))
	return c.token
}

func (c *CoreClient) dial() (*rpc.Client, error) {
	conn, err := net.DialTimeout("tcp", coreRPCAddr, 400*time.Millisecond)
	if err != nil {
		return nil, err
	}
	return rpc.NewClient(conn), nil
}

// Call invokes an RPC method on the core process.
func (c *CoreClient) Call(method string, args any, reply any) error {
	client, err := c.dial()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Call(method, args, reply)
}

// EnsureRunning makes sure the core process is alive and responding.
func (c *CoreClient) EnsureRunning() error {
	return c.ensureRunningWithElevation(false)
}

func (c *CoreClient) ensureRunningWithElevation(requireElevated bool) error {
	wasLogCaptureEnabled := false
	wasProxyRunning := false
	var pong BoolReply
	if err := c.Call("Core.Ping", EmptyArgs{}, &pong); err == nil && pong.Value {
		wasLogCaptureEnabled = c.IsLogCaptureEnabled()
		wasProxyRunning = c.IsProxyRunning()
		execPath, pathErr := os.Executable()
		if pathErr != nil {
			return pathErr
		}
		info, infoErr := c.getInfo()
		currentFileInfo, currentErr := os.Stat(execPath)
		isSameFile := false
		if infoErr == nil && currentErr == nil {
			isSameFile = sameExecutable(info.Executable, execPath) && (info.ModTime == currentFileInfo.ModTime().UnixNano())
		}
		if isSameFile && (!requireElevated || info.Elevated) {
			return nil
		}
		var empty EmptyArgs
		_ = c.Call("Core.Shutdown", EmptyArgs{}, &empty)
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Millisecond)
			if err := c.Call("Core.Ping", EmptyArgs{}, &pong); err != nil || !pong.Value {
				break
			}
		}
	}
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := startCoreProcess(execPath, requireElevated); err != nil {
		return err
	}
	for i := 0; i < 75; i++ {
		time.Sleep(200 * time.Millisecond)
		if err := c.Call("Core.Ping", EmptyArgs{}, &pong); err == nil && pong.Value {
			// Authenticate with the core process
			token := c.readToken()
			if token != "" {
				var authReply BoolReply
				if err := c.Call("Core.Authenticate", AuthArgs{Token: token}, &authReply); err != nil || !authReply.Value {
					continue // Authentication failed, retry
				}
			}
			if wasLogCaptureEnabled {
				var empty EmptyArgs
				_ = c.Call("Core.StartLogCapture", EmptyArgs{}, &empty)
			}
			if wasProxyRunning {
				var empty EmptyArgs
				if err := c.Call("Core.StartProxy", EmptyArgs{}, &empty); err != nil {
					return fmt.Errorf("restore proxy after core restart: %w", err)
				}
			}
			if requireElevated {
				info, infoErr := c.getInfo()
				if infoErr != nil {
					return infoErr
				}
				if !info.Elevated {
					return fmt.Errorf("core restarted but is still not elevated")
				}
			}
			return nil
		}
	}
	return fmt.Errorf("core process did not become ready after 75 retries (15s): check admin rights, antivirus, and core process logs")
}

func (c *CoreClient) getInfo() (CoreInfoReply, error) {
	var reply CoreInfoReply
	err := c.Call("Core.GetInfo", EmptyArgs{}, &reply)
	return reply, err
}

// ReloadIfRunning tells the core to reload its configuration.
func (c *CoreClient) ReloadIfRunning() {
	var pong BoolReply
	if err := c.Call("Core.Ping", EmptyArgs{}, &pong); err != nil || !pong.Value {
		return
	}
	var empty EmptyArgs
	_ = c.Call("Core.ReloadConfig", EmptyArgs{}, &empty)
}

// ReloadCertificateIfRunning tells the core to reload its certificate.
func (c *CoreClient) ReloadCertificateIfRunning() {
	var pong BoolReply
	if err := c.Call("Core.Ping", EmptyArgs{}, &pong); err != nil || !pong.Value {
		return
	}
	var empty EmptyArgs
	_ = c.Call("Core.ReloadCertificate", EmptyArgs{}, &empty)
}

// ShutdownIfRunning shuts down the core process if it's alive.
func (c *CoreClient) ShutdownIfRunning() {
	var pong BoolReply
	if err := c.Call("Core.Ping", EmptyArgs{}, &pong); err != nil || !pong.Value {
		return
	}
	var empty EmptyArgs
	_ = c.Call("Core.Shutdown", EmptyArgs{}, &empty)
}

func (c *CoreClient) StartProxy() error {
	if err := c.EnsureRunning(); err != nil {
		return err
	}
	var empty EmptyArgs
	return c.Call("Core.StartProxy", EmptyArgs{}, &empty)
}

func (c *CoreClient) StopProxy() error {
	var empty EmptyArgs
	return c.Call("Core.StopProxy", EmptyArgs{}, &empty)
}

func (c *CoreClient) IsProxyRunning() bool {
	var reply BoolReply
	return c.Call("Core.IsProxyRunning", EmptyArgs{}, &reply) == nil && reply.Value
}

func (c *CoreClient) GetStats() (int64, int64, int64) {
	var reply StatsReply
	if err := c.Call("Core.GetStats", EmptyArgs{}, &reply); err != nil {
		return 0, 0, 0
	}
	return reply.Down, reply.Up, reply.Etc
}

func (c *CoreClient) StartTUN() error {
	if err := checkWintun(); err != nil {
		return fmt.Errorf("wintun check: %w", err)
	}
	if err := c.ensureRunningWithElevation(true); err != nil {
		return fmt.Errorf("ensure elevated core failed: %w", err)
	}
	var empty EmptyArgs
	if err := c.Call("Core.StartTUN", EmptyArgs{}, &empty); err != nil {
		return fmt.Errorf("Core.StartTUN RPC failed: %w", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	var lastStatus proxy.TUNStatus
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		status, err := c.getTUNStatusWithError()
		if err != nil {
			return fmt.Errorf("Core.GetTUNStatus RPC failed: %w", err)
		}
		lastStatus = status
		if status.Running {
			return nil
		}
		if strings.TrimSpace(status.Message) != "" &&
			!strings.EqualFold(strings.TrimSpace(status.Message), "Windows real TUN dataplane is ready") &&
			!strings.Contains(strings.ToLower(status.Message), "startup in progress") &&
			!strings.Contains(strings.ToLower(status.Message), "starting") &&
			!strings.Contains(strings.ToLower(status.Message), "creating") &&
			!strings.Contains(strings.ToLower(status.Message), "selected") &&
			!strings.Contains(strings.ToLower(status.Message), "not running") {
			return errors.New(status.Message)
		}
	}
	if strings.TrimSpace(lastStatus.Message) != "" {
		return fmt.Errorf("TUN startup failed: %s", strings.TrimSpace(lastStatus.Message))
	}
	return fmt.Errorf("TUN did not enter running state")
}

func (c *CoreClient) StopTUN() error {
	var empty EmptyArgs
	return c.Call("Core.StopTUN", EmptyArgs{}, &empty)
}

func (c *CoreClient) GetTUNStatus() proxy.TUNStatus {
	status, err := c.getTUNStatusWithError()
	if err != nil {
		status.Supported = runtime.GOOS == "windows"
		status.Message = "Core not running"
	}
	return status
}

func (c *CoreClient) getTUNStatusWithError() (proxy.TUNStatus, error) {
	var reply TUNStatusReply
	err := c.Call("Core.GetTUNStatus", EmptyArgs{}, &reply)
	return reply.Status, err
}

func (c *CoreClient) StartLogCapture() error {
	if err := c.EnsureRunning(); err != nil {
		return err
	}
	var empty EmptyArgs
	return c.Call("Core.StartLogCapture", EmptyArgs{}, &empty)
}

func (c *CoreClient) StopLogCapture() error {
	var empty EmptyArgs
	return c.Call("Core.StopLogCapture", EmptyArgs{}, &empty)
}

func (c *CoreClient) IsLogCaptureEnabled() bool {
	var reply BoolReply
	return c.Call("Core.IsLogCaptureEnabled", EmptyArgs{}, &reply) == nil && reply.Value
}

func (c *CoreClient) GetRecentLogs(limit int) string {
	var reply StringReply
	_ = c.Call("Core.GetRecentLogs", LogsArgs{Limit: limit}, &reply)
	return reply.Value
}

func (c *CoreClient) ClearLogs() error {
	var empty EmptyArgs
	return c.Call("Core.ClearLogs", EmptyArgs{}, &empty)
}

type RouteEvent struct {
	Domain string
	Mode   string
}

type RouteEventsReply struct {
	Events []RouteEvent
}

func (c *CoreClient) GetRouteEvents() []RouteEvent {
	var reply RouteEventsReply
	if err := c.Call("Core.GetRouteEvents", EmptyArgs{}, &reply); err != nil {
		return nil
	}
	return reply.Events
}

func (c *CoreClient) SetProxyMode(mode string) error {
	if err := c.EnsureRunning(); err != nil {
		return err
	}
	var empty EmptyArgs
	return c.Call("Core.SetProxyMode", SetModeArgs{Mode: mode}, &empty)
}

func (c *CoreClient) GetProxyMode() string {
	var reply StringReply
	_ = c.Call("Core.GetProxyMode", EmptyArgs{}, &reply)
	return reply.Value
}

func sameExecutable(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	left = filepath.Clean(strings.ToLower(left))
	right = filepath.Clean(strings.ToLower(right))
	return left == right
}
