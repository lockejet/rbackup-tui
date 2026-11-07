//go:build windows

package main

import "syscall"

// enableUTF8Console 把 Windows 控制台输出代码页切换到 UTF-8（65001）。
//
// 中文 Windows 的控制台默认是 936（GBK）。Go 程序按 UTF-8 输出，
// 若不切换代码页，中文会显示成乱码，且 ✓/✗ 等字符会导致列对不齐。
// 仅影响控制台输出（SetConsoleOutputCP），不改变程序内部编码；
// 管道/重定向场景下调用失败也无副作用。TUI 由 tcell 自行管理屏幕，不受影响。
func enableUTF8Console() {
	const cpUTF8 = 65001 // CP_UTF8
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	_, _, _ = setConsoleOutputCP.Call(cpUTF8)
}
