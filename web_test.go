package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "0123456789abcdef0123456789abcdef"

// writeTestFixture 创建 stub rbackup.sh 与测试配置。
func writeTestFixture(t *testing.T) (script, config, dir string) {
	t.Helper()
	dir = t.TempDir()
	script = filepath.Join(dir, "rbackup.sh")
	body := `#!/bin/bash
cfg=""; task=""; dry=0; mount=0
while [ $# -gt 0 ]; do
  case "$1" in
    --config) cfg="$2"; shift 2 ;;
    --task) task="$2"; shift 2 ;;
    --dry-run) dry=1; shift ;;
    --check-mount) mount=1; shift ;;
    *) shift ;;
  esac
done
if [ "$mount" = 1 ]; then echo "mounted: ok"; exit 0; fi
echo "task=$task dry=$dry config=$cfg"
if [ "$task" = "slow" ]; then sleep 30; fi
echo "[STATS] files=2/3 bytes_total=100 bytes_sent=50 bytes_recv=10 rsync_ms=10 list_ms=5 prep_ms=5"
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	config = filepath.Join(dir, "config.ini")
	cfg := `HOST=example.com
SSH_PORT=22
SSH_USER=admin
SSH_KEY=~/.ssh/id_ed25519
LOG_DIR=` + filepath.Join(dir, "log") + `
GLOBAL_OPTS=-avzhu
RSYNC_PATH=sudo rsync
MOUNT_POLICY=skip

[task_safe]
src=/tmp/a/
dst=/tmp/b

[task_danger]
src=/tmp/c/
dst=/tmp/d
delete=yes

[task_slow]
src=/tmp/e/
dst=/tmp/f
`
	if err := os.WriteFile(config, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return script, config, dir
}

type testEnv struct {
	srv    *webServer
	ts     *httptest.Server
	client *http.Client
	csrf   string
}

func newTestEnv(t *testing.T, mutate func(*webOptions)) *testEnv {
	t.Helper()
	script, config, dir := writeTestFixture(t)
	o := defaultWebOptions()
	o.Token = testToken
	o.Addr = "127.0.0.1:0"
	o.AuditLog = filepath.Join(dir, "audit.log")
	if mutate != nil {
		mutate(&o)
	}
	s, err := newWebServer(o, script, config)
	if err != nil {
		t.Fatalf("newWebServer: %v", err)
	}
	ts := httptest.NewServer(s.handler)
	t.Cleanup(func() { ts.Close(); s.close() })

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	env := &testEnv{srv: s, ts: ts, client: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
	if o.Token == "" {
		env.csrf = env.login(t, "", true)
	} else {
		env.csrf = env.login(t, testToken, true)
	}
	return env
}

func (e *testEnv) do(t *testing.T, method, path string, body any, csrf string) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(method, e.ts.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set(webCSRFHeader, csrf)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func (e *testEnv) login(t *testing.T, token string, wantOK bool) string {
	t.Helper()
	code, data := e.do(t, http.MethodPost, "/api/session", map[string]string{"token": token}, "")
	if wantOK && code != http.StatusOK {
		t.Fatalf("login: code=%d body=%s", code, data)
	}
	var resp sessionResp
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("login: bad json %s: %v", data, err)
	}
	return resp.CSRF
}

// waitLastJob 轮询直到 lastJob 进入期望状态。
func (e *testEnv) waitLastJob(t *testing.T, states ...string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		code, data := e.do(t, http.MethodGet, "/api/jobs/current", nil, "")
		if code == http.StatusOK {
			var resp struct {
				LastJob map[string]any `json:"lastJob"`
			}
			_ = json.Unmarshal(data, &resp)
			if resp.LastJob != nil {
				got, _ := resp.LastJob["state"].(string)
				for _, s := range states {
					if got == s {
						return resp.LastJob
					}
				}
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("等待 job 状态 %v 超时", states)
	return nil
}

func TestWebOptionsValidate(t *testing.T) {
	base := defaultWebOptions()
	base.Addr = "127.0.0.1:8080"
	if err := base.validate(); err != nil {
		t.Fatalf("loopback 默认应通过: %v", err)
	}

	o := defaultWebOptions()
	o.Addr = "0.0.0.0:8080"
	if err := o.validate(); err == nil {
		t.Fatal("非 loopback 无 token 应拒绝")
	}

	o.Token = testToken
	if err := o.validate(); err == nil {
		t.Fatal("非 loopback 无 TLS 应拒绝")
	}
	o.InsecureHTTP = true
	if err := o.validate(); err != nil {
		t.Fatalf("显式 insecure-http 应通过: %v", err)
	}

	o2 := defaultWebOptions()
	o2.TLSCert = "/tmp/a.crt"
	if err := o2.validate(); err == nil {
		t.Fatal("只给 cert 不给 key 应拒绝")
	}

	o3 := defaultWebOptions()
	o3.Addr = "127.0.0.1:8080"
	o3.Token = "short"
	if err := o3.validate(); err == nil {
		t.Fatal("过短 token 应拒绝")
	}
}

func TestWebAuthRequired(t *testing.T) {
	env := newTestEnv(t, nil)

	// 未登录（无 cookie）
	anon := &http.Client{Timeout: 5 * time.Second}
	resp, err := anon.Get(env.ts.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未登录 state 应 401，got %d", resp.StatusCode)
	}

	// 错误 token
	code, _ := env.do(t, http.MethodPost, "/api/session", map[string]string{"token": "wrong-token-wrong-token-wrong-token"}, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("错误 token 应 401，got %d", code)
	}

	// 正确状态下可见任务
	code, data := env.do(t, http.MethodGet, "/api/state", nil, "")
	if code != http.StatusOK {
		t.Fatalf("state: %d", code)
	}
	var st stateResp
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Tasks) != 3 {
		t.Fatalf("应读到 3 个任务，got %d", len(st.Tasks))
	}
}

func TestWebCSRF(t *testing.T) {
	env := newTestEnv(t, nil)

	// 缺 CSRF 头
	code, _ := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"safe"}}, "")
	if code != http.StatusForbidden {
		t.Fatalf("缺 CSRF 应 403，got %d", code)
	}

	// 外站 Origin
	req, _ := http.NewRequest(http.MethodPost, env.ts.URL+"/api/jobs",
		strings.NewReader(`{"tasks":["task_safe"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(webCSRFHeader, env.csrf)
	req.Header.Set("Origin", "http://evil.example.com")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("外站 Origin 应 403，got %d", resp.StatusCode)
	}

	// 正常写操作
	code, data := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"safe"}}, env.csrf)
	if code != http.StatusAccepted {
		t.Fatalf("正常提交应 202，got %d body=%s", code, data)
	}
	env.waitLastJob(t, "done")
}

func TestWebUnknownTask(t *testing.T) {
	env := newTestEnv(t, nil)
	code, data := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"nope"}}, env.csrf)
	if code != http.StatusBadRequest {
		t.Fatalf("未知任务应 400，got %d body=%s", code, data)
	}
	if !strings.Contains(string(data), "unknown_task") {
		t.Fatalf("错误码应为 unknown_task: %s", data)
	}
}

func TestWebDangerRequiresPreview(t *testing.T) {
	env := newTestEnv(t, nil)

	// 直接跑危险任务：需要确认
	code, data := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"danger"}}, env.csrf)
	if code != http.StatusForbidden || !strings.Contains(string(data), "confirm_required") {
		t.Fatalf("应 confirm_required，got %d %s", code, data)
	}

	// 带 confirm 但没有预览：需要预览
	code, data = env.do(t, http.MethodPost, "/api/jobs",
		map[string]any{"tasks": []string{"danger"}, "confirm": true}, env.csrf)
	if code != http.StatusForbidden || !strings.Contains(string(data), "preview_required") {
		t.Fatalf("应 preview_required，got %d %s", code, data)
	}

	// Dry-run 预览
	code, _ = env.do(t, http.MethodPost, "/api/jobs",
		map[string]any{"tasks": []string{"danger"}, "dryRun": true}, env.csrf)
	if code != http.StatusAccepted {
		t.Fatalf("dryrun 应 202，got %d", code)
	}
	last := env.waitLastJob(t, "done")
	token, _ := last["previewToken"].(string)
	if token == "" {
		t.Fatal("dry-run 成功后应返回 previewToken")
	}

	// 带预览 token 执行
	code, data = env.do(t, http.MethodPost, "/api/jobs",
		map[string]any{"tasks": []string{"danger"}, "confirm": true, "previewToken": token}, env.csrf)
	if code != http.StatusAccepted {
		t.Fatalf("带预览执行应 202，got %d %s", code, data)
	}
	env.waitLastJob(t, "done")
}

func TestWebBusyAndCancel(t *testing.T) {
	env := newTestEnv(t, nil)

	code, _ := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"slow"}}, env.csrf)
	if code != http.StatusAccepted {
		t.Fatalf("slow job 应 202，got %d", code)
	}
	// 运行中再提交 → 409
	code, data := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"safe"}}, env.csrf)
	if code != http.StatusConflict || !strings.Contains(string(data), "busy") {
		t.Fatalf("应 busy 409，got %d %s", code, data)
	}

	// 取消
	code, _ = env.do(t, http.MethodPost, "/api/jobs/current/cancel", nil, env.csrf)
	if code != http.StatusOK {
		t.Fatalf("cancel 应 200，got %d", code)
	}
	env.waitLastJob(t, "canceled")
}

func TestWebReadOnly(t *testing.T) {
	env := newTestEnv(t, func(o *webOptions) { o.ReadOnly = true })
	code, data := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"safe"}}, env.csrf)
	if code != http.StatusForbidden || !strings.Contains(string(data), "read_only") {
		t.Fatalf("只读模式应 403 read_only，got %d %s", code, data)
	}
}

func TestWebTokenFreeLoopback(t *testing.T) {
	env := newTestEnv(t, func(o *webOptions) { o.Token = "" })
	if env.csrf == "" {
		t.Fatal("token-free 模式也应返回 csrf")
	}
	code, _ := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"safe"}}, env.csrf)
	if code != http.StatusAccepted {
		t.Fatalf("token-free 提交应 202，got %d", code)
	}
}

func TestWebHealthAndStatic(t *testing.T) {
	env := newTestEnv(t, nil)
	code, data := env.do(t, http.MethodGet, "/health", nil, "")
	if code != http.StatusOK || !strings.Contains(string(data), `"status":"ok"`) {
		t.Fatalf("health 异常: %d %s", code, data)
	}
	for _, p := range []string{"/", "/app.js", "/style.css"} {
		code, data := env.do(t, http.MethodGet, p, nil, "")
		if code != http.StatusOK || len(data) == 0 {
			t.Fatalf("静态资源 %s 异常: %d", p, code)
		}
	}
}

// TestWebSSE 校验 SSE 至少收到 log 与 job_state 事件。
func TestWebSSE(t *testing.T) {
	env := newTestEnv(t, nil)

	req, err := http.NewRequest(http.MethodGet, env.ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("SSE Content-Type 异常: %s", ct)
	}

	lines := make(chan string, 512)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	// 等连接建立后再触发 job
	time.Sleep(50 * time.Millisecond)
	code, data := env.do(t, http.MethodPost, "/api/jobs", map[string]any{"tasks": []string{"safe"}}, env.csrf)
	if code != http.StatusAccepted {
		t.Fatalf("job 提交失败: %d %s", code, data)
	}

	var gotLog, gotJobState, gotTaskResult bool
	deadline := time.After(10 * time.Second)
	for !(gotLog && gotJobState && gotTaskResult) {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("SSE 流提前结束")
			}
			switch {
			case strings.HasPrefix(line, "event: log"):
				gotLog = true
			case strings.HasPrefix(line, "event: task_result"):
				gotTaskResult = true
			case strings.HasPrefix(line, "event: job_state"):
				gotJobState = true
			}
		case <-deadline:
			t.Fatalf("SSE 超时（log=%v task_result=%v job_state=%v）", gotLog, gotTaskResult, gotJobState)
		}
	}
	_ = resp.Body.Close()
	wg.Wait()
}

func TestWebMountCheck(t *testing.T) {
	env := newTestEnv(t, nil)
	code, data := env.do(t, http.MethodPost, "/api/mountcheck", nil, env.csrf)
	if code != http.StatusAccepted {
		t.Fatalf("mountcheck 应 202，got %d %s", code, data)
	}
	last := env.waitLastJob(t, "done", "failed")
	if last["kind"] != "mountcheck" {
		t.Fatalf("kind 应为 mountcheck: %v", last["kind"])
	}
}

func TestWebDecodeStrict(t *testing.T) {
	env := newTestEnv(t, nil)
	code, _ := env.do(t, http.MethodPost, "/api/jobs",
		map[string]any{"tasks": []string{"safe"}, "unknownField": 1}, env.csrf)
	if code != http.StatusBadRequest {
		t.Fatalf("未知字段应 400，got %d", code)
	}
	// 确认没有把 job 启起来
	cur, _ := env.srv.jobs.snapshots()
	if cur != nil {
		t.Fatalf("非法请求不应启动 job: %v", cur)
	}
}

func TestParseCLIWebFlags(t *testing.T) {
	oldMode, oldOpts := webMode, webOpts
	defer func() { webMode, webOpts = oldMode, oldOpts }()

	reset := func() { webMode = false; webOpts = defaultWebOptions() }

	// 只给 --web-token-file（没有 --web）也应自动进入 Web 模式
	reset()
	if _, _, err := parseCLI([]string{"rbackup-tui", "--web-token-file", "/tmp/web.token", "--web-addr", "127.0.0.1:9999"}); err != nil {
		t.Fatalf("parseCLI: %v", err)
	}
	if !webMode {
		t.Fatal("出现 --web-* 选项时应自动启用 Web 模式")
	}
	if webOpts.TokenFile != "/tmp/web.token" || webOpts.Addr != "127.0.0.1:9999" {
		t.Fatalf("web 选项解析错误: %+v", webOpts)
	}

	// 缺值必须报错（而不是静默忽略）
	reset()
	if _, _, err := parseCLI([]string{"rbackup-tui", "--web-token-file"}); err == nil {
		t.Fatal("--web-token-file 缺值应报错")
	}
	reset()
	if _, _, err := parseCLI([]string{"rbackup-tui", "--web-addr"}); err == nil {
		t.Fatal("--web-addr 缺值应报错")
	}

	// -c/-s 缺值同样报错
	reset()
	if _, _, err := parseCLI([]string{"rbackup-tui", "-c"}); err == nil {
		t.Fatal("-c 缺值应报错")
	}

	// 仅 --web 仍可用
	reset()
	if _, _, err := parseCLI([]string{"rbackup-tui", "--web"}); err != nil || !webMode {
		t.Fatalf("--web 解析失败: err=%v mode=%v", err, webMode)
	}

	// 非 web 参数不应误启用
	reset()
	if _, _, err := parseCLI([]string{"rbackup-tui", "-c", "/tmp/x.ini"}); err != nil || webMode {
		t.Fatalf("普通参数不应启用 web: err=%v mode=%v", err, webMode)
	}
}
