package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type WarpStatus struct {
	Running bool   `json:"running"`
	Account string `json:"account"`
	Mode    string `json:"mode"`
	Error   string `json:"error,omitempty"`
}

type WarpManager struct {
	BinPath     string
	ConfigDir   string
	SocksPort   int
	cmd         *exec.Cmd
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	running     bool
	lastError   error
	onLogUpdate func(string)
}

func NewWarpManager(execDir string) *WarpManager {
	binName := "usque.exe"
	if runtime.GOOS != "windows" {
		binName = "usque"
	}

	return &WarpManager{
		BinPath:   filepath.Join(execDir, "warp", binName),
		ConfigDir: filepath.Join(execDir, "warp"),
		SocksPort: 1080,
	}
}

func (m *WarpManager) SetEndpoint(endpoint string) error {
	if endpoint == "" {
		return nil
	}
	configPath := filepath.Join(m.ConfigDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 尚未注册，忽略
		}
		return err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	config["endpoint_v4"] = endpoint
	// endpoint_v6 通常保持默认或由 Cloudflare 分配，此处暂不强制修改

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, newData, 0644)
}

func (m *WarpManager) SetLogCallback(cb func(string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onLogUpdate = cb
}

func (m *WarpManager) appendLog(msg string) {
	m.mu.RLock()
	cb := m.onLogUpdate
	m.mu.RUnlock()
	if cb != nil {
		cb("[Warp] " + msg)
	}
	log.Printf("[Warp] %s", msg)
}

func (m *WarpManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 检查二进制文件是否存在
	if _, err := os.Stat(m.BinPath); os.IsNotExist(err) {
		return fmt.Errorf("usque binary not found at %s", m.BinPath)
	}

	m.ctx, m.cancel = context.WithCancel(context.Background())
	ctx := m.ctx

	go m.runLoop(ctx)

	m.running = true
	return nil
}

func (m *WarpManager) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}

	cancel := m.cancel
	cmd := m.cmd
	m.running = false
	m.cancel = nil
	m.ctx = nil
	m.cmd = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if cmd != nil && cmd.Process != nil {
		// Windows 平滑退出由于是控制台程序可能不理想，先常规 Kill
		_ = cmd.Process.Kill()

		// 额外尝试针对 Windows 的强力清理 (如果进程没死)
		if runtime.GOOS == "windows" {
			killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid))
			m.setupHiddenWindow(killCmd)
			_ = killCmd.Run()
		}
	}

	m.appendLog("Stopped")
	return nil
}

func (m *WarpManager) runLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := m.execute(ctx)
			if err != nil {
				m.mu.Lock()
				m.lastError = err
				m.mu.Unlock()
				m.appendLog(fmt.Sprintf("Process exited with error: %v", err))
			}
			
			// 如果上下文没结束，等待一段时间后重启
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
				m.appendLog("Restarting usque...")
			}
		}
	}
}

func (m *WarpManager) execute(ctx context.Context) error {
	args := []string{"socks", "-p", fmt.Sprintf("%d", m.SocksPort)}
	
	cmd := exec.CommandContext(ctx, m.BinPath, args...)
	cmd.Dir = m.ConfigDir
	m.setupHiddenWindow(cmd)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	m.mu.Lock()
	m.cmd = cmd
	m.lastError = nil
	m.mu.Unlock()

	m.appendLog(fmt.Sprintf("Starting usque socks on port %d...", m.SocksPort))
	
	if err := cmd.Start(); err != nil {
		return err
	}

	// 实时捕获日志
	go m.copyLog(stdout)
	go m.copyLog(stderr)

	err := cmd.Wait()
	if err != nil {
		return fmt.Errorf("exit: %v", err)
	}
	return nil
}

func (m *WarpManager) copyLog(r io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			m.appendLog(string(buf[:n]))
		}
		if err != nil {
			break
		}
	}
}

func (m *WarpManager) IsReady() bool {
	addr := fmt.Sprintf("127.0.0.1:%d", m.SocksPort)
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (m *WarpManager) Register(deviceName string) (string, error) {
	args := []string{"register"}
	if deviceName != "" {
		args = append(args, "-n", deviceName)
	}

	cmd := exec.Command(m.BinPath, args...)
	cmd.Dir = m.ConfigDir
	cmd.Stdin = strings.NewReader("y\n") // 自动同意使用条款
	m.setupHiddenWindow(cmd)

	m.appendLog(fmt.Sprintf("Executing: %s register (at %s)", m.BinPath, m.ConfigDir))
	m.appendLog("Registering new Warp account... (Please wait, CF API might be slow)")
	
	out, err := cmd.CombinedOutput()
	if err != nil {
		m.appendLog(fmt.Sprintf("Register failed with error: %v, Output: %s", err, string(out)))
		return string(out), err
	}
	m.appendLog("Register success output: " + string(out))
	return string(out), nil
}

func (m *WarpManager) Enroll() (string, error) {
	cmd := exec.Command(m.BinPath, "enroll")
	cmd.Dir = m.ConfigDir
	cmd.Stdin = strings.NewReader("y\n") // 自动同意使用条款 (如果需要)
	m.setupHiddenWindow(cmd)

	m.appendLog(fmt.Sprintf("Executing: %s enroll (at %s)", m.BinPath, m.ConfigDir))
	m.appendLog("Enrolling existing Warp key...")
	
	out, err := cmd.CombinedOutput()
	if err != nil {
		m.appendLog(fmt.Sprintf("Enroll failed with error: %v, Output: %s", err, string(out)))
		return string(out), err
	}
	m.appendLog("Enroll success output: " + string(out))
	return string(out), nil
}

func (m *WarpManager) GetStatus() WarpStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := WarpStatus{
		Running: m.running,
		Mode:    "MASQUE",
	}

	if m.lastError != nil {
		status.Error = m.lastError.Error()
	}

	// 尝试从 config.json 读取账号 ID
	configPath := filepath.Join(m.ConfigDir, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data, &cfg); err == nil {
			status.Account = cfg.ID
		}
	}

	return status
}
