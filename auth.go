package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Auth 给管理界面加一层登录。
// 口令存在工作目录下，首次启动自动生成，避免公网上裸奔。
type Auth struct {
	dir      string
	password string
	mu       sync.RWMutex
	sessions map[string]time.Time
	// 按来源 IP 记录登录失败，挡低速凭据喷洒
	fails map[string]*loginFails
}

// loginFails 跟踪单个来源 IP 的连续失败。
type loginFails struct {
	count   int
	last    time.Time
	blocked time.Time
}

const sessionTTL = 12 * time.Hour

// 登录失败节流：同一 IP 连续错 loginMaxFails 次后，锁 loginBlockFor。
// 阈值给得宽松，正常用户偶尔输错不受影响；成功登录会清零。
const (
	loginMaxFails  = 8
	loginBlockFor  = 2 * time.Minute
	loginFailReset = 10 * time.Minute
)

// NewAuth 载入或生成访问口令。返回口令是否为本次新建。
func NewAuth(dir string) (*Auth, bool, error) {
	path := filepath.Join(dir, "password")
	created := false

	blob, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		pw, gerr := randomToken(9)
		if gerr != nil {
			return nil, false, gerr
		}
		if werr := os.WriteFile(path, []byte(pw+"\n"), 0600); werr != nil {
			return nil, false, fmt.Errorf("写口令文件失败: %w", werr)
		}
		blob = []byte(pw)
		created = true
	} else if err != nil {
		return nil, false, err
	}

	return &Auth{
		dir:      dir,
		password: strings.TrimSpace(string(blob)),
		sessions: map[string]time.Time{},
		fails:    map[string]*loginFails{},
	}, created, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// check 比对口令，用恒定时间比较避免时序泄漏。
func (a *Auth) check(pw string) bool {
	a.mu.RLock()
	cur := a.password
	a.mu.RUnlock()
	want := sha256.Sum256([]byte(cur))
	got := sha256.Sum256([]byte(pw))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

// SetPassword 改访问口令并落盘。空口令拒绝，避免误改成无密码裸奔。
// 改完不动已有会话：当前登录的浏览器不会被踢，新登录才用新口令。
func (a *Auth) SetPassword(pw string) error {
	pw = strings.TrimSpace(pw)
	if pw == "" {
		return fmt.Errorf("口令不能为空")
	}
	if len(pw) < 4 {
		return fmt.Errorf("口令至少 4 位")
	}
	path := filepath.Join(a.dir, "password")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(pw+"\n"), 0600); err != nil {
		return fmt.Errorf("写口令文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("保存口令失败: %w", err)
	}
	a.mu.Lock()
	a.password = pw
	a.mu.Unlock()
	return nil
}

// issue 发一个会话 token。
func (a *Auth) issue() (string, error) {
	tok, err := randomToken(16)
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.sessions[tok] = time.Now().Add(sessionTTL)
	// 顺手清掉过期会话
	for k, exp := range a.sessions {
		if time.Now().After(exp) {
			delete(a.sessions, k)
		}
	}
	a.mu.Unlock()
	return tok, nil
}

func (a *Auth) valid(tok string) bool {
	a.mu.RLock()
	exp, ok := a.sessions[tok]
	a.mu.RUnlock()
	return ok && time.Now().Before(exp)
}

const sessionCookie = "home-broadband_session"

// Wrap 保护一个 handler，未登录时 API 返回 401、页面跳登录。
func (a *Auth) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			a.handleLogin(w, r)
			return
		}
		// 订阅得让客户端直接拉，带不了登录态，所以这条路放行。
		// 它不是无门槛：handleSub 自己校验一串独立口令，且仍在访问路径之后。
		if r.URL.Path == "/sub" {
			next.ServeHTTP(w, r)
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil && a.valid(c.Value) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(loginHTML))
	})
}

// blocked 判断某来源 IP 是否处于登录冷却期。
func (a *Auth) blocked(ip string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	f, ok := a.fails[ip]
	return ok && time.Now().Before(f.blocked)
}

// recordFail 记一次失败，达到阈值就进入冷却。
func (a *Auth) recordFail(ip string) {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	f, ok := a.fails[ip]
	// 距上次失败太久就重新计数，避免长期累积误伤
	if !ok || (f.blocked.IsZero() && now.Sub(f.last) > loginFailReset) {
		f = &loginFails{}
		a.fails[ip] = f
	}
	f.count++
	f.last = now
	if f.count >= loginMaxFails {
		f.blocked = now.Add(loginBlockFor)
		f.count = 0
	}
	// 顺手清掉早已过期的记录，别让 map 无限增长
	for k, v := range a.fails {
		if now.Sub(v.last) > loginFailReset && now.After(v.blocked) {
			delete(a.fails, k)
		}
	}
}

// clearFails 登录成功后清掉该 IP 的失败记录。
func (a *Auth) clearFails(ip string) {
	a.mu.Lock()
	delete(a.fails, ip)
	a.mu.Unlock()
}

// clientIP 从 RemoteAddr 取来源 IP。服务直接监听公网端口、不在反代后，
// 所以不采信 X-Forwarded-For 之类可伪造的头。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *Auth) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(loginHTML))
		return
	}
	ip := clientIP(r)
	if a.blocked(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "登录失败次数过多，请稍后再试"})
		return
	}
	if !a.check(r.FormValue("password")) {
		a.recordFail(ip)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "口令不对"})
		return
	}
	a.clearFails(ip)
	tok, err := a.issue()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "已登录"})
}

const loginHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>home-broadband</title>
<script>
try{var _t=localStorage.getItem('hb-theme');if(_t==='dark'||_t==='light')document.documentElement.setAttribute('data-theme',_t);}catch(e){}
</script>
<style>
:root{color-scheme:dark;--bg:#0b0e13;--text:#e8edf4;--dim:#9aa5b4;--accent:#5aa9e6;--bad:#d1605c;--glass:rgba(22,26,34,.6);--glass-strong:rgba(22,26,34,.85);--glass-border:rgba(255,255,255,.13);--glass-shadow:0 18px 50px rgba(0,0,0,.35);--field:rgba(8,10,14,.5);--blob1:rgba(74,158,218,.22);--blob2:rgba(139,92,246,.17);--blob3:rgba(63,166,107,.13);--on-accent:#fff}
html[data-theme="light"]{color-scheme:light;--bg:#edf0f5;--text:#1d2431;--dim:#5c6879;--accent:#2b7cc2;--bad:#bd443e;--glass:rgba(255,255,255,.62);--glass-strong:rgba(255,255,255,.88);--glass-border:rgba(25,35,55,.12);--glass-shadow:0 18px 44px rgba(45,65,95,.18);--field:rgba(255,255,255,.65);--blob1:rgba(125,185,245,.42);--blob2:rgba(185,155,250,.36);--blob3:rgba(140,220,180,.36);--on-accent:#fff}
@media (prefers-color-scheme:light){html:not([data-theme]){color-scheme:light;--bg:#edf0f5;--text:#1d2431;--dim:#5c6879;--accent:#2b7cc2;--bad:#bd443e;--glass:rgba(255,255,255,.62);--glass-strong:rgba(255,255,255,.88);--glass-border:rgba(25,35,55,.12);--glass-shadow:0 18px 44px rgba(45,65,95,.18);--field:rgba(255,255,255,.65);--blob1:rgba(125,185,245,.42);--blob2:rgba(185,155,250,.36);--blob3:rgba(140,220,180,.36);--on-accent:#fff}}
body{margin:0;min-height:100vh;display:flex;flex-direction:column;gap:18px;
  align-items:center;justify-content:center;color:var(--text);
  font:13px/1.5 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;
  -webkit-font-smoothing:antialiased;
  background:radial-gradient(900px 500px at 20% 10%,var(--blob1),transparent 60%),radial-gradient(800px 500px at 80% 90%,var(--blob2),transparent 60%),radial-gradient(700px 500px at 85% 15%,var(--blob3),transparent 60%),var(--bg);
  background-attachment:fixed}
.links{display:flex;gap:16px}
.links a{color:var(--dim);text-decoration:none;font-size:12px}
.links a:hover{color:var(--accent)}
form{background:var(--glass-strong);border:1px solid var(--glass-border);border-radius:16px;
  -webkit-backdrop-filter:blur(26px) saturate(160%);backdrop-filter:blur(26px) saturate(160%);
  box-shadow:var(--glass-shadow);padding:26px 28px;width:320px}
h1{font-size:14px;font-weight:600;margin:0 0 18px;letter-spacing:.5px}
label{display:block;color:var(--dim);font-size:11px;margin-bottom:6px}
input{width:100%;box-sizing:border-box;background:var(--field);border:1px solid var(--glass-border);
  color:var(--text);border-radius:10px;padding:8px 10px;font:inherit}
input:focus{outline:none;border-color:var(--accent)}
button{width:100%;margin-top:16px;background:linear-gradient(135deg,rgba(255,255,255,.22),rgba(255,255,255,0) 45%),var(--accent);border:0;color:var(--on-accent);
  font:inherit;font-weight:600;border-radius:10px;padding:9px;cursor:pointer}
.err{color:var(--bad);font-size:11px;margin-top:10px;min-height:14px}
</style>
</head>
<body>
<form id="f">
  <h1>home-broadband</h1>
  <label for="pw">访问口令</label>
  <input type="password" id="pw" autofocus autocomplete="current-password">
  <button type="submit">进入</button>
  <div class="err" id="err"></div>
</form>
<div class="links">
  <a href="https://github.com/hao6789/Home-Broadband" target="_blank" rel="noopener">GitHub</a>
</div>
<script>
document.getElementById('f').onsubmit = async e => {
  e.preventDefault();
  const body = new URLSearchParams({password: document.getElementById('pw').value});
  const r = await fetch('login', {method:'POST', body});
  if(r.ok){ location.reload(); return; }
  const d = await r.json().catch(()=>({}));
  document.getElementById('err').textContent = d.error || '登录失败';
};
</script>
</body>
</html>`
