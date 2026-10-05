package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

type XUI struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	BasePath string `json:"base_path"`
	Scheme   string `json:"scheme"`
	token    string
	client   *http.Client
	// workDir 是 home-broadband 的工作目录，新建 TLS 入站时自签证书落在这里。
	workDir string
}

// base 返回访问面板用的前缀。
func (x *XUI) base() string {
	return fmt.Sprintf("%s://%s:%d%s", x.Scheme, x.Host, x.Port, x.BasePath)
}

func (x *XUI) Kind() string { return "3x-ui" }

func (x *XUI) Describe() string {
	return fmt.Sprintf("接管本机 3x-ui 面板（%s:%d）", x.Host, x.Port)
}

const (
	// 面板主程序，用来读设置和取 API token
	xuiBinary = "/usr/local/x-ui/x-ui"
	// 交互式管理脚本，第 11 项会打印面板的对外访问地址
	xuiMenu = "/usr/bin/x-ui"
)

// 每次调用 `x-ui setting -getApiToken` 面板都会新生成一个 token 且不回收，
// 重启多了会把面板的 api_tokens 表撑爆。所以：进程内缓存复用，
// 跨重启则把 token 落盘（<workDir>/xui-token），下次先验证旧的还能不能用，
// 能用就不再新建。
var (
	cachedToken   string
	cachedTokenMu sync.Mutex
)

// xuiTokenFile 是 token 落盘的文件名，放在 home-broadband 工作目录下。
const xuiTokenFile = "xui-token"

var (
	// 值一律限定在本行之内取：`\s*` 会跨过换行，字段为空时会把下一行的内容当成值。
	reXUIPort = regexp.MustCompile(`(?m)^port:[^\S\r\n]*(\d+)`)
	reXUIBase = regexp.MustCompile(`(?m)^webBasePath:[^\S\r\n]*(\S+)`)
	// 只认 "apiToken: xxx" 这一行，避免匹配到提示文字里的长单词
	reXUIToken = regexp.MustCompile(`(?m)^apiToken:[^\S\r\n]*([A-Za-z0-9]+)`)
	// 面板开了 TLS 时 setting -show 打印 "Panel is secure with SSL"，
	// 没开则打印 "Warning: Panel is not secure with SSL"——后者包含前者，必须先排除。
	reXUISSLOff = regexp.MustCompile(`(?i)panel is not secure with ssl`)
	reXUISSLOn  = regexp.MustCompile(`(?i)panel is secure with ssl`)
	// 兜底：证书路径非空也说明启用了 TLS
	reXUICert = regexp.MustCompile(`(?m)^cert:[^\S\r\n]*(\S+)`)
	// x-ui 菜单第 11 项打印的 Access URL，绑了域名时给的是域名而不是 IP
	reXUIAccessURL = regexp.MustCompile(`Access URL:\s*(https?)://([^:/\s]+):(\d+)(\S*)`)
	reANSI         = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// xuiSSLFromSettings 判断 `x-ui setting -show` 的输出说的是"开了 SSL"还是"没开"。
// 面板的 inbounds/clients 写接口都收 JSON 而不是表单，所以不能走 x.call。
func (x *XUI) postJSON(path string, payload any, what string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/%s", x.base(), strings.TrimPrefix(path, "/"))
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+x.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	blob, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(blob, &envelope); err != nil {
		return fmt.Errorf("解析%s响应失败: %s", what, strings.TrimSpace(string(blob)))
	}
	if !envelope.Success {
		return fmt.Errorf("%s失败: %s", what, envelope.Msg)
	}
	return nil
}

// inboundPayload 把面板返回的原始入站转成 update 接口要的形状。
// 同步过，这里再写一次只会多重启一遍面板的 Xray，把已有连接打断。
func (x *XUI) OnTunnelsChanged(tunnels []*Tunnel) error { return nil }

// Close 对 3x-ui 是空操作：Xray 由面板自己管，不该被 home-broadband 停掉。
func (x *XUI) Close() {}

// ResyncOutbound 重写某条隧道对应的出站配置。
// 用于隧道原地重连（节点名没变）后刷新端口等信息。
func (x *XUI) ResyncOutbound(t *Tunnel, tunnels []*Tunnel) error {
	setting, testURL, err := x.loadXray()
	if err != nil {
		return err
	}
	x.syncOutbounds(setting, tunnels)
	return x.saveXray(setting, testURL)
}

// forceIPv4 让直连类出站只走 IPv4。
//
// 隧道内没有 IPv6，但没被路由规则匹配上的流量会走 direct 出站直连；
// 母机有全局 IPv6 时这部分会从 IPv6 出去，暴露服务器真实地址。
func forceIPv4(outbound map[string]any) {
	if proto, _ := outbound["protocol"].(string); proto != "freedom" {
		return
	}
	settings, _ := outbound["settings"].(map[string]any)
	if settings == nil {
		settings = map[string]any{}
		outbound["settings"] = settings
	}
	settings["domainStrategy"] = "UseIPv4"
}

// xuiRunning 判断 3x-ui 服务是否在跑。
//
// Alpine 这类发行版用 OpenRC 而不是 systemd，只查 systemctl 会误判成"没装"，
// 于是装了面板也会退回自建模式，两个 Xray 抢端口。
func xuiRunning() bool {
	if exec.Command("systemctl", "is-active", "--quiet", "x-ui").Run() == nil {
		return true
	}
	if exec.Command("rc-service", "x-ui", "status").Run() == nil {
		return true
	}
	return false
}

// CreateInbound 通过面板的 inbounds/add API 新建一个入站。
//
// 走 API 而不是直接写库：面板会自己维护 tag、客户端关联和分享链接，
// home-broadband 插手它的 sqlite 只会两边打架。载荷字段照面板自己生成的入站抄，
