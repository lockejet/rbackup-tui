#!/bin/bash
# ============================================================
# 远程备份核心脚本 - 基于 rsync + ssh 的多任务备份工具
# 支持远端挂载门禁（gocryptfs 等）
# 用法：./rbackup.sh [选项]
# ============================================================

set -euo pipefail

# ---------- 解析脚本真实路径（支持符号链接） ----------
SCRIPT_PATH="$0"
while [ -L "$SCRIPT_PATH" ]; do
    link="$(readlink "$SCRIPT_PATH")"
    case "$link" in
        /*) SCRIPT_PATH="$link" ;;
        *)  SCRIPT_PATH="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)/$link" ;;
    esac
done
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"

# ---------- 默认配置 ----------
DEFAULT_CONFIG="${SCRIPT_DIR}/config.ini"
DEFAULT_HOST="192.168.8.254"
DEFAULT_SSH_PORT="28375"
DEFAULT_SSH_KEY="/home/admin/.ssh/id_ed25519-host_admin"
DEFAULT_LOG_DIR="/var/log/rbackup"
DEFAULT_GLOBAL_OPTS="-avzhu --progress"
DEFAULT_RSYNC_PATH="sudo rsync"
DEFAULT_REMOVE_SOURCE="no"
DEFAULT_MOUNT_POLICY="skip"

# ---------- 全局变量 ----------
CONFIG_FILE="$DEFAULT_CONFIG"
DRY_RUN=0
AUTO_DRY=0
LIST_MODE=0
FORCE_MODE=0
CHECK_MOUNT_ONLY=0
NO_MOUNT_CHECK=0
HOST="$DEFAULT_HOST"
SSH_PORT="$DEFAULT_SSH_PORT"
SSH_KEY="$DEFAULT_SSH_KEY"
LOG_DIR="$DEFAULT_LOG_DIR"
GLOBAL_OPTS="$DEFAULT_GLOBAL_OPTS"
RSYNC_PATH="$DEFAULT_RSYNC_PATH"
REMOVE_SOURCE_GLOBAL="$DEFAULT_REMOVE_SOURCE"
MOUNT_POLICY="$DEFAULT_MOUNT_POLICY"
ENABLE_RSYNC_PATH=0
RUN_MODE=""
TEMP_MODE=0

declare -A TASK_SRC
declare -A TASK_DST
declare -A TASK_OPTS
declare -A TASK_DELETE
declare -A TASK_REMOVE_SOURCE
declare -A TASK_REQUIRE_MOUNTED
declare -A TASK_REQUIRE_UNMOUNTED
declare -A TASK_MOUNT_POINT
declare -A TASK_MOUNT_FSTYPE
TASK_NAMES=()

TASK_MODE="none"
SPECIFIED_TASKS=()
RUN_TASKS=()

TEMP_SRC=""
TEMP_DST=""
TEMP_OPTS=""
TEMP_DELETE=0
TEMP_REMOVE_SOURCE=0
TEMP_REQUIRE_MOUNTED=""
TEMP_REQUIRE_UNMOUNTED=""
TEMP_MOUNT_POINT=""
TEMP_MOUNT_FSTYPE=""

IS_INTERACTIVE=0
if [ -t 0 ] && [ -t 1 ]; then
    IS_INTERACTIVE=1
fi

# ---------- 路径展开 ----------
expand_local_path() {
    local p="$1"
    case "$p" in
        "~")   echo "$HOME" ;;
        "~/"*) echo "${HOME}/${p#\~/}" ;;
        *)     echo "$p" ;;
    esac
}

# ---------- 解析命令行参数 ----------
OPTS=$(getopt -o hC:AT:s:d:o:l \
    --long help,config:,dry-run,all,tasks:,task:,src:,dst:,opts:,delete,sudo,list,remove-source-files,force,no-mount-check,check-mount,require-mounted,require-unmounted,mount-point:,mount-path:,mount-fstype: \
    -n "$0" -- "$@") || { echo "参数解析错误，请使用 -h 查看帮助" >&2; exit 2; }
eval set -- "$OPTS"

while true; do
    case "$1" in
        -C|--config)           CONFIG_FILE="$2"; shift 2 ;;
        --dry-run)             DRY_RUN=1; shift ;;
        -A|--all)              TASK_MODE="all"; shift ;;
        -T|--tasks)
            IFS=',' read -ra TASKS <<< "$2"
            for t in "${TASKS[@]}"; do
                t="$(echo "$t" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
                [ -n "$t" ] && SPECIFIED_TASKS+=("$t")
            done
            TASK_MODE="specified"; shift 2 ;;
        --task)                SPECIFIED_TASKS+=("$2"); TASK_MODE="specified"; shift 2 ;;
        -s|--src)              TEMP_SRC="$(expand_local_path "$2")"; shift 2 ;;
        -d|--dst)              TEMP_DST="$2"; shift 2 ;;
        -o|--opts)             TEMP_OPTS="$2"; shift 2 ;;
        --delete)              TEMP_DELETE=1; shift ;;
        --sudo)                ENABLE_RSYNC_PATH=1; shift ;;
        -l|--list)             LIST_MODE=1; shift ;;
        --remove-source-files) TEMP_REMOVE_SOURCE=1; shift ;;
        --force)               FORCE_MODE=1; shift ;;
        --no-mount-check)      NO_MOUNT_CHECK=1; shift ;;
        --check-mount)         CHECK_MOUNT_ONLY=1; shift ;;
        --require-mounted)     TEMP_REQUIRE_MOUNTED="yes"; shift ;;
        --require-unmounted)   TEMP_REQUIRE_UNMOUNTED="yes"; shift ;;
        --mount-point)         TEMP_MOUNT_POINT="$2"; shift 2 ;;
        --mount-path)          TEMP_MOUNT_POINT="$2"; shift 2 ;;
        --mount-fstype)        TEMP_MOUNT_FSTYPE="$2"; shift 2 ;;
        -h|--help)
            cat <<EOF
用法: $0 [选项]

核心备份脚本，ssh 模式，支持远端挂载门禁。

选项:
  -C, --config <文件>     指定配置文件（默认脚本同目录 config.ini）
  --dry-run               预览模式，仅显示命令不执行
  -A, --all               同步所有任务
  -T, --tasks <列表>      仅同步指定任务，逗号分隔
  --task <名称>           单个任务名，可重复
  -l, --list              列出任务及属性后退出

临时任务:
  -s, --src <路径>        -d, --dst <路径>
  -o, --opts <选项>
  --delete                启用删除（回收站模式）
  --remove-source-files   同步后删除本地源文件（危险）
  --sudo                  启用 RSYNC_PATH 提权
  --require-mounted       要求 mount_point 已挂载
  --require-unmounted     要求 mount_point 未挂载
  --mount-point <路径>    要检查的挂载点路径
  --mount-fstype <类型>   require_mounted 时的期望文件系统类型

挂载门禁:
  --check-mount           只做挂载检查，不实际同步
  --no-mount-check        跳过远端挂载门禁（危险）

通用:
  --force                 跳过所有交互确认（慎用）
  -h, --help              显示此帮助

说明:
  - 未指定任务选择选项（-A/-T/--task）时，默认自动 dry-run 所有任务。
  - 未挂载/已挂载的处理由 MOUNT_POLICY 决定: skip(默认) / fail / ignore。
  - src 末尾带 / 表示同步目录内容；不带 / 表示同步目录本身。
  - 删除模式下，远端 .deleted_files 回收站会被 --exclude 排除。
  - 非交互环境下交互确认自动跳过，但 remove_source 未用 --force 会报错退出。

挂载门禁语义:
  require_mounted=yes    目标必须已挂载（用于 源明文 → 目标明文挂载点）
  require_unmounted=yes  目标必须未挂载（用于 源密文 → 目标密文目录）
  二者互斥，同时为 yes 会报错退出。
EOF
            exit 0 ;;
        --) shift; break ;;
        *) echo "内部错误" >&2; exit 1 ;;
    esac
done

# ---------- 临时任务模式检查 ----------
if [ -n "$TEMP_SRC" ] || [ -n "$TEMP_DST" ]; then
    if [ -z "$TEMP_SRC" ] || [ -z "$TEMP_DST" ]; then
        echo "错误：临时任务必须同时指定 -s 和 -d" >&2
        exit 1
    fi
    if [ ! -f "$CONFIG_FILE" ]; then
        echo "错误：临时任务需要配置文件 '$CONFIG_FILE' 获取全局参数，但文件不存在" >&2
        exit 1
    fi
    TEMP_MODE=1
fi

# ---------- 加载配置文件 ----------
if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误：配置文件 '$CONFIG_FILE' 不存在" >&2
    exit 1
fi

current_section=""
while IFS= read -r line || [ -n "$line" ]; do
    line="$(printf '%s' "$line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
    [ -z "$line" ] && continue
    case "$line" in \#*) continue ;; esac

    if [[ "$line" =~ ^\[task_([^]]+)\]$ ]]; then
        task_name="${BASH_REMATCH[1]}"
        current_section="task"
        TASK_NAMES+=("$task_name")
        continue
    fi

    if [[ "$line" =~ ^([^=]+)=(.*)$ ]]; then
        key="${BASH_REMATCH[1]}"
        value="${BASH_REMATCH[2]}"
        key="$(printf '%s' "$key" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
        value="$(printf '%s' "$value" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

        if [ "$current_section" = "task" ]; then
            case "$key" in
                src)                 TASK_SRC["$task_name"]="$(expand_local_path "$value")" ;;
                dst)                 TASK_DST["$task_name"]="$value" ;;
                opts)                TASK_OPTS["$task_name"]="$value" ;;
                delete)              TASK_DELETE["$task_name"]="$value" ;;
                remove_source)       TASK_REMOVE_SOURCE["$task_name"]="$value" ;;
                require_mounted)     TASK_REQUIRE_MOUNTED["$task_name"]="$value" ;;
                require_unmounted)   TASK_REQUIRE_UNMOUNTED["$task_name"]="$value" ;;
                # 兼容旧键：require_mount=yes 映射到 require_mounted
                require_mount)
                    case "$value" in
                        yes) TASK_REQUIRE_MOUNTED["$task_name"]="yes" ;;
                        no)  TASK_REQUIRE_MOUNTED["$task_name"]="no" ;;
                    esac
                    ;;
                mount_point)         TASK_MOUNT_POINT["$task_name"]="$value" ;;
                # 兼容旧键：mount_path 视同 mount_point
                mount_path)          TASK_MOUNT_POINT["$task_name"]="$value" ;;
                mount_fstype)        TASK_MOUNT_FSTYPE["$task_name"]="$value" ;;
                *) echo "警告：任务节 '$task_name' 中存在未知键 '$key'，已忽略" >&2 ;;
            esac
        else
            case "$key" in
                HOST)                   HOST="$value" ;;
                SSH_PORT)               SSH_PORT="$value" ;;
                SSH_KEY)                SSH_KEY="$value" ;;
                LOG_DIR)                LOG_DIR="$value" ;;
                GLOBAL_OPTS)            GLOBAL_OPTS="$value" ;;
                RSYNC_PATH)             RSYNC_PATH="$value" ;;
                DEFAULT_REMOVE_SOURCE)  REMOVE_SOURCE_GLOBAL="$value" ;;
                MOUNT_POLICY)           MOUNT_POLICY="$value" ;;
                *) echo "警告：全局配置中存在未知键 '$key'，已忽略" >&2 ;;
            esac
        fi
    else
        echo "警告：无法解析的行：$line" >&2
    fi
done < "$CONFIG_FILE"

case "$MOUNT_POLICY" in
    skip|fail|ignore) ;;
    *) echo "警告：MOUNT_POLICY='$MOUNT_POLICY' 无效，重置为 skip" >&2; MOUNT_POLICY="skip" ;;
esac

# ---------- 配置校验：require_mounted / require_unmounted 互斥 ----------
for task_name in "${TASK_NAMES[@]}"; do
    rm_val="${TASK_REQUIRE_MOUNTED[$task_name]:-}"
    ru_val="${TASK_REQUIRE_UNMOUNTED[$task_name]:-}"
    if [ "$rm_val" = "yes" ] && [ "$ru_val" = "yes" ]; then
        echo "错误：任务 '$task_name' 同时设置了 require_mounted=yes 和 require_unmounted=yes" >&2
        echo "      二者互斥，请二选一。" >&2
        exit 2
    fi
done

# ---------- 列表模式 ----------
if [ $LIST_MODE -eq 1 ]; then
    echo "配置文件: $CONFIG_FILE"
    echo "远程主机: $HOST:$SSH_PORT"
    echo "挂载策略: MOUNT_POLICY=$MOUNT_POLICY"
    echo ""
    if [ ${#TASK_NAMES[@]} -eq 0 ]; then
        echo "未定义任何任务。"
    else
        echo "共找到 ${#TASK_NAMES[@]} 个任务:"
        echo ""
        for task_name in "${TASK_NAMES[@]}"; do
            rm_val="${TASK_REQUIRE_MOUNTED[$task_name]:-no}"
            ru_val="${TASK_REQUIRE_UNMOUNTED[$task_name]:-no}"
            if [ "$rm_val" = "yes" ]; then
                gate_desc="require_mounted"
            elif [ "$ru_val" = "yes" ]; then
                gate_desc="require_unmounted"
            else
                gate_desc="(不检查)"
            fi
            src_val="${TASK_SRC[$task_name]:-}"
            case "$src_val" in
                */) src_note="（末尾有 /，同步内容）" ;;
                *)  src_note="（末尾无 /，同步目录本身）" ;;
            esac
            echo "任务名: $task_name"
            echo "  src: ${src_val:-(未设置)} $src_note"
            echo "  dst: ${TASK_DST[$task_name]:-(未设置)}"
            echo "  opts: ${TASK_OPTS[$task_name]:-(空)}"
            echo "  delete: ${TASK_DELETE[$task_name]:-no}"
            echo "  remove_source: ${TASK_REMOVE_SOURCE[$task_name]:-$REMOVE_SOURCE_GLOBAL}"
            echo "  挂载门禁: $gate_desc"
            echo "    mount_point: ${TASK_MOUNT_POINT[$task_name]:-(默认取 dst)}"
            echo "    mount_fstype: ${TASK_MOUNT_FSTYPE[$task_name]:-(空)}"
            echo ""
        done
    fi
    exit 0
fi

# ---------- 确定运行模式 ----------
if [ $TEMP_MODE -eq 1 ]; then
    RUN_MODE="temp"
else
    if [ ${#TASK_NAMES[@]} -eq 0 ]; then
        echo "错误：配置文件中未定义任何任务（至少需要一个 [task_xxx] 节）" >&2
        exit 1
    fi

    if [ "$TASK_MODE" = "all" ]; then
        RUN_TASKS=("${TASK_NAMES[@]}")
        if [ ${#SPECIFIED_TASKS[@]} -gt 0 ]; then
            echo "警告：指定了 -A，将忽略其他任务指定 (${SPECIFIED_TASKS[*]})" >&2
        fi
        AUTO_DRY=0
    elif [ "$TASK_MODE" = "specified" ]; then
        RUN_TASKS=()
        for t in "${SPECIFIED_TASKS[@]}"; do
            found=0
            for task in "${TASK_NAMES[@]}"; do
                if [ "$task" = "$t" ]; then found=1; break; fi
            done
            if [ $found -eq 1 ]; then
                RUN_TASKS+=("$t")
            else
                echo "警告：指定的任务 '$t' 在配置文件中不存在，已忽略" >&2
            fi
        done
        if [ ${#RUN_TASKS[@]} -eq 0 ]; then
            echo "错误：所有指定的任务均不存在，无法执行任何任务。" >&2
            exit 1
        fi
        AUTO_DRY=0
    else
        RUN_TASKS=("${TASK_NAMES[@]}")
        if [ $CHECK_MOUNT_ONLY -eq 1 ]; then
            AUTO_DRY=0
        else
            AUTO_DRY=1
        fi
    fi
    RUN_MODE="task"
fi

RUN_MODE="${RUN_MODE:-task}"

# ---------- 初始化日志 ----------
SCRIPT_NAME="$(basename "$SCRIPT_PATH" .sh)"
LOG_DATE="$(date +%Y%m%d)"
LOG_FILE="${LOG_DIR}/${SCRIPT_NAME}_${LOG_DATE}.log"

if ! mkdir -p "$LOG_DIR" 2>/dev/null; then
    LOG_FILE="./${SCRIPT_NAME}_${LOG_DATE}.log"
    echo "警告：无法创建日志目录 $LOG_DIR，将使用当前目录 $LOG_FILE" >&2
fi

if ! touch "$LOG_FILE" 2>/dev/null; then
    LOG_FILE="./${SCRIPT_NAME}_${LOG_DATE}.log"
    echo "警告：无法写入 $LOG_FILE，将使用当前目录 $LOG_FILE" >&2
    touch "$LOG_FILE" || { echo "错误：无法创建日志文件" >&2; exit 1; }
fi

# ---------- 辅助函数 ----------
contains_host() {
    case "$1" in
        *@*:*) return 0 ;;
        *) return 1 ;;
    esac
}

get_basename() {
    local path="$1"
    path="${path%/}"
    echo "${path##*/}"
}

# 远端挂载检查
#   参数: mode  path  want_fstype
#         mode = "mounted"   要求已挂载（TARGET == path 且 FSTYPE 匹配）
#         mode = "unmounted" 要求未挂载（TARGET != path）
#   返回: 0=通过  1=未通过  2=SSH/检查错误
#   stdout: 通过时输出当前状态描述；失败时输出诊断标记（__UNMOUNTED__ 等）
remote_mount_check2() {
    local mode="$1"
    local path="$2"
    local want_fstype="${3:-}"

    local out rc
    out=$(ssh -p "$SSH_PORT" -i "$SSH_KEY" "admin@${HOST}" \
          "if [ ! -e '$path' ]; then echo '__NO_PATH__'; exit 0; fi; \
           findmnt -rn -T '$path' -o TARGET,FSTYPE" 2>/dev/null) \
        && rc=0 || rc=$?

    if [ $rc -ne 0 ]; then
        echo "__SSH_ERR__ rc=$rc"
        return 2
    fi

    case "$out" in
        __NO_PATH__)
            if [ "$mode" = "unmounted" ]; then
                echo "路径不存在（视为未挂载）"
                return 0
            else
                echo "__NO_PATH__"
                return 1
            fi
            ;;
    esac

    if [ -z "$out" ]; then
        echo "__EMPTY__"
        return 2
    fi

    local tgt fstype
    read -r tgt fstype <<<"$out"

    if [ "$mode" = "mounted" ]; then
        if [ "$tgt" != "$path" ]; then
            echo "__UNMOUNTED__ tgt=$tgt fstype=$fstype"
            return 1
        fi
        if [ -n "$want_fstype" ]; then
            case "$fstype" in
                *"$want_fstype"*) ;;
                *)
                    echo "__WRONG_FSTYPE__ got=$fstype want=$want_fstype"
                    return 1
                    ;;
            esac
        fi
        echo "$fstype"
        return 0
    else
        # unmounted
        if [ "$tgt" = "$path" ]; then
            echo "__MOUNTED__ fstype=$fstype"
            return 1
        fi
        echo "未挂载（最近挂载点 $tgt，fstype=$fstype）"
        return 0
    fi
}

# 打印挂载失败诊断信息（含完整 SSH 命令、绝对路径、多级备选）
print_mount_fail_hint() {
    local mode="$1"
    local path="$2"
    local diag="$3"
    local ssh_prefix="ssh -p ${SSH_PORT} -i ${SSH_KEY} admin@${HOST}"

    case "$diag" in
        __UNMOUNTED__*)
            local detail="${diag#__UNMOUNTED__ }"
            {
                echo "  远端挂载: [FAIL] $path 未挂载"
                echo "            期望状态: 已挂载"
                echo "            实际状态: ${detail}"
                echo "            → 目标不是解密挂载点，同步明文会写入未加密磁盘，已拒绝"
                echo ""
                echo "            排查命令（在目标主机 ${HOST} 上执行）："
                echo "              $ssh_prefix \\"
                echo "                  '/usr/bin/findmnt -rn -T $path -o TARGET,FSTYPE'"
                echo ""
                echo "            若确认未挂载，请重新挂载 gocryptfs 后重试。"
            } | tee -a "$LOG_FILE"
            ;;
        __WRONG_FSTYPE__*)
            local detail="${diag#__WRONG_FSTYPE__ }"
            {
                echo "  远端挂载: [FAIL] $path 已挂载，但文件系统类型不匹配"
                echo "            期望类型: ${TASK_MOUNT_FSTYPE[$task_name]:-}"
                echo "            实际状态: ${detail}"
                echo "            → 目标挂载的不是期望的加密文件系统，已拒绝"
                echo ""
                echo "            排查命令（在目标主机 ${HOST} 上执行）："
                echo "              $ssh_prefix \\"
                echo "                  '/usr/bin/findmnt -rn -T $path -o TARGET,FSTYPE'"
            } | tee -a "$LOG_FILE"
            ;;
        __NO_PATH__)
            {
                echo "  远端挂载: [FAIL] $path 不存在"
                echo "            期望状态: 已挂载"
                echo "            → 目标路径不存在，无法作为挂载点，已拒绝"
                echo ""
                echo "            排查命令（在目标主机 ${HOST} 上执行）："
                echo "              $ssh_prefix \\"
                echo "                  'ls -ld $path'"
            } | tee -a "$LOG_FILE"
            ;;
        __MOUNTED__*)
            local detail="${diag#__MOUNTED__ }"
            {
                echo "  远端挂载: [FAIL] $path 已挂载"
                echo "            期望状态: 未挂载"
                echo "            实际状态: ${detail}"
                echo "            → gocryptfs 正在使用底层密文，此时同步密文会破坏数据一致性"
                echo "            → 已拒绝同步"
                echo ""
                echo "            排查命令（在目标主机 ${HOST} 上执行）："
                echo "              $ssh_prefix \\"
                echo "                  '/usr/bin/findmnt -rn -T $path -o TARGET,FSTYPE'"
                echo ""
                echo "            卸载目标挂载点（按推荐顺序）："
                echo ""
                echo "              【1】systemd 管理时（最干净）："
                echo "                $ssh_prefix \\"
                echo "                    'sudo /usr/bin/systemctl stop <mount-unit>'"
                echo "                （用 'systemctl list-units --type=mount' 查找对应 unit）"
                echo ""
                echo "              【2】直接卸载 FUSE（推荐）："
                echo "                $ssh_prefix \\"
                echo "                    '/usr/bin/fusermount3 -u $path'"
                echo ""
                echo "              【3】旧版 gocryptfs（无 fusermount3 时）："
                echo "                $ssh_prefix \\"
                echo "                    '/usr/bin/fusermount -u $path'"
                echo ""
                echo "              【4】以上失败时，检查是否有进程占用："
                echo "                $ssh_prefix \\"
                echo "                    '/usr/sbin/lsof +D $path'"
                echo ""
                echo "            注意：不要使用 fusermount -z（lazy unmount），"
                echo "                  它会在内核清理前返回，可能导致数据丢失。"
            } | tee -a "$LOG_FILE"
            ;;
        __SSH_ERR__*)
            local detail="${diag#__SSH_ERR__ }"
            {
                echo "  远端挂载: [FAIL] 无法检查 $path（SSH 失败）"
                echo "            错误: $detail"
                echo ""
                echo "            排查命令（从当前主机执行）："
                echo "              $ssh_prefix 'echo ok'"
            } | tee -a "$LOG_FILE"
            ;;
        *)
            {
                echo "  远端挂载: [FAIL] $path 检查失败"
                echo "            详情: $diag"
            } | tee -a "$LOG_FILE"
            ;;
    esac
}

# 仅当「源不带 / 且 basename 与目标 basename 相同」时才警告嵌套
confirm_task() {
    local task_name="$1" src="$2" dst="$3"

    case "$src" in
        */) return 0 ;;
    esac

    if [ $FORCE_MODE -eq 1 ] || [ $IS_INTERACTIVE -eq 0 ]; then
        return 0
    fi

    local src_basename dst_basename
    src_basename="$(get_basename "$src")"
    dst_basename="$(get_basename "$dst")"

    if [ "$src_basename" = "$dst_basename" ]; then
        echo "[WARN] 任务 '$task_name'：源目录名 '$src_basename' 与目标目录名 '$dst_basename' 相同。" >&2
        echo "       源不带 /，将在目标下创建同名子目录：${dst}/${src_basename}" >&2
        echo "       如果希望同步内容而不是嵌套目录，请在 src 末尾加 /。" >&2
        read -r -p "       是否继续同步该任务？[y/N] " answer
        case "$answer" in
            y|Y|"") return 0 ;;
            *) echo "       跳过任务 '$task_name'。" >&2; return 1 ;;
        esac
    fi
    return 0
}

# ---------- do_backup ----------
do_backup() {
    local task_name="$1"
    local src="$2"
    local dst="$3"
    local opts="$4"
    local delete_flag="$5"
    local remove_source_flag="$6"
    local is_temp="$7"
    local require_mounted="$8"
    local require_unmounted="$9"
    local mount_point="${10}"
    local mount_fstype="${11}"

    if [ "$remove_source_flag" = "yes" ] && [ $FORCE_MODE -eq 0 ] && [ $IS_INTERACTIVE -eq 0 ]; then
        echo "错误：任务 '$task_name' 启用了 remove_source，但当前为非交互环境且未使用 --force，拒绝执行。" | tee -a "$LOG_FILE"
        return 1
    fi

    # ---------- 挂载门禁 ----------
    local gate_mode=""
    if [ $NO_MOUNT_CHECK -eq 0 ] && [ "$MOUNT_POLICY" != "ignore" ]; then
        if [ "$require_mounted" = "yes" ]; then
            gate_mode="mounted"
        elif [ "$require_unmounted" = "yes" ]; then
            gate_mode="unmounted"
        fi
    fi

    local mount_status=""
    if [ -n "$gate_mode" ]; then
        local check_path="$mount_point"
        [ -z "$check_path" ] && check_path="$dst"

        local mount_info=""
        local mount_rc=0
        mount_info=$(remote_mount_check2 "$gate_mode" "$check_path" "$mount_fstype") || mount_rc=$?

        case $mount_rc in
            0)
                if [ "$gate_mode" = "mounted" ]; then
                    mount_status="  远端挂载: [OK] $check_path 已挂载 ($mount_info)"
                else
                    mount_status="  远端挂载: [OK] $check_path $mount_info"
                fi
                ;;
            1)
                # 门禁失败，打印诊断
                print_mount_fail_hint "$gate_mode" "$check_path" "$mount_info"
                if [ $CHECK_MOUNT_ONLY -eq 1 ]; then
                    return 2
                fi
                if [ "$MOUNT_POLICY" = "fail" ]; then
                    echo "[$(date '+%Y-%m-%d %H:%M:%S')] 任务 '$task_name' 因挂载门禁失败。" | tee -a "$LOG_FILE"
                    return 3
                else
                    echo "[$(date '+%Y-%m-%d %H:%M:%S')] 任务 '$task_name' 因挂载门禁跳过。" | tee -a "$LOG_FILE"
                    return 2
                fi
                ;;
            2)
                # SSH 或检查错误
                print_mount_fail_hint "$gate_mode" "$check_path" "$mount_info"
                echo "[$(date '+%Y-%m-%d %H:%M:%S')] 任务 '$task_name' 挂载检查失败。" | tee -a "$LOG_FILE"
                return 3
                ;;
        esac
    fi

    if [ $CHECK_MOUNT_ONLY -eq 1 ]; then
        echo "[CHECK-MOUNT] 任务 '$task_name' 挂载检查通过。" | tee -a "$LOG_FILE"
        return 0
    fi

    # ---------- 远端父目录检查 ----------
    local remote_dst="$dst"
    local parent_dir
    parent_dir="$(dirname "$remote_dst")"

    if ! ssh -p "$SSH_PORT" -i "$SSH_KEY" "admin@${HOST}" "test -d '$parent_dir'" 2>/dev/null; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] 错误：远程父目录 '$parent_dir' 不存在，无法创建目标目录 '$remote_dst'。" | tee -a "$LOG_FILE"
        return 1
    fi

    if [ $DRY_RUN -eq 0 ] && [ $AUTO_DRY -eq 0 ]; then
        if ! ssh -p "$SSH_PORT" -i "$SSH_KEY" "admin@${HOST}" "mkdir -p '$remote_dst'" 2>&1 | tee -a "$LOG_FILE"; then
            echo "[$(date '+%Y-%m-%d %H:%M:%S')] 错误：无法创建远程目标目录 '$remote_dst'" | tee -a "$LOG_FILE"
            return 1
        fi
    else
        echo "[DRY-RUN] 跳过创建目标目录 '$remote_dst'（仅预览）" | tee -a "$LOG_FILE"
    fi

    local full_dst="$dst"
    if ! contains_host "$full_dst"; then
        full_dst="admin@${HOST}:${full_dst}"
    fi
    local remote_path
    remote_path="${full_dst#*:}"

    local delete_opts=""
    local recycle_bin=""
    if [ "$delete_flag" = "yes" ]; then
        local timestamp
        timestamp="$(date +%Y%m%d_%H%M)"
        recycle_bin="${remote_path}/.deleted_files/${SCRIPT_NAME}/${timestamp}"
        # --exclude 防止 --delete 删除回收站自身
        delete_opts="--delete --exclude='/.deleted_files/' --backup --backup-dir=\"${recycle_bin}\""
    fi

    local remove_source_opts=""
    if [ "$remove_source_flag" = "yes" ]; then
        remove_source_opts="--remove-source-files"
    fi

    local rsync_opts="--progress"
    [ -n "$GLOBAL_OPTS" ] && rsync_opts="$rsync_opts $GLOBAL_OPTS"
    if [ $ENABLE_RSYNC_PATH -eq 1 ] && [ -n "$RSYNC_PATH" ]; then
        rsync_opts="$rsync_opts --rsync-path=\"$RSYNC_PATH\""
    fi
    [ -n "$opts" ] && rsync_opts="$rsync_opts $opts"
    [ -n "$delete_opts" ] && rsync_opts="$rsync_opts $delete_opts"
    [ -n "$remove_source_opts" ] && rsync_opts="$rsync_opts $remove_source_opts"

    local ssh_cmd="ssh -p ${SSH_PORT} -i ${SSH_KEY}"
    local ts
    ts="$(date +%s)"
    local cmd="rsync $rsync_opts --suffix=\"_${ts}\" -e \"$ssh_cmd\" \"$src\" \"$full_dst\""

    echo "========================================" | tee -a "$LOG_FILE"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] 任务: $task_name" | tee -a "$LOG_FILE"
    echo "  源: $src" | tee -a "$LOG_FILE"
    case "$src" in
        */) echo "  源末尾斜杠: 有（同步内容到目标目录）" | tee -a "$LOG_FILE" ;;
        *)  echo "  源末尾斜杠: 无（同步目录本身到目标下）" | tee -a "$LOG_FILE" ;;
    esac
    echo "  目标: $full_dst" | tee -a "$LOG_FILE"
    [ -n "$mount_status" ] && echo "$mount_status" | tee -a "$LOG_FILE"
    echo "  Rsync 选项: $rsync_opts" | tee -a "$LOG_FILE"
    if [ -n "$delete_opts" ]; then
        echo "  删除选项: $delete_opts" | tee -a "$LOG_FILE"
        echo "  回收站排除: /.deleted_files/（防止 --delete 删除回收站）" | tee -a "$LOG_FILE"
    else
        echo "  删除选项: 未启用" | tee -a "$LOG_FILE"
    fi
    if [ "$remove_source_flag" = "yes" ]; then
        echo "  源端删除: 已启用 (--remove-source-files)" | tee -a "$LOG_FILE"
    else
        echo "  源端删除: 未启用" | tee -a "$LOG_FILE"
    fi
    if [ $ENABLE_RSYNC_PATH -eq 1 ] && [ -n "$RSYNC_PATH" ]; then
        echo "  提权命令: $RSYNC_PATH" | tee -a "$LOG_FILE"
    fi

    if [ $DRY_RUN -eq 1 ] || [ $AUTO_DRY -eq 1 ]; then
        echo "[DRY-RUN] 将要执行的命令:" | tee -a "$LOG_FILE"
        echo "  $cmd" | tee -a "$LOG_FILE"
        return 0
    fi

    if [ $is_temp -eq 0 ]; then
        if ! confirm_task "$task_name" "$src" "$dst"; then
            echo "[$(date '+%Y-%m-%d %H:%M:%S')] 任务 '$task_name' 因用户取消而跳过。" | tee -a "$LOG_FILE"
            return 1
        fi
    fi

    if [ "$delete_flag" = "yes" ]; then
        local mkdir_cmd="mkdir -p \"${recycle_bin}\""
        if ! ssh -p "$SSH_PORT" -i "$SSH_KEY" "admin@${HOST}" "$mkdir_cmd" 2>/dev/null; then
            echo "错误：无法在远程创建回收站目录 ${recycle_bin}" | tee -a "$LOG_FILE"
            return 1
        fi
    fi

    if [ "$remove_source_flag" = "yes" ] && [ $FORCE_MODE -eq 0 ] && [ $IS_INTERACTIVE -eq 1 ]; then
        echo "[WARN] 任务 '$task_name' 启用了 --remove-source-files，同步后将删除本地源文件！" | tee -a "$LOG_FILE"
        read -r -p "       确认继续？[y/N] " ans
        case "$ans" in
            y|Y|"") ;;
            *) echo "       已取消任务 '$task_name'。" | tee -a "$LOG_FILE"; return 1 ;;
        esac
    fi

    echo "----------------------------------------" | tee -a "$LOG_FILE"
    eval "$cmd" 2>&1 | tee -a "$LOG_FILE"
    local rsync_exit=${PIPESTATUS[0]}

    if [ $rsync_exit -eq 0 ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] 任务 '$task_name' 成功完成。" | tee -a "$LOG_FILE"
        return 0
    else
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] 任务 '$task_name' 失败 (退出码: $rsync_exit)" | tee -a "$LOG_FILE"
        return 1
    fi
}

# ---------- 主执行 ----------
echo "============================================================" | tee -a "$LOG_FILE"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] 备份脚本启动 (PID: $$)" | tee -a "$LOG_FILE"
echo "脚本路径: $SCRIPT_PATH" | tee -a "$LOG_FILE"
echo "配置文件: $CONFIG_FILE" | tee -a "$LOG_FILE"
echo "远程主机: $HOST:$SSH_PORT" | tee -a "$LOG_FILE"

if [ $NO_MOUNT_CHECK -eq 1 ]; then
    echo "挂载门禁: 已禁用 (--no-mount-check)" | tee -a "$LOG_FILE"
else
    echo "挂载门禁: MOUNT_POLICY=$MOUNT_POLICY" | tee -a "$LOG_FILE"
fi

if [ "$RUN_MODE" = "temp" ]; then
    echo "模式: 临时任务" | tee -a "$LOG_FILE"
    echo "  源: $TEMP_SRC" | tee -a "$LOG_FILE"
    echo "  目标: $TEMP_DST" | tee -a "$LOG_FILE"
    [ -n "$TEMP_OPTS" ] && echo "  额外选项: $TEMP_OPTS" | tee -a "$LOG_FILE"
    [ $TEMP_DELETE -eq 1 ] && echo "  删除模式: 启用" | tee -a "$LOG_FILE"
    [ $TEMP_REMOVE_SOURCE -eq 1 ] && echo "  源端删除: 启用" | tee -a "$LOG_FILE"
    [ -n "$TEMP_REQUIRE_MOUNTED" ] && echo "  挂载门禁: require_mounted=$TEMP_REQUIRE_MOUNTED" | tee -a "$LOG_FILE"
    [ -n "$TEMP_REQUIRE_UNMOUNTED" ] && echo "  挂载门禁: require_unmounted=$TEMP_REQUIRE_UNMOUNTED" | tee -a "$LOG_FILE"
    [ -n "$TEMP_MOUNT_POINT" ] && echo "  挂载点: $TEMP_MOUNT_POINT" | tee -a "$LOG_FILE"
    [ -n "$TEMP_MOUNT_FSTYPE" ] && echo "  挂载类型: $TEMP_MOUNT_FSTYPE" | tee -a "$LOG_FILE"
else
    if [ $CHECK_MOUNT_ONLY -eq 1 ]; then
        echo "模式: 仅挂载检查 (--check-mount)" | tee -a "$LOG_FILE"
    elif [ $AUTO_DRY -eq 1 ] && [ $DRY_RUN -eq 0 ]; then
        echo "模式: 任务模式（未指定任务，自动 dry-run 预览所有任务）" | tee -a "$LOG_FILE"
        DRY_RUN=1
    elif [ $DRY_RUN -eq 1 ]; then
        echo "模式: 任务模式 (dry-run)" | tee -a "$LOG_FILE"
    else
        echo "模式: 任务模式 (实际执行)" | tee -a "$LOG_FILE"
    fi
    if [ "$TASK_MODE" = "all" ]; then
        echo "任务选择: 全部 (显式 -A)" | tee -a "$LOG_FILE"
    elif [ "$TASK_MODE" = "specified" ]; then
        echo "任务选择: 指定 (${RUN_TASKS[*]})" | tee -a "$LOG_FILE"
    else
        echo "任务选择: 全部 (默认)" | tee -a "$LOG_FILE"
    fi
fi
if [ $ENABLE_RSYNC_PATH -eq 1 ] && [ -n "$RSYNC_PATH" ]; then
    echo "提权已启用: $RSYNC_PATH" | tee -a "$LOG_FILE"
fi
if [ $FORCE_MODE -eq 1 ]; then
    echo "强制模式: 已启用（跳过所有交互确认）" | tee -a "$LOG_FILE"
fi
echo "日志文件: $LOG_FILE" | tee -a "$LOG_FILE"
echo "============================================================" | tee -a "$LOG_FILE"

success_tasks=()
failed_tasks=()
skipped_tasks=()
mount_failed_tasks=()

if [ "$RUN_MODE" = "temp" ]; then
    delete_flag="no"; [ $TEMP_DELETE -eq 1 ] && delete_flag="yes"
    remove_flag="no"; [ $TEMP_REMOVE_SOURCE -eq 1 ] && remove_flag="yes"

    rc=0
    do_backup "__temp__" "$TEMP_SRC" "$TEMP_DST" "$TEMP_OPTS" \
        "$delete_flag" "$remove_flag" 1 \
        "$TEMP_REQUIRE_MOUNTED" "$TEMP_REQUIRE_UNMOUNTED" \
        "$TEMP_MOUNT_POINT" "$TEMP_MOUNT_FSTYPE" || rc=$?
    case $rc in
        0) success_tasks+=("__temp__") ;;
        1) failed_tasks+=("__temp__") ;;
        2) skipped_tasks+=("__temp__") ;;
        3) mount_failed_tasks+=("__temp__") ;;
    esac
else
    for task_name in "${RUN_TASKS[@]}"; do
        src="${TASK_SRC[$task_name]:-}"
        dst="${TASK_DST[$task_name]:-}"
        opts="${TASK_OPTS[$task_name]:-}"
        delete_flag="${TASK_DELETE[$task_name]:-}"
        remove_flag="${TASK_REMOVE_SOURCE[$task_name]:-}"
        require_mounted="${TASK_REQUIRE_MOUNTED[$task_name]:-no}"
        require_unmounted="${TASK_REQUIRE_UNMOUNTED[$task_name]:-no}"
        mount_point="${TASK_MOUNT_POINT[$task_name]:-}"
        mount_fstype="${TASK_MOUNT_FSTYPE[$task_name]:-}"

        if [ -z "$src" ] || [ -z "$dst" ]; then
            echo "错误：任务 '$task_name' 缺少 src 或 dst，跳过。" | tee -a "$LOG_FILE"
            failed_tasks+=("$task_name")
            continue
        fi
        if [ -z "$delete_flag" ]; then
            delete_flag="no"
        elif [ "$delete_flag" != "yes" ] && [ "$delete_flag" != "no" ]; then
            echo "警告：任务 '$task_name' 的 delete 值 '$delete_flag' 无效，重置为 'no'" | tee -a "$LOG_FILE"
            delete_flag="no"
        fi
        if [ -z "$remove_flag" ]; then
            remove_flag="$REMOVE_SOURCE_GLOBAL"
        elif [ "$remove_flag" != "yes" ] && [ "$remove_flag" != "no" ]; then
            echo "警告：任务 '$task_name' 的 remove_source 值 '$remove_flag' 无效，重置为全局默认 '$REMOVE_SOURCE_GLOBAL'" | tee -a "$LOG_FILE"
            remove_flag="$REMOVE_SOURCE_GLOBAL"
        fi

        rc=0
        do_backup "$task_name" "$src" "$dst" "$opts" \
            "$delete_flag" "$remove_flag" 0 \
            "$require_mounted" "$require_unmounted" \
            "$mount_point" "$mount_fstype" || rc=$?
        case $rc in
            0) success_tasks+=("$task_name") ;;
            1) failed_tasks+=("$task_name") ;;
            2) skipped_tasks+=("$task_name") ;;
            3) mount_failed_tasks+=("$task_name") ;;
        esac
    done
fi

echo "============================================================" | tee -a "$LOG_FILE"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] 汇总:" | tee -a "$LOG_FILE"

if [ ${#success_tasks[@]} -gt 0 ]; then
    echo "  成功 ${#success_tasks[@]}: ${success_tasks[*]}" | tee -a "$LOG_FILE"
else
    echo "  成功 0" | tee -a "$LOG_FILE"
fi

if [ ${#skipped_tasks[@]} -gt 0 ]; then
    echo "  跳过 ${#skipped_tasks[@]}: ${skipped_tasks[*]}" | tee -a "$LOG_FILE"
else
    echo "  跳过 0" | tee -a "$LOG_FILE"
fi

if [ ${#failed_tasks[@]} -gt 0 ]; then
    echo "  失败 ${#failed_tasks[@]}: ${failed_tasks[*]}" | tee -a "$LOG_FILE"
else
    echo "  失败 0" | tee -a "$LOG_FILE"
fi

if [ ${#mount_failed_tasks[@]} -gt 0 ]; then
    echo "  挂载门禁失败 ${#mount_failed_tasks[@]}: ${mount_failed_tasks[*]}" | tee -a "$LOG_FILE"
else
    echo "  挂载门禁失败 0" | tee -a "$LOG_FILE"
fi

if [ ${#failed_tasks[@]} -gt 0 ]; then
    exit 1
elif [ ${#mount_failed_tasks[@]} -gt 0 ]; then
    exit 3
else
    exit 0
fi