//go:build windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// setProcessGroup 让子进程成为新进程组（Windows 下用于整棵树终止）。
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// terminateProcessGroup Windows 没有 SIGTERM 语义，直接结束整棵进程树。
func terminateProcessGroup(p *os.Process) {
	forceKillProcessGroup(p)
}

// forceKillProcessGroup 用 taskkill /T /F 结束进程树。
func forceKillProcessGroup(p *os.Process) {
	if p == nil || p.Pid <= 0 {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid)).Run()
}

// shutdownSignals Windows 下只处理 Ctrl+C。
func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
