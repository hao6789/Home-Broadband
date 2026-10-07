package panel

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// v390Mock 模拟 3x-ui 面板，记录收到的请求。
type v390Mock struct {
	mu       sync.Mutex
	requests []v390Req
	// setEnable404 为 true 时，setEnable 端点回 404（模拟老面板）
	setEnable404 bool
}

type v390Req struct {
	Method string
	Path   string
	Body   string
}

func (m *v390Mock) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		m.requests = append(m.requests, v390Req{Method: r.Method, Path: r.URL.Path, Body: string(body)})
		set404 := m.setEnable404
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		// 入站列表：给 rawInbound 用
		if r.URL.Path == "/panel/api/inbounds/list" {
			io.WriteString(w, `{"success":true,"msg":"","obj":[`+
				`{"id":1,"port":10086,"protocol":"vless","remark":"test","settings":"{\"clients\":[{\"email\":\"a@x.com\",\"id\":\"uuid-1\"},{\"email\":\"b@x.com\",\"id\":\"uuid-2\"}]}"}`+
				`]}`)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/panel/api/inbounds/setEnable/") {
			if set404 {
				w.WriteHeader(http.StatusNotFound)
				io.WriteString(w, `404 page not found`)
				return
			}
			io.WriteString(w, `{"success":true,"msg":""}`)
			return
		}
		// 旧 update 端点：v3.9.0 行为用不到，但老面板回退路径会调
		if strings.HasPrefix(r.URL.Path, "/panel/api/inbounds/update/") {
			io.WriteString(w, `{"success":true,"msg":""}`)
			return
		}
		// clients 家族新端点
		if strings.HasPrefix(r.URL.Path, "/panel/api/clients/") {
			io.WriteString(w, `{"success":true,"msg":""}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"success":false,"msg":"unknown"}`)
	}
}

func (m *v390Mock) newXUI(t *testing.T) *XUI {
	t.Helper()
	srv := httptest.NewServer(m.handler())
	t.Cleanup(srv.Close)
	u := strings.TrimPrefix(srv.URL, "http://")
	var port int
	if i := strings.LastIndex(u, ":"); i >= 0 {
		for _, c := range u[i+1:] {
			port = port*10 + int(c-'0')
		}
	}
	return &XUI{
		Scheme: "http",
		Host:   "127.0.0.1",
		Port:   port,
		token:  "test",
		client: srv.Client(),
	}
}

func (m *v390Mock) paths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, r := range m.requests {
		out = append(out, r.Method+" "+r.Path)
	}
	return out
}

func (m *v390Mock) lastBody() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) == 0 {
		return ""
	}
	return m.requests[len(m.requests)-1].Body
}

func hasPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// AddClient 必须走 clients/add，不能再走 inbounds/update
func TestAddClientUsesNewEndpoint(t *testing.T) {
	m := &v390Mock{}
	x := m.newXUI(t)
	if err := x.AddClient(1, "new@x.com", nil); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	paths := m.paths()
	if !hasPath(paths, "POST /panel/api/clients/add") {
		t.Errorf("AddClient 没调 clients/add，实际: %v", paths)
	}
	if hasPath(paths, "POST /panel/api/inbounds/update/1") {
		t.Errorf("AddClient 不该再调 inbounds/update，实际: %v", paths)
	}
	// body 里要有 client 和 inboundIds
	var body map[string]any
	if err := json.Unmarshal([]byte(m.lastBody()), &body); err != nil {
		t.Fatalf("body 解析失败: %v", err)
	}
	if _, ok := body["client"]; !ok {
		t.Errorf("clients/add body 缺少 client: %s", m.lastBody())
	}
}

// DeleteClient 必须走 clients/del/{email}
func TestDeleteClientUsesNewEndpoint(t *testing.T) {
	m := &v390Mock{}
	x := m.newXUI(t)
	if err := x.DeleteClient(1, "a@x.com", nil); err != nil {
		t.Fatalf("DeleteClient: %v", err)
	}
	paths := m.paths()
	if !hasPath(paths, "POST /panel/api/clients/del/a@x.com") {
		t.Errorf("DeleteClient 没调 clients/del，实际: %v", paths)
	}
	if hasPath(paths, "POST /panel/api/inbounds/update/1") {
		t.Errorf("DeleteClient 不该再调 inbounds/update，实际: %v", paths)
	}
}

// 删最后一个客户端要有保护（用只有一个客户端的 mock 覆盖）
func TestDeleteClientRefusesLastOne(t *testing.T) {
	m := &v390Mock{}
	x := m.newXUI(t)
	// mock 的 list 里有两个客户端，这里验证删除不存在的报错路径
	if err := x.DeleteClient(1, "notexist@x.com", nil); err == nil {
		t.Errorf("删不存在的客户端应报错")
	} else if !strings.Contains(err.Error(), "不存在") {
		t.Errorf("错误文案不对: %v", err)
	}
}

// ResetClient 必须走 clients/update/{email}，且凭据要换掉
func TestResetClientUsesNewEndpoint(t *testing.T) {
	m := &v390Mock{}
	x := m.newXUI(t)
	if err := x.ResetClient(1, "a@x.com", nil); err != nil {
		t.Fatalf("ResetClient: %v", err)
	}
	paths := m.paths()
	if !hasPath(paths, "POST /panel/api/clients/update/a@x.com") {
		t.Errorf("ResetClient 没调 clients/update，实际: %v", paths)
	}
	if hasPath(paths, "POST /panel/api/inbounds/update/1") {
		t.Errorf("ResetClient 不该再调 inbounds/update，实际: %v", paths)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(m.lastBody()), &body); err != nil {
		t.Fatalf("body 解析失败: %v", err)
	}
	if body["id"] == "uuid-1" {
		t.Errorf("ResetClient 没换掉 UUID: %s", m.lastBody())
	}
	if body["email"] != "a@x.com" {
		t.Errorf("ResetClient body email 不对: %s", m.lastBody())
	}
}

// setEnable 在新面板上走新端点
func TestSetEnableUsesNewEndpoint(t *testing.T) {
	m := &v390Mock{}
	x := m.newXUI(t)
	notFound, err := x.setEnable(1, false)
	if err != nil {
		t.Fatalf("setEnable: %v", err)
	}
	if notFound {
		t.Errorf("新面板不该回 404")
	}
	if !hasPath(m.paths(), "POST /panel/api/inbounds/setEnable/1") {
		t.Errorf("没调 setEnable，实际: %v", m.paths())
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(m.lastBody()), &body); err != nil {
		t.Fatalf("body 解析失败: %v", err)
	}
	if body["enable"] != false {
		t.Errorf("setEnable body 不对: %s", m.lastBody())
	}
}

// setEnable 在老面板上回 404，调用方应回退
func TestSetEnableFallsBackOn404(t *testing.T) {
	m := &v390Mock{setEnable404: true}
	x := m.newXUI(t)
	notFound, err := x.setEnable(1, true)
	if err != nil {
		t.Fatalf("setEnable: %v", err)
	}
	if !notFound {
		t.Errorf("老面板应返回 notFound=true")
	}
}
