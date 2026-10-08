package web

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"home-broadband/internal/config"
	"home-broadband/internal/panel"
	"home-broadband/internal/tunnel"
	"home-broadband/internal/vpngate"
)

// fakePanel 是个只会列入站、给链接的假后端。
// panel.SwapPanelForTest 把 panelState.current 换成假后端，绕开真面板。
// 需要走 Manager 的测试还要再调 m.SetBackend(p) 显式注入。
type fakePanel struct {
	inbounds []tunnel.Inbound
	linkErr  error
}

func (f *fakePanel) Kind() string     { return "fake" }
func (f *fakePanel) Describe() string { return "测试用" }
func (f *fakePanel) Inbounds(live map[string]bool) ([]tunnel.Inbound, error) {
	return f.inbounds, nil
}
func (f *fakePanel) InboundDetail(id int, publicHost string) (*panel.InboundDetail, error) {
	return nil, fmt.Errorf("用不到")
}
func (f *fakePanel) InboundLinks(ids []int, publicHost string) ([]string, error) {
	if f.linkErr != nil {
		return nil, f.linkErr
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, fmt.Sprintf("vless://uuid-%d@%s:100%d?type=tcp#节点%d", id, publicHost, id, id))
	}
	return out, nil
}
func (f *fakePanel) Bind(string, string, []*tunnel.Tunnel) error           { return nil }
func (f *fakePanel) Rebind(string, *tunnel.Tunnel, []*tunnel.Tunnel) error { return nil }
func (f *fakePanel) ResyncOutbound(*tunnel.Tunnel, []*tunnel.Tunnel) error { return nil }
func (f *fakePanel) CloneToTunnels(int, []string, []*tunnel.Tunnel) ([]int, error) {
	return nil, nil
}
func (f *fakePanel) DeleteInbounds([]int, []*tunnel.Tunnel) error { return nil }
func (f *fakePanel) CreateInbound(panel.NewInboundSpec, []*tunnel.Tunnel) (*panel.CreatedInbound, error) {
	return nil, nil
}
func (f *fakePanel) UpdateInbound(int, panel.InboundPatch, []*tunnel.Tunnel) error { return nil }
func (f *fakePanel) AddClient(int, string, []*tunnel.Tunnel) error                 { return nil }
func (f *fakePanel) DeleteClient(int, string, []*tunnel.Tunnel) error              { return nil }
func (f *fakePanel) ResetClient(int, string, []*tunnel.Tunnel) error               { return nil }
func (f *fakePanel) OnTunnelsChanged([]*tunnel.Tunnel) error                       { return nil }
func (f *fakePanel) Close()                                                        {}

// usePanel 临时把后端换成假的，测完还原。
func usePanel(t *testing.T, p panel.Panel) {
	t.Helper()
	t.Cleanup(panel.SwapPanelForTest(p))
	tunnel.InvalidateInbounds()
	t.Cleanup(tunnel.InvalidateInbounds)
}

// useSettings 把设置指向一个临时目录，避免测试互相污染或写到真配置上。
func useSettings(t *testing.T) {
	t.Helper()
	restore, err := config.SwapSettingsForTest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
}

func TestSubTokenGeneratedAndPersisted(t *testing.T) {
	useSettings(t)
	tok, err := subToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != subTokenLen*2 {
		t.Fatalf("口令应是 %d 个十六进制字符，实际 %q", subTokenLen*2, tok)
	}
	// 再问一次要拿到同一串，不然已经配好的客户端会莫名失效
	again, err := subToken()
	if err != nil {
		t.Fatal(err)
	}
	if again != tok {
		t.Fatalf("口令不该每次都变: %q -> %q", tok, again)
	}
	// 落盘了才算，重启后要能读回来
	if got := config.GetWebSettings().SubToken; got != tok {
		t.Fatalf("口令没写进设置: %q", got)
	}
}

func TestResetSubTokenChangesIt(t *testing.T) {
	useSettings(t)
	old, err := subToken()
	if err != nil {
		t.Fatal(err)
	}
	neu, err := resetSubToken()
	if err != nil {
		t.Fatal(err)
	}
	if neu == old {
		t.Fatal("重置后口令应当变掉")
	}
	if got, _ := subToken(); got != neu {
		t.Fatal("重置后再读应拿到新的")
	}
}

// 订阅地址要挂在访问路径下面，那层门槛不能因为加了订阅就漏掉。
func TestSubPathUnderBasePath(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.InitBasePath(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := config.SetBasePath("myPath"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = config.SetBasePath("") })
	if got := subPath("tok123"); got != "/myPath/sub?token=tok123" {
		t.Fatalf("subPath=%q", got)
	}
}

func TestEncodeSub(t *testing.T) {
	links := []string{"vless://a", "vmess://b"}
	if got := encodeSub(links, "links"); got != "vless://a\nvmess://b" {
		t.Fatalf("明文格式不对: %q", got)
	}
	got := encodeSub(links, "base64")
	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("应当是整份 base64，解不开: %v", err)
	}
	if string(raw) != "vless://a\nvmess://b" {
		t.Fatalf("解出来是 %q", raw)
	}
}

// 口令不对就当这地址不存在，别告诉扫端口的这儿有订阅服务。
func TestHandleSubWrongTokenIs404(t *testing.T) {
	useSettings(t)
	p := &fakePanel{}
	usePanel(t, p)
	m := tunnel.NewManager(20, t.TempDir())
	m.SetBackend(p)
	h := handleSub(m)

	for _, q := range []string{"", "?token=", "?token=猜的"} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/sub"+q, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("token=%q 应当 404，实际 %d", q, rec.Code)
		}
	}
}

// 没有节点时回 404 而不是空正文：客户端拿到空订阅会把已有节点清光。
func TestHandleSubEmptyIsNotEmptyBody(t *testing.T) {
	useSettings(t)
	p := &fakePanel{}
	usePanel(t, p)
	m := tunnel.NewManager(20, t.TempDir())
	m.SetBackend(p)
	tok, _ := subToken()

	rec := httptest.NewRecorder()
	handleSub(m)(rec, httptest.NewRequest(http.MethodGet, "/sub?token="+tok, nil))
	if rec.Code == http.StatusOK {
		t.Fatal("一个节点都没有时不能回 200 空正文")
	}
}

// 正常路径：只出绑在出口上的入站，整份 base64。
func TestHandleSubOnlyBoundInbounds(t *testing.T) {
	useSettings(t)
	p := &fakePanel{inbounds: []tunnel.Inbound{
		{ID: 1, Port: 1001, Enable: true, Tag: "in-1001-tcp", BoundTo: "jp-home"},
		{ID: 2, Port: 1002, Enable: true, Tag: "in-1002-tcp"}, // 没绑出口，走直连
		{ID: 3, Port: 1003, Enable: false, Tag: "in-1003-tcp", BoundTo: "jp-home"},
	}}
	usePanel(t, p)
	m := tunnel.NewManager(20, t.TempDir())
	m.SetBackend(p)
	m.SetTunnelForTest(1, &tunnel.Tunnel{Slot: 1, Node: vpngate.Node{HostName: "jp-home"}, Status: "up"})
	tok, _ := subToken()

	rec := httptest.NewRecorder()
	handleSub(m)(rec, httptest.NewRequest(http.MethodGet, "/sub?token="+tok+"&host=1.2.3.4", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("状态 %d，正文 %s", rec.Code, rec.Body.String())
	}
	raw, err := base64.StdEncoding.DecodeString(rec.Body.String())
	if err != nil {
		t.Fatalf("默认应当是 base64: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "uuid-1@1.2.3.4") {
		t.Fatalf("少了绑在出口上的节点: %q", body)
	}
	if strings.Contains(body, "uuid-2") {
		t.Fatal("没绑出口的入站走直连，不该混进家宽订阅")
	}
	if strings.Contains(body, "uuid-3") {
		t.Fatal("停用的入站不该进订阅")
	}
	if got := rec.Header().Get("Profile-Update-Interval"); got != "12" {
		t.Fatalf("少了客户端自动更新间隔: %q", got)
	}
}

// bound=0 把直连入站也带上，留给"我就想一次拿全"的场景。
func TestHandleSubAllInbounds(t *testing.T) {
	useSettings(t)
	p := &fakePanel{inbounds: []tunnel.Inbound{
		{ID: 1, Port: 1001, Enable: true, BoundTo: "jp-home"},
		{ID: 2, Port: 1002, Enable: true},
	}}
	usePanel(t, p)
	m := tunnel.NewManager(20, t.TempDir())
	m.SetBackend(p)
	tok, _ := subToken()

	rec := httptest.NewRecorder()
	handleSub(m)(rec, httptest.NewRequest(http.MethodGet,
		"/sub?token="+tok+"&bound=0&target=links", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("状态 %d，正文 %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "uuid-1") || !strings.Contains(body, "uuid-2") {
		t.Fatalf("bound=0 应当两个都在: %q", body)
	}
	if strings.HasPrefix(body, "vless://") == false {
		t.Fatalf("target=links 应当出明文: %q", body)
	}
}

func TestHandleSubRejectsUnknownTarget(t *testing.T) {
	useSettings(t)
	p := &fakePanel{}
	usePanel(t, p)
	m := tunnel.NewManager(20, t.TempDir())
	m.SetBackend(p)
	tok, _ := subToken()

	rec := httptest.NewRecorder()
	handleSub(m)(rec, httptest.NewRequest(http.MethodGet, "/sub?token="+tok+"&target=clash", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("不认识的 target 应当 400，实际 %d", rec.Code)
	}
}

// 后端不给链接时要把原因透出来，别静默回空。
func TestHandleSubSurfacesPanelError(t *testing.T) {
	useSettings(t)
	p := &fakePanel{
		inbounds: []tunnel.Inbound{{ID: 1, Enable: true, BoundTo: "jp-home"}},
		linkErr:  fmt.Errorf("这个后端不生成链接"),
	}
	usePanel(t, p)
	m := tunnel.NewManager(20, t.TempDir())
	m.SetBackend(p)
	m.SetTunnelForTest(1, &tunnel.Tunnel{Slot: 1, Node: vpngate.Node{HostName: "jp-home"}, Status: "up"})
	tok, _ := subToken()

	rec := httptest.NewRecorder()
	handleSub(m)(rec, httptest.NewRequest(http.MethodGet, "/sub?token="+tok, nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("应当 502，实际 %d", rec.Code)
	}
	// 5xx 不透传内部细节，给通用文案
	if !strings.Contains(rec.Body.String(), "生成订阅失败") {
		t.Fatalf("应返回通用文案: %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "不生成链接") {
		t.Fatalf("内部细节不应外泄: %q", rec.Body.String())
	}
}

// 订阅必须能免登录访问，否则客户端根本拉不动。
func TestAuthLetsSubThrough(t *testing.T) {
	dir := t.TempDir()
	a, _, err := NewAuth(dir)
	if err != nil {
		t.Fatal(err)
	}
	reached := false
	h := a.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sub?token=x", nil))
	if !reached {
		t.Fatal("/sub 应当放行给 handleSub 自己验口令")
	}

	// 别的接口该拦的还得拦
	reached = false
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/exits", nil))
	if reached || rec.Code != http.StatusUnauthorized {
		t.Fatalf("其他接口仍应要求登录，实际 %d", rec.Code)
	}
}
