# rbackup-tui

rbackup.sh 的终端用户界面（TUI），用于管理和执行多任务 rsync 远程备份。

- 语言: Go
- TUI 框架: tview + tcell
- 后端: rbackup.sh
- 平台: Linux、Windows (MSYS2)

---

## 目录

- 功能
- 界面说明
- 安装
- 配置
- 使用
- 快捷键
- 挂载门禁
- 构建
- 环境变量
- 常见问题
- 文件结构
- 作者

---

## 功能

- 多任务管理：读取 config.ini，列出全部任务
- 批量选择：空格 / a / n 选择任务
- 运行 / 预览：Enter 运行，d 预演（dry-run）
- 危险操作二次确认：--delete、--remove-source-files 逐个确认，支持 y/n/a/s
- 挂载门禁：支持 require_mounted / require_unmounted
- 实时日志：rsync 输出实时滚动，可暂停、翻页
- 累积统计：成功 / 跳过 / 失败 / 挂载门禁失败
- 错误摘要：失败任务自动提取错误行
- 等效命令：任务详情区实时显示将要执行的 rsync 命令
- 焦点切换：任务区 / 交互区
- 帮助浮层：F1 或 ?
- 版本信息：编译时通过 -ldflags 注入 git 版本

---

## 界面说明

以 config1.ini.example 为例：

    ┌─ rbackup ─────────────────────────────────────────────────────┐
    │ 脚本: /home/user/rbackup/rbackup.sh  |  配置: config1.ini     │
    │ 远端: admin@example.com:22  |  策略: skip  |  日志: ...      │
    ├─ [1] 任务列表 [Tab/1] ────────────────────────────────────────┤
    │  ● alice          /d/alice/my_company/  → /srv/.../doc-alice │
    │  ● bob            /d/bob/my_company/    → /srv/.../doc-bob   │
    │  ○ charlie        /d/charlie/DevOps     → /srv/st1000dm      │
    │  ○ Test1          ~/rsync/test1.d/      → /srv/st1000dm/t1   │
    │  ○ Test2          ~/rsync/test2.d/      → /srv/st1000dm/t2   │
    │                                                                │
    │ 任务数: 5  已选择: 2  alice bob                               │
    │ 选择[空格]  全选[a]  全不选[n]  运行[Enter]  预览[d]  挂载[m]  │
    │ 移动[↑↓/j/k]  翻页[PgUp/PgDn]  首尾[Home/End/g/G]              │
    │ 命令: rsync -avzhu --progress --delete ...                    │
    ├─ [2] 交互区 [Tab/2] ──────────────────────────────────────────┤
    │ 模式: 实际执行   进度: 2/5   当前: bob                         │
    │ [2026-01-01 10:00:15] 任务: bob                                │
    │   sending incremental file list                                │
    │   ...                                                          │
    │ 滚动[↑↓/j/k]  翻页[PgUp/PgDn]  首尾[Home/End/g/G]  暂停[p]    │
    ├────────────────────────────────────────────────────────────────┤
    │ 就绪  |  已选: 2/5  |  交互: 同步中 (2/5)  |  焦点: 任务区     │
    │ 切换焦点[Tab/1/2]  退出[q/Esc/Ctrl+C]  帮助[F1/?]             │
    └────────────────────────────────────────────────────────────────┘

---

## 安装

### 依赖

- Go 1.21+
- rbackup.sh（同目录或指定路径）

### 源码安装

    git clone <repo-url> ~/rbackup-tui
    cd ~/rbackup-tui
    make build

### 用户级安装（无需 sudo）

    make install

安装到：

- ~/.local/bin/rbackup-tui
- ~/.local/bin/rbackup.sh

### 系统级安装

    sudo make install PREFIX=/usr/local

### 卸载

    make uninstall

只删除二进制和脚本，不删除配置文件目录。

---

## 配置

rbackup-tui 与 rbackup.sh 共用 config.ini，格式一致。

### 全局配置

| 键 | 说明 | 默认 |
|---|---|---|
| HOST | 远端主机 | localhost |
| SSH_PORT | SSH 端口 | 22 |
| SSH_USER | SSH 用户名 | admin |
| SSH_KEY | SSH 私钥路径 | ~/.ssh/id_ed25519 |
| LOG_DIR | 日志目录 | /var/log/rbackup |
| GLOBAL_OPTS | rsync 全局选项 | -avzhu --progress |
| RSYNC_PATH | 提权命令 | sudo rsync |
| DEFAULT_REMOVE_SOURCE | 源端删除默认值 | no |
| MOUNT_POLICY | 门禁失败策略 | skip |

MOUNT_POLICY 取值：

- skip：跳过，不算失败
- fail：记失败，影响退出码
- ignore：不检查

### 任务配置

    [task_example]
    src=/path/to/source/
    dst=/path/to/target/
    opts=--no-perms --chown=user:group --update
    delete=yes
    remove_source=no
    require_mounted=yes
    mount_point=/path/to/mount/point
    mount_fstype=fuse.gocryptfs

| 键 | 说明 |
|---|---|
| src | 本地源路径（~ 会展开） |
| dst | 远端目标路径 |
| opts | 任务级 rsync 附加选项 |
| delete | yes / no，启用回收站模式 |
| remove_source | yes / no，同步后删除源文件 |
| require_mounted | yes，要求 mount_point 已挂载（新键） |
| require_unmounted | yes，要求 mount_point 未挂载（新键） |
| mount_point | 要检查的挂载点（新键，省略时默认取 dst） |
| mount_fstype | require_mounted 时的期望类型 |

兼容旧键：

- require_mount=yes 等价于 require_mounted=yes
- require_mount=no 等价于不检查
- mount_path 等价于 mount_point

推荐使用新键，但旧键依然有效。

### 示例配置一：明文源 → 明文挂载点

对应 `config1.ini.example`。源是明文目录，目标是已解密的 gocryptfs 挂载点，使用旧键 `require_mount` / `mount_path` 书写。

    # ============================================================
    # 全局配置
    # ============================================================
    HOST=example.com
    SSH_PORT=22
    SSH_KEY=~/.ssh/id_ed25519
    LOG_DIR=/var/log/rbackup
    GLOBAL_OPTS=-avzhu --progress
    RSYNC_PATH=sudo rsync
    DEFAULT_REMOVE_SOURCE=no

    # 挂载门禁策略: skip | fail | ignore
    MOUNT_POLICY=skip

    # ============================================================
    # 任务
    # ============================================================
    [task_alice]
    src=/d/alice/my_company/
    dst=/srv/st1000dm/Work/doc-alice
    opts=--no-perms --chown=admin:users --update --delete-after
    delete=yes
    remove_source=no
    require_mount=yes
    mount_path=/srv/st1000dm/Work
    mount_fstype=fuse.gocryptfs

    [task_bob]
    src=/d/bob/my_company/
    dst=/srv/st1000dm/Work/doc-bob
    opts=--no-perms --chown=admin:users --update --delete-after
    delete=yes
    remove_source=no
    require_mount=yes
    mount_path=/srv/st1000dm/Work
    mount_fstype=fuse.gocryptfs

    [task_charlie]
    src=/d/charlie/DevOps
    dst=/srv/st1000dm
    opts=--no-perms --chown=admin:users --update --delete-after
    delete=yes
    remove_source=no
    require_mount=no

    [task_Test1]
    src=~/rsync/test1.d/
    dst=/srv/st1000dm/test1.d
    delete=yes
    require_mount=yes
    mount_path=/srv/st1000dm/test1.d
    mount_fstype=fuse.gocryptfs

    [task_Test2]
    src=~/rsync/test2.d/
    dst=/srv/st1000dm/test2.d
    delete=yes
    require_mount=no
    mount_path=/srv/st1000dm/test2.d
    mount_fstype=fuse.gocryptfs

**特点**：

- 使用旧键 `require_mount` / `mount_path`（兼容写法）
- 门禁模式：`require_mounted`（目标必须是已挂载的明文挂载点）
- 源是明文，目标是已解密的 gocryptfs 挂载点

### 示例配置二：密文源 → 密文目录

对应 `config2.ini.example`。源是 gocryptfs 底层密文目录，目标是另一个 gocryptfs 底层密文目录，使用新键 `require_unmounted` / `mount_point` 书写。

    # ============================================================
    # 备份配置文件示例
    # 放置于脚本同目录下，或通过 --config 指定其他路径
    # ============================================================

    # 全局 SSH 参数
    HOST=example.com
    SSH_PORT=22
    SSH_KEY=/home/admin/.ssh/id_ed25519

    # 日志目录（自动生成 ${SCRIPT_NAME}_${DATE}.log）
    LOG_DIR=/var/log

    # 全局 rsync 默认选项（可被任务级 opts 追加覆盖）
    GLOBAL_OPTS=-avzhu --progress

    # 提权选项，需要命令行中用 --sudo 激活
    RSYNC_PATH=sudo rsync

    # ============================================================
    # 任务定义
    # ============================================================

    [task_Personal_Vault]
    src=/srv/st1000dm/Personal_Vault
    dst=/mnt/wd4000g
    opts=--size-only
    delete=yes

    [task_Study]
    src=/srv/st1000dm/Study
    dst=/mnt/wd4000g
    delete=yes

    [task_Life]
    src=/srv/st1000dm/Life
    dst=/mnt/wd4000g
    opts=-rtvh
    delete=yes

    [task_infra_secrets_cipher]
    src=/srv/st1000dm/.cipher.d/infra_secrets
    dst=/mnt/wd4000g/.cipher.d
    opts=-avh
    delete=yes
    require_unmounted=yes
    mount_point=/mnt/wd4000g/infra_secrets
    mount_fstype=fuse.gocryptfs

    [task_Work_Cipher]
    src=/srv/st1000dm/.cipher.d/Work
    dst=/mnt/wd4000g/.cipher.d
    opts=-avh
    delete=yes
    require_unmounted=yes
    mount_point=/mnt/wd4000g/Work
    mount_fstype=fuse.gocryptfs

**特点**：

- 使用新键 `require_unmounted` / `mount_point`
- 门禁模式：`require_unmounted`（目标明文挂载点必须未挂载）
- 源是密文，目标是 gocryptfs 底层密文目录
- 前三个任务（Personal_Vault、Study、Life）未加密，不设门禁

### 两份示例的对比

| 项 | 示例一（config1） | 示例二（config2） |
|---|---|---|
| 源 | 明文目录 | 密文目录 |
| 目标 | 明文挂载点 | 密文目录 |
| 门禁 | require_mount=yes（旧键） | require_unmounted=yes（新键） |
| 键风格 | 兼容旧键 | 推荐新键 |
| 典型任务 | task_alice、task_bob、task_charlie | task_Work_Cipher、task_infra_secrets_cipher |
| 门禁判定 | 目标必须已挂载 | 目标必须未挂载 |

### 使用示例配置

    cp config1.ini.example config1.ini
    vim config1.ini

    rbackup-tui -c config1.ini -s rbackup.sh

---

## 使用

### 启动

    # 默认读取 $HOME/rbackup/config.ini
    rbackup-tui

    # 指定配置
    rbackup-tui -c ~/rbackup/config1.ini

    # 指定脚本
    rbackup-tui -c ~/rbackup/config1.ini -s ~/rbackup/rbackup.sh

### 命令行参数

| 参数 | 说明 |
|---|---|
| -c, --config | 配置文件路径 |
| -s, --script | rbackup.sh 路径 |

### 典型流程

1. 启动 TUI
2. 空格选择要备份的任务
3. Enter 运行
4. 如有危险操作，逐个确认（y/n/a/s）
5. 观察实时日志
6. 完成后按 Tab 切到交互区翻页查看

---

## 快捷键

### 全局

| 键 | 功能 |
|---|---|
| F1 / ? | 显示 / 关闭帮助 |
| Tab / 1 / 2 | 切换焦点 |
| Ctrl+D ×3 | 强制退出程序 |

### 空闲（无运行、无确认）

| 键 | 任务区 | 交互区 |
|---|---|---|
| ↑ ↓ / j k | 移动光标 | 滚动日志 |
| PgUp / PgDn | 翻页 | 翻页 |
| Home / End / g G | 首 / 尾 | 首 / 尾 |
| 空格 | 选择 | 暂停 |
| a / n | 全选 / 全不选 | — |
| Enter | 运行 | — |
| d | 预览 | — |
| m | 挂载检查 | — |
| r | 刷新配置 | — |
| q / Esc / Ctrl+C | 退出 | 退出 |

### 运行中

| 键 | 功能 |
|---|---|
| Ctrl+C | 停止任务 |
| p / 空格 | 暂停 / 继续自动滚动 |
| ↑ ↓ / j k | 滚动日志 |
| PgUp / PgDn | 翻页 |
| Home / End / g G | 首 / 尾 |
| Esc / q | 无效 |

### 危险确认

| 键 | 功能 |
|---|---|
| y | 确认当前任务 |
| n | 跳过当前任务 |
| a | 全部确认 |
| s | 全部跳过 |
| Ctrl+C | 全部跳过 |
| ↑ ↓ / PgUp / PgDn | 滚动详情 |
| Esc / q | 无效 |

---

## 挂载门禁

### 语义

| 场景 | 源 | 目标 | 门禁 |
|---|---|---|---|
| 明文 → 明文挂载点 | 明文 | gocryptfs 挂载点 | require_mounted |
| 密文 → 密文目录 | 密文 | gocryptfs 底层 | require_unmounted |

### require_mounted

用于同步明文到已解密的挂载点。

- 目标必须已挂载，且类型匹配
- 未挂载 → 拒绝（避免明文写入未加密磁盘）

配置示例：

    require_mount=yes
    mount_path=/srv/st1000dm/Work
    mount_fstype=fuse.gocryptfs

或新键写法：

    require_mounted=yes
    mount_point=/srv/st1000dm/Work
    mount_fstype=fuse.gocryptfs

### require_unmounted

用于同步密文到 gocryptfs 底层。

- 目标必须未挂载
- 已挂载 → 拒绝（gocryptfs 正在使用底层密文，写入会破坏一致性）

配置示例：

    require_unmounted=yes
    mount_point=/mnt/wd4000g/Work
    mount_fstype=fuse.gocryptfs

### 两者互斥

同时为 yes 会报错退出（退出码 2）。

---

## 构建

### 本地平台

    make build

### 交叉编译

    make build-win              # Windows exe
    make build-linux            # Linux amd64
    make build-linux-arm64      # Linux arm64
    make build-all              # 全部

### 产物位置

    bin/
    ├── windows/rbackup-tui.exe
    └── linux/rbackup-tui

### 版本注入

Makefile 会自动注入版本：

    LDFLAGS := -X main.Version=$(git describe --tags --always --dirty) \
               -X main.BuildTime=$(date +%Y-%m-%d) \
               -X main.GitCommit=$(git rev-parse --short HEAD)

打 tag 后版本号会显示为 v1.1.0，未打 tag 显示 commit hash。

---

## 环境变量

| 变量 | 说明 | 默认 |
|---|---|---|
| RBACKUP_CONFIG | 配置文件路径 | $HOME/rbackup/config.ini |
| RBACKUP_SCRIPT | rbackup.sh 路径 | $HOME/rbackup/rbackup.sh |
| RBACKUP_BASH | bash 可执行文件路径 | 自动查找 |
| RBACKUP_DEBUG_KEYS | 设为 1 输出按键调试日志 | — |
| RBACKUP_SEP | 分隔线字符 | ┄ |

---

## 常见问题

### F1 无反应？

部分终端（gnome-terminal、konsole）把 F1 保留为帮助。使用 ? 代替。

### 按 F1 或 ? 后帮助浮层出现但按键无响应？

检查 RBACKUP_DEBUG_KEYS=1，按一次 F1 看 key.log。若无 Key=xxx 记录，说明按键被终端拦截。

### Windows 下按键重复触发？

MSYS2 mintty 会重复分发事件。代码内置 50ms 去重。

### Windows 下 ESC 无法退出？

MSYS2 + winpty 会吞掉 ESC。使用 q 或 Ctrl+C。

### Linux 下 rbackup-tui 找不到 bash？

设置 RBACKUP_BASH=/bin/bash，或在 config.ini 中确认 SSH_KEY 等路径正确。

### rsync 输出没显示？

先做挂载检查和远端目录检查，输出有几秒延迟。等待即可。

### 日志目录不可写？

rbackup.sh 会自动回退到脚本同目录的 log/。

### 如何只做挂载检查？

命令行 rbackup.sh --check-mount；TUI 中按 m。

### 危险确认时怎么快速跳过全部？

按 s（no to all）或 Ctrl+C。

### 如何强制退出？

1.5 秒内连按 3 次 Ctrl+D。

---

## 文件结构

    rbackup-tui/
    ├── main.go                  TUI 主程序
    ├── config.go                配置解析
    ├── runner.go                执行 rbackup.sh
    ├── go.mod
    ├── go.sum
    ├── Makefile
    ├── README.md
    ├── LICENSE
    ├── config1.ini.example      示例一：明文源 → 明文挂载点
    ├── config2.ini.example      示例二：密文源 → 密文目录
    └── bin/                     编译产物（不入库）
        ├── windows/
        └── linux/

---

## 作者

Jet Locke

---

## 许可证

MIT License

Copyright (c) 2026 Jet Locke

详见同目录 LICENSE 文件。
