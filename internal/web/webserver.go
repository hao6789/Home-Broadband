package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"home-broadband/internal/config"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// webServer 管理 HTTP 监听，支持在运行时切换端口/监听地址而不重启进程。
// 切换端口或监听地址会新起一个 net.Listener，旧的优雅关闭。
type webServer struct {
	handler http.Handler

	mu   sync.Mutex
	ln   net.Listener
	srv  *http.Server
	addr string
	// reloadMu 串行化 reload：防两次 ApplyWebSettings 并发时第二个关掉第一个刚绑好的监听
	reloadMu sync.Mutex
}

func NewWebServer(h http.Handler) *webServer {
	return &webServer{handler: recoverMiddleware(h)}
}

// recoverMiddleware 兜住 handler 里的 panic，记日志后回 500，
// 避免单个坏请求拖垮整个服务进程。
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("HTTP panic %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, "内部错误", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// serve 用当前 config.WebSettings 起第一个监听并阻塞。返回时说明监听彻底退出。
func (s *webServer) Serve() error {
	cfg := config.GetWebSettings()
	if err := s.reload(cfg); err != nil {
		return err
	}
	// 主 goroutine 就地阻塞，等监听被 reload 或退出替换。
	// 这里靠一个永不返回的 select 挂住：真正的 Serve 在 reload 里各自的 goroutine 跑。
	select {}
}

// reload 切换到新的监听地址。
// 地址变了：先绑新、再关旧——新地址绑不上时旧监听不受影响。
// 地址没变（比如 HTTP↔HTTPS 切换）：用 SO_REUSEPORT 先绑新监听，
// 成功后再关旧的，避免"先关后绑"之间端口被抢导致面板失联。
func (s *webServer) reload(cfg config.WebSettings) error {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	addr := cfg.ListenAddrString()

	bindNew := func(reusePort bool) (net.Listener, error) {
		var lc net.ListenConfig
		if reusePort {
			lc.Control = func(network, address string, c syscall.RawConn) error {
				var opErr error
				err := c.Control(func(fd uintptr) {
					opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
				})
				if err != nil {
					return err
				}
				return opErr
			}
		}
		ln, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			return nil, err
		}
		// 配了证书就包一层 TLS：面板走 HTTPS。和端口/地址一样热切换。
		if cfg.TlsEnabled() {
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
		// （ApplyWebSettings 里已经校验过，这里是纵深防御。）
		if cfg.TlsEnabled() {
			if _, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey); err != nil {
				return fmt.Errorf("证书加载失败：%v", err)
			}
		}
		// SO_REUSEPORT 允许新旧监听同时绑同端口：先绑新，成了再关旧
		ln, err := bindNew(true)
		if err != nil {
			return fmt.Errorf("无法监听 %s：%w", addr, err)
		}
		_ = oldLn.Close()
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
		if oldSrv != nil {
			go func() {
				time.Sleep(1 * time.Second)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = oldSrv.Shutdown(ctx)
			}()
		}
		return nil
	}

	// 初始监听也带 SO_REUSEPORT，否则同地址热重载时新监听 bind 会 EADDRINUSE
	//（Linux 要求同端口的所有 socket 都设置 SO_REUSEPORT）
	ln, err := bindNew(true)
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

// ApplyWebSettings 校验、落盘并切换监听。任一步失败都不改动线上监听。
func (s *webServer) ApplyWebSettings(next config.WebSettings) error {
	if err := config.ValidatePort(next.Port); err != nil {
		return err
	}
	norm, err := config.NormalizeListenAddr(next.ListenAddr)
	if err != nil {
		return err
	}
	next.ListenAddr = norm

	if err := validateTLSCert(next.TLSCert, next.TLSKey); err != nil {
		return err
	}

	cur := config.GetWebSettings()
	// 端口、监听地址和证书都没变就只需要确保已生效，避免无谓重绑
	if next.Port == cur.Port && next.ListenAddr == cur.ListenAddr &&
		next.TLSCert == cur.TLSCert && next.TLSKey == cur.TLSKey {
		return nil
	}

	if err := s.reload(next); err != nil {
		return err
	}

	if err := config.UpdateWebSettings(next); err != nil {
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
	kp, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("证书加载失败：%v", err)
	}
	// 检查过期：过期证书加载不报错，但客户端会拒绝，提前拦下
	if len(kp.Certificate) > 0 {
		if c, err := x509.ParseCertificate(kp.Certificate[0]); err == nil {
			if time.Now().After(c.NotAfter) {
				return fmt.Errorf("证书已于 %s 过期，请先续签", c.NotAfter.Format("2006-01-02"))
			}
		}
	}
	return nil
}

// TlsCertInfo 读证书的域名与过期时间，给设置页展示用。
// 没有配证书返回 ok=false。优先取叶子证书；自签证书没有链，
// 就直接取第一张（自签的 IsCA 为 true，不能一概跳过）。
func TlsCertInfo(certFile string) (cn string, notAfter time.Time, ok bool) {
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
