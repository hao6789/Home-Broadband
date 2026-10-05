package web

import (
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"home-broadband/internal/config"
)

func TestWebServerReloadSwitchesPort(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.LoadWebSettings(dir, 0, false); err != nil {
		t.Fatalf("LoadWebSettings: %v", err)
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	srv := NewWebServer(h)

	// 用两个系统分配的空闲端口验证切换
	p1 := freePort(t)
	if err := srv.reload(config.WebSettings{Port: p1, ListenAddr: "127.0.0.1"}); err != nil {
		t.Fatalf("reload p1: %v", err)
	}
	waitServe(t, p1)

	p2 := freePort(t)
	if err := srv.ApplyWebSettings(config.WebSettings{Port: p2, ListenAddr: "127.0.0.1"}); err != nil {
		t.Fatalf("ApplyWebSettings p2: %v", err)
	}
	waitServe(t, p2)

	// 旧端口应在优雅关闭后不再接受连接
	time.Sleep(1500 * time.Millisecond)
	if c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p1)), 300*time.Millisecond); err == nil {
		c.Close()
		t.Fatalf("旧端口 %d 切换后仍在监听", p1)
	}

	// 非法端口应被拒，且不影响现有监听
	if err := srv.ApplyWebSettings(config.WebSettings{Port: 70000, ListenAddr: "127.0.0.1"}); err == nil {
		t.Fatal("非法端口应被拒")
	}
	waitServe(t, p2)
}

func waitServe(t *testing.T, port int) {
	t.Helper()
	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/"
	for i := 0; i < 40; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("端口 %d 未在预期时间内提供服务", port)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("取空闲端口失败: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
