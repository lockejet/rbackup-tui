package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type TaskResult struct {
	TaskName string
	ExitCode int
	Err      error
}

// ---------- 查找 bash ----------
// Windows 不能直接执行 .sh，必须通过 bash 解释器。
func findBash() string {
	if p := os.Getenv("RBACKUP_BASH"); p != "" {
		return p
	}
	if p, err := exec.LookPath("bash"); err == nil {
		return p
	}
	candidates := []string{
		`D:\msys64\usr\bin\bash.exe`,
		`C:\msys64\usr\bin\bash.exe`,
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files\Git\usr\bin\bash.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "bash"
}

// ---------- 对外接口 ----------

func StreamTask(ctx context.Context, scriptPath, configPath, taskName string, onLine func(string)) *TaskResult {
	args := []string{"--config", configPath, "--task", taskName}
	return streamExec(ctx, scriptPath, args, taskName, onLine)
}

func StreamCheckMount(ctx context.Context, scriptPath, configPath string, onLine func(string)) *TaskResult {
	args := []string{"--config", configPath, "--check-mount"}
	return streamExec(ctx, scriptPath, args, "__check_mount__", onLine)
}

func StreamDryRun(ctx context.Context, scriptPath, configPath, taskName string, onLine func(string)) *TaskResult {
	args := []string{"--config", configPath, "--task", taskName, "--dry-run"}
	return streamExec(ctx, scriptPath, args, taskName, onLine)
}

// ---------- 核心执行 ----------

func streamExec(ctx context.Context, scriptPath string, args []string, name string, onLine func(string)) *TaskResult {
	bash := findBash()

	// 用 bash 执行 .sh 脚本
	fullArgs := append([]string{scriptPath}, args...)
	cmd := exec.CommandContext(ctx, bash, fullArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &TaskResult{TaskName: name, ExitCode: -1, Err: err}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &TaskResult{TaskName: name, ExitCode: -1, Err: err}
	}

	result := &TaskResult{TaskName: name, ExitCode: -1}

	if err := cmd.Start(); err != nil {
		result.Err = err
		return result
	}

	var wg sync.WaitGroup
	wg.Add(2)

	readLines := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			if onLine != nil {
				onLine(scanner.Text())
			}
		}
	}

	go readLines(stdout)
	go readLines(stderr)

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Err = err
		}
	} else {
		result.ExitCode = 0
	}
	return result
}

// ---------- 路径工具 ----------

func expandTilde(p string) string {
	if p == "" {
		return p
	}
	if p == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}