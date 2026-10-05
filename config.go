package main

import (
	"bufio"
	"os"
	"strings"
)

type GlobalConfig struct {
	Host         string
	SSHPort      string
	SSHUser      string
	SSHKey       string
	LogDir       string
	GlobalOpts   string
	RsyncPath    string
	RemoveSource string
	MountPolicy  string
}

type Task struct {
	Name string

	Src          string
	Dst          string
	Opts         string
	Delete       string
	RemoveSource string

	// 挂载门禁
	RequireMounted   string
	RequireUnmounted string
	MountPoint       string
	MountFstype      string

	// 运行时字段
	Selected    bool
	MountState  string
	MountDetail string
}

type Config struct {
	Path    string
	Global  GlobalConfig
	Tasks   []*Task
	TaskMap map[string]*Task
}

func defaultGlobal() GlobalConfig {
	return GlobalConfig{
		Host:         "localhost",
		SSHPort:      "22",
		SSHUser:      "admin",
		SSHKey:       "~/.ssh/id_ed25519",
		LogDir:       "/var/log/rbackup",
		GlobalOpts:   "-avzhu --progress",
		RsyncPath:    "sudo rsync",
		RemoveSource: "no",
		MountPolicy:  "skip",
	}
}

func ParseConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &Config{
		Path:    path,
		Global:  defaultGlobal(),
		TaskMap: make(map[string]*Task),
	}

	var current *Task
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[task_") && strings.HasSuffix(line, "]") {
			name := line[len("[task_") : len(line)-1]
			current = &Task{Name: name}
			cfg.Tasks = append(cfg.Tasks, current)
			cfg.TaskMap[name] = current
			continue
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		if current == nil {
			switch key {
			case "HOST":
				cfg.Global.Host = val
			case "SSH_PORT":
				cfg.Global.SSHPort = val
			case "SSH_USER":
				cfg.Global.SSHUser = val
			case "SSH_KEY":
				cfg.Global.SSHKey = val
			case "LOG_DIR":
				cfg.Global.LogDir = val
			case "GLOBAL_OPTS":
				cfg.Global.GlobalOpts = val
			case "RSYNC_PATH":
				cfg.Global.RsyncPath = val
			case "DEFAULT_REMOVE_SOURCE":
				cfg.Global.RemoveSource = val
			case "MOUNT_POLICY":
				cfg.Global.MountPolicy = val
			}
		} else {
			switch key {
			case "src":
				current.Src = val
			case "dst":
				current.Dst = val
			case "opts":
				current.Opts = val
			case "delete":
				current.Delete = val
			case "remove_source":
				current.RemoveSource = val

			case "require_mounted":
				current.RequireMounted = val
			case "require_unmounted":
				current.RequireUnmounted = val
			// 兼容旧键
			case "require_mount":
				if val == "yes" {
					current.RequireMounted = "yes"
				} else if val == "no" {
					current.RequireMounted = "no"
				}

			case "mount_point":
				current.MountPoint = val
			// 兼容旧键
			case "mount_path":
				current.MountPoint = val

			case "mount_fstype":
				current.MountFstype = val
			}
		}
	}

	return cfg, scanner.Err()
}

// NeedsMountCheck 是否需要挂载门禁
func (t *Task) NeedsMountCheck(policy string) bool {
	if policy == "ignore" {
		return false
	}
	return t.MountGateMode() != ""
}

// MountGateMode 返回门禁模式
//
//	""                 不检查
//	"require_mounted"   要求已挂载
//	"require_unmounted" 要求未挂载
func (t *Task) MountGateMode() string {
	if t.RequireMounted == "yes" {
		return "require_mounted"
	}
	if t.RequireUnmounted == "yes" {
		return "require_unmounted"
	}
	return ""
}

// NeedsDelete 是否启用删除模式
func (t *Task) NeedsDelete() bool {
	return t.Delete == "yes"
}

// NeedsRemoveSource 是否删除源文件
func (t *Task) NeedsRemoveSource(g GlobalConfig) bool {
	if t.RemoveSource == "" {
		return g.RemoveSource == "yes"
	}
	return t.RemoveSource == "yes"
}

// IsDangerous 是否危险操作
func (t *Task) IsDangerous(g GlobalConfig) bool {
	return t.NeedsDelete() || t.NeedsRemoveSource(g)
}

// SrcHasSlash 源末尾是否有 /
func (t *Task) SrcHasSlash() bool {
	return strings.HasSuffix(t.Src, "/")
}
