package main

// Web UI 认证扩展：用户名+密码（PBKDF2-HMAC-SHA256）与 TOTP 二次验证（RFC 6238）。
//
// 设计要点：
//   - 密码只存哈希（pbkdf2-sha256$迭代数$盐$哈希），不存明文；
//   - token 登录保持兼容，并作为"忘记认证器"时的恢复方式（token 登录不要求 TOTP）；
//   - TOTP 只对密码登录生效，验证通过前只发短期待验证 Cookie；
//   - 标准库实现，零新增模块依赖（golang.org/x/term 用于无回显读密码）。

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

const (
	pbkdf2Iterations = 600000
	pbkdf2KeyLen     = 32
	totpPeriod       = 30
	totpDigits       = 6
	pending2FATTL    = 5 * time.Minute
	pending2FAMaxTry = 5
)

// ---------- 密码哈希 ----------

// hashPassword 生成 "pbkdf2-sha256$<iter>$<saltB64>$<hashB64>"。
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// verifyPassword 校验密码哈希（常数时间比较）。
func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// readPasswordFromTerminal 交互式读取两次密码（不回显）。
func readPasswordFromTerminal() (string, error) {
	fmt.Fprint(os.Stderr, "输入新密码（不回显）: ")
	b1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "再输入一次确认: ")
	b2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(b1) != string(b2) {
		return "", errors.New("两次输入不一致")
	}
	if len(b1) < 8 {
		return "", errors.New("密码太短（至少 8 位）")
	}
	return string(b1), nil
}

// ---------- TOTP ----------

// decodeTOTPSecret 解析 base32 secret（容忍空格、大小写、填充）。
func decodeTOTPSecret(s string) ([]byte, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimRight(s, "=")
	if s == "" {
		return nil, errors.New("TOTP secret 为空")
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
}

func totpCode(secret []byte, t time.Time) string {
	counter := uint64(t.Unix() / totpPeriod)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	m := hmac.New(sha1.New, secret)
	m.Write(buf)
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[off])&0x7f)<<24 |
		uint32(sum[off+1])<<16 |
		uint32(sum[off+2])<<8 |
		uint32(sum[off+3])
	return fmt.Sprintf("%0*d", totpDigits, bin%1_000_000)
}

// verifyTOTP 校验 6 位动态码，允许 ±1 个时间窗（时钟漂移）；
// 返回命中的 counter（用于防重放）与是否通过。
func verifyTOTP(secret []byte, code string) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return 0, false
	}
	now := time.Now()
	for _, delta := range []int64{-1, 0, 1} {
		t := now.Add(time.Duration(delta) * totpPeriod * time.Second)
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, t)), []byte(code)) == 1 {
			return t.Unix() / totpPeriod, true
		}
	}
	return 0, false
}

func newTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

func otpauthURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		label, secret, url.QueryEscape(issuer), totpDigits, totpPeriod)
}

// ---------- 待验证（2FA 中间态）会话 ----------

type pending2FA struct {
	mu sync.Mutex
	m  map[string]*pendingRec
}

type pendingRec struct {
	until    time.Time
	attempts int
}

func newPending2FA() *pending2FA {
	return &pending2FA{m: map[string]*pendingRec{}}
}

func (p *pending2FA) create() string {
	id := randToken(18)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleanupLocked()
	p.m[id] = &pendingRec{until: time.Now().Add(pending2FATTL)}
	return id
}

// check 校验 pending 是否存在且未过期，返回剩余尝试次数。
func (p *pending2FA) check(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleanupLocked()
	rec, ok := p.m[id]
	if !ok {
		return false
	}
	if time.Now().After(rec.until) {
		delete(p.m, id)
		return false
	}
	return true
}

// fail 记录一次失败；超过上限则销毁。
func (p *pending2FA) fail(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.m[id]
	if !ok {
		return
	}
	rec.attempts++
	if rec.attempts >= pending2FAMaxTry {
		delete(p.m, id)
	}
}

func (p *pending2FA) consume(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, id)
}

func (p *pending2FA) cleanupLocked() {
	now := time.Now()
	for k, rec := range p.m {
		if now.After(rec.until) {
			delete(p.m, k)
		}
	}
}

// ---------- CLI 辅助：生成密码哈希 / TOTP secret ----------

// runHashPasswordCLI 对应 --web-hash-password：交互式生成密码哈希后打印。
func runHashPasswordCLI() int {
	pw, err := readPasswordFromTerminal()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	hash, err := hashPassword(pw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	fmt.Println(hash)
	fmt.Fprintln(os.Stderr, "把输出写入文件（如 ~/.config/rbackup-ui/web.passwd，chmod 600），")
	fmt.Fprintln(os.Stderr, "然后启动时用 --web-password-hash-file 指定。")
	fmt.Fprintln(os.Stderr, "注意：如果 Web 服务已在运行，修改密码/哈希文件后必须重启服务才生效。")
	return 0
}

// defaultTOTPSecretFile 返回默认的 TOTP secret 文件路径（XDG 配置目录）。
func defaultTOTPSecretFile() string {
	return filepath.Join(xdgConfigDir(), "rbackup-ui", "web.totp")
}

// runShowTOTPCLI 对应 --web-show-totp-qr：为"现有" secret 显示二维码（不轮换）。
func runShowTOTPCLI(secretFile, account string) int {
	if secretFile == "" {
		secretFile = defaultTOTPSecretFile()
	}
	b, err := os.ReadFile(secretFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 读取 TOTP secret 失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "提示: 用 --web-totp-secret-file 指定路径，或先运行 --web-gen-totp 生成\n")
		return 1
	}
	if _, err := decodeTOTPSecret(string(b)); err != nil {
		fmt.Fprintf(os.Stderr, "错误: secret 格式不对: %v\n", err)
		return 1
	}
	if account == "" {
		account = "admin"
	}
	secretStr := strings.TrimRight(strings.ToUpper(strings.TrimSpace(string(b))), "=")
	uri := otpauthURI("rbackup-ui", account, secretStr)
	fmt.Println("otpauth URI:")
	fmt.Println(uri)
	fmt.Println()
	printTOTPQR(uri)
	return 0
}

// runGenTOTPCLI 对应 --web-gen-totp：生成 secret 与 otpauth URI（会轮换 secret）。
func runGenTOTPCLI(account string) int {
	secret, err := newTOTPSecret()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	if account == "" {
		account = "admin"
	}
	fmt.Println("TOTP secret（base32）:")
	fmt.Println(secret)
	fmt.Println()
	uri := otpauthURI("rbackup-ui", account, secret)
	fmt.Println("otpauth URI（手动添加到 Authenticator / 支持 otpauth 的密码管理器）:")
	fmt.Println(uri)
	fmt.Println()
	// 在终端里直接画二维码（stdout 重定向时只输出上面的文本，二维码走 stderr）
	printTOTPQR(uri)
	fmt.Fprintln(os.Stderr, "把 secret 保存到文件（如 ~/.config/rbackup-ui/web.totp，chmod 600），")
	fmt.Fprintln(os.Stderr, "然后启动时用 --web-2fa totp --web-totp-secret-file 指定。")
	fmt.Fprintln(os.Stderr, "注意：如果 Web 服务已在运行，修改 secret 文件后必须重启服务才生效。")
	fmt.Fprintln(os.Stderr, "注意：token 登录不受 TOTP 限制（token 作为恢复方式），")
	fmt.Fprintln(os.Stderr, "所以请妥善保管 token、并确保 2FA 与 TLS/VPN 一起使用。")
	return 0
}

// validPasswordHash 校验密码哈希字符串格式（不验证密码本身）。
func validPasswordHash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return false
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[3])
	return err == nil && len(key) > 0
}

// runShowTOTPCodeCLI 对应 --web-show-totp-code：用指定 secret 计算"当前"动态码，
// 仅用于排查"App 里的码和服务端不一致"这类问题。
func runShowTOTPCodeCLI(secretFile string) int {
	if secretFile == "" {
		secretFile = defaultTOTPSecretFile()
	}
	b, err := os.ReadFile(secretFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 读取 TOTP secret 失败: %v\n", err)
		return 1
	}
	secret, err := decodeTOTPSecret(string(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: secret 格式不对: %v\n", err)
		return 1
	}
	now := time.Now()
	code := totpCode(secret, now)
	left := totpPeriod - int(now.Unix()%totpPeriod)
	fmt.Printf("secret 文件: %s\n", secretFile)
	fmt.Printf("当前动态码: %s（%d 秒后刷新）\n", code, left)
	fmt.Fprintln(os.Stderr, "提示: 把这个码和你 App 里显示的对比；不一致通常是 secret 不同或时钟偏差。")
	fmt.Fprintln(os.Stderr, "      修改 secret/密码/token 文件后，运行中的服务必须重启才会生效。")
	return 0
}
