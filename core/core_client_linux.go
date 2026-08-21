package core

import (
	"log"
	"os/exec"
)

// startCoreProcess starts the core RPC process. When the app itself was
// elevated via sudo the child inherits root, so no wrapper is required.
func startCoreProcess(execPath string, _ bool) error {
	log.Printf("[core-client] startCoreProcess: exec=%q args=[--core]", execPath)
	cmd := exec.Command(execPath, "--core")
	if err := cmd.Start(); err != nil {
		log.Printf("[core-client] startCoreProcess Start failed: %v", err)
		return err
	}
	log.Printf("[core-client] startCoreProcess started pid=%d", cmd.Process.Pid)
	return nil
}