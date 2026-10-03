package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// webServer 管理 HTTP 监听，支持在运行时切换端口/监听地址而不重启进程。
// 切换端口或监听地址会新起一个 net.Listener，旧的优雅关闭。
type webServer struct {
	handler http.Handler

	mu   sync.Mutex
	ln   net.Listener
	srv  *http.Server
	addr string
}

func newWebServer(h http.Handler) *webServer {
	return &webServer{handler: h}
}

// serve 用当前 WebSettings 起第一个监听并阻塞。返回时说明监听彻底退出。
func (s *webServer) serve() error {
	cfg := getWebSettings()
	if err := s.reload(cfg); err != nil {
		return err
	}
	// 主 goroutine 就地阻塞，等监听被 reload 或退出替换。
	// 这里靠一个永不返回的 select 挂住：真正的 Serve 在 reload 里各自的 goroutine 跑。
	select {}
}

// reload 切换到新的监听地址。
// 地址变了：先绑新、再关旧——新地址绑不上时旧监听不受影响。
// 地址没变（比如 HTTP↔HTTPS 切换）：必须先关旧监听才能重绑同端口；
// 证书已经在 applyWebSettings 里校验过，这里 bind 失败概率极低。
func (s *webServer) reload(cfg WebSettings) error {
	addr := cfg.listenAddrString()

	bindNew := func() (net.Listener, error) {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, err
		}
		// 配了证书就包一层 TLS：面板走 HTTPS。和端口/地址一样热切换。
		if cfg.tlsEnabled() {
			cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
			if err != nil {
				_ = ln.Close()
				return nil, fmt.Errorf("证书加载失败：%v", err)
			}
			ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}})
		}
		return ln, nil
	}

	s.mu.Lock()
	oldSrv := s.srv
	oldLn := s.ln
	sameAddr := oldLn != nil && s.addr == addr
	s.mu.Unlock()

	if sameAddr {
		// 先预加载一次证书：文件被删/损坏在这里就报错，旧监听还活着。
		// （applyWebSettings 里已经校验过，这里是纵深防御。）
		if cfg.tlsEnabled() {
			if _, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey); err != nil {
				return fmt.Errorf("证书加载失败：%v", err)
			}
		}
		// 再停旧监听：不再接受新连接、端口释放；在途请求由下面的 Shutdown 优雅收尾。
		_ = oldLn.Close()
	}
	ln, err := bindNew()
	if err != nil {
		return fmt.Errorf("无法监听 %s：%w", addr, err)
	}

	s.mu.Lock()
	srv := &http.Server{Handler: s.handler}
	s.srv = srv
	s.ln = ln
	s.addr = addr
	s.mu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP 监听 %s 退出: %v", addr, err)
		}
	}()

	// 关掉旧服务。给正在处理的请求一点收尾时间，
	// 尤其是触发这次 reload 的那个请求本身要先把响应写完。
	if oldSrv != nil {
		go func() {
			time.Sleep(1 * time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = oldSrv.Shutdown(ctx)
			_ = oldLn.Close()
		}()
	}

	log.Printf("管理界面监听已切换到 %s", addr)
	return nil
}

// applyWebSettings 校验、落盘并切换监听。任一步失败都不改动线上监听。
func (s *webServer) applyWebSettings(next WebSettings) error {
	if err := validatePort(next.Port); err != nil {
		return err
	}
	norm, err := normalizeListenAddr(next.ListenAddr)
	if err != nil {
		return err
	}
	next.ListenAddr = norm

	if err := validateTLSCert(next.TLSCert, next.TLSKey); err != nil {
		return err
	}

	cur := getWebSettings()
	// 端口、监听地址和证书都没变就只需要确保已生效，避免无谓重绑
	if next.Port == cur.Port && next.ListenAddr == cur.ListenAddr &&
		next.TLSCert == cur.TLSCert && next.TLSKey == cur.TLSKey {
		return nil
	}

	if err := s.reload(next); err != nil {
		return err
	}

	webSettingsMu.Lock()
	webSettingsCur = next
	webSettingsMu.Unlock()
	if err := saveWebSettings(); err != nil {
		log.Printf("保存 Web 设置失败: %v", err)
		return err
	}
	return nil
}

// validateTLSCert 校验证书配置：要么都不填（HTTP），要么都填且能解析成证书。
func validateTLSCert(certFile, keyFile string) error {
	certFile = strings.TrimSpace(certFile)
	keyFile = strings.TrimSpace(keyFile)
	if certFile == "" && keyFile == "" {
		return nil
	}
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("证书和私钥要一起填，只填一个启用不了 HTTPS")
	}
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		return fmt.Errorf("证书加载失败：%v", err)
	}
	return nil
}

// tlsCertInfo 读证书的域名与过期时间，给设置页展示用。
// 没有配证书返回 ok=false。优先取叶子证书；自签证书没有链，
// 就直接取第一张（自签的 IsCA 为 true，不能一概跳过）。
func tlsCertInfo(certFile string) (cn string, notAfter time.Time, ok bool) {
	blob, err := os.ReadFile(certFile)
	if err != nil {
		return "", time.Time{}, false
	}
	var first *x509.Certificate
	for {
		var block *pem.Block
		block, blob = pem.Decode(blob)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if first == nil {
			first = cert
		}
		if !cert.IsCA {
			return certName(cert), cert.NotAfter, true
		}
	}
	if first != nil {
		return certName(first), first.NotAfter, true
	}
	return "", time.Time{}, false
}

func certName(cert *x509.Certificate) string {
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName
	}
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	return ""
}
