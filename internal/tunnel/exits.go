package tunnel

import (
	"errors"
	"strings"
	"sync"
	"time"

	"home-broadband/internal/vpngate"
)

// Inbound 是面板里已有的一个入站。
type Inbound struct {
	ID       int    `json:"id"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Remark   string `json:"remark"`
	Enable   bool   `json:"enable"`
	Tag      string `json:"tag"`      // Xray 里的 inboundTag
	BoundTo  string `json:"bound_to"` // 已绑定的节点主机名，空表示未绑定
	BoundUp  bool   `json:"bound_up"` // 绑定的节点当前是否有运行中的隧道
}

// SanitizeTag 把主机名收敛成安全的 tag 片段。
func SanitizeTag(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// ExitInbound 是挂在某个出口上的一个 3x-ui 入站。
type ExitInbound struct {
	ID       int    `json:"id"`
	Port     int    `json:"port"`
	Remark   string `json:"remark"`
	Protocol string `json:"protocol"`
	Enable   bool   `json:"enable"`
	Tag      string `json:"tag"`
}

// Exit 是界面上的一行：一条隧道加上挂在它出口的所有入站。
// 用户脑子里的单位是"一个出口"，不是"一条隧道"和"一个入站"两样东西。
type Exit struct {
	Slot    int       `json:"slot"`
	Port    int       `json:"port"` // SOCKS5 端口
	Host    string    `json:"host"`
	Region  string    `json:"region"`
	Country string    `json:"country"`
	ExitIP  string    `json:"exit_ip"`
	Status  string    `json:"status"`
	Err     string    `json:"err,omitempty"`
	Since   time.Time `json:"since"`
	// SOCKS5 凭据：界面要能看、能复制、能改
	SocksUser string        `json:"socks_user"`
	SocksPass string        `json:"socks_pass"`
	Inbounds  []ExitInbound `json:"inbounds"`
}

// ExitsView 是主界面需要的全部数据。
type ExitsView struct {
	Exits []Exit `json:"exits"`
	// Direct 是没绑到任何出口的入站，仍然要能看见，否则用户会以为它们不见了
	Direct []ExitInbound `json:"direct"`
	Panel  string        `json:"panel"` // 面板不可用时的原因，空表示正常
	// Backend 是 "3x-ui" 或 "native"。界面据此决定是否提供新建入站入口：
	// 接管面板时入站归面板管，自建模式才由 home-broadband 自己建。
	Backend string `json:"backend"`
	// PanelInfo 是后端的一行说明，显示在标题旁
	PanelInfo string `json:"panel_info"`
	// PublicIP 是母机公网 IPv4，前端用它当 SOCKS5/分享链接的连接地址
	PublicIP string `json:"public_ip"`
}

// inboundCache 给入站列表做很短的缓存。界面每几秒轮询一次，
// 而每次读入站都要顺带解析一遍完整的 Xray 配置，没必要每次都真的去问面板。
type inboundCache struct {
	mu   sync.Mutex
	at   time.Time
	list []Inbound
	err  error
}

const inboundCacheTTL = 2500 * time.Millisecond

var ibCache inboundCache

func CachedInbounds(b Backend, live map[string]bool) ([]Inbound, error) {
	ibCache.mu.Lock()
	defer ibCache.mu.Unlock()
	if time.Since(ibCache.at) < inboundCacheTTL {
		return ibCache.list, ibCache.err
	}

	var list []Inbound
	var err error
	if b == nil {
		err = errors.New("节点链接后端不可用")
	} else {
		list, err = b.Inbounds(live)
	}
	ibCache.at, ibCache.list, ibCache.err = time.Now(), list, err
	return list, err
}

// InvalidateInbounds 在写操作之后调用，让下一次读立刻反映改动。
func InvalidateInbounds() {
	ibCache.mu.Lock()
	ibCache.at = time.Time{}
	ibCache.mu.Unlock()
}

// ExitsOf 把隧道和入站 join 成界面直接可用的形态。
func (m *Manager) ExitsOf() ExitsView {
	tunnels := m.Tunnels()
	view := ExitsView{Exits: make([]Exit, 0, len(tunnels)), PublicIP: HostPublicIP()}

	// 先填后端类型：入站读取失败时界面仍要知道当前是哪种模式
	if p := m.backendOf(); p != nil {
		view.Backend = p.Kind()
		view.PanelInfo = p.Describe()
	}

	live := map[string]bool{}
	for _, t := range tunnels {
		if t.Status == "up" {
			live[SanitizeTag(t.Node.HostName)] = true
		}
	}

	byHost := map[string]int{}
	for i, t := range tunnels {
		byHost[SanitizeTag(t.Node.HostName)] = i
		cred := t.credential()
		view.Exits = append(view.Exits, Exit{
			Slot: t.Slot, Port: t.Port, Host: t.Node.HostName,
			Region: t.Node.CountryCode, Country: vpngate.NodeLabel(t.Node),
			ExitIP: t.ExitIP, Status: t.Status, Err: t.Err, Since: t.Since,
			SocksUser: cred.User, SocksPass: cred.Pass,
		})
	}

	list, err := CachedInbounds(m.backendOf(), live)
	if err != nil {
		view.Panel = err.Error()
		return view
	}

	for _, ib := range list {
		row := ExitInbound{
			ID: ib.ID, Port: ib.Port, Remark: ib.Remark,
			Protocol: ib.Protocol, Enable: ib.Enable, Tag: ib.Tag,
		}
		if i, ok := byHost[ib.BoundTo]; ib.BoundTo != "" && ok {
			view.Exits[i].Inbounds = append(view.Exits[i].Inbounds, row)
			continue
		}
		view.Direct = append(view.Direct, row)
	}
	return view
}
