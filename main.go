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
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

var debugKeys = os.Getenv("RBACKUP_DEBUG_KEYS") != ""
var debugCount int

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

// ---------- 结果记录 ----------
type taskOutcome struct {
	Name   string
	Result string // "成功" / "跳过" / "失败" / "挂载门禁失败"
}

// ---------- 危险确认状态 ----------
type ConfirmState struct {
	Queue        []*Task
	Index        int
	Decisions    map[string]bool
	NonDangerous []*Task
	DryRun       bool
}

type App struct {
	app *tview.Application

	configPath string
	scriptPath string
	cfg        *Config

	header    *tview.TextView
	table     *tview.Table
	tableArea *tview.Flex
	tableHint *tview.TextView

	interact     *tview.Flex
	statusPart   *tview.TextView
	logPart      *tview.TextView
	interactHint *tview.TextView

	status *tview.TextView
	help   *tview.TextView

	running bool
	cancel  context.CancelFunc

	focusArea string // "table" / "interact"

	confirmState *ConfirmState
	paused       bool

	blinkStop chan struct{}
	blinkGen  int

	// 当前任务的错误摘要（按任务收集）
	errMu         sync.Mutex
	currentErrors []string
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
		app:        tview.NewApplication(),
		configPath: configPath,
		scriptPath: scriptPath,
		focusArea:  "table",
	}
}

// ---------- UI ----------

func (a *App) setupUI() {
	a.header = tview.NewTextView().SetDynamicColors(true)
	a.header.SetBorder(true).SetTitle(" rbackup ")

	a.table = tview.NewTable().SetSelectable(true, false).SetBorders(false)
	a.table.SetFixed(1, 0)
	a.table.SetBorder(false)

	a.tableHint = tview.NewTextView().SetDynamicColors(true).
		SetText("[gray]移动[white][↑↓][gray]  选择[white][空格][gray]  " +
			"全选[white][a][gray]  全不选[white][n][gray]  " +
			"翻页[white][PgUp/PgDn][gray]  首尾[white][Home/End][gray]  " +
			"运行[white][Enter][-]")

	a.tableArea = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.table, 0, 1, true).
		AddItem(a.tableHint, 1, 0, false)
	a.tableArea.SetBorder(true).SetTitle(" [1] 任务列表 [Tab/1] ")

	a.statusPart = tview.NewTextView().SetDynamicColors(true)

	a.logPart = tview.NewTextView().SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetWordWrap(true)

	a.interactHint = tview.NewTextView().SetDynamicColors(true).
		SetText("[gray]滚动[white][↑↓/j/k][gray]  翻页[white][PgUp/PgDn][gray]  " +
			"首尾[white][Home/End/g/G][gray]  暂停[white][p/空格][-]")

	a.interact = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.statusPart, 2, 0, false).
		AddItem(a.logPart, 0, 1, false).
		AddItem(a.interactHint, 1, 0, false)
	a.interact.SetBorder(true).SetTitle(" [2] 交互区 [Tab/2] ")

	a.status = tview.NewTextView().SetDynamicColors(true)
	a.help = tview.NewTextView().SetDynamicColors(false).
		SetText("切换焦点[Tab/1/2]  运行[Enter]  dry-run[d]  " +
			"挂载检查[m]  刷新[r]  退出[q]")

	a.app.SetInputCapture(a.globalInputCapture)
	a.updateFocusStyle()
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
	if a.focusArea == "table" {
		a.tableArea.SetBorderColor(tcell.ColorGreen)
		a.interact.SetBorderColor(tcell.ColorGray)
	} else {
		a.tableArea.SetBorderColor(tcell.ColorGray)
		a.interact.SetBorderColor(tcell.ColorGreen)
	}
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
	// 分隔线一律过滤
	if strings.HasPrefix(t, "====") {
		return true
	}
	// 每任务汇总：成功 N: xxx / 跳过 N[: xxx] / 失败 N[: xxx] / 挂载门禁失败 N[: xxx]
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
		"rsync error",
		"rsync:",
		"[fail]",
		"failed",
		"error:",
		"错误",
		"失败",
		"拒绝",
		"不可恢复",
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

// ---------- 滚动 ----------

func (a *App) scrollLog(delta int) {
	row, col := a.logPart.GetScrollOffset()
	newRow := row + delta
	if newRow < 0 {
		newRow = 0
	}
	a.logPart.ScrollTo(newRow, col)
}

func (a *App) handleScrollKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyUp:
		a.scrollLog(-1)
		return true
	case tcell.KeyDown:
		a.scrollLog(1)
		return true
	case tcell.KeyPgUp:
		a.scrollLog(-10)
		return true
	case tcell.KeyPgDn:
		a.scrollLog(10)
		return true
	case tcell.KeyHome:
		a.logPart.ScrollToBeginning()
		return true
	case tcell.KeyEnd:
		a.logPart.ScrollToEnd()
		return true
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'j':
			a.scrollLog(1)
			return true
		case 'k':
			a.scrollLog(-1)
			return true
		case 'g':
			a.logPart.ScrollToBeginning()
			return true
		case 'G':
			a.logPart.ScrollToEnd()
			return true
		}
	}
	return false
}

// ---------- 全局按键 ----------

func (a *App) globalInputCapture(event *tcell.EventKey) *tcell.EventKey {
	if isDuplicateKey(event) {
		return nil
	}
	debugKey("global", event)

	switch event.Key() {
	case tcell.KeyTab:
		a.toggleFocus()
		return nil
	case tcell.KeyCtrlC:
		if a.confirmState != nil {
			a.confirmAllSkip()
			return nil
		}
		if a.running && a.cancel != nil {
			a.cancel()
			a.cancel = nil
			a.setStatus("[yellow]已请求取消[-]")
			return nil
		}
		a.app.Stop()
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

	if event.Key() == tcell.KeyEscape {
		if a.running && a.cancel != nil {
			a.cancel()
			a.cancel = nil
			a.setStatus("[yellow]已请求取消[-]")
			return nil
		}
		if a.focusArea == "interact" {
			a.setFocus("table")
			return nil
		}
		a.app.Stop()
		return nil
	}

	if a.running {
		if a.focusArea == "interact" && a.handleScrollKey(event) {
			return nil
		}
		return event
	}

	if a.focusArea == "interact" {
		if a.handleScrollKey(event) {
			return nil
		}
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'p', 'P', ' ':
				a.togglePause()
				return nil
			}
		}
		return event
	}
	return a.handleTableKey(event)
}

func (a *App) handleConfirmKey(event *tcell.EventKey) *tcell.EventKey {
	if a.handleScrollKey(event) {
		return nil
	}

	cs := a.confirmState
	if cs == nil || cs.Index >= len(cs.Queue) {
		return event
	}

	switch event.Key() {
	case tcell.KeyEscape:
		a.confirmAllSkip()
		return nil
	case tcell.KeyEnter:
		a.confirmCurrent(true)
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
	return event
}

func (a *App) handleTableKey(event *tcell.EventKey) *tcell.EventKey {
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
		case 'q', 'Q':
			a.app.Stop()
			return nil
		}
	}
	return event
}

// ---------- 暂停 ----------

func (a *App) togglePause() {
	a.paused = !a.paused
	if a.paused {
		a.setStatus("[yellow]⏸ 已暂停（按 p 继续）[-]")
	} else {
		a.logPart.ScrollToEnd()
		a.updateStatus()
	}
}

// ---------- 状态刷新 ----------

func (a *App) updateHeader() {
	txt := fmt.Sprintf(
		"[yellow]脚本:[-] %s\n"+
			"[yellow]配置:[-] %s\n"+
			"[yellow]远端:[-] admin@%s:%s    "+
			"[yellow]策略:[-] MOUNT_POLICY=%s\n"+
			"[yellow]日志:[-] %s    "+
			"[yellow]任务:[-] %d",
		a.scriptPath,
		a.configPath,
		a.cfg.Global.Host, a.cfg.Global.SSHPort,
		a.cfg.Global.MountPolicy,
		filepath.Join(a.cfg.Global.LogDir,
			"rbackup_"+time.Now().Format("20060102")+".log"),
		len(a.cfg.Tasks),
	)
	a.header.SetText(txt)
}

func (a *App) updateStatus() {
	sel := 0
	for _, t := range a.cfg.Tasks {
		if t.Selected {
			sel++
		}
	}
	a.status.SetText(fmt.Sprintf(
		"[green]就绪[-]  |  已选: %d/%d  |  焦点: [yellow]%s[-]",
		sel, len(a.cfg.Tasks), a.focusArea))
}

func (a *App) updateTableHint() {
	var selected []string
	for _, t := range a.cfg.Tasks {
		if t.Selected {
			selected = append(selected, t.Name)
		}
	}
	if len(selected) == 0 {
		a.tableHint.SetText("[gray]未选择任何任务[-]")
		return
	}
	a.tableHint.SetText(fmt.Sprintf("[green]已选 %d:[-] %s",
		len(selected), strings.Join(selected, " ")))
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

	a.updateTableHint()
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

	a.logPart.Clear()
	fmt.Fprintf(a.logPart,
		"[yellow][%s] 危险操作确认开始：%d 个危险任务需要逐个确认[-]\n",
		time.Now().Format("15:04:05"), len(dangerous))
	fmt.Fprintln(a.logPart, "")

	a.showConfirmStep()
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

	a.statusPart.SetText(fmt.Sprintf(
		"[red::b]⚠ 危险确认 %d/%d[-:-:-]  "+
			"[white]y[-]=确认  [white]n[-]=跳过  "+
			"[white]a[-]=全部确认  [white]s[-]=全部跳过\n"+
			"[yellow]任务:[-] %s",
		idx, total, task.Name))

	a.startBlink(fmt.Sprintf("⚠ 危险确认 %d/%d：y=确认 n=跳过 a=全部确认 s=全部跳过 ⚠",
		idx, total))

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
	fmt.Fprintln(a.logPart, "  确认[white][y][-]  跳过[white][n][-]  "+
		"全部确认[white][a][-]  全部跳过[white][s][-]  取消全部[white][Esc][-]")
	fmt.Fprintln(a.logPart, "")
	fmt.Fprintln(a.logPart, "  [gray]↑↓ 滚动查看详情[-]")
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
		a.setFocus("table")
		a.setStatus("[yellow]无可执行任务[-]")
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
	a.paused = false
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	names := make([]string, 0, len(tasks))
	for _, t := range tasks {
		names = append(names, t.Name)
	}
	mode := "实际执行"
	if dryRun {
		mode = "dry-run 预览"
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
	go a.doRun(ctx, tasks, dryRun, mode)
}

func (a *App) doRun(ctx context.Context, tasks []*Task, dryRun bool, mode string) {
	defer func() {
		a.app.QueueUpdateDraw(func() {
			a.running = false
			a.cancel = nil
		})
	}()

	success, failed, skipped, mountFailed := 0, 0, 0, 0
	total := len(tasks)
	outcomes := make([]taskOutcome, 0, total)
	cancelled := false

	for i, t := range tasks {
		i, t := i, t

		// 任务开始前，重置错误收集
		a.errMu.Lock()
		a.currentErrors = nil
		a.errMu.Unlock()

		// 更新状态摘要
		a.app.QueueUpdateDraw(func() {
			a.statusPart.SetText(fmt.Sprintf(
				"[yellow]模式:[-] %s    "+
					"[yellow]进度:[-] %d/%d    "+
					"[yellow]当前:[-] %s\n"+
					"[green]成功 %d[-]  [yellow]跳过 %d[-]  "+
					"[red]失败 %d[-]  [orange]挂载门禁失败 %d[-]",
				mode, i+1, total, t.Name,
				success, skipped, failed, mountFailed))
		})

		var res *TaskResult
		if dryRun {
			res = StreamDryRun(ctx, a.scriptPath, a.configPath, t.Name, a.outputLine)
		} else {
			res = StreamTask(ctx, a.scriptPath, a.configPath, t.Name, a.outputLine)
		}

		// 判定结果
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

		// 取错误摘要
		a.errMu.Lock()
		errs := append([]string(nil), a.currentErrors...)
		a.currentErrors = nil
		a.errMu.Unlock()

		// 追加累积行到日志
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
			if !a.paused {
				a.logPart.ScrollToEnd()
			}
		})

		// 更新累积计数
		a.app.QueueUpdateDraw(func() {
			a.statusPart.SetText(fmt.Sprintf(
				"[yellow]模式:[-] %s    "+
					"[yellow]进度:[-] %d/%d    "+
					"[yellow]当前:[-] %s [green](完成)[-]\n"+
					"[green]成功 %d[-]  [yellow]跳过 %d[-]  "+
					"[red]失败 %d[-]  [orange]挂载门禁失败 %d[-]",
				mode, i+1, total, t.Name,
				success, skipped, failed, mountFailed))
		})

		// 若被取消，跳出
		select {
		case <-ctx.Done():
			cancelled = true
			goto finish
		default:
		}
	}

finish:
	// 最终汇总
	a.app.QueueUpdateDraw(func() {
		// 若被取消，剩余任务记为"未执行"
		var notRun []string
		if cancelled {
			done := len(outcomes)
			for i := done; i < total; i++ {
				notRun = append(notRun, tasks[i].Name)
			}
		}

		// 分隔线
		fmt.Fprintln(a.logPart,
			"[cyan]════════════════════════════════════════════════════════[-]")
		if cancelled {
			fmt.Fprintf(a.logPart,
				" [yellow][%s] 运行被取消[-]\n", time.Now().Format("15:04:05"))
		} else {
			fmt.Fprintf(a.logPart,
				" [green][%s] 全部任务完成[-]\n", time.Now().Format("15:04:05"))
		}

		// 按类别收集任务名
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

		// 状态摘要
		a.statusPart.SetText(fmt.Sprintf(
			"[green]运行完成[-]  "+
				"[green]成功 %d[-]  [yellow]跳过 %d[-]  "+
				"[red]失败 %d[-]  [orange]挂载门禁失败 %d[-]",
			success, skipped, failed, mountFailed))
		a.setStatus(fmt.Sprintf(
			"[green]运行完成[-] 成功 %d  跳过 %d  失败 %d  挂载门禁失败 %d",
			success, skipped, failed, mountFailed))
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
	a.app.QueueUpdateDraw(func() {
		fmt.Fprintln(a.logPart, tview.Escape(clean))
		if !a.paused {
			a.logPart.ScrollToEnd()
		}
	})
}

// ---------- 挂载检查 ----------

func (a *App) runMountCheck() {
	if a.running || a.confirmState != nil {
		return
	}
	a.running = true
	a.paused = false
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	a.logPart.Clear()
	a.statusPart.SetText("[yellow]挂载检查[-]\n[gray]正在检查远端挂载状态...[-]")
	fmt.Fprintln(a.logPart, tview.Escape("正在检查远端挂载状态..."))
	a.setFocus("interact")

	go func() {
		defer func() {
			a.app.QueueUpdateDraw(func() {
				a.running = false
				a.cancel = nil
			})
		}()

		res := StreamCheckMount(ctx, a.scriptPath, a.configPath, func(line string) {
			clean := ansiRe.ReplaceAllString(line, "")
			if shouldFilterLine(clean) {
				return
			}
			a.app.QueueUpdateDraw(func() {
				fmt.Fprintln(a.logPart, tview.Escape(clean))
				if !a.paused {
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
		fmt.Fprintf(os.Stderr, "[DEBUG] script=%s\n", a.scriptPath)
		fmt.Fprintf(os.Stderr, "[DEBUG] config=%s\n", a.configPath)
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

	a.app.SetRoot(a.mainLayout(), true).EnableMouse(true)
	a.setFocus("table")

	if err := a.app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}