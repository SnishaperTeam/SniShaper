package app

func (a *App) startRulesWatcher() {
	if a.ruleManager == nil {
		return
	}

	stop, err := a.ruleManager.WatchRulesFile(func() {
		if a.shouldQuit {
			return
		}
		if err := a.ruleManager.LoadConfig(); err != nil {
			a.appendLog("[watch] rules auto reload failed: " + err.Error())
			return
		}
		a.proxyServer.UpdateCloudflareConfig(a.ruleManager.GetCloudflareConfig())
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
		a.appendLog("[watch] rules file changed, reloaded automatically")
		a.emitFrontendState()
		a.emitEvent("app:rules_changed", map[string]interface{}{"source": "file_watch"})
	})
	if err != nil {
		a.appendLog("[watch] rules file watch unavailable: " + err.Error())
		return
	}

	a.rulesWatchMu.Lock()
	a.rulesWatchStop = stop
	a.rulesWatchMu.Unlock()
}

func (a *App) stopRulesWatcher() {
	a.rulesWatchMu.Lock()
	stop := a.rulesWatchStop
	a.rulesWatchStop = nil
	a.rulesWatchMu.Unlock()
	if stop != nil {
		stop()
	}
}
