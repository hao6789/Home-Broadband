package panel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// v390Full 模拟 3x-ui v3.9.0 的真实行为：
// - 维护 inbounds 表（含 clients）
// - inbounds/update 按 v3.9.0 语义：改端口/备注，但忽略 clients 和 enable
// - clients/* 和 setEnable 按新语义工作
type v390Full struct {
	mu       sync.Mutex
	inbounds map[int]*v390Inbound
}

type v390Inbound struct {
	ID       int
	Port     int
	Protocol string
	Remark   string
	Enable   bool
	Clients  []map[string]any
}

func newV390Full() *v390Full {
	return &v390Full{
		inbounds: map[int]*v390Inbound{
			1: {
				ID: 1, Port: 10086, Protocol: "vless", Remark: "test", Enable: true,
				Clients: []map[string]any{
					{"email": "a@x.com", "id": "uuid-1", "enable": true},
					{"email": "b@x.com", "id": "uuid-2", "enable": true},
				},
			},
		},
	}
}

func (m *v390Full) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (m *v390Full) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		p := r.URL.Path
		m.mu.Lock()
		defer m.mu.Unlock()

		ok := func() { m.writeJSON(w, map[string]any{"success": true, "msg": ""}) }
		fail := func(msg string) { m.writeJSON(w, map[string]any{"success": false, "msg": msg}) }

		switch {
		case p == "/panel/api/xray/":
			inner, _ := json.Marshal(map[string]any{
				"outboundTestUrl": "http://example.com",
				"xraySetting":     map[string]any{"routing": map[string]any{"rules": []any{}}},
			})
			innerStr, _ := json.Marshal(string(inner))
			m.writeJSON(w, map[string]any{"success": true, "msg": "", "obj": json.RawMessage(innerStr)})
			return

		case p == "/panel/api/inbounds/list":
			var obj []map[string]any
			for _, ib := range m.inbounds {
				clientsJSON, _ := json.Marshal(map[string]any{"clients": ib.Clients})
				obj = append(obj, map[string]any{
					"id": ib.ID, "port": ib.Port, "protocol": ib.Protocol,
					"remark": ib.Remark, "enable": ib.Enable,
					"settings": string(clientsJSON),
				})
			}
			m.writeJSON(w, map[string]any{"success": true, "obj": obj})
			return

		case strings.HasPrefix(p, "/panel/api/clients/add"):
			var req struct {
				Client     map[string]any `json:"client"`
				InboundIds []int          `json:"inboundIds"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				fail("bad body")
				return
			}
			email := fmt.Sprint(req.Client["email"])
			for _, id := range req.InboundIds {
				ib := m.inbounds[id]
				if ib == nil {
					fail("inbound not found")
					return
				}
				for _, c := range ib.Clients {
					if fmt.Sprint(c["email"]) == email {
						fail("Duplicate email")
						return
					}
				}
				ib.Clients = append(ib.Clients, req.Client)
			}
			ok()
			return

		case strings.HasPrefix(p, "/panel/api/clients/del/"):
			email := strings.TrimPrefix(p, "/panel/api/clients/del/")
			found := false
			for _, ib := range m.inbounds {
				kept := ib.Clients[:0]
				for _, c := range ib.Clients {
					if fmt.Sprint(c["email"]) == email {
						found = true
						continue
					}
					kept = append(kept, c)
				}
				ib.Clients = kept
			}
			if !found {
				fail("client not found")
				return
			}
			ok()
			return

		case strings.HasPrefix(p, "/panel/api/clients/update/"):
			email := strings.TrimPrefix(p, "/panel/api/clients/update/")
			var client map[string]any
			if err := json.Unmarshal(body, &client); err != nil {
				fail("bad body")
				return
			}
			found := false
			for _, ib := range m.inbounds {
				for i, c := range ib.Clients {
					if fmt.Sprint(c["email"]) == email {
						ib.Clients[i] = client
						found = true
					}
				}
			}
			if !found {
				fail("client not found")
				return
			}
			ok()
			return

		case strings.HasPrefix(p, "/panel/api/inbounds/setEnable/"):
			var id int
			fmt.Sscanf(strings.TrimPrefix(p, "/panel/api/inbounds/setEnable/"), "%d", &id)
			var req struct {
				Enable bool `json:"enable"`
			}
			json.Unmarshal(body, &req)
			ib := m.inbounds[id]
			if ib == nil {
				fail("inbound not found")
				return
			}
			ib.Enable = req.Enable
			ok()
			return

		case strings.HasPrefix(p, "/panel/api/inbounds/update/"):
			// v3.9.0 语义：只改端口/备注，忽略 clients 和 enable
			var id int
			fmt.Sscanf(strings.TrimPrefix(p, "/panel/api/inbounds/update/"), "%d", &id)
			var req map[string]any
			json.Unmarshal(body, &req)
			ib := m.inbounds[id]
			if ib == nil {
				fail("inbound not found")
				return
			}
			if port, ok := req["port"].(float64); ok {
				ib.Port = int(port)
			}
			if remark, ok := req["remark"].(string); ok {
				ib.Remark = remark
			}
			// 注意：故意忽略 req["enable"] 和 req["settings"]，模拟 v3.9.0
			ok()
			return
		}
		w.WriteHeader(404)
	}
}

func (m *v390Full) newXUI(t *testing.T) *XUI {
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
	return &XUI{Scheme: "http", Host: "127.0.0.1", Port: port, token: "test", client: srv.Client()}
}

// 端到端：加客户端后 mock 状态里真多了一条
func TestE2EAddClient(t *testing.T) {
	m := newV390Full()
	x := m.newXUI(t)
	if err := x.AddClient(1, "new@x.com", nil); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.inbounds[1].Clients) != 3 {
		t.Fatalf("客户端数量不对: %d", len(m.inbounds[1].Clients))
	}
	found := false
	for _, c := range m.inbounds[1].Clients {
		if fmt.Sprint(c["email"]) == "new@x.com" {
			found = true
		}
	}
	if !found {
		t.Errorf("new@x.com 没加上")
	}
}

// 端到端：删客户端后真没了
func TestE2EDeleteClient(t *testing.T) {
	m := newV390Full()
	x := m.newXUI(t)
	if err := x.DeleteClient(1, "a@x.com", nil); err != nil {
		t.Fatalf("DeleteClient: %v", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.inbounds[1].Clients {
		if fmt.Sprint(c["email"]) == "a@x.com" {
			t.Errorf("a@x.com 还在")
		}
	}
	if len(m.inbounds[1].Clients) != 1 {
		t.Errorf("数量不对: %d", len(m.inbounds[1].Clients))
	}
}

// 端到端：重置凭据后 UUID 真换了
func TestE2EResetClient(t *testing.T) {
	m := newV390Full()
	x := m.newXUI(t)
	if err := x.ResetClient(1, "a@x.com", nil); err != nil {
		t.Fatalf("ResetClient: %v", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.inbounds[1].Clients {
		if fmt.Sprint(c["email"]) == "a@x.com" {
			if fmt.Sprint(c["id"]) == "uuid-1" {
				t.Errorf("UUID 没换")
			}
		}
	}
}

// 端到端：启停开关真生效（走 setEnable，不走 update）
func TestE2ESetEnable(t *testing.T) {
	m := newV390Full()
	x := m.newXUI(t)
	if err := x.UpdateInbound(1, InboundPatch{Enable: boolPtr(false)}, nil); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inbounds[1].Enable {
		t.Errorf("enable 没关掉")
	}
}

// 端到端：改端口走 update 生效
func TestE2EUpdatePort(t *testing.T) {
	m := newV390Full()
	x := m.newXUI(t)
	if err := x.UpdateInbound(1, InboundPatch{Port: intPtr(20086)}, nil); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inbounds[1].Port != 20086 {
		t.Errorf("端口没改: %d", m.inbounds[1].Port)
	}
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int   { return &i }
