# 历史清理与公开化操作单

> **本文档里的命令请你自己执行。** 它们包含 `filter-repo` 重写历史与 `push --force`，
> 都是不可逆操作，不适合由自动化代劳。
>
> **本文件不写真实值。** 文中出现的地址一律使用 RFC 5737 文档保留网段
> （`192.0.2.0/24`、`198.51.100.0/24`、`203.0.113.0/24`），主机名一律用
> `example.com`，用户名一律用 `<user>`。

## 一句话现状（2026-10-30 实测）

| 项目 | 结论 |
|---|---|
| 仓库可见性 | **public**（`gh repo view` / GitHub API 均为 `private: false`） |
| 历史是否已重写 | **否**。历史里仍能取到内网地址与本机密钥路径，且 `origin/main` 与本地一致 |
| 当前工作树 | 干净；真实配置（`config-*.ini`）、真实日志、私有脚本**从未进过本仓库历史** |
| 本文档自身 | 旧版本在 HEAD 里写有真实内网地址与用户名，已于 2026-10-30 脱敏 |

也就是说：**真正要清的只有"文件内容里的字符串"和一个 4.9 MB 二进制**，而不是
"删掉某个被跟踪的配置文件"。这一点和常见做法（`--invert-paths` 删文件）不同，
用错会连源码一起删掉，见第 1 步。

## 前置检查（可复现）

```bash
cd ~/rbackup-tui

# ① 历史里的内网地址（形如 192.0.2.10 / 192.0.2.10:2222）
git log --all -p | grep -oE '192\.168\.[0-9]{1,3}\.[0-9]{1,3}(:[0-9]+)?' | sort | uniq -c

# ② 历史里的本机密钥路径（只是路径，不是密钥本体）
git log --all -p | grep -oE '/home/[^/ ]+/\.ssh/[A-Za-z0-9_.-]+' | sort | uniq -c

# ③ 私钥 / 证书 / 口令 / Token（应全部为空）
git log --all -p | grep -nE 'BEGIN [A-Z ]*PRIVATE KEY|BEGIN OPENSSH|ghp_|AKIA[0-9A-Z]{16}|password=' | head

# ④ 历史里的大对象（二进制残留）
git rev-list --objects --all \
  | git cat-file --batch-check='%(objecttype) %(objectsize) %(rest)' \
  | awk '$2>1048576 {printf "%.2f MB  %s\n", $2/1048576, $3}'

# ⑤ 历史里出现过的全部路径（确认没有真实配置/日志）
git log --all --pretty=format: --name-only | sed '/^$/d' | sort -u
```

2026-10-30 的实测结果：

| 项目 | 结论 |
|---|---|
| 私钥 / 证书 | 历史中**没有**（`PRIVATE KEY`、`BEGIN OPENSSH`、`id_rsa`、`.pem` 零命中） |
| 口令 / Token | 历史中**没有**（`password=`、`passphrase`、`ghp_`、`AKIA` 零命中） |
| 内网信息 | **有**：内网地址若干处，命中 `README.md`、`config.go`、`rbackup.sh`、`config2.ini.example` |
| 本机密钥路径 | **有**：`/home/<user>/.ssh/id_ed25519` 及其变体若干处（只有路径，无密钥内容） |
| 真实配置 / 日志 / 私有脚本 | **没有**：`config-*.ini`、`rbackup.d/`、`log/`、`bin/`、`dist/`、`backup/`、`.staging/` **从未被提交** |
| 二进制残留 | **有**：历史中的 `rbackup-tui`（约 4.93 MB，初始化提交引入、下一次提交删除） |
| 当前 HEAD 泄露 | 本文件旧版本含真实内网地址与用户名（已脱敏）；其余跟踪文件无命中 |

结论：需要重写历史的只有 **4 个文本文件的内容 + 1 个二进制路径**。
`config1.ini.example` / `config2.ini.example` 是保留文件，只做内容替换，不要删。

---

## 第 1 步：备份 + 确认工作区干净（不可跳过）

`filter-repo` 会重写 refs 并重置工作区，未提交的改动会丢。

```bash
cd ~/rbackup-tui
git status --short        # 必须无输出；有改动先提交

# ① 远端镜像（含全部历史与 tag）——唯一的回滚手段
git clone --mirror git@github.com:lockejet/rbackup-tui.git \
    ~/rbackup-tui-backup-$(date +%F).git

# ② 记录清理前的 refs 指纹，便于事后比对
git tag -l --format='%(refname:short) %(objectname)' > ~/rbackup-tui-tags-before.txt
git rev-parse HEAD >> ~/rbackup-tui-tags-before.txt
```

> **不要**在这里用 `filter-repo --invert-paths --path README.md` 之类的做法：
> 本仓库的敏感内容是**写在文件里的字符串**，按路径删会把源码/文档整体删掉。
> 正确解法是 `--replace-text` 就地替换（下一步）。

## 第 2 步：清史

### 2.1 先删除二进制路径

```bash
cd ~/rbackup-tui
git filter-repo --force --invert-paths --path rbackup-tui
```

### 2.2 再替换文件内容里的敏感字符串

```bash
cat > ~/rbackup-redact.txt <<'EOF'
regex:192\.168\.[0-9]{1,3}\.[0-9]{1,3}(:[0-9]+)?==>192.0.2.10
regex:/home/[^/ \n]+/\.ssh/[A-Za-z0-9_.-]+==>~/.ssh/id_ed25519
EOF

git filter-repo --force --replace-text ~/rbackup-redact.txt
```

说明：

- 替换规则是 Python 正则，**先在一次性副本上试跑**（`git clone --mirror` 到 /tmp 再跑），
  确认 `git log -p` 里语义没被破坏。
- 想保留端口号，就把第一条写成带捕获组的版本：
  `regex:192\.168\.[0-9]{1,3}\.[0-9]{1,3}(:\d+)?==>192.0.2.10\1`
- `filter-repo` 会移除 `origin` remote，这是正常的，第 4 步加回来。

## 第 3 步：校验（必须全部为空）

```bash
# ① 内容级扫描：内网地址 / 本机密钥路径 / 私钥（应无输出）
git log --all -p | grep -nE '192\.168\.|/home/[^/ ]+/\.ssh/|BEGIN [A-Z ]*PRIVATE KEY'

# ② 二进制不应再出现在任何提交里（应无输出）
git log --all --pretty=format: --name-only | sort -u | grep -x 'rbackup-tui' || echo "✓ 二进制已清除"

# ③ 保留文件仍在（应全部存在）
git log --all --pretty=format: --name-only | sort -u \
  | grep -E '^(README\.md|config\.go|rbackup\.sh|config2\.ini\.example)$' | wc -l   # 期望 4

# ④ 体积应明显下降（清掉 4.9 MB 二进制后应小于 1 MB 量级）
git count-objects -vH

# ⑤ tag 仍在（SHA 已变）
git tag -l
```

## 第 4 步：强制推送

> **顺序很重要：先推分支，再推 tag。** 若 tag 事件先送达，此时默认分支上还没有
> `.github/workflows/release.yml`，GitHub 不会为这次 tag 推送创建 workflow 运行。
> 补推的办法是删掉 tag 再推一次：
> `git push origin :refs/tags/vX.Y.Z && git push origin refs/tags/vX.Y.Z`

```bash
git remote add origin git@github.com:lockejet/rbackup-tui.git
git push --force --all     # 先分支
git push --force --tags    # 再 tag（会触发 Release workflow）
```

推送后逐个确认 Release 仍正常（附件独立于 git 历史，通常不受影响，但 tag 的 SHA 变了）：

```bash
gh release list --repo lockejet/rbackup-tui
for t in $(git tag -l); do
    printf '%-8s ' "$t"
    gh release view "$t" --repo lockejet/rbackup-tui --json assets \
        --jq '.assets | length' 2>/dev/null || echo "无 Release"
done
```

## 第 5 步：旧对象残留（force push ≠ 删除）

重写后，按旧 SHA 匿名探测仍可能取到旧对象（GitHub 不会立即 GC）：

- `/commits/<old-sha>`、`tree/<old-sha>` 仍可能 200；
- 已 clone 的副本、fork、第三方归档不会因 force push 而消失；
- 仓库是 public 期间被爬虫抓取的内容无法收回。

若要求"彻底不可达"，需向 GitHub Support 提交清除不可达对象请求，并评估是否重建仓库。
对本次的暴露面（内网地址 + 密钥路径，且无密钥本体），更现实的目标是：
**新历史里不再有、新 clone 拿不到**，并接受旧 SHA 可发现性极低的残留。

## 第 6 步：回滚

用第 1 步的镜像恢复远端：

```bash
cd ~/rbackup-tui-backup-*.git
git push --mirror git@github.com:lockejet/rbackup-tui.git
```

## 影响与代价

1. **所有已 clone 的副本作废**：历史被重写，旧副本无法直接 push/pull，必须重新 clone。
2. **tag SHA 全部变化**：本地旧 clone 的 tag 与远端不一致，需要
   `git fetch --force --tags` 或重新 clone。
3. **commit 历史内容会变**：README/config 里的示例文字被替换为文档保留值，
   这是预期结果；不要试图保留原始明文。
4. **与代码无关的东西不要动**：真实配置与日志本就在 `.gitignore` 里，
   且从未入库，清理时无需处理（也不要把它们提交进去）。
