package tunnel

// Backend 是 Manager 需要的面板能力子集。
//
// panel 包的完整 Panel 接口嵌入它：tunnel 只依赖这 6 个方法，
// 不感知 3x-ui / 自建模式这些具体后端。Manager 的 backend 由外部
// （main.go、apiPanelMode）注入，后端不可用时为 nil，各调用点优雅跳过。
type Backend interface {
	// Kind 返回 "3x-ui" 或 "native"，界面据此提示当前模式。
	Kind() string
	// Describe 给出一行人能读的后端说明。
	Describe() string

	Inbounds(live map[string]bool) ([]Inbound, error)

	Rebind(oldHost string, target *Tunnel, tunnels []*Tunnel) error
	ResyncOutbound(t *Tunnel, tunnels []*Tunnel) error

	CloneToTunnels(templateID int, hosts []string, tunnels []*Tunnel) ([]int, error)

	// OnTunnelsChanged 在隧道集合变化后调用。
	//
	// 自建模式的出站完全由隧道列表推导，新开的出口必须重建配置才有对应出站；
	// 接管 3x-ui 时出站在 Bind/Clone 里顺带同步，这里是空操作，
	// 免得每开一条隧道就白重启一次面板的 Xray。
	OnTunnelsChanged(tunnels []*Tunnel) error
}
