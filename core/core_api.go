package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"snishaper/common"
	"snishaper/proxy"
)

const coreRPCAddr = "127.0.0.1:18933"

var coreRPCToken string

func init() {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		coreRPCToken = hex.EncodeToString(b)
	} else {
		coreRPCToken = fmt.Sprintf("%d", time.Now().UnixNano())
	}
}

type coreService struct {
	runtime *coreRuntime
	stop    func()
}

type EmptyArgs struct{}

type AuthArgs struct {
	Token string
}

type BoolReply struct {
	Value bool
}

type IntReply struct {
	Value int
}

type StringReply struct {
	Value string
}

type StatsReply struct {
	Down int64
	Up   int64
	Etc  int64
}

type CoreInfoReply struct {
	PID        int
	Executable string
	Elevated   bool
	ModTime    int64
}

type TUNStatusReply struct {
	Status proxy.TUNStatus
}

type SetModeArgs struct {
	Mode string
}

type LogsArgs struct {
	Limit int
}

func (s *coreService) Authenticate(args AuthArgs, reply *BoolReply) error {
	reply.Value = args.Token == coreRPCToken
	return nil
}

// Ping 要求 token 匹配：使"core 存活"判定按实例隔离——持有旧实例
// token 的客户端（如正在退出的 --serve）不会把复用固定端口的新 core
// 当成自己的 core，从而既不会误杀它，也不会因 ping 成功而永不退出。
func (s *coreService) Ping(args AuthArgs, reply *BoolReply) error {
	reply.Value = args.Token == coreRPCToken
	return nil
}

func (s *coreService) GetInfo(_ EmptyArgs, reply *CoreInfoReply) error {
	reply.PID = os.Getpid()
	reply.Executable = s.runtime.execPath
	reply.Elevated = isProcessElevated()
	
	info, err := os.Stat(s.runtime.execPath)
	if err == nil {
		reply.ModTime = info.ModTime().UnixNano()
	}
	return nil
}

func (s *coreService) ReloadConfig(_ EmptyArgs, _ *EmptyArgs) error {
	return s.runtime.reloadConfig()
}

func (s *coreService) ReloadCertificate(_ EmptyArgs, _ *EmptyArgs) error {
	return s.runtime.reloadCertificate()
}

// Shutdown 要求 token：否则任何本地进程都能经固定 RPC 端口杀掉 core，
// 且旧 --serve 退出流程会误杀刚接任固定端口的新 core。
func (s *coreService) Shutdown(args AuthArgs, _ *EmptyArgs) error {
	if args.Token != coreRPCToken {
		return errors.New("shutdown rejected: invalid token")
	}
	if s.stop != nil {
		s.stop()
	}
	return nil
}

func (s *coreService) StartProxy(_ EmptyArgs, _ *EmptyArgs) error {
	return s.runtime.startProxy()
}

func (s *coreService) StopProxy(_ EmptyArgs, _ *EmptyArgs) error {
	return s.runtime.stopProxy()
}

func (s *coreService) IsProxyRunning(_ EmptyArgs, reply *BoolReply) error {
	reply.Value = s.runtime.proxyServer.IsRunning()
	return nil
}

func (s *coreService) GetStats(_ EmptyArgs, reply *StatsReply) error {
	reply.Down, reply.Up, reply.Etc = s.runtime.proxyServer.GetStats()
	return nil
}

func (s *coreService) StartTUN(_ EmptyArgs, _ *EmptyArgs) error {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.runtime.failTUNStart(fmt.Errorf("core StartTUN panic: %v", r))
			}
		}()
		if err := s.runtime.startTUN(); err != nil {
			s.runtime.appendLog("[core] StartTUN failed: " + err.Error())
		}
	}()
	return nil
}

func (s *coreService) StopTUN(_ EmptyArgs, _ *EmptyArgs) error {
	go func() {
		// StopTUN 与 StartTUN 一样需要 recover：释放路径要动 Wintun 设备与
		// gvisor 栈，两者都可能 panic，缺了保护会连带整个 core 子进程退出，
		// 表现为 GUI 侧所有 RPC 一起断连。
		defer func() {
			if rec := recover(); rec != nil {
				s.runtime.appendLog("[core] StopTUN panic: " + fmt.Sprint(rec))
				s.runtime.appendLog(string(debug.Stack()))
			}
		}()
		if err := s.runtime.stopTUN(); err != nil {
			s.runtime.appendLog("[core] StopTUN failed: " + err.Error())
		}
	}()
	return nil
}

func (s *coreService) GetTUNStatus(_ EmptyArgs, reply *TUNStatusReply) error {
	reply.Status = s.runtime.getTUNStatus()
	return nil
}

func (s *coreService) StartLogCapture(_ EmptyArgs, _ *EmptyArgs) error {
	s.runtime.startLogCapture()
	return nil
}

func (s *coreService) StopLogCapture(_ EmptyArgs, _ *EmptyArgs) error {
	s.runtime.stopLogCapture()
	return nil
}

func (s *coreService) IsLogCaptureEnabled(_ EmptyArgs, reply *BoolReply) error {
	reply.Value = s.runtime.isLogCaptureEnabled()
	return nil
}

func (s *coreService) GetRecentLogs(args LogsArgs, reply *StringReply) error {
	reply.Value = s.runtime.recentLogs(args.Limit)
	return nil
}

func (s *coreService) ClearLogs(_ EmptyArgs, _ *EmptyArgs) error {
	s.runtime.clearLogs()
	return nil
}

func (s *coreService) GetRouteEvents(_ EmptyArgs, reply *RouteEventsReply) error {
	reply.Events = s.runtime.popRouteEvents()
	return nil
}

func (s *coreService) SetProxyMode(args SetModeArgs, _ *EmptyArgs) error {
	return s.runtime.proxyServer.SetMode(args.Mode)
}

func (s *coreService) GetProxyMode(_ EmptyArgs, reply *StringReply) error {
	reply.Value = s.runtime.proxyServer.GetMode()
	return nil
}

// RunCoreMain starts the core RPC server. Called from main when --core flag is present.
func RunCoreMain() error {
	// The core hosts the proxy, and its stderr goes nowhere when the desktop app
	// spawned it, so a fatal error would be lost: write crash reports to a file.
	if execPath, err := os.Executable(); err == nil {
		if path := common.EnableCrashLog(filepath.Join(filepath.Dir(execPath), "log")); path != "" {
			fmt.Println("[core] Crash report file: " + path)
		}
	}

	runtime, err := newCoreRuntime()
	if err != nil {
		return err
	}
	defer runtime.shutdown()

	// Write RPC token to file for client to read
	tokenPath := filepath.Join(runtime.execDir, "core_rpc_token")
	if err := os.WriteFile(tokenPath, []byte(coreRPCToken), 0600); err != nil {
		return fmt.Errorf("failed to write RPC token: %w", err)
	}
	defer func() {
		// 仅当文件仍属于本实例时才删除：新实例可能已覆盖写入自己的
		// token，退出时不得删掉别人的。
		if data, err := os.ReadFile(tokenPath); err == nil && strings.TrimSpace(string(data)) == coreRPCToken {
			_ = os.Remove(tokenPath)
		}
	}()

	server := rpc.NewServer()
	var (
		stopOnce sync.Once
		listener net.Listener
	)
	stopFn := func() {
		stopOnce.Do(func() {
			if listener != nil {
				_ = listener.Close()
			}
		})
	}

	if err := server.RegisterName("Core", &coreService{runtime: runtime, stop: stopFn}); err != nil {
		return err
	}

	listener, err = net.Listen("tcp", coreRPCAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			if ne, ok := err.(net.Error); ok && (ne.Timeout() || ne.Temporary()) {
				runtime.appendLog(fmt.Sprintf("[core] rpc accept temporary error: %v", err))
				continue
			}
			runtime.appendLog(fmt.Sprintf("[core] rpc accept error: %v", err))
			continue
		}
		go func(conn net.Conn) {
			defer func() {
				if r := recover(); r != nil {
					runtime.appendLog(fmt.Sprintf("[core] rpc panic: %v\n%s", r, string(debug.Stack())))
				}
				_ = conn.Close()
			}()
			server.ServeConn(conn)
		}(conn)
	}
}
