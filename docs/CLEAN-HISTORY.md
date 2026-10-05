# 历史清理与公开化操作单

> **本文档里的命令请你自己执行。** 它们包含 `filter-repo` 重写历史、`push --force`
> 和仓库可见性变更，都是不可逆操作，不适合由自动化代劳。

## 前置检查（已完成的盘点结论）

| 项目 | 结论 |
|---|---|
| 私钥 / 证书 | 历史中**没有**（`PRIVATE KEY`、`BEGIN OPENSSH`、`id_rsa`、`.pem` 全部零命中） |
| 口令 / Token | 历史中**没有**（`password=`、`passphrase`、`ghp_`、`AKIA` 零命中） |
| 内网信息 | **有**：`config-lnas.ini`（`192.168.8.254:28375`、`admin`、密钥路径）、`config-office.ini`、历史里的 `rbackup.d/config.ini` |
| 真实备份日志 | **有**：`rbackup.d/log/rsync-*.log` 约 30 天，单份最大 39KB |
| 私有脚本 | **有**：`rbackup.d/hd/rsync-workdir-*-aliyun.sh`、`rbackup.d/xg/*.sh` 等 |
| 二进制残留 | **有**：历史里的 `bin/`、`dist/`、`backup/`、`rbackup-tui.d/`（体积与误导） |
| 当前仍被跟踪的敏感文件 | `config-lnas.ini`、`config-office.ini`（`.gitignore` 是后加的，对已跟踪文件无效） |

公开前必须让上面这些**从历史里消失**，而不只是从当前提交里删除。

---

## 第 0 步：先提交手上的改动

`filter-repo` 会重写 refs 并重置工作区，未提交的改动会丢。先确认干净：

```bash
cd ~/rbackup-tui
git status --short          # 应无输出
git add -A && git commit -m "feat: 打包与安装体系（install.sh / Makefile / GH Actions）"
```

> 新增的 `install.sh`、`.github/`、`docs/`、`integrations/` 都要先提交，
> 否则 `filter-repo` 之后它们会随工作区重置而丢失。

## 第 1 步：两份备份（不可跳过）

```bash
# ① 远端镜像（含全部历史与 tag）——这是唯一的回滚手段
git clone --mirror git@github.com:lockejet/rbackup-tui.git \
    ~/rbackup-tui-backup-$(date +%F).git

# ② 记录当前 refs 指纹，便于事后比对
git tag -l --format='%(refname:short) %(objectname)' > ~/rbackup-tui-tags-before.txt
git rev-parse HEAD >> ~/rbackup-tui-tags-before.txt
cat ~/rbackup-tui-tags-before.txt
```

## 第 2 步：清史

```bash
cd ~/rbackup-tui

git filter-repo --force --invert-paths \
  --path rbackup.d/ \
  --path rbackup-tui.d/ \
  --path backup/ \
  --path log/ \
  --path bin/ \
  --path dist/ \
  --path .staging/ \
  --path config-lnas.ini \
  --path config-office.ini \
  --path .gitignore.save
```

保留项说明：
- `config1.ini.example` / `config2.ini.example` **不在**删除清单里，会被保留（示例已脱敏）。
- `.gitattributes`、`.gitignore`、源码、README、LICENSE、`integrations/`、`.github/` 全部保留。
- 想更保险地覆盖全部真实配置，可以把两条 `--path config-*.ini` 换成
  `--path-glob 'config-*.ini'`（该 glob 不会误伤 `config1.ini.example`）。

`filter-repo` 会移除 `origin` remote，这是正常的，下一步加回来。

## 第 3 步：校验（必须全部为空 / 无输出）

```bash
# ① 这些文件不应再出现在任何提交里
git log --all --name-only --pretty=format: | sort -u \
  | grep -E 'rbackup\.d|rbackup-tui\.d|^log/|^bin/|^dist/|^backup/|config-(lnas|office)' \
  && echo "✗ 仍有残留" || echo "✓ 历史干净"

# ② 内容级扫描：内网 IP / 本机密钥路径 / 私钥
git log --all -p | grep -nE '192\.168\.|([0-9]{1,3}\.){3}[0-9]{1,3}|/home/admin/\.ssh|BEGIN [A-Z ]*PRIVATE KEY' \
  | head -20 && echo "↑ 需人工确认" || echo "✓ 无敏感特征串"

# ③ 体积应明显下降
git count-objects -vH

# ④ tag 仍在（SHA 已变）
git tag -l
```

## 第 4 步：强制推送

```bash
git remote add origin git@github.com:lockejet/rbackup-tui.git
git push --force --all
git push --force --tags
```

推送后逐个确认 7 个 Release 仍正常（附件的存储独立于 git 历史，通常不受影响，
但 tag 的 SHA 变了，需要确认关联没断）：

```bash
gh release list --repo lockejet/rbackup-tui
for t in $(git tag -l); do
    printf '%-8s ' "$t"
    gh release view "$t" --repo lockejet/rbackup-tui --json assets \
        --jq '.assets | length' 2>/dev/null || echo "无 Release"
done
```

若某个 Release 关联断了，用本地已备份的包重建：

```bash
ls dist/          # 之前 make release 的产物
gh release create v1.1.5 dist/rbackup-tui-v1.1.5-*.tar.gz dist/rbackup-tui-v1.1.5-*.zip \
    --title v1.1.5 --notes-file dist/RELEASE_NOTES.md --repo lockejet/rbackup-tui
```

## 第 5 步：改为 public

```bash
gh repo edit lockejet/rbackup-tui \
    --visibility public \
    --accept-visibility-change-consequences
```

**注意**：GitHub 在公开后会被爬虫与第三方归档立即抓取，这一步没有"反悔窗口"，
所以必须在第 3 步校验全绿之后再做。

## 第 6 步：公开后验证懒人模式（匿名、无 token）

```bash
# 安装器可取
curl -fsSL https://github.com/lockejet/rbackup-tui/releases/latest/download/install.sh | head -3

# 旧的 v1.1.5 没有 SHA256SUMS，属正常；下一个版本的 Release 才带
curl -fsSLI https://github.com/lockejet/rbackup-tui/releases/latest/download/SHA256SUMS | head -1

# 走一遍真实安装（会装到 ~/.local/bin 与 ~/rbackup-tui/）
curl -fsSL https://github.com/lockejet/rbackup-tui/releases/latest/download/install.sh | bash -s -- --dry-run
```

之后本机即可删除 `GH_TOKEN` 依赖：

```bash
unset GH_TOKEN
make release       # 本地打包（不需要网络）
make verify-release VERSION=v1.1.6
```

## 第 7 步：触发第一个自动化发布

新版本的 Release 必须由 Actions 生成，才能带上 `SHA256SUMS` 与 `install.sh`：

```bash
git tag -a v1.1.6 -m "v1.1.6 打包与安装体系"
git push origin v1.1.6
gh run watch --repo lockejet/rbackup-tui
gh release view v1.1.6 --repo lockejet/rbackup-tui
```

---

## 回滚

任何一步出问题，用第 1 步的镜像恢复远端：

```bash
cd ~/rbackup-tui-backup-*.git
git push --mirror git@github.com:lockejet/rbackup-tui.git
```

可见性改回私有：

```bash
gh repo edit lockejet/rbackup-tui --visibility private --accept-visibility-change-consequences
```

## 影响与代价

1. **所有已 clone 的副本作废**：历史被重写，旧副本无法再直接 push/pull，必须重新 clone。
   仓库是私有的，受众应只有你本人。
2. **tag SHA 全部变化**：本地旧 clone 里的 tag 与远端不再一致，需要
   `git fetch --force --tags` 或重新 clone。
3. **本地文件不受影响**：`config-lnas.ini`、`config-office.ini`、
   `rbackup.d/`、`log/` 都只是从 git 里移除，磁盘上的文件原样保留
   （`filter-repo` 之后它们会变成未跟踪状态，`.gitignore` 已覆盖，不会再次被误提交）。
4. **`GH_TOKEN` 路径仍保留**：`install.sh` 同时支持公开匿名下载与私有 token 下载，
   公开后无需改动任何脚本。
