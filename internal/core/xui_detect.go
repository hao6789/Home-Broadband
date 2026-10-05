package core

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"home-broadband/internal/config"
)

// 第三个返回值表示这段输出里到底有没有提到 SSL，没提到时调用方才去看证书。
func xuiSSLFromSettings(text string) (on, stated bool) {
	if reXUISSLOff.MatchString(text) {
		return false, true
	}
	if reXUISSLOn.MatchString(text) {
		return true, true
	}
	return false, false
}

// xuiCertConfigured 判断 `x-ui setting -getCert` 的输出里证书路径是否非空。
func xuiCertConfigured(text string) bool {
	return reXUICert.MatchString(text)
}

// panelAccess 从 `x-ui` 的「View Current Settings」里取面板地址。
//
// 那段逻辑已经处理好了绑定域名的情况：有证书就用证书里的域名，没有才退回公网 IP。
// 比我们自己拼 127.0.0.1 靠谱，尤其是面板启用了 TLS 时证书不会签给回环地址。
func panelAccess() (scheme, host string, ok bool) {
	cmd := exec.Command(xuiMenu)
	cmd.Stdin = strings.NewReader("11\n\n0\n")
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return "", "", false
	}
	// 输出带 ANSI 颜色码，先剥掉再匹配
	m := reXUIAccessURL.FindStringSubmatch(stripANSI(string(out)))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// stripANSI 去掉终端颜色控制码。
func stripANSI(s string) string {
	return reANSI.ReplaceAllString(s, "")
}

// DetectXUI 探测本机 3x-ui。未安装或未运行时返回错误。
// workDir 用于落盘/复用 API token；传空则退回每次新建的旧行为。
func DetectXUI(workDir string) (*XUI, error) {
	if !xuiRunning() {
		return nil, fmt.Errorf("本机未安装或未运行 3x-ui")
	}

	out, err := exec.Command(xuiBinary, "setting", "-show").Output()
	if err != nil {
		return nil, fmt.Errorf("读取面板设置失败: %w", err)
	}
	text := string(out)

	scheme := "http"
	host := "127.0.0.1"
	// 优先信 x-ui 自己给出的地址（可能是域名）
	if sc, h, ok := panelAccess(); ok {
		scheme, host = sc, h
	} else if on, stated := xuiSSLFromSettings(text); stated {
		// 面板自己说了开没开，直接采信，不必再查证书
		if on {
			scheme = "https"
		}
	} else if certOut, err := exec.Command(xuiBinary, "setting", "-getCert").Output(); err == nil {
		if xuiCertConfigured(string(certOut)) {
			scheme = "https"
		}
	}

	pm := reXUIPort.FindStringSubmatch(text)
	bm := reXUIBase.FindStringSubmatch(text)
	if pm == nil || bm == nil {
		return nil, fmt.Errorf("无法从面板设置中解析端口或路径")
	}
	var port int
	fmt.Sscanf(pm[1], "%d", &port)

	newXUI := func(token string) *XUI {
		return &XUI{
			Host:     host,
			Port:     port,
			BasePath: strings.TrimSuffix(bm[1], "/"),
			Scheme:   scheme,
			token:    token,
			client:   localClient(),
			workDir:  workDir,
		}
	}

	cachedTokenMu.Lock()
	defer cachedTokenMu.Unlock()
	if cachedToken != "" {
		return newXUI(cachedToken), nil
	}

	// 先试盘上存的 token：验证还能用就复用，避免每次启动都新建一个。
	if workDir != "" {
		if saved := readSavedToken(workDir); saved != "" {
			x := newXUI(saved)
			if x.tokenValid() {
				cachedToken = saved
				return x, nil
			}
		}
	}

	// 没有可用 token，这条命令会自动生成一个
	tokOut, err := exec.Command(xuiBinary, "setting", "-getApiToken").Output()
	if err != nil {
		return nil, fmt.Errorf("获取 API token 失败: %w", err)
	}
	tm := reXUIToken.FindStringSubmatch(string(tokOut))
	if tm == nil {
		return nil, fmt.Errorf("未能取得 API token")
	}
	token := tm[1]

	cachedToken = token
	if workDir != "" {
		saveToken(workDir, token)
	}
	return newXUI(token), nil
}

// tokenValid 用一次只读调用验证当前 token 还能用。
func (x *XUI) tokenValid() bool {
	_, err := x.get("panel/api/inbounds/list")
	return err == nil
}

// readSavedToken 读回上次落盘的 token，没有就返回空串。
func readSavedToken(workDir string) string {
	s, err := config.Open(workDir)
	if err != nil {
		return ""
	}
	return s.XUIToken()
}

// saveToken 把 token 落盘（0600），失败只记日志不阻断。
func saveToken(workDir, token string) {
	s, err := config.Open(workDir)
	if err != nil {
		log.Printf("保存 API token 失败（不影响本次运行）: %v", err)
		return
	}
	if err := s.SetXUIToken(token); err != nil {
		log.Printf("保存 API token 失败（不影响本次运行）: %v", err)
	}
}

// localClient 用于访问本机面板。
//
// 面板启用 TLS 时证书通常签给公网 IP 或域名，而我们走的是 127.0.0.1，
// 校验必然失败。这是同一台机器上的进程间调用，不经过网络，
// 没有中间人风险，所以跳过证书校验。
func localClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //#nosec G402 -- 仅用于 127.0.0.1
		},
	}
}

// post 调用面板 API。v3.5.0 里 Bearer token 只对 /panel/api/ 前缀生效。
func (x *XUI) post(path string, form url.Values) ([]byte, error) {
	return x.call(http.MethodPost, path, form)
}

// get 调用面板的只读 API。inbounds/list 是 GET。
func (x *XUI) get(path string) ([]byte, error) {
	return x.call(http.MethodGet, path, nil)
}

func (x *XUI) call(method, path string, form url.Values) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/%s", x.base(), strings.TrimPrefix(path, "/"))

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+x.token)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := x.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 %s 失败: %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("调用 %s 返回 HTTP %d", path, resp.StatusCode)
	}

	var envelope struct {
		Success bool            `json:"success"`
		Msg     string          `json:"msg"`
		Obj     json.RawMessage `json:"obj"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("解析 %s 响应失败: %w", path, err)
	}
	if !envelope.Success {
		return nil, fmt.Errorf("面板返回失败: %s", envelope.Msg)
	}
	return envelope.Obj, nil
}

// xrayConfig 是 /panel/api/xray/ 返回的结构。
