package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"home-broadband/internal/config"
)

// Auth 给管理界面加一层登录。
// 口令与会话存在统一存储里，首次启动自动生成口令，避免公网上裸奔。
type Auth struct {
	store *config.Store
	mu    sync.RWMutex
	// fails 按来源 IP 记录登录失败，挡低速凭据喷洒（纯内存，不落盘）
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
	s, err := config.Open(dir)
	if err != nil {
		return nil, false, err
	}
	pw := s.Password()
	created := false
	if pw == "" {
		pw, err = randomToken(9)
		if err != nil {
			return nil, false, err
		}
		if err := s.SetPassword(pw); err != nil {
			return nil, false, fmt.Errorf("写口令失败: %w", err)
		}
		created = true
	}

	return &Auth{store: s, fails: map[string]*loginFails{}}, created, nil
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
	cur := a.store.Password()
	want := sha256.Sum256([]byte(cur))
	got := sha256.Sum256([]byte(pw))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

// SetPassword 改访问口令并落盘。空口令拒绝，避免误改成无密码裸奔。
// 改完清掉所有已有会话：口令泄露后换口令能把攻击者的会话一起踢掉，
// 当前浏览器也需要重新登录。
func (a *Auth) SetPassword(pw string) error {
	pw = strings.TrimSpace(pw)
	if pw == "" {
		return fmt.Errorf("口令不能为空")
	}
	if len(pw) < 4 {
		return fmt.Errorf("口令至少 4 位")
	}
	if err := a.store.SetPassword(pw); err != nil {
		return err
	}
	return a.store.ClearSessions()
}

// Logout 销毁当前会话。
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = a.store.DeleteSession(c.Value)
	}
	// 清掉浏览器里的 cookie（Secure/SameSite 与登录时保持一致，否则严格实现可能清不掉）
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "已退出"})
}

// issue 发一个会话 token。
func (a *Auth) issue() (string, error) {
	tok, err := randomToken(16)
	if err != nil {
		return "", err
	}
	if err := a.store.AddSession(tok, time.Now().Add(sessionTTL)); err != nil {
		return "", err
	}
	return tok, nil
}

func (a *Auth) valid(tok string) bool {
	return a.store.ValidSession(tok)
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
	// 限流请求体防 DoS：登录表单 64KB 绰绰有余
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
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
		Secure:   r.TLS != nil,
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
:root{color-scheme:dark;--bg:#0e1116;--card:#161c26;--border:#242e3d;--text:#e9ecf2;--dim:#8d96a6;--accent:#10b981;--accent-d:#0b9b6c;--accent-soft:rgba(16,185,129,.13);--bad:#f87171;--shadow:0 18px 50px rgba(0,0,0,.45)}
html[data-theme="light"]{color-scheme:light;--bg:#f2f4f7;--card:#ffffff;--border:#e2e6ec;--text:#181c24;--dim:#687180;--accent:#059669;--accent-d:#047857;--accent-soft:rgba(5,150,105,.1);--bad:#dc2626;--shadow:0 18px 44px rgba(25,35,55,.14)}
@media (prefers-color-scheme:light){html:not([data-theme]){color-scheme:light;--bg:#f2f4f7;--card:#ffffff;--border:#e2e6ec;--text:#181c24;--dim:#687180;--accent:#059669;--accent-d:#047857;--accent-soft:rgba(5,150,105,.1);--bad:#dc2626;--shadow:0 18px 44px rgba(25,35,55,.14)}}
body{margin:0;min-height:100vh;display:flex;flex-direction:column;gap:20px;align-items:center;justify-content:center;
  background:var(--bg);color:var(--text);
  font:14px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif;
  -webkit-font-smoothing:antialiased}
form{background:var(--card);border:1px solid var(--border);border-radius:16px;box-shadow:var(--shadow);
  padding:30px 30px 26px;width:330px}
.brand{display:flex;align-items:center;gap:10px;margin-bottom:22px}
.logo{width:34px;height:34px;flex:none;border-radius:10px;color:#fff;display:flex;align-items:center;justify-content:center;
  background:linear-gradient(135deg,var(--accent),#0ea5e9)}
.logo svg{width:19px;height:19px;stroke:#fff;fill:none;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}
.brand b{font-size:16px}
label{display:block;color:var(--dim);font-size:12px;margin-bottom:7px;font-weight:500}
input{width:100%;box-sizing:border-box;background:var(--bg);border:1px solid var(--border);color:var(--text);
  border-radius:10px;padding:9px 12px;font:inherit}
input:focus{outline:none;border-color:var(--accent);box-shadow:0 0 0 3px var(--accent-soft)}
button{width:100%;margin-top:18px;background:var(--accent);border:0;color:#fff;font:inherit;font-weight:600;
  border-radius:10px;padding:10px;cursor:pointer;transition:background .14s}
button:hover{background:var(--accent-d)}
.err{color:var(--bad);font-size:12px;margin-top:12px;min-height:16px}
.links a{color:var(--dim);text-decoration:none;font-size:12px}
.links a:hover{color:var(--accent)}
</style>
</head>
<body>
<form id="f">
  <div class="brand">
    <span class="logo"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3c3.2 3.6 3.2 14.4 0 18"/><path d="M12 3c-3.2 3.6-3.2 14.4 0 18"/></svg></span>
    <b>home-broadband</b>
  </div>
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
