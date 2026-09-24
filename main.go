package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ---------- 版本信息（-ldflags 注入） ----------
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

var debugKeys = os.Getenv("RBACKUP_DEBUG_KEYS") != ""
var debugCount int

// ---------- 分隔线字符 ----------
var sepChar = "┄"

func init() {
	if c := os.Getenv("RBACKUP_SEP"); c != "" {
		sepChar = c
	}
}

// ---------- 事件去重 ----------
var (
	dedupMu  sync.Mutex
	lastKey  tcell.Key
	lastRune rune
	lastMod  tcell.ModMask
	lastTime time.Time
)

func isDuplicateKey(event *tcell.EventKey) bool {
	dedupMu.Lock()
	defer dedupMu.Unlock()
	now := time.Now()
	dup := event.Key() == lastKey &&
		event.Rune() == lastRune &&
		event.Modifiers() == lastMod &&
		now.Sub(lastTime) < 50*time.Millisecond
	lastKey = event.Key()
	lastRune = event.Rune()
	lastMod = event.Modifiers()
	lastTime = now
	return dup
}

// ---------- 按键提示构造 ----------

func hint(pairs ...string) string {
	var b strings.Builder
	for i := 0; i < len(pairs); i += 2 {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(pairs[i])
		b.WriteString("[white]")
		b.WriteString(tview.Escape("[" + pairs[i+1] + "]"))
		b.WriteString("[-]")
	}
	return b.String()
}

func hintDark(pairs ...string) string {
	var parts []string
	for i := 0; i < len(pairs); i += 2 {
		parts = append(parts, pairs[i]+"["+pairs[i+1]+"]")
	}
	return "[darkgray]" + tview.Escape(strings.Join(parts, "  ")) + "[-]"
}

// ---------- 交互区状态 ----------
type InteractState int

const (
	InteractIdle InteractState = iota
	InteractConfirming
	InteractRunning
	InteractDone
	InteractCancelled
)

func (s InteractState) String() string {
	switch s {
	case InteractIdle:
		return "空闲"
	case InteractConfirming:
		return "等待确认"
	case InteractRunning:
		return "同步中"
	case InteractDone:
		return "已完成"
	case InteractCancelled:
		return "已取消"
	}
	return "未知"
}

type taskOutcome struct {
	Name   string
	Result string
}

type ConfirmState struct {
	Queue        []*Task
	Index        int
	Decisions    map[string]bool
	NonDangerous []*Task
	DryRun       bool
}

type App struct {
	app   *tview.Application
	pages *tview.Pages

	configPath string
	scriptPath string
	cfg        *Config

	header      *tview.TextView
	table       *tview.Table
	tableArea   *tview.Flex
	tableSep    *tview.TextView
	tableFooter *tview.TextView

	interact     *tview.Flex
	statusPart   *tview.TextView
	statusSep    *tview.TextView
	logPart      *tview.TextView
	logSep       *tview.TextView
	interactHint *tview.TextView

	status *tview.TextView
	help   *tview.TextView

	// 帮助浮层
	helpVisible bool
	helpPage    *tview.TextView

	running         bool
	cancel          context.CancelFunc
	cancelRequested bool

	focusArea string

	confirmState  *ConfirmState
	interactState InteractState
	interactExtra string

	paused atomic.Bool

	logBufMu sync.Mutex
	logBuf   []string

	blinkStop chan struct{}
	blinkGen  int

	errMu         sync.Mutex
	currentErrors []string

	ctrlDCount int
	lastCtrlD  time.Time
}

// ---------- 路径查找 ----------

func findScriptPath() string {
	if p := os.Getenv("RBACKUP_SCRIPT"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "rbackup", "rbackup.sh"),
		filepath.Join(home, ".local", "bin", "rbackup"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if p, err := exec.LookPath("rbackup"); err == nil {
		return p
	}
	return "rbackup"
}

func findConfigPath() string {
	if p := os.Getenv("RBACKUP_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rbackup", "config.ini")
}

func NewApp() *App {
	configPath := findConfigPath()
	scriptPath := findScriptPath()

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-c", "--config":
			if i+1 < len(os.Args) {
				configPath = expandTilde(os.Args[i+1])
				i++
			}
		case "-s", "--script":
			if i+1 < len(os.Args) {
				scriptPath = expandTilde(os.Args[i+1])
				i++
			}
		}
	}

	return &App{
		app:           tview.NewApplication(),
		configPath:    configPath,
		scriptPath:    scriptPath,
		focusArea:     "table",
		interactState: InteractIdle,
	}
}

// ---------- 分隔线 ----------

func newSeparator() *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(false)
	tv.SetText(strings.Repeat(sepChar, 500))
	tv.SetTextColor(tcell.ColorRed)
	tv.SetWrap(false)
	tv.SetWordWrap(false)
	return tv
}

// ---------- UI ----------

func (a *App) setupUI() {
	a.header = tview.NewTextView().SetDynamicColors(true)
	a.header.SetBorder(true).SetTitle(" rbackup ")

	a.table = tview.NewTable().SetSelectable(true, false).SetBorders(false)
	a.table.SetFixed(1, 0)
	a.table.SetBorder(false)

	a.tableSep = newSeparator()

	a.tableFooter = tview.NewTextView().SetDynamicColors(true)

	a.tableArea = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.table, 0, 1, true).
		AddItem(a.tableSep, 1, 0, false).
		AddItem(a.tableFooter, 6, 0, false)
	a.tableArea.SetBorder(true).SetTitle(" [1] 任务列表 [Tab/1] ")

	a.statusPart = tview.NewTextView().SetDynamicColors(true)
	a.statusSep = newSeparator()

	a.logPart = tview.NewTextView().SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetWordWrap(true)

	a.logSep = newSeparator()

	a.interactHint = tview.NewTextView().SetDynamicColors(true)

	a.interact = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.statusPart, 2, 0, false).
		AddItem(a.statusSep, 1, 0, false).
		AddItem(a.logPart, 0, 1, false).
		AddItem(a.logSep, 1, 0, false).
		AddItem(a.interactHint, 1, 0, false)
	a.interact.SetBorder(true).SetTitle(" [2] 交互区 [Tab/2] ")

	a.status = tview.NewTextView().SetDynamicColors(true)
	a.help = tview.NewTextView().SetDynamicColors(false)

	a.table.SetSelectionChangedFunc(func(row, col int) {
		a.updateTableFooter()
	})

	a.app.SetInputCapture(a.globalInputCapture)
	a.updateFocusStyle()
	a.updateHelp()

	a.pages = tview.NewPages()
	a.pages.AddPage("main", a.mainLayout(), true, true)
}

func (a *App) mainLayout() tview.Primitive {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 4, 0, false).
		AddItem(a.tableArea, 0, 2, true).
		AddItem(a.interact, 0, 3, false).
		AddItem(a.status, 1, 0, false).
		AddItem(a.help, 1, 0, false)
}

func (a *App) updateFocusStyle() {
	focusedTable := a.focusArea == "table"

	if focusedTable {
		a.tableArea.SetBorderColor(tcell.ColorGreen)
		a.interact.SetBorderColor(tcell.ColorGray)
	} else {
		a.tableArea.SetBorderColor(tcell.ColorGray)
		a.interact.SetBorderColor(tcell.ColorGreen)
	}

	if focusedTable {
		a.interactHint.SetText(hintDark(
			"滚动", "↑↓/j/k",
			"翻页", "PgUp/PgDn",
			"首尾", "Home/End/g/G",
			"暂停", "p/空格"))
	} else {
		a.interactHint.SetText(hint(
			"滚动", "↑↓/j/k",
			"翻页", "PgUp/PgDn",
			"首尾", "Home/End/g/G",
			"暂停", "p/空格"))
	}

	a.updateTableFooter()
}

// ---------- 任务区底部 ----------

func (a *App) updateTableFooter() {
	focused := a.focusArea == "table"

	var selected []string
	for _, t := range a.cfg.Tasks {
		if t.Selected {
			selected = append(selected, t.Name)
		}
	}
	var line1 string
	if len(selected) == 0 {
		if focused {
			line1 = "[gray]未选择任何任务[-]"
		} else {
			line1 = "[darkgray]未选择任何任务[-]"
		}
	} else {
		if focused {
			line1 = fmt.Sprintf("[green]已选 %d:[-] %s",
				len(selected), strings.Join(selected, " "))
		} else {
			line1 = fmt.Sprintf("[darkgreen]已选 %d:[-] [darkgray]%s[-]",
				len(selected), strings.Join(selected, " "))
		}
	}

	var line2 string
	if focused {
		line2 = hint(
			"选择", "空格", "全选", "a", "全不选", "n",
			"运行", "Enter", "预览", "d", "挂载", "m", "刷新", "r")
	} else {
		line2 = hintDark(
			"选择", "空格", "全选", "a", "全不选", "n",
			"运行", "Enter", "预览", "d", "挂载", "m", "刷新", "r")
	}

	var line3 string
	if focused {
		line3 = hint(
			"移动", "↑↓/j/k",
			"翻页", "PgUp/PgDn",
			"首尾", "Home/End/g/G")
	} else {
		line3 = hintDark(
			"移动", "↑↓/j/k",
			"翻页", "PgUp/PgDn",
			"首尾", "Home/End/g/G")
	}

	var cmdLines []string
	row, _ := a.table.GetSelection()
	if row > 0 && row <= len(a.cfg.Tasks) {
		cmd := a.buildRsyncCommand(a.cfg.Tasks[row-1])
		cmdLines = wrapCommand(cmd, 3, 90)
	}
	if len(cmdLines) == 0 {
		cmdLines = []string{"(未选择任务)"}
	}

	var b strings.Builder
	b.WriteString(line1)
	b.WriteString("\n")
	b.WriteString(line2)
	b.WriteString("\n")
	b.WriteString(line3)
	for i, l := range cmdLines {
		b.WriteString("\n")
		if i == 0 {
			if focused {
				b.WriteString("[gray]命令:[-] ")
			} else {
				b.WriteString("[darkgray]命令:[-] ")
			}
		} else {
			if focused {
				b.WriteString("[gray]      [-] ")
			} else {
				b.WriteString("[darkgray]      [-] ")
			}
		}
		if focused {
			b.WriteString(tview.Escape(l))
		} else {
			b.WriteString("[darkgray]" + tview.Escape(l) + "[-]")
		}
	}

	a.tableFooter.SetText(b.String())
}

func (a *App) buildRsyncCommand(t *Task) string {
	if t == nil {
		return ""
	}
	g := a.cfg.Global
	parts := []string{"rsync"}

	if g.GlobalOpts != "" {
		parts = append(parts, g.GlobalOpts)
	}
	if t.Opts != "" {
		parts = append(parts, t.Opts)
	}

	if t.NeedsDelete() {
		dst := strings.TrimRight(t.Dst, "/")
		recycle := dst + "/.deleted_files/rbackup/<ts>"
		parts = append(parts,
			"--delete",
			"--exclude='/.deleted_files/'",
			"--backup",
			fmt.Sprintf("--backup-dir=\"%s\"", recycle))
	}

	if t.NeedsRemoveSource(g) {
		parts = append(parts, "--remove-source-files")
	}

	sshCmd := fmt.Sprintf("ssh -p %s -i %s", g.SSHPort, g.SSHKey)
	parts = append(parts, "-e", fmt.Sprintf("\"%s\"", sshCmd))
	parts = append(parts, fmt.Sprintf("\"%s\"", t.Src))

	fullDst := t.Dst
	if !strings.Contains(fullDst, "@") {
		fullDst = fmt.Sprintf("admin@%s:%s", g.Host, fullDst)
	}
	parts = append(parts, fmt.Sprintf("\"%s\"", fullDst))

	return strings.Join(parts, " ")
}

func wrapCommand(cmd string, maxLines, maxWidth int) []string {
	if len(cmd) == 0 {
		return nil
	}
	var lines []string
	remaining := cmd
	for len(lines) < maxLines {
		if len(remaining) <= maxWidth {
			lines = append(lines, remaining)
			return lines
		}
		cut := maxWidth
		for cut > maxWidth/2 && remaining[cut] != ' ' {
			cut--
		}
		if cut <= maxWidth/2 {
			cut = maxWidth
		}
		lines = append(lines, remaining[:cut])
		remaining = strings.TrimLeft(remaining[cut:], " ")
	}
	if len(remaining) > 0 {
		last := lines[maxLines-1]
		if len(last) > maxWidth-3 {
			last = last[:maxWidth-3]
		}
		lines[maxLines-1] = last + "..."
	}
	return lines
}

// ---------- 帮助浮层 ----------

func (a *App) helpContent() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf(
		"[yellow::b]rbackup-tui[-:-:-]  [white]%s[-]  [gray](git: %s, 构建: %s)[-]\n",
		Version, GitCommit, BuildTime))
	b.WriteString("\n")
	b.WriteString("[yellow]作者:[-]  Jet Locke\n")
	b.WriteString("\n")

	kv := func(key, desc string) string {
		return fmt.Sprintf("  %-18s %s\n", key, desc)
	}

	b.WriteString("[yellow::b]─── 全局按键 ────────────────────────────────────────[-:-:-]\n")
	b.WriteString(kv("F1 / ?", "显示帮助"))
	b.WriteString(kv("q / Esc", "关闭帮助 / 退出程序（空闲时）"))
	b.WriteString(kv("Tab / 1 / 2", "切换焦点"))
	b.WriteString(kv("Ctrl+C", "停止任务（运行中）/ 退出（空闲）"))
	b.WriteString(kv("Ctrl+D ×3", "强制退出程序"))
	b.WriteString("\n")

	b.WriteString("[yellow::b]─── 焦点在任务区 ────────────────────────────────────[-:-:-]\n")
	b.WriteString(kv("↑ ↓ / j k", "移动光标"))
	b.WriteString(kv("PgUp / PgDn", "翻页"))
	b.WriteString(kv("Home / End / g G", "首 / 尾"))
	b.WriteString(kv("空格", "选择 / 取消选择"))
	b.WriteString(kv("a / n", "全选 / 全不选"))
	b.WriteString(kv("Enter", "运行选中任务"))
	b.WriteString(kv("d", "预览（dry-run）"))
	b.WriteString(kv("m", "挂载检查"))
	b.WriteString(kv("r", "刷新配置"))
	b.WriteString("\n")

	b.WriteString("[yellow::b]─── 焦点在交互区 ────────────────────────────────────[-:-:-]\n")
	b.WriteString(kv("↑ ↓ / j k", "滚动日志"))
	b.WriteString(kv("PgUp / PgDn", "翻页"))
	b.WriteString(kv("Home / End / g G", "首 / 尾"))
	b.WriteString(kv("p / 空格", "暂停 / 继续自动滚动"))
	b.WriteString("\n")

	b.WriteString("[yellow::b]─── 危险确认 ───────────────────────────────────────[-:-:-]\n")
	b.WriteString(kv("y", "确认当前任务"))
	b.WriteString(kv("n", "跳过当前任务"))
	b.WriteString(kv("a", "全部确认"))
	b.WriteString(kv("s", "全部跳过"))
	b.WriteString("\n")

	b.WriteString("[yellow::b]─── 关于 ────────────────────────────────────────────[-:-:-]\n")
	b.WriteString("  rbackup-tui 是 rbackup.sh 的 TUI 前端\n")
	b.WriteString("  配置文件: 与 rbackup.sh 共用 config.ini\n")
	b.WriteString("  日志目录: 由 config.ini 的 LOG_DIR 决定\n")
	b.WriteString("\n")
	b.WriteString("[gray]按 q 或 Esc 关闭本帮助[-]\n")

	return b.String()
}

func (a *App) showHelp() {
	if a.helpVisible {
		return
	}
	a.helpVisible = true

	tv := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	tv.SetBorder(true).SetTitle(" 帮助 ")
	tv.SetText(a.helpContent())

	a.helpPage = tv
	a.pages.AddPage("help", tv, true, true)
	a.app.SetFocus(tv)
}

func (a *App) hideHelp() {
	if !a.helpVisible {
		return
	}
	a.helpVisible = false
	a.pages.RemovePage("help")
	a.helpPage = nil
	a.setFocus(a.focusArea)
}

// ---------- 帮助栏 ----------

func (a *App) updateHelp() {
	var txt string
	switch {
	case a.confirmState != nil:
		txt = "切换焦点[Tab/1/2]  强制退出[Ctrl+D×3]  帮助[F1/?]"
	case a.running:
		if a.cancelRequested {
			txt = "正在停止任务...  强制退出[Ctrl+D×3]  帮助[F1/?]"
		} else {
			txt = "切换焦点[Tab/1/2]  暂停[p/空格]  " +
				"停止任务[Ctrl+C]  强制退出[Ctrl+D×3]  帮助[F1/?]"
		}
	default:
		txt = "切换焦点[Tab/1/2]  退出[q/Esc/Ctrl+C]  " +
			"强制退出[Ctrl+D×3]  帮助[F1/?]"
	}
	a.help.SetText(txt)
}

// ---------- 调试 ----------

func debugKey(scope string, event *tcell.EventKey) {
	if !debugKeys {
		return
	}
	if event.Key() == 64 && event.Modifiers() == 2 {
		return
	}
	debugCount++
	fmt.Fprintf(os.Stderr, "[KEY#%d][%s] Key=%v Rune=%q Mod=%v\n",
		debugCount, scope, event.Key(), event.Rune(), event.Modifiers())
}

// ---------- 过滤与错误识别 ----------

var filterPrefixes = []string{
	"备份脚本启动 (PID:",
	"脚本路径:",
	"配置文件:",
	"远程主机:",
	"日志文件:",
	"挂载门禁:",
	"任务选择:",
	"强制模式:",
	"提权已启用:",
	"模式: 任务模式",
	"模式: 临时任务",
	"模式: 仅挂载检查",
	"[CHECK-MOUNT]",
	"[DRY-RUN]",
	"汇总:",
	"跳过 0",
	"失败 0",
	"挂载门禁失败 0",
}

func shouldFilterLine(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return false
	}
	if strings.HasPrefix(t, "====") {
		return true
	}
	if regexp.MustCompile(`^(成功|跳过|失败|挂载门禁失败)\s+\d+`).MatchString(t) {
		return true
	}
	for _, p := range filterPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

func isErrorLine(line string) bool {
	l := strings.ToLower(line)
	markers := []string{
		"rsync error", "rsync:", "[fail]", "failed", "error:",
		"错误", "失败", "拒绝", "不可恢复",
	}
	for _, m := range markers {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

// ---------- 闪烁 ----------

func (a *App) startBlink(title string) {
	if a.blinkStop != nil {
		close(a.blinkStop)
		a.blinkStop = nil
	}
	a.blinkGen++
	gen := a.blinkGen

	stop := make(chan struct{})
	a.blinkStop = stop

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		on := false
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				on = !on
				cur := on
				a.app.QueueUpdateDraw(func() {
					if a.blinkGen != gen {
						return
					}
					if cur {
						a.interact.SetTitle(fmt.Sprintf(" [red::b]%s[-:-:-] ", title))
					} else {
						a.interact.SetTitle(fmt.Sprintf(" [yellow::b]%s[-:-:-] ", title))
					}
				})
			}
		}
	}()
}

func (a *App) stopBlink() {
	if a.blinkStop != nil {
		close(a.blinkStop)
		a.blinkStop = nil
	}
	a.blinkGen++
	a.interact.SetTitle(" [2] 交互区 [Tab/2] ")
}

// ---------- 焦点 ----------

func (a *App) setFocus(area string) {
	a.focusArea = area
	if area == "interact" {
		a.app.SetFocus(a.interact)
	} else {
		a.app.SetFocus(a.table)
	}
	a.updateFocusStyle()
	a.updateStatus()
}

func (a *App) toggleFocus() {
	if a.focusArea == "table" {
		a.setFocus("interact")
	} else {
		a.setFocus("table")
	}
}

// ---------- 日志滚动 ----------

func (a *App) scrollLog(delta int) {
	row, col := a.logPart.GetScrollOffset()
	newRow := row + delta
	if newRow < 0 {
		newRow = 0
	}
	a.logPart.ScrollTo(newRow, col)
}

func (a *App) flushLogBuf() {
	a.logBufMu.Lock()
	buf := a.logBuf
	a.logBuf = nil
	a.logBufMu.Unlock()
	for _, line := range buf {
		fmt.Fprintln(a.logPart, tview.Escape(line))
	}
}

func (a *App) handleLogScroll(event *tcell.EventKey) bool {
	scrolled := false
	upward := false
	toBottom := false
	switch event.Key() {
	case tcell.KeyUp:
		a.scrollLog(-1)
		scrolled = true
		upward = true
	case tcell.KeyDown:
		a.scrollLog(1)
		scrolled = true
	case tcell.KeyPgUp:
		a.scrollLog(-10)
		scrolled = true
		upward = true
	case tcell.KeyPgDn:
		a.scrollLog(10)
		scrolled = true
	case tcell.KeyHome:
		a.logPart.ScrollToBeginning()
		scrolled = true
		upward = true
	case tcell.KeyEnd:
		a.logPart.ScrollToEnd()
		scrolled = true
		toBottom = true
	}
	if !scrolled && event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'j':
			a.scrollLog(1)
			scrolled = true
		case 'k':
			a.scrollLog(-1)
			scrolled = true
			upward = true
		case 'g':
			a.logPart.ScrollToBeginning()
			scrolled = true
			upward = true
		case 'G':
			a.logPart.ScrollToEnd()
			scrolled = true
			toBottom = true
		}
	}
	if !scrolled {
		return false
	}

	if a.running {
		if upward && !a.paused.Load() {
			a.paused.Store(true)
			a.setStatus("[yellow]⏸ 已暂停（按 p 继续）[-]")
		} else if toBottom && a.paused.Load() {
			a.paused.Store(false)
			a.flushLogBuf()
			a.updateStatus()
		}
	}
	return true
}

func (a *App) handleTableNav(event *tcell.EventKey) bool {
	row, col := a.table.GetSelection()
	maxRow := len(a.cfg.Tasks)
	if maxRow == 0 {
		return false
	}
	moveTo := func(r int) {
		if r < 1 {
			r = 1
		}
		if r > maxRow {
			r = maxRow
		}
		a.table.Select(r, col)
	}
	switch event.Key() {
	case tcell.KeyUp:
		moveTo(row - 1)
		return true
	case tcell.KeyDown:
		moveTo(row + 1)
		return true
	case tcell.KeyPgUp:
		moveTo(row - 10)
		return true
	case tcell.KeyPgDn:
		moveTo(row + 10)
		return true
	case tcell.KeyHome:
		moveTo(1)
		return true
	case tcell.KeyEnd:
		moveTo(maxRow)
		return true
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'j':
			moveTo(row + 1)
			return true
		case 'k':
			moveTo(row - 1)
			return true
		case 'g':
			moveTo(1)
			return true
		case 'G':
			moveTo(maxRow)
			return true
		}
	}
	return false
}

// ---------- 请求取消 ----------

func (a *App) requestCancel() {
	if a.cancelRequested {
		return
	}
	if a.cancel == nil {
		return
	}
	a.cancel()
	a.cancel = nil
	a.cancelRequested = true
	a.updateHelp()
	a.updateStatus()
}

// ---------- Ctrl+D 提示 ----------

func (a *App) showCtrlDPrompt(remaining int) {
	a.setStatus(fmt.Sprintf("[yellow]Ctrl+D 再按 %d 次退出程序[-]", remaining))
	gen := a.ctrlDCount
	go func() {
		time.Sleep(1600 * time.Millisecond)
		a.app.QueueUpdateDraw(func() {
			if a.ctrlDCount == gen &&
				time.Since(a.lastCtrlD) >= 1500*time.Millisecond {
				a.ctrlDCount = 0
				a.updateStatus()
			}
		})
	}()
}

// ---------- 全局按键 ----------

func (a *App) globalInputCapture(event *tcell.EventKey) *tcell.EventKey {
	if isDuplicateKey(event) {
		return nil
	}
	debugKey("global", event)

	// 帮助键（F1 在部分终端被拦截，同时支持 ?）
	if event.Key() == tcell.KeyF1 {
		if a.helpVisible {
			a.hideHelp()
		} else {
			a.showHelp()
		}
		return nil
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case '?':
			if a.helpVisible {
				a.hideHelp()
			} else {
				a.showHelp()
			}
			return nil
		}
	}

	// 帮助显示中：拦截关闭键，其余（方向键等）交给 help TextView
	if a.helpVisible {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			a.hideHelp()
			return nil
		}
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'q', 'Q':
				a.hideHelp()
				return nil
			}
		}
		return event
	}

	if event.Key() == tcell.KeyCtrlD {
		now := time.Now()
		if now.Sub(a.lastCtrlD) < 1500*time.Millisecond {
			a.ctrlDCount++
		} else {
			a.ctrlDCount = 1
		}
		a.lastCtrlD = now
		if a.ctrlDCount >= 3 {
			a.app.Stop()
			return nil
		}
		a.showCtrlDPrompt(3 - a.ctrlDCount)
		return nil
	}
	if a.ctrlDCount > 0 {
		a.ctrlDCount = 0
	}

	if event.Key() == tcell.KeyTab {
		a.toggleFocus()
		return nil
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case '1':
			a.setFocus("table")
			return nil
		case '2':
			a.setFocus("interact")
			return nil
		}
	}

	if a.confirmState != nil {
		return a.handleConfirmKey(event)
	}
	if a.running {
		return a.handleRunningKey(event)
	}
	return a.handleIdleKey(event)
}

// ---------- 危险确认 ----------

func (a *App) handleConfirmKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyCtrlC {
		a.confirmAllSkip()
		return nil
	}
	if event.Key() == tcell.KeyEscape {
		return nil
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'q', 'Q':
			return nil
		}
	}
	if a.focusArea != "interact" {
		return nil
	}
	if a.handleLogScroll(event) {
		return nil
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'y', 'Y':
			a.confirmCurrent(true)
			return nil
		case 'n', 'N':
			a.confirmCurrent(false)
			return nil
		case 'a', 'A':
			a.confirmAllConfirm()
			return nil
		case 's', 'S':
			a.confirmAllSkip()
			return nil
		}
	}
	return nil
}

// ---------- 运行中 ----------

func (a *App) handleRunningKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlC:
		a.requestCancel()
		return nil
	case tcell.KeyEscape:
		return nil
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'p', 'P', ' ':
			a.togglePause()
			return nil
		case 'q', 'Q':
			return nil
		}
	}
	if a.handleLogScroll(event) {
		return nil
	}
	return nil
}

// ---------- 空闲 ----------

func (a *App) handleIdleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		a.app.Stop()
		return nil
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'q', 'Q':
			a.app.Stop()
			return nil
		}
	}
	if a.focusArea == "interact" {
		return a.handleInteractIdleKey(event)
	}
	return a.handleTableIdleKey(event)
}

func (a *App) handleTableIdleKey(event *tcell.EventKey) *tcell.EventKey {
	if a.handleTableNav(event) {
		return nil
	}
	switch event.Key() {
	case tcell.KeyEnter:
		a.runSelectedTasks()
		return nil
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case ' ':
			a.toggleCurrent()
			return nil
		case 'a', 'A':
			a.selectAll(true)
			return nil
		case 'n', 'N':
			a.selectAll(false)
			return nil
		case 'd', 'D':
			a.dryRunSelected()
			return nil
		case 'm', 'M':
			a.runMountCheck()
			return nil
		case 'r', 'R':
			a.reloadConfig()
			return nil
		}
	}
	return event
}

func (a *App) handleInteractIdleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'p', 'P', ' ':
			a.togglePause()
			return nil
		}
	}
	if a.handleLogScroll(event) {
		return nil
	}
	return nil
}

// ---------- 暂停 ----------

func (a *App) togglePause() {
	newPaused := !a.paused.Load()
	a.paused.Store(newPaused)
	if newPaused {
		a.setStatus("[yellow]⏸ 已暂停（按 p 继续）[-]")
	} else {
		a.flushLogBuf()
		a.logPart.ScrollToEnd()
		a.updateStatus()
	}
}

// ---------- 状态刷新 ----------

func (a *App) updateHeader() {
	logPath := filepath.Join(a.cfg.Global.LogDir,
		"rbackup_"+time.Now().Format("20060102")+".log")
	txt := fmt.Sprintf(
		"[yellow]脚本:[-] %s  [gray]|[-]  [yellow]配置:[-] %s\n"+
			"[yellow]远端:[-] admin@%s:%s  [gray]|[-]  [yellow]策略:[-] %s  [gray]|[-]  [yellow]日志:[-] %s",
		a.scriptPath,
		a.configPath,
		a.cfg.Global.Host, a.cfg.Global.SSHPort,
		a.cfg.Global.MountPolicy,
		logPath)
	a.header.SetText(txt)
}

func (a *App) updateStatus() {
	sel := 0
	for _, t := range a.cfg.Tasks {
		if t.Selected {
			sel++
		}
	}

	var statusLabel, statusColor string
	switch {
	case a.confirmState != nil:
		statusLabel = "等待确认"
		statusColor = "[red]"
	case a.running:
		if a.paused.Load() {
			statusLabel = "已暂停"
			statusColor = "[yellow]"
		} else {
			statusLabel = "运行中"
			statusColor = "[green]"
		}
	default:
		statusLabel = "就绪"
		statusColor = "[green]"
	}

	var interactColor string
	switch a.interactState {
	case InteractIdle:
		interactColor = "[gray]"
	case InteractConfirming, InteractCancelled:
		interactColor = "[yellow]"
	case InteractRunning, InteractDone:
		interactColor = "[green]"
	}
	interactStr := a.interactState.String()
	if a.interactExtra != "" {
		interactStr = interactStr + " " + a.interactExtra
	}

	focusCN := "任务区"
	if a.focusArea == "interact" {
		focusCN = "交互区"
	}

	a.status.SetText(fmt.Sprintf(
		"%s%s[-]  |  已选: %d/%d  |  交互: %s%s[-]  |  焦点: [yellow]%s[-]",
		statusColor, statusLabel,
		sel, len(a.cfg.Tasks),
		interactColor, interactStr,
		focusCN))
}

func headerCell(text string) *tview.TableCell {
	return tview.NewTableCell(text).
		SetTextColor(tcell.ColorYellow).
		SetSelectable(false).
		SetAlign(tview.AlignLeft).
		SetExpansion(0)
}

func (a *App) refreshTasks() {
	a.table.Clear()

	headers := []string{"", "任务名", "源", "目标", "门禁"}
	for i, h := range headers {
		a.table.SetCell(0, i, headerCell(h))
	}

	var currentConfirmName string
	if a.confirmState != nil && a.confirmState.Index < len(a.confirmState.Queue) {
		currentConfirmName = a.confirmState.Queue[a.confirmState.Index].Name
	}

	for i, task := range a.cfg.Tasks {
		row := i + 1
		highlight := currentConfirmName != "" && task.Name == currentConfirmName
		bg := tcell.ColorDarkRed
		fg := tcell.ColorWhite

		check := "○"
		if task.Selected {
			check = "●"
		}
		checkCell := tview.NewTableCell(check).
			SetAlign(tview.AlignCenter).SetExpansion(0)
		if task.Selected {
			checkCell.SetTextColor(tcell.ColorGreen)
		}
		if highlight {
			checkCell.SetBackgroundColor(bg).SetTextColor(fg)
		}
		a.table.SetCell(row, 0, checkCell)

		nameCell := tview.NewTableCell(task.Name).SetExpansion(0)
		if task.IsDangerous(a.cfg.Global) {
			nameCell.SetTextColor(tcell.ColorOrange)
		}
		if highlight {
			nameCell.SetBackgroundColor(bg).SetTextColor(fg)
		}
		a.table.SetCell(row, 1, nameCell)

		srcCell := tview.NewTableCell(task.Src).SetExpansion(1)
		if highlight {
			srcCell.SetBackgroundColor(bg).SetTextColor(fg)
		}
		a.table.SetCell(row, 2, srcCell)

		dstCell := tview.NewTableCell(task.Dst).SetExpansion(1)
		if highlight {
			dstCell.SetBackgroundColor(bg).SetTextColor(fg)
		}
		a.table.SetCell(row, 3, dstCell)

		gateTxt := "—"
		gateColor := tcell.ColorGray
		switch task.MountGateMode() {
		case "require_mounted":
			gateTxt = "已挂载"
			gateColor = tcell.ColorLightBlue
		case "require_unmounted":
			gateTxt = "未挂载"
			gateColor = tcell.ColorLightBlue
		}
		gateCell := tview.NewTableCell(gateTxt).
			SetTextColor(gateColor).SetExpansion(0)
		if highlight {
			gateCell.SetBackgroundColor(bg).SetTextColor(fg)
		}
		a.table.SetCell(row, 4, gateCell)
	}

	if len(a.cfg.Tasks) > 0 {
		a.table.Select(1, 0)
	}
	a.updateTableFooter()
}

// ---------- 选择操作 ----------

func (a *App) toggleCurrent() {
	row, _ := a.table.GetSelection()
	if row <= 0 || row > len(a.cfg.Tasks) {
		return
	}
	t := a.cfg.Tasks[row-1]
	t.Selected = !t.Selected
	a.refreshTasks()
	a.table.Select(row, 0)
	a.updateStatus()
}

func (a *App) selectAll(sel bool) {
	for _, t := range a.cfg.Tasks {
		t.Selected = sel
	}
	row, _ := a.table.GetSelection()
	a.refreshTasks()
	if row > 0 && row <= len(a.cfg.Tasks) {
		a.table.Select(row, 0)
	}
	a.updateStatus()
}

func (a *App) selectedTasks() []*Task {
	var out []*Task
	for _, t := range a.cfg.Tasks {
		if t.Selected {
			out = append(out, t)
		}
	}
	return out
}

func (a *App) setStatus(msg string) {
	a.status.SetText(msg)
}

// ---------- 运行触发 ----------

func (a *App) runSelectedTasks() {
	if a.running || a.confirmState != nil {
		return
	}
	sel := a.selectedTasks()
	if len(sel) == 0 {
		a.setStatus("[yellow]未选择任何任务[-]")
		return
	}
	a.planRun(sel, false)
}

func (a *App) dryRunSelected() {
	if a.running || a.confirmState != nil {
		return
	}
	sel := a.selectedTasks()
	if len(sel) == 0 {
		a.setStatus("[yellow]未选择任何任务[-]")
		return
	}
	a.planRun(sel, true)
}

func (a *App) planRun(tasks []*Task, dryRun bool) {
	var dangerous, safe []*Task
	for _, t := range tasks {
		if t.IsDangerous(a.cfg.Global) {
			dangerous = append(dangerous, t)
		} else {
			safe = append(safe, t)
		}
	}
	if len(dangerous) == 0 {
		a.startRun(tasks, dryRun)
		return
	}
	a.confirmState = &ConfirmState{
		Queue:        dangerous,
		Index:        0,
		Decisions:    make(map[string]bool),
		NonDangerous: safe,
		DryRun:       dryRun,
	}
	a.interactState = InteractConfirming

	a.logPart.Clear()
	fmt.Fprintf(a.logPart,
		"[yellow][%s] 危险操作确认开始：%d 个危险任务需要逐个确认[-]\n",
		time.Now().Format("15:04:05"), len(dangerous))
	fmt.Fprintln(a.logPart, "")

	a.showConfirmStep()
	a.updateStatus()
	a.updateHelp()
}

func (a *App) showConfirmStep() {
	cs := a.confirmState
	if cs == nil {
		return
	}
	if cs.Index >= len(cs.Queue) {
		a.finishConfirm()
		return
	}

	task := cs.Queue[cs.Index]
	total := len(cs.Queue)
	idx := cs.Index + 1

	a.interactExtra = fmt.Sprintf("(%d/%d)", idx, total)

	hintLine := hint(
		"确认", "y",
		"跳过", "n",
		"全部确认", "a",
		"全部跳过", "s")
	a.statusPart.SetText(fmt.Sprintf(
		"[red::b]⚠ 危险确认 %d/%d[-:-:-]  %s\n"+
			"[yellow]任务:[-] %s",
		idx, total, hintLine, task.Name))

	a.startBlink(fmt.Sprintf("⚠ 危险确认 %d/%d ⚠", idx, total))

	fmt.Fprintf(a.logPart, "[red::b]=== 危险操作确认 (%d/%d) ===[-:-:-]\n\n", idx, total)
	fmt.Fprintf(a.logPart, "  任务: %s\n", task.Name)

	var flags []string
	if task.NeedsDelete() {
		flags = append(flags, "--delete")
	}
	if task.NeedsRemoveSource(a.cfg.Global) {
		flags = append(flags, "--remove-source-files")
	}
	fmt.Fprintf(a.logPart, "  ├─ 危险选项: [yellow]%s[-]\n", strings.Join(flags, ", "))
	fmt.Fprintf(a.logPart, "  ├─ 源:   %s\n", task.Src)
	fmt.Fprintf(a.logPart, "  ├─ 目标: admin@%s:%s\n", a.cfg.Global.Host, task.Dst)
	if task.NeedsDelete() {
		fmt.Fprintln(a.logPart,
			"  ├─ 说明: [yellow]--delete 会删除目标端多余文件（进回收站）[-]")
	}
	if task.NeedsRemoveSource(a.cfg.Global) {
		fmt.Fprintln(a.logPart,
			"  └─ 说明: [red]--remove-source-files 会删除本地源文件（不可恢复）[-]")
	}
	fmt.Fprintln(a.logPart, "")
	a.logPart.ScrollToEnd()

	for i, t := range a.cfg.Tasks {
		if t.Name == task.Name {
			a.table.Select(i+1, 0)
			break
		}
	}
	a.refreshTasks()
	a.setFocus("interact")
	a.updateStatus()
}

func (a *App) confirmCurrent(confirm bool) {
	cs := a.confirmState
	if cs == nil || cs.Index >= len(cs.Queue) {
		return
	}
	task := cs.Queue[cs.Index]
	cs.Decisions[task.Name] = confirm
	if confirm {
		fmt.Fprintf(a.logPart, "[green][%s] %s → 确认 (y)[-]\n",
			time.Now().Format("15:04:05"), task.Name)
	} else {
		fmt.Fprintf(a.logPart, "[yellow][%s] %s → 跳过 (n)[-]\n",
			time.Now().Format("15:04:05"), task.Name)
	}
	a.logPart.ScrollToEnd()
	cs.Index++
	a.showConfirmStep()
}

func (a *App) confirmAllConfirm() {
	cs := a.confirmState
	if cs == nil {
		return
	}
	task := cs.Queue[cs.Index]
	fmt.Fprintf(a.logPart, "[green][%s] %s → 确认 (a = 全部确认)[-]\n",
		time.Now().Format("15:04:05"), task.Name)
	for i := cs.Index; i < len(cs.Queue); i++ {
		cs.Decisions[cs.Queue[i].Name] = true
	}
	remaining := len(cs.Queue) - cs.Index - 1
	if remaining > 0 {
		fmt.Fprintf(a.logPart, "[green]剩余 %d 个自动确认[-]\n", remaining)
	}
	a.logPart.ScrollToEnd()
	a.finishConfirm()
}

func (a *App) confirmAllSkip() {
	cs := a.confirmState
	if cs == nil {
		return
	}
	task := cs.Queue[cs.Index]
	fmt.Fprintf(a.logPart, "[yellow][%s] %s → 跳过 (s = 全部跳过)[-]\n",
		time.Now().Format("15:04:05"), task.Name)
	for i := cs.Index; i < len(cs.Queue); i++ {
		cs.Decisions[cs.Queue[i].Name] = false
	}
	remaining := len(cs.Queue) - cs.Index - 1
	if remaining > 0 {
		fmt.Fprintf(a.logPart, "[yellow]剩余 %d 个自动跳过[-]\n", remaining)
	}
	a.logPart.ScrollToEnd()
	a.finishConfirm()
}

func (a *App) finishConfirm() {
	cs := a.confirmState
	a.stopBlink()
	a.confirmState = nil
	a.interactExtra = ""
	a.refreshTasks()

	var toRun []*Task
	var confirmed, skipped []string
	for _, t := range cs.Queue {
		if cs.Decisions[t.Name] {
			toRun = append(toRun, t)
			confirmed = append(confirmed, t.Name)
		} else {
			skipped = append(skipped, t.Name)
		}
	}
	toRun = append(toRun, cs.NonDangerous...)

	fmt.Fprintln(a.logPart, "")
	fmt.Fprintln(a.logPart, "[yellow]=== 危险确认完成 ===[-]")
	if len(confirmed) > 0 {
		fmt.Fprintf(a.logPart, "[green]已确认 %d: %s[-]\n",
			len(confirmed), strings.Join(confirmed, " "))
	} else {
		fmt.Fprintln(a.logPart, "[gray]已确认 0[-]")
	}
	if len(skipped) > 0 {
		fmt.Fprintf(a.logPart, "[yellow]已跳过 %d: %s[-]\n",
			len(skipped), strings.Join(skipped, " "))
	} else {
		fmt.Fprintln(a.logPart, "[gray]已跳过 0[-]")
	}

	if len(toRun) == 0 {
		fmt.Fprintln(a.logPart, "")
		fmt.Fprintln(a.logPart, "[red]无可执行任务，返回主界面。[-]")
		a.logPart.ScrollToEnd()
		a.interactState = InteractIdle
		a.setFocus("table")
		a.setStatus("[yellow]无可执行任务[-]")
		a.updateHelp()
		return
	}

	fmt.Fprintln(a.logPart, "")
	fmt.Fprintf(a.logPart, "[yellow]开始执行 %d 个任务：[-]\n", len(toRun))
	for _, t := range toRun {
		mark := "普通"
		if t.IsDangerous(a.cfg.Global) {
			mark = "[orange]危险[-]"
		}
		fmt.Fprintf(a.logPart, "  - %s (%s)\n", t.Name, mark)
	}
	a.logPart.ScrollToEnd()

	go func() {
		time.Sleep(800 * time.Millisecond)
		a.app.QueueUpdateDraw(func() {
			a.startRun(toRun, cs.DryRun)
		})
	}()
}

// ---------- 执行 ----------

func (a *App) startRun(tasks []*Task, dryRun bool) {
	if a.running {
		return
	}
	a.running = true
	a.paused.Store(false)
	a.cancelRequested = false
	a.interactState = InteractRunning
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	mode := "实际执行"
	if dryRun {
		mode = "预览"
	}

	a.logPart.Clear()
	a.statusPart.SetText(fmt.Sprintf(
		"[yellow]模式:[-] %s    [yellow]任务数:[-] %d    [yellow]当前:[-] -\n"+
			"[green]成功 0[-]  [yellow]跳过 0[-]  [red]失败 0[-]  "+
			"[orange]挂载门禁失败 0[-]",
		mode, len(tasks)))

	fmt.Fprintln(a.logPart, tview.Escape("正在准备..."))
	fmt.Fprintln(a.logPart, tview.Escape("  - 远端挂载检查"))
	fmt.Fprintln(a.logPart, tview.Escape("  - 远端父目录检查"))
	fmt.Fprintln(a.logPart, tview.Escape("  - 创建远端目标目录"))
	fmt.Fprintln(a.logPart, "")
	fmt.Fprintln(a.logPart, tview.Escape("以上步骤可能需要几秒，请稍候..."))
	fmt.Fprintln(a.logPart, "")

	a.setFocus("interact")
	a.updateStatus()
	a.updateHelp()
	go a.doRun(ctx, tasks, dryRun, mode)
}

func (a *App) doRun(ctx context.Context, tasks []*Task, dryRun bool, mode string) {
	defer func() {
		a.app.QueueUpdateDraw(func() {
			a.running = false
			a.cancel = nil
			a.cancelRequested = false
		})
	}()

	success, failed, skipped, mountFailed := 0, 0, 0, 0
	total := len(tasks)
	outcomes := make([]taskOutcome, 0, total)
	cancelled := false

	for i, t := range tasks {
		i, t := i, t

		a.errMu.Lock()
		a.currentErrors = nil
		a.errMu.Unlock()

		a.app.QueueUpdateDraw(func() {
			a.interactExtra = fmt.Sprintf("(%d/%d)", i+1, total)
			a.statusPart.SetText(fmt.Sprintf(
				"[yellow]模式:[-] %s    "+
					"[yellow]进度:[-] %d/%d    "+
					"[yellow]当前:[-] %s\n"+
					"[green]成功 %d[-]  [yellow]跳过 %d[-]  "+
					"[red]失败 %d[-]  [orange]挂载门禁失败 %d[-]",
				mode, i+1, total, t.Name,
				success, skipped, failed, mountFailed))
			a.updateStatus()
		})

		var res *TaskResult
		if dryRun {
			res = StreamDryRun(ctx, a.scriptPath, a.configPath, t.Name, a.outputLine)
		} else {
			res = StreamTask(ctx, a.scriptPath, a.configPath, t.Name, a.outputLine)
		}

		var result string
		switch {
		case res.Err != nil || res.ExitCode < 0:
			result = "失败"
			failed++
		case res.ExitCode == 0:
			result = "成功"
			success++
		case res.ExitCode == 2:
			result = "跳过"
			skipped++
		case res.ExitCode == 3:
			result = "挂载门禁失败"
			mountFailed++
		default:
			result = "失败"
			failed++
		}
		outcomes = append(outcomes, taskOutcome{Name: t.Name, Result: result})

		a.errMu.Lock()
		errs := append([]string(nil), a.currentErrors...)
		a.currentErrors = nil
		a.errMu.Unlock()

		a.app.QueueUpdateDraw(func() {
			var color string
			switch result {
			case "成功":
				color = "[green]成功[-]"
			case "跳过":
				color = "[yellow]跳过[-]"
			case "挂载门禁失败":
				color = "[orange]挂载门禁失败[-]"
			default:
				color = "[red]失败[-]"
			}
			fmt.Fprintf(a.logPart,
				"\n[cyan]>>> [%d/%d] %s %s[-]    "+
					"[gray]累积:[-] "+
					"[green]成功 %d[-]  [yellow]跳过 %d[-]  "+
					"[red]失败 %d[-]  [orange]挂载门禁失败 %d[-]\n",
				i+1, total, t.Name, color,
				success, skipped, failed, mountFailed)

			if len(errs) > 0 {
				max := 5
				if len(errs) < max {
					max = len(errs)
				}
				fmt.Fprintln(a.logPart, "[red]    错误摘要：[-]")
				for j := 0; j < max; j++ {
					fmt.Fprintf(a.logPart, "[red]      %s[-]\n",
						tview.Escape(errs[j]))
				}
				if len(errs) > max {
					fmt.Fprintf(a.logPart,
						"[red]      ... 还有 %d 行[-]\n", len(errs)-max)
				}
			}
			fmt.Fprintln(a.logPart, "")
			if !a.paused.Load() {
				a.logPart.ScrollToEnd()
			}
			a.updateStatus()
		})

		select {
		case <-ctx.Done():
			cancelled = true
			goto finish
		default:
		}
	}

finish:
	a.app.QueueUpdateDraw(func() {
		var notRun []string
		if cancelled {
			done := len(outcomes)
			for i := done; i < total; i++ {
				notRun = append(notRun, tasks[i].Name)
			}
			a.interactState = InteractCancelled
		} else {
			a.interactState = InteractDone
		}
		a.interactExtra = ""

		fmt.Fprintln(a.logPart,
			"[cyan]════════════════════════════════════════════════════════[-]")
		if cancelled {
			fmt.Fprintf(a.logPart,
				" [yellow][%s] 运行被取消[-]\n", time.Now().Format("15:04:05"))
		} else {
			fmt.Fprintf(a.logPart,
				" [green][%s] 全部任务完成[-]\n", time.Now().Format("15:04:05"))
		}

		var sName, skName, fName, mName []string
		for _, o := range outcomes {
			switch o.Result {
			case "成功":
				sName = append(sName, o.Name)
			case "跳过":
				skName = append(skName, o.Name)
			case "失败":
				fName = append(fName, o.Name)
			case "挂载门禁失败":
				mName = append(mName, o.Name)
			}
		}
		if len(sName) > 0 {
			fmt.Fprintf(a.logPart, " [green]成功 %d: %s[-]\n",
				len(sName), strings.Join(sName, " "))
		} else {
			fmt.Fprintln(a.logPart, " [gray]成功 0[-]")
		}
		if len(skName) > 0 {
			fmt.Fprintf(a.logPart, " [yellow]跳过 %d: %s[-]\n",
				len(skName), strings.Join(skName, " "))
		} else {
			fmt.Fprintln(a.logPart, " [gray]跳过 0[-]")
		}
		if len(fName) > 0 {
			fmt.Fprintf(a.logPart, " [red]失败 %d: %s[-]\n",
				len(fName), strings.Join(fName, " "))
		} else {
			fmt.Fprintln(a.logPart, " [gray]失败 0[-]")
		}
		if len(mName) > 0 {
			fmt.Fprintf(a.logPart, " [orange]挂载门禁失败 %d: %s[-]\n",
				len(mName), strings.Join(mName, " "))
		} else {
			fmt.Fprintln(a.logPart, " [gray]挂载门禁失败 0[-]")
		}
		if len(notRun) > 0 {
			fmt.Fprintf(a.logPart, " [gray]未执行 %d: %s[-]\n",
				len(notRun), strings.Join(notRun, " "))
		}
		fmt.Fprintln(a.logPart,
			"[cyan]════════════════════════════════════════════════════════[-]")
		a.logPart.ScrollToEnd()

		if cancelled {
			a.statusPart.SetText(fmt.Sprintf(
				"[yellow]运行被取消[-]  "+
					"[green]成功 %d[-]  [yellow]跳过 %d[-]  "+
					"[red]失败 %d[-]  [orange]挂载门禁失败 %d[-]",
				success, skipped, failed, mountFailed))
		} else {
			a.statusPart.SetText(fmt.Sprintf(
				"[green]运行完成[-]  "+
					"[green]成功 %d[-]  [yellow]跳过 %d[-]  "+
					"[red]失败 %d[-]  [orange]挂载门禁失败 %d[-]",
				success, skipped, failed, mountFailed))
		}
		a.updateStatus()
		a.updateHelp()
	})
}

func (a *App) outputLine(line string) {
	clean := ansiRe.ReplaceAllString(line, "")
	if shouldFilterLine(clean) {
		return
	}
	if isErrorLine(clean) {
		a.errMu.Lock()
		if len(a.currentErrors) < 20 {
			a.currentErrors = append(a.currentErrors, clean)
		}
		a.errMu.Unlock()
	}

	if a.paused.Load() {
		a.logBufMu.Lock()
		a.logBuf = append(a.logBuf, clean)
		a.logBufMu.Unlock()
		return
	}

	a.app.QueueUpdateDraw(func() {
		fmt.Fprintln(a.logPart, tview.Escape(clean))
		a.logPart.ScrollToEnd()
	})
}

// ---------- 挂载检查 ----------

func (a *App) runMountCheck() {
	if a.running || a.confirmState != nil {
		return
	}
	a.running = true
	a.paused.Store(false)
	a.cancelRequested = false
	a.interactState = InteractRunning
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	a.logPart.Clear()
	a.statusPart.SetText("[yellow]挂载检查[-]\n[gray]正在检查远端挂载状态...[-]")
	fmt.Fprintln(a.logPart, tview.Escape("正在检查远端挂载状态..."))
	a.setFocus("interact")
	a.updateStatus()
	a.updateHelp()

	go func() {
		defer func() {
			a.app.QueueUpdateDraw(func() {
				a.running = false
				a.cancel = nil
				a.cancelRequested = false
				a.interactState = InteractDone
				a.updateStatus()
				a.updateHelp()
			})
		}()

		res := StreamCheckMount(ctx, a.scriptPath, a.configPath, func(line string) {
			clean := ansiRe.ReplaceAllString(line, "")
			if shouldFilterLine(clean) {
				return
			}
			a.app.QueueUpdateDraw(func() {
				fmt.Fprintln(a.logPart, tview.Escape(clean))
				if !a.paused.Load() {
					a.logPart.ScrollToEnd()
				}
			})
		})

		a.app.QueueUpdateDraw(func() {
			fmt.Fprintf(a.logPart, "\n[退出码: %d]\n", res.ExitCode)
			if res.Err != nil {
				fmt.Fprintf(a.logPart, "[red]错误: %v[-]\n", res.Err)
			}
			a.logPart.ScrollToEnd()
			a.setStatus("[green]挂载检查完成[-]")
		})
	}()
}

// ---------- 重载 ----------

func (a *App) reloadConfig() {
	cfg, err := ParseConfig(a.configPath)
	if err != nil {
		a.setStatus(fmt.Sprintf("[red]重新加载失败: %v[-]", err))
		return
	}
	for _, t := range cfg.Tasks {
		if old, ok := a.cfg.TaskMap[t.Name]; ok && old.Selected {
			t.Selected = true
		}
	}
	a.cfg = cfg
	a.updateHeader()
	a.refreshTasks()
	a.updateStatus()
}

// ---------- main ----------

func main() {
	a := NewApp()

	if debugKeys {
		fmt.Fprintf(os.Stderr, "[DEBUG] version=%s git=%s build=%s\n",
			Version, GitCommit, BuildTime)
		fmt.Fprintf(os.Stderr, "[DEBUG] script=%s\n", a.scriptPath)
		fmt.Fprintf(os.Stderr, "[DEBUG] config=%s\n", a.configPath)
		fmt.Fprintf(os.Stderr, "[DEBUG] sep=%q\n", sepChar)
	}

	cfg, err := ParseConfig(a.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "尝试路径: %s\n", a.configPath)
		fmt.Fprintf(os.Stderr, "可通过 RBACKUP_CONFIG 或 -c 指定配置文件\n")
		os.Exit(1)
	}
	a.cfg = cfg

	a.setupUI()
	a.updateHeader()
	a.refreshTasks()
	a.updateStatus()

	a.app.SetRoot(a.pages, true).EnableMouse(true)
	a.setFocus("table")

	if err := a.app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}