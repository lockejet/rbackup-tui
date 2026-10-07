#!/usr/bin/env python3
"""生成并校验 README 的目录锚点。

GitHub 的标题锚点规则（github-slugger）：转小写 → 去掉标点/符号（保留 - 和 _）
→ 空格换 - → 同名追加 -1/-2。本脚本的实现已与线上渲染结果逐条比对过。

用法：
    python3 tools/gen-toc.py --check   # 校验目录链接是否与标题一致（CI 用）
    python3 tools/gen-toc.py --write   # 按当前标题重新生成目录
"""
from __future__ import annotations

import pathlib
import re
import sys
import unicodedata

README = pathlib.Path(__file__).resolve().parent.parent / "README.md"
TOC_HEADING = "## 目录"
TOC_LEVELS = (2, 3)          # 目录里收录的标题层级


def slug(text: str) -> str:
    """GitHub 标题锚点（小写、去标点、空格转连字符）。"""
    s = text.strip().lower()
    s = "".join(c for c in s if c in "-_" or unicodedata.category(c)[0] not in "PSC")
    return re.sub(r"\s", "-", s)


def headings(md: str) -> list[tuple[int, str, str]]:
    """抽取标题，返回 [(层级, 文本, 锚点)]，自动跳过代码块。"""
    out, seen, in_fence = [], {}, False
    for line in md.splitlines():
        if line.lstrip().startswith("```"):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        m = re.match(r"^(#{1,6})\s+(.*?)\s*$", line)
        if not m:
            continue
        level, text = len(m.group(1)), m.group(2)
        base = slug(text)
        n = seen.get(base, 0)
        seen[base] = n + 1
        out.append((level, text, base if n == 0 else f"{base}-{n}"))
    return out


def toc_block(hs: list[tuple[int, str, str]]) -> str:
    lines = [TOC_HEADING, ""]
    for level, text, anchor in hs:
        if level not in TOC_LEVELS or text == "目录":
            continue
        indent = "  " * (level - TOC_LEVELS[0])
        lines.append(f"{indent}- [{text}](#{anchor})")
    return "\n".join(lines) + "\n"


def current_block(md: str) -> tuple[int, int]:
    """定位现有目录块的行号范围 [start, end)。"""
    lines = md.splitlines()
    try:
        start = next(i for i, l in enumerate(lines) if l.strip() == TOC_HEADING)
    except StopIteration:
        sys.exit(f"✗ 找不到 {TOC_HEADING}")
    end = start + 1
    while end < len(lines) and not lines[end].startswith("---"):
        end += 1
    return start, end


def main() -> None:
    md = README.read_text()
    hs = headings(md)
    want = toc_block(hs)
    start, end = current_block(md)

    if "--write" in sys.argv:
        lines = md.splitlines()
        new = lines[:start] + want.splitlines() + [""] + lines[end:]
        README.write_text("\n".join(new) + "\n")
        n = sum(1 for lv, _, _ in hs if lv in TOC_LEVELS and _ != "目录")
        print(f"✓ 已重建目录，共 {n} 条")
        return

    # --check：目录必须与标题完全一致，且每条链接都能落到真实锚点
    have = "\n".join(md.splitlines()[start:end]).strip()
    if have != want.strip():
        print("✗ 目录与标题不一致，运行：python3 tools/gen-toc.py --write", file=sys.stderr)
        have_links = set(re.findall(r"\]\(#([^)]+)\)", have))
        want_links = set(re.findall(r"\]\(#([^)]+)\)", want))
        for a in sorted(want_links - have_links):
            print(f"    缺少: #{a}", file=sys.stderr)
        for a in sorted(have_links - want_links):
            print(f"    多余/失效: #{a}", file=sys.stderr)
        sys.exit(1)

    valid = {a for _, _, a in hs}
    broken = [a for a in re.findall(r"\]\(#([^)]+)\)", have) if a not in valid]
    if broken:
        print(f"✗ 目录链接指向不存在的标题: {broken}", file=sys.stderr)
        sys.exit(1)
    print(f"✓ 目录校验通过（{len(re.findall(r'\]\(#', have))} 条链接全部有效）")


if __name__ == "__main__":
    main()
