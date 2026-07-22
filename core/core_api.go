package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"runtime/debug"
	"sync"
	"time"

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

func (s *coreService) Ping(_ EmptyArgs, reply *BoolReply) error {
	reply.Value = true
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

func (s *coreService) Shutdown(_ EmptyArgs, _ *EmptyArgs) error {
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

func (s *coreService) SetSocks5Enabled(args BoolReply, _ *EmptyArgs) error {
	s.runtime.proxyServer.SetSocks5Enabled(args.Value)
	s.runtime.ruleManager.SetSocks5Enabled(args.Value)
	return s.runtime.ruleManager.SaveConfig()
}

func (s *coreService) GetSocks5Enabled(_ EmptyArgs, reply *BoolReply) error {
	reply.Value = s.runtime.proxyServer.IsSocks5Enabled()
	return nil
}

func (s *coreService) GetSocks5Port(_ EmptyArgs, reply *StringReply) error {
	reply.Value = s.runtime.ruleManager.GetSocks5Port()
	return nil
}

func (s *coreService) SetSocks5Port(args StringReply, _ *EmptyArgs) error {
	s.runtime.ruleManager.SetSocks5Port(args.Value)
	s.runtime.proxyServer.SetSocks5Addr("127.0.0.1:" + args.Value)
	return s.runtime.ruleManager.SaveConfig()
}

// RunCoreMain starts the core RPC server. Called from main when --core flag is present.
func RunCoreMain() error {
	runtime, err := newCoreRuntime()
	if err != nil {
		return err
	}
	defer runtime.shutdown()
	writeCoreMarker(runtime.execDir, "run_core_main", markerDetail("entered pid=%d", os.Getpid()))

	// Write RPC token to file for client to read
	tokenPath := fmt.Sprintf("%s\\core_rpc_token", runtime.execDir)
	if err := os.WriteFile(tokenPath, []byte(coreRPCToken), 0600); err != nil {
		return fmt.Errorf("failed to write RPC token: %w", err)
	}
	defer os.Remove(tokenPath)

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
		writeCoreMarker(runtime.execDir, "run_core_main", markerDetail("listen failed: %v", err))
		return err
	}
	defer listener.Close()
	writeCoreMarker(runtime.execDir, "run_core_main", markerDetail("listen ok addr=%s", coreRPCAddr))

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				writeCoreMarker(runtime.execDir, "run_core_main", "listener closed")
				return nil
			}
			if ne, ok := err.(net.Error); ok && (ne.Timeout() || ne.Temporary()) {
				runtime.appendLog(fmt.Sprintf("[core] rpc accept temporary error: %v", err))
				writeCoreMarker(runtime.execDir, "run_core_main", markerDetail("accept temporary error: %v", err))
				continue
			}
			runtime.appendLog(fmt.Sprintf("[core] rpc accept error: %v", err))
			writeCoreMarker(runtime.execDir, "run_core_main", markerDetail("accept error: %v", err))
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
