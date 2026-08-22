package app

import (
	"encoding/json"
	"fmt"
	"time"

	"snishaper/evolution"
	"snishaper/proxy"
)

// ensureEvolutionTester lazily creates the evolution tester on first use.
// The field is not initialized in NewApp because the tester depends on the
// proxy server's DoH resolver and the rule manager's auto router.
// 加锁防止连续触发时重复创建 tester（第二个实例的任务事件将无人接收）。
func (a *App) ensureEvolutionTester() *evolution.Tester {
	a.evolutionTesterMu.Lock()
	defer a.evolutionTesterMu.Unlock()
	if a.evolutionTester == nil {
		a.evolutionTester = evolution.NewTester(
			a.ruleManager,
			a.proxyServer,
			a.proxyServer.GetDoHResolver(),
			a.ruleManager.GetAutoRouter(),
			func(msg string) {
				a.appendLog(msg)
			},
		)
	}
	return a.evolutionTester
}

// StartEvolutionTest 启动进化模式测试任务，并在后台轮询任务状态，
// 通过 evolution:progress / evolution:complete 事件向前端推送进度与结果。
func (a *App) StartEvolutionTest(domains []string, enableIPv6 bool) (map[string]interface{}, error) {
	a.appendLog("[Evolution] 开始进化模式测试")

	config := evolution.DefaultTestConfig()
	config.EnableIPv6 = enableIPv6

	tester := a.ensureEvolutionTester()
	task, err := tester.StartTest(domains, config)
	if err != nil {
		a.appendLog("[Evolution] 启动测试失败: " + err.Error())
		return nil, err
	}

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			<-ticker.C
			currentTask := tester.Snapshot()
			if currentTask == nil || currentTask.Status != evolution.StatusRunning {
				break
			}

			a.emit("evolution:progress", map[string]interface{}{
				"progress":   currentTask.Progress,
				"total":      currentTask.Total,
				"status":     string(currentTask.Status),
				"results":    tester.GetAllResults(),
				"temp_rules": tester.GetTempRules(),
			})
		}

		finalTask := tester.Snapshot()
		if finalTask != nil {
			a.emit("evolution:complete", map[string]interface{}{
				"id":         finalTask.ID,
				"status":     string(finalTask.Status),
				"progress":   finalTask.Progress,
				"total":      finalTask.Total,
				"results":    tester.GetAllResults(),
				"temp_rules": tester.GetTempRules(),
			})
		}
	}()

	return map[string]interface{}{
		"id":     task.ID,
		"status": string(task.Status),
		"total":  task.Total,
	}, nil
}

// GetEvolutionTestStatus 返回当前测试任务的状态、结果与待应用规则。
func (a *App) GetEvolutionTestStatus() map[string]interface{} {
	if a.evolutionTester == nil {
		return map[string]interface{}{
			"status": "idle",
		}
	}

	task := a.evolutionTester.Snapshot()
	if task == nil {
		return map[string]interface{}{
			"status": "idle",
		}
	}

	return map[string]interface{}{
		"id":         task.ID,
		"status":     string(task.Status),
		"progress":   task.Progress,
		"total":      task.Total,
		"results":    a.evolutionTester.GetAllResults(),
		"temp_rules": a.evolutionTester.GetTempRules(),
	}
}

func (a *App) StopEvolutionTest() error {
	a.appendLog("[Evolution] 停止测试")
	if a.evolutionTester != nil {
		a.evolutionTester.StopTest()
	}
	return nil
}

// ApplyEvolutionRule 把进化测试生成的临时规则转换为站点组规则并写入配置，
// 成功后从临时规则列表中移除。
func (a *App) ApplyEvolutionRule(ruleID string) error {
	a.appendLog("[Evolution] 应用规则: " + ruleID)

	if a.evolutionTester == nil {
		return fmt.Errorf("evolution tester not initialized")
	}

	tempRule := a.evolutionTester.GetTempRule(ruleID)
	if tempRule == nil {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	siteGroupMap := tempRule.ToSiteGroup()
	siteGroupJSON, err := json.Marshal(siteGroupMap)
	if err != nil {
		return fmt.Errorf("failed to marshal site group: %v", err)
	}

	var siteGroup proxy.SiteGroup
	if err := json.Unmarshal(siteGroupJSON, &siteGroup); err != nil {
		return fmt.Errorf("failed to unmarshal site group: %v", err)
	}

	if err := a.ruleManager.AddSiteGroup(siteGroup); err != nil {
		return fmt.Errorf("failed to add site group: %v", err)
	}

	a.evolutionTester.MarkRuleApplied(ruleID)
	a.appendLog("[Evolution] 规则已成功应用: " + ruleID)
	return nil
}
