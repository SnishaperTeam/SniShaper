package app

import (
	"github.com/SnishaperTeam/snishaper-core/proxy"
)

func (a *App) GetFragmentConfig() proxy.FragmentConfig {
	return a.ruleManager.GetFragmentConfig()
}

func (a *App) SetFragmentConfig(cfg proxy.FragmentConfig) error {
	a.appendLog("[action] SetFragmentConfig called")
	return a.ruleManager.SetFragmentConfig(cfg)
}

func (a *App) ResetFragmentConfig() error {
	a.appendLog("[action] ResetFragmentConfig called")
	return a.ruleManager.SetFragmentConfig(proxy.DefaultFragmentConfig())
}
