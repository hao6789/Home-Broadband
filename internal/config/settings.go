package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// WebSettings 是落盘的可改配置：管理界面的监听端口与地址，外加几个功能开关。
// 界面改完即时生效。访问口令与访问路径各有专门的文件（password / basepath），
// 不放这里，但都能在设置面板里改。
type WebSettings struct {
	// Port 是管理界面监听端口。
	Port int `json:"port"`
	// ListenAddr 是监听地址：空或 0.0.0.0 表示所有网卡；127.0.0.1 表示只本机。
	ListenAddr string `json:"listen_addr"`
	// ResidentialOnly 决定挑节点时是否只用志愿者家宽，跳过 vpngate 自营机房。
	// 用指针是为了区分"没配过"和"明确关掉"：老版本升上来的配置文件里没有这个
	// 字段，nil 按默认的开启处理。
	ResidentialOnly *bool `json:"residential_only,omitempty"`
	// SubToken 是订阅地址里的口令。订阅要免登录才能被客户端拉取，
	// 所以这串就是它唯一的门槛，等同于密码，不要外传。
	SubToken string `json:"sub_token,omitempty"`
	// TLSCert / TLSKey 是面板 HTTPS 的证书与私钥路径。两个都填才启用 HTTPS，
	// 只填一个不行；都不填就是普通 HTTP。
	TLSCert string `json:"tls_cert,omitempty"`
	TLSKey  string `json:"tls_key,omitempty"`
}

// TlsEnabled 两个路径都填了才算启用 HTTPS。
func (s WebSettings) TlsEnabled() bool {
	return strings.TrimSpace(s.TLSCert) != "" && strings.TrimSpace(s.TLSKey) != ""
}

// residentialOnly 返回"只用家宽"是否开启。没配过时默认开：
// home-broadband 存在的意义就是把家宽扇成出口，机房 IP 对用户没价值。
// 方法名保持小写：字段 ResidentialOnly 已经占了导出名。
func (s WebSettings) residentialOnly() bool {
	if s.ResidentialOnly == nil {
		return true
	}
	return *s.ResidentialOnly
}

// ResidentialOnly 是给业务代码用的简写，省掉每处都去取一遍设置。
func ResidentialOnly() bool { return GetWebSettings().residentialOnly() }

// SetResidentialOnly 改"只用家宽"开关并落盘。
func SetResidentialOnly(v bool) error {
	webSettingsMu.Lock()
	webSettingsCur.ResidentialOnly = &v
	webSettingsMu.Unlock()
	return SaveWebSettings()
}

var (
	webSettingsMu   sync.RWMutex
	webSettingsCur  WebSettings
	webSettingsPath string
)

func webSettingsFilePath(dir string) string { return filepath.Join(dir, "settings.json") }

// LoadWebSettings 读盘并返回当前配置。
//
// portExplicit 表示用户在命令行显式给了 -web。界面上改过端口之后会落盘，
// 之前这里一律以盘上为准，导致再带 -web 启动会被静默忽略——用户敲了参数却
// 连不上，也没有任何提示。显式指定时以命令行为准并写回，让参数说话算话。
func LoadWebSettings(dir string, defaultPort int, portExplicit bool) (WebSettings, error) {
	webSettingsPath = webSettingsFilePath(dir)

	s := WebSettings{Port: defaultPort, ListenAddr: ""}
	blob, err := os.ReadFile(webSettingsPath)
	switch {
	case os.IsNotExist(err):
		webSettingsMu.Lock()
		webSettingsCur = s
		webSettingsMu.Unlock()
		return s, SaveWebSettings()
	case err != nil:
		return s, err
	}
	if err := json.Unmarshal(blob, &s); err != nil {
		return s, err
	}
	if s.Port == 0 {
		s.Port = defaultPort
	}
	changed := false
	if portExplicit && s.Port != defaultPort {
		s.Port = defaultPort
		changed = true
	}
	webSettingsMu.Lock()
	webSettingsCur = s
	webSettingsMu.Unlock()
	if changed {
		return s, SaveWebSettings()
	}
	return s, nil
}

func GetWebSettings() WebSettings {
	webSettingsMu.RLock()
	defer webSettingsMu.RUnlock()
	return webSettingsCur
}

func SaveWebSettings() error {
	webSettingsMu.RLock()
	blob, err := json.MarshalIndent(webSettingsCur, "", "  ")
	webSettingsMu.RUnlock()
	if err != nil {
		return err
	}
	tmp := webSettingsPath + ".tmp"
	if err := os.WriteFile(tmp, blob, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, webSettingsPath)
}

// NormalizeListenAddr 把用户填的监听地址规整成合法值：空 / 0.0.0.0 / 127.0.0.1 / 具体 IP。
func NormalizeListenAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" || addr == "0.0.0.0" || strings.EqualFold(addr, "all") {
		return "", nil
	}
	if ip := net.ParseIP(addr); ip != nil {
		return addr, nil
	}
	return "", fmt.Errorf("监听地址必须是合法 IP，或留空表示所有网卡")
}

// ValidatePort 校验端口范围。
func ValidatePort(p int) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("端口必须在 1-65535 之间")
	}
	return nil
}

// ListenAddrString 拼出 net.Listen 用的地址串。
func (s WebSettings) ListenAddrString() string {
	return net.JoinHostPort(s.ListenAddr, strconv.Itoa(s.Port))
}

// UpdateWebSettings 原子替换当前 Web 设置并落盘（web 包在 ApplyWebSettings 里用）。
func UpdateWebSettings(next WebSettings) error {
	webSettingsMu.Lock()
	webSettingsCur = next
	webSettingsMu.Unlock()
	return SaveWebSettings()
}

// SetSubToken 设置订阅口令并落盘（web 包用）。
func SetSubToken(tok string) error {
	webSettingsMu.Lock()
	webSettingsCur.SubToken = tok
	webSettingsMu.Unlock()
	return SaveWebSettings()
}

// SwapSettingsForTest 仅供测试：备份当前设置与路径，换到 dir 并加载；返回还原函数。
func SwapSettingsForTest(dir string) (restore func(), err error) {
	webSettingsMu.Lock()
	prev, prevPath := webSettingsCur, webSettingsPath
	webSettingsMu.Unlock()
	if _, err := LoadWebSettings(dir, 8899, false); err != nil {
		return nil, err
	}
	return func() {
		webSettingsMu.Lock()
		webSettingsCur, webSettingsPath = prev, prevPath
		webSettingsMu.Unlock()
	}, nil
}

// SwapResidentialOnly 仅供测试：临时替换内存中的 residential_only 开关，不落盘；返回还原函数。
func SwapResidentialOnly(v *bool) (restore func()) {
	webSettingsMu.Lock()
	prev := webSettingsCur.ResidentialOnly
	webSettingsCur.ResidentialOnly = v
	webSettingsMu.Unlock()
	return func() {
		webSettingsMu.Lock()
		webSettingsCur.ResidentialOnly = prev
		webSettingsMu.Unlock()
	}
}
