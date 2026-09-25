package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const rulesWatchDebounce = 400 * time.Millisecond

func (rm *RuleManager) RulesPath() string {
	return rm.rulesPath
}

func (rm *RuleManager) WatchRulesFile(onChange func()) (func(), error) {
	path := rm.rulesPath
	if path == "" {
		return func() {}, nil
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	rm.setRulesHash(fileContentHash(path))

	stopCh := make(chan struct{})
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			close(stopCh)
			_ = watcher.Close()
		})
	}

	go func() {
		var timerMu sync.Mutex
		var timer *time.Timer

		schedule := func() {
			timerMu.Lock()
			defer timerMu.Unlock()
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(rulesWatchDebounce, func() {
				select {
				case <-stopCh:
					return
				default:
				}
				current := fileContentHash(path)
				if current == "" || current == rm.rulesHashValue() {
					return
				}
				rm.setRulesHash(current)
				if onChange != nil {
					onChange()
				}
			})
		}

		for {
			select {
			case <-stopCh:
				return
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Base(ev.Name) != base {
					continue
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
					continue
				}
				schedule()
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return stop, nil
}

func fileContentHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return dataHash(data)
}

func dataHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (rm *RuleManager) setRulesHash(hash string) {
	rm.rulesHashMu.Lock()
	rm.rulesHash = hash
	rm.rulesHashMu.Unlock()
}

func (rm *RuleManager) rulesHashValue() string {
	rm.rulesHashMu.Lock()
	defer rm.rulesHashMu.Unlock()
	return rm.rulesHash
}
