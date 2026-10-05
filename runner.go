package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type TaskStats struct {
	FilesTransferred int
	FilesTotal       int
	BytesTotal       int64
	BytesSent        int64
	BytesReceived    int64
	RsyncMS          int64
	ListMS           int64
	PrepMS           int64
}

type TaskResult struct {
	TaskName string
	ExitCode int
	Err      error
	Stats    *TaskStats
}

// 解析 rbackup.sh 输出的 [STATS] 行
func parseStatsLine(line string) *TaskStats {
	if !strings.HasPrefix(line, "[STATS] ") {
		return nil
	}
	s := &TaskStats{}
	fields := strings.Fields(line)
	for _, f := range fields {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], kv[1]
		switch k {
		case "files":
			parts := strings.SplitN(v, "/", 2)
			if len(parts) == 2 {
				s.FilesTransferred, _ = strconv.Atoi(parts[0])
				s.FilesTotal, _ = strconv.Atoi(parts[1])
			}
		case "bytes_total":
			s.BytesTotal, _ = strconv.ParseInt(v, 10, 64)
		case "bytes_sent":
			s.BytesSent, _ = strconv.ParseInt(v, 10, 64)
		case "bytes_recv":
			s.BytesReceived, _ = strconv.ParseInt(v, 10, 64)
		case "rsync_ms":
			s.RsyncMS, _ = strconv.ParseInt(v, 10, 64)
		case "list_ms":
			s.ListMS, _ = strconv.ParseInt(v, 10, 64)
		case "prep_ms":
			s.PrepMS, _ = strconv.ParseInt(v, 10, 64)
		}
	}
	return s
}

// ---------- 查找 bash ----------
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

	// 脚本路径验证
	if scriptPath == "" {
		return &TaskResult{TaskName: name, ExitCode: -1, Err: fmt.Errorf("脚本路径为空")}
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return &TaskResult{TaskName: name, ExitCode: -1,
			Err: fmt.Errorf("脚本不存在: %s", scriptPath)}
	}

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

	var (
		statsMu     sync.Mutex
		parsedStats *TaskStats
	)

	var wg sync.WaitGroup
	wg.Add(2)

	readLines := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			// 拦截 [STATS] 行，不传给 UI
			if s := parseStatsLine(line); s != nil {
				statsMu.Lock()
				parsedStats = s
				statsMu.Unlock()
				continue
			}
			if onLine != nil {
				onLine(line)
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

	statsMu.Lock()
	result.Stats = parsedStats
	statsMu.Unlock()

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
