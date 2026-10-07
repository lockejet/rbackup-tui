#!/usr/bin/env bash
# ============================================================
# rbackup-gate.sh —— 把 rbackup 的“挂载门禁”接到 lazyrsync 上
#
# 原理：lazyrsync 通过 settings.toml 的 rsync_path 调用“rsync”，
#       本脚本冒充 rsync 被调用，检查通过后再 exec 真正的 rsync。
#
# 覆盖范围：TUI 运行(run.rs)、TUI 预览(preview.rs)、无头运行(headless.rs)
#           三处都走 rsync::binary()，即 rsync_path，所以一处接入即全生效。
#
# 用法（settings.toml）：
#   rsync_path = "/home/admin/rbackup-tui/integrations/lazyrsync/rbackup-gate.sh"
#
# lazyrsync 任务的 advanced.raw_args 里带上任务名，门禁参数即可直接复用
# 你现有的 config*.ini（单一配置源，不重复维护）：
#   [profile.task.advanced]
#   raw_args = "--rbackup-task=alice --rbackup-config=/home/admin/rbackup-tui/config-lnas.ini"
#
# 环境变量：
#   RBACKUP_REAL_RSYNC  真正的 rsync（默认 /usr/bin/rsync，或 $PATH 里第一个）
#   RBACKUP_CONFIG      默认 config.ini 路径（raw_args 里不写 --rbackup-config 时用）
#   RBACKUP_SSH         ssh 客户端（默认 ssh；测试时可指向桩程序）
#   RBACKUP_GATE_DRYRUN dry-run 时的行为：warn(默认，只告警不拦截) | block(拦截)
#   RBACKUP_GATE_EXIT   门禁拒绝时的退出码，默认 40（rsync 官方码止于 35/255，不冲突）
# ============================================================
set -uo pipefail

SELF_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

# ---------- 参数解析 ----------
ARGS=("$@")

TASK_NAME=""
CONFIG="${RBACKUP_CONFIG:-}"
DRY_RUN=0
RSH_STR=""
POSITIONAL=()

seen_dd=0
for ((i = 0; i < ${#ARGS[@]}; i++)); do
    a="${ARGS[$i]}"
    if [ "$seen_dd" -eq 1 ]; then
        POSITIONAL+=("$a")
        continue
    fi
    case "$a" in
        --) seen_dd=1 ;;
        --rbackup-task=*)   TASK_NAME="${a#--rbackup-task=}" ;;
        --rbackup-config=*) CONFIG="${a#--rbackup-config=}" ;;
        --rsh=*)            RSH_STR="${a#--rsh=}" ;;
        --dry-run)          DRY_RUN=1 ;;
        -*) # 短选项簇里出现 n 即 --dry-run（lazyrsync 只可能生成 -n）
            case "${a#-}" in
                *n*) [ "${a#--}" = "$a" ] && DRY_RUN=1 ;;
            esac
            ;;
        *)  POSITIONAL+=("$a") ;;
    esac
done

REAL_RSYNC_CMD=()
real_rsync_cmd() { # 支持多词命令，例如 RBACKUP_REAL_RSYNC="sudo rsync"（对应 RSYNC_PATH=sudo rsync）
    if [ "${#REAL_RSYNC_CMD[@]}" -eq 0 ]; then
        if [ -n "${RBACKUP_REAL_RSYNC:-}" ]; then
            read -r -a REAL_RSYNC_CMD <<<"$RBACKUP_REAL_RSYNC"
        else
            REAL_RSYNC_CMD=("$(command -v rsync 2>/dev/null || printf '%s' /usr/bin/rsync)")
        fi
    fi
}
exec_real() {
    real_rsync_cmd
    exec "${REAL_RSYNC_CMD[@]}" "$@"
}

# 探测调用（--version/--help）：原样交给真 rsync，stdout 必须干净
for a in "${ARGS[@]}"; do
    case "$a" in
        --version|--help)
            exec_real "${ARGS[@]}"
            ;;
    esac
done

# ---------- 剥掉本脚本私有参数，其余原样传给真 rsync ----------
PASS=()
for a in "${ARGS[@]}"; do
    case "$a" in
        --rbackup-task=*|--rbackup-config=*) continue ;;
    esac
    PASS+=("$a")
done

gate_exit() { # $1=诊断文本
    printf '%s\n' "$1" >&2
    exit "${RBACKUP_GATE_EXIT:-40}"
}

# 无任务名 → 不启用门禁，直接放行
if [ -z "$TASK_NAME" ]; then
    exec_real "${PASS[@]}"
fi

# ---------- 读取门禁配置（复用 rbackup 的 config*.ini） ----------
if [ -z "$CONFIG" ]; then
    for c in "$SELF_DIR/config.ini" ./config.ini; do
        [ -f "$c" ] && { CONFIG="$c"; break; }
    done
fi
if [ -z "$CONFIG" ] || [ ! -f "$CONFIG" ]; then
    gate_exit "[GATE][FAIL] 任务 '$TASK_NAME'：找不到门禁配置文件（用 --rbackup-config= 或 RBACKUP_CONFIG 指定）"
fi

MOUNT_POLICY="skip"
REQ_MOUNTED="no"
REQ_UNMOUNTED="no"
MOUNT_PATH=""
MOUNT_FSTYPE=""
in_task=0
while IFS= read -r line || [ -n "$line" ]; do
    line="${line%%#*}"
    line="$(printf '%s' "$line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
    [ -z "$line" ] && continue
    case "$line" in
        \[*)
            # 节名同时容忍 [task_name] 与 [name] 两种写法
            sec="${line#\[}"; sec="${sec%\]}"
            if [ "$sec" = "task_$TASK_NAME" ] || [ "$sec" = "$TASK_NAME" ]; then
                in_task=1
                continue
            fi
            if [ "$in_task" -eq 1 ]; then break; fi
            continue
            ;;
    esac
    key="${line%%=*}"; value="${line#*=}"
    key="$(printf '%s' "$key" | sed -e 's/[[:space:]]*$//')"
    value="$(printf '%s' "$value" | sed -e 's/^[[:space:]]*//')"
    if [ "$in_task" -eq 1 ]; then
        case "$key" in
            require_mounted)   REQ_MOUNTED="$value" ;;
            require_unmounted) REQ_UNMOUNTED="$value" ;;
            require_mount)     case "$value" in yes) REQ_MOUNTED=yes ;; no) REQ_MOUNTED=no ;; esac ;;
            mount_point|mount_path) MOUNT_PATH="$value" ;;
            mount_fstype)      MOUNT_FSTYPE="$value" ;;
        esac
    else
        case "$key" in
            MOUNT_POLICY) MOUNT_POLICY="$value" ;;
        esac
    fi
done < "$CONFIG"

if [ "$MOUNT_POLICY" = "ignore" ]; then
    exec_real "${PASS[@]}"
fi
if [ "$REQ_MOUNTED" != "yes" ] && [ "$REQ_UNMOUNTED" != "yes" ]; then
    exec_real "${PASS[@]}"   # 该任务没配门禁
fi
if [ "$REQ_MOUNTED" = "yes" ] && [ "$REQ_UNMOUNTED" = "yes" ]; then
    gate_exit "[GATE][FAIL] 任务 '$TASK_NAME'：require_mount 与 require_unmounted 互斥，拒绝执行"
fi

# 目标路径：mount_path 优先，否则取最后一个位置参数（rsync 的 dst）
if [ "${#POSITIONAL[@]}" -lt 2 ]; then
    exec_real "${PASS[@]}"
fi
DST_ARG="${POSITIONAL[${#POSITIONAL[@]}-1]}"
CHECK_PATH="${MOUNT_PATH:-$DST_ARG}"

case "$CHECK_PATH" in
    *"'"*) gate_exit "[GATE][FAIL] 任务 '$TASK_NAME'：路径含单引号，无法安全传给远端探测：$CHECK_PATH" ;;
esac

# ---------- 远端 / 本地 判定 ----------
REMOTE_SPEC=""
LOCAL_PATH="$CHECK_PATH"
case "$CHECK_PATH" in
    *:*)
        # user@host:/path 或 host:/path（排除 Windows 盘符误判：单字母:）
        head="${CHECK_PATH%%:*}"
        if [ "${CHECK_PATH#*:}" != "$CHECK_PATH" ] && [ "${#head}" -gt 1 ]; then
            REMOTE_SPEC="$head"
            LOCAL_PATH="${CHECK_PATH#*:}"
        fi
        ;;
esac

PROBE="if [ ! -e '$LOCAL_PATH' ]; then echo __NO_PATH__; exit 0; fi; \
real_path=\$(realpath -m '$LOCAL_PATH' 2>/dev/null || echo '$LOCAL_PATH'); \
info=\$(findmnt -rn -T \"\$real_path\" -o TARGET,FSTYPE 2>/dev/null); \
if [ -z \"\$info\" ]; then echo __EMPTY__; exit 0; fi; \
tgt=\$(echo \"\$info\" | awk '{print \$1}'); \
fstype=\$(echo \"\$info\" | awk '{print \$2}'); \
real_tgt=\$(realpath -m \"\$tgt\" 2>/dev/null || echo \"\$tgt\"); \
echo \"\$real_path|\$real_tgt|\$fstype\""

OUT=""
RC=0
if [ -n "$REMOTE_SPEC" ]; then
    SSH_OPTS=()
    if [ -n "$RSH_STR" ]; then
        read -r -a rsh_tokens <<<"$RSH_STR"
        for ((i = 1; i < ${#rsh_tokens[@]}; i++)); do SSH_OPTS+=("${rsh_tokens[$i]}"); done
    fi
    OUT="$("${RBACKUP_SSH:-ssh}" -o BatchMode=yes -o ConnectTimeout=10 "${SSH_OPTS[@]}" \
            "$REMOTE_SPEC" "$PROBE" 2>/dev/null)" && RC=0 || RC=$?
else
    OUT="$(bash -c "$PROBE" 2>/dev/null)" && RC=0 || RC=$?
fi

if [ "$RC" -ne 0 ]; then
    gate_exit "[GATE][FAIL] 任务 '$TASK_NAME'：无法检查 '$CHECK_PATH'（ssh/探测失败 rc=$RC）
           排查： ssh -o BatchMode=yes \"${REMOTE_SPEC}\" 'realpath -m \"$LOCAL_PATH\" && findmnt -rn -T \$(realpath -m \"$LOCAL_PATH\") -o TARGET,FSTYPE'"
fi

STATUS=""; DETAIL=""; BLOCKED=0; ALLOWED_DESC=""
case "$OUT" in
    __NO_PATH__)
        if [ "$REQ_UNMOUNTED" = "yes" ]; then STATUS="OK"; ALLOWED_DESC="路径不存在（视为未挂载）"
        else STATUS="FAIL"; BLOCKED=1; DETAIL="路径不存在，无法作为挂载点"; fi ;;
    __EMPTY__)
        STATUS="FAIL"; BLOCKED=1; DETAIL="findmnt 未返回信息" ;;
    *)
        IFS='|' read -r real_path real_tgt fstype <<<"$OUT"
        if [ "$REQ_MOUNTED" = "yes" ]; then
            if [ "$real_tgt" != "$real_path" ]; then
                STATUS="FAIL"; BLOCKED=1
                DETAIL="$real_path 未挂载（最近挂载点 $real_tgt，fstype=$fstype）"
            elif [ -n "$MOUNT_FSTYPE" ]; then
                case "$fstype" in
                    *"$MOUNT_FSTYPE"*) STATUS="OK"; ALLOWED_DESC="$real_path 已挂载 ($fstype)" ;;
                    *) STATUS="FAIL"; BLOCKED=1
                       DETAIL="$real_path 已挂载但类型不符：got=$fstype want=$MOUNT_FSTYPE" ;;
                esac
            else
                STATUS="OK"; ALLOWED_DESC="$real_path 已挂载 ($fstype)"
            fi
        else
            if [ "$real_tgt" = "$real_path" ]; then
                STATUS="FAIL"; BLOCKED=1
                DETAIL="$real_path 已挂载 (fstype=$fstype)，此时同步密文会破坏数据一致性"
            else
                STATUS="OK"; ALLOWED_DESC="未挂载（最近挂载点 $real_tgt，fstype=$fstype）"
            fi
        fi ;;
esac

# ---------- dry-run：默认只告警不拦截，保证 lazyrsync 的 p 预览可用 ----------
if [ "$DRY_RUN" -eq 1 ]; then
    if [ "$STATUS" = "OK" ]; then
        printf '[GATE][DRY-RUN][OK]   %s: %s\n' "$TASK_NAME" "$ALLOWED_DESC" >&2
    else
        printf '[GATE][DRY-RUN][WARN] %s: 正式执行会被拒绝 —— %s\n' "$TASK_NAME" "$DETAIL" >&2
        if [ "${RBACKUP_GATE_DRYRUN:-warn}" = "block" ]; then
            gate_exit "[GATE][DRY-RUN] 已按 RBACKUP_GATE_DRYRUN=block 拦截"
        fi
    fi
    exec_real "${PASS[@]}"
fi

if [ "$BLOCKED" -eq 1 ]; then
    MODE_DESC="require_mount"
    [ "$REQ_UNMOUNTED" = "yes" ] && MODE_DESC="require_unmounted"
    MSG="[GATE][FAIL] 任务 '$TASK_NAME' 挂载门禁拒绝（$MODE_DESC）
           路径: $CHECK_PATH
           详情: $DETAIL"
    case "$MODE_DESC" in
        require_mount)
            MSG="$MSG
           → 目标不是解密挂载点，同步明文会写入未加密磁盘。请重新挂载 gocryptfs 后重试。"
            ;;
        require_unmounted)
            MSG="$MSG
           → gocryptfs 正在使用底层密文，此时同步密文会破坏数据一致性。
             卸载： /usr/bin/fusermount3 -u $CHECK_PATH    （旧版：/usr/bin/fusermount -u）
             占用： /usr/sbin/lsof +D $CHECK_PATH
             注意： 不要用 fusermount -z（lazy unmount）。"
            ;;
    esac
    if [ -n "$REMOTE_SPEC" ]; then
        MSG="$MSG
           排查(远端): ssh \"${REMOTE_SPEC}\" 'realpath -m \"$LOCAL_PATH\" && findmnt -rn -T \$(realpath -m \"$LOCAL_PATH\") -o TARGET,FSTYPE'"
    fi
    gate_exit "$MSG"
fi

printf '[GATE][OK] %s: %s\n' "$TASK_NAME" "$ALLOWED_DESC" >&2
exec_real "${PASS[@]}"
