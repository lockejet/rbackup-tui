package main

// 终端二维码渲染：把 otpauth:// URI 画成可被手机直接扫描的 QR。
// 使用半块字符 + ANSI 颜色（上黑下白/上白下黑），在常见终端里清晰可扫；
// 输出走 stderr，重定向 stdout 时不会污染数据。

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
	"rsc.io/qr"
)

const qrQuietZone = 4

// terminalSupportsQR 判断是否适合往 stderr 画二维码。
//
//	RBACKUP_NO_QR=1        关闭
//	RBACKUP_QR=1|always    强制执行（即使 stderr 不是 TTY，便于日志/CI 预览）
//	默认：stderr 是 TTY 才画
func terminalSupportsQR() bool {
	if strings.TrimSpace(os.Getenv("RBACKUP_NO_QR")) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RBACKUP_QR"))) {
	case "1", "yes", "true", "always":
		return true
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// renderQRTerminal 把 text 编码为 QR 并渲染到 out。
func renderQRTerminal(out io.Writer, text string) error {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return err
	}
	size := code.Size + qrQuietZone*2
	isBlack := func(x, y int) bool {
		if x < qrQuietZone || y < qrQuietZone {
			return false
		}
		x -= qrQuietZone
		y -= qrQuietZone
		if x >= code.Size || y >= code.Size {
			return false
		}
		return code.Black(x, y)
	}

	w := bufio.NewWriter(out)
	defer w.Flush()
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x++ {
			top := isBlack(x, y)
			bottom := y+1 < size && isBlack(x, y+1)
			// '▀' 上半块：前景=上格颜色，背景=下格颜色
			fg, bg := "37", "47" // 白/白
			if top {
				fg = "30" // 黑
			}
			if bottom {
				bg = "40"
			}
			fmt.Fprintf(w, "\x1b[%s;%sm▀", fg, bg)
		}
		fmt.Fprint(w, "\x1b[0m\n")
	}
	return nil
}

// printTOTPQR 在终端里打印 otpauth URI 的二维码；非 TTY 时静默跳过。
func printTOTPQR(uri string) {
	if !terminalSupportsQR() {
		return
	}
	fmt.Fprintln(os.Stderr, "用手机 Authenticator 扫描下方二维码（或手工录入上面的 secret）：")
	fmt.Fprintln(os.Stderr)
	if err := renderQRTerminal(os.Stderr, uri); err != nil {
		fmt.Fprintf(os.Stderr, "（二维码渲染失败：%v，请改用 otpauth URI 手工添加）\n", err)
		return
	}
	fmt.Fprintln(os.Stderr, "（扫不上可把上面的 otpauth URI 复制到浏览器做二维码，或在终端设置 RBACKUP_NO_QR=1 关闭此图）")
}
