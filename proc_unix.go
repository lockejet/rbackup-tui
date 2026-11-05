//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup 让子进程成为独立进程组，便于整组取消（bash 之下的 rsync 等）。
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcessGroup 向整个进程组发送 SIGTERM（含 bash 与其子进程）。
func terminateProcessGroup(p *os.Process) {
	if p == nil || p.Pid <= 0 {
		return
	}
	_ = syscall.Kill(-p.Pid, syscall.SIGTERM)
}

// forceKillProcessGroup 向整个进程组发送 SIGKILL。
func forceKillProcessGroup(p *os.Process) {
	if p == nil || p.Pid <= 0 {
		return
	}
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
}

// shutdownSignals 返回需要优雅退出的信号集合。
func shutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}
