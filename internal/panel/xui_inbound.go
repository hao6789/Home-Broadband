package panel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"home-broadband/internal/tunnel"
	"home-broadband/internal/vpngate"
)

// 注意 obj 本身是一个 JSON 字符串，要二次解析。
type xrayConfig struct {
	OutboundTestURL string          `json:"outboundTestUrl"`
	XraySetting     json.RawMessage `json:"xraySetting"`
}

// loadXray 读取当前 Xray 配置模板。
func (x *XUI) loadXray() (map[string]any, string, error) {
	obj, err := x.post("panel/api/xray/", nil)
	if err != nil {
		return nil, "", err
	}

	// obj 是被再次编码成字符串的 JSON
	var inner string
	if err := json.Unmarshal(obj, &inner); err != nil {
		return nil, "", fmt.Errorf("解析 Xray 配置外层失败: %w", err)
	}
	var cfg xrayConfig
	if err := json.Unmarshal([]byte(inner), &cfg); err != nil {
		return nil, "", fmt.Errorf("解析 Xray 配置失败: %w", err)
	}

	var setting map[string]any
	if err := json.Unmarshal(cfg.XraySetting, &setting); err != nil {
		return nil, "", fmt.Errorf("解析 xraySetting 失败: %w", err)
	}
	return setting, cfg.OutboundTestURL, nil
}

// saveXray 写回 Xray 配置模板并让面板重启 Xray。
func (x *XUI) saveXray(setting map[string]any, testURL string) error {
	blob, err := json.Marshal(setting)
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("xraySetting", string(blob))
	if testURL != "" {
		form.Set("outboundTestUrl", testURL)
	}
	if _, err := x.post("panel/api/xray/update", form); err != nil {
		return err
	}

	// 只写模板不够：面板要重载 Xray 才会用新的 outbounds 与 routing 生成运行配置，
	// 否则路由改动看起来保存成功了，实际流量还按旧规则走。
	if _, err := x.post("panel/api/server/restartXrayService", nil); err != nil {
		return fmt.Errorf("配置已保存但重载 Xray 失败: %w", err)
	}
	return nil
}

// Inbounds 列出面板里已有的入站。
func (x *XUI) Inbounds(live map[string]bool) ([]tunnel.Inbound, error) {
	obj, err := x.get("panel/api/inbounds/list")
	if err != nil {
		return nil, err
	}
	var raw []struct {
		ID       int             `json:"id"`
		Port     int             `json:"port"`
		Protocol string          `json:"protocol"`
		Remark   string          `json:"remark"`
		Enable   bool            `json:"enable"`
		Tag      string          `json:"tag"`
		Stream   json.RawMessage `json:"streamSettings"`
	}
	if err := json.Unmarshal(obj, &raw); err != nil {
		return nil, fmt.Errorf("解析入站列表失败: %w", err)
	}

	bound, err := x.boundInbounds()
	if err != nil {
		return nil, err
	}

	out := make([]tunnel.Inbound, 0, len(raw))
	for _, r := range raw {
		// 3x-ui persists the authoritative Xray tag and returns it in this API.
		// Do not reconstruct it from the transport: recent 3x-ui versions may
		// keep a "-tcp" tag for WebSocket inbounds, so reconstruction produces
		// a routing rule that can never match the running Xray inbound.
		tag := resolvedInboundTag(r.Tag, r.Port, r.Stream)
		out = append(out, tunnel.Inbound{
			ID: r.ID, Port: r.Port, Protocol: r.Protocol,
			Remark: r.Remark, Enable: r.Enable,
			Tag: tag, BoundTo: bound[tag], BoundUp: live[bound[tag]],
		})
	}
	return out, nil
}

// inboundTag 复原 3x-ui 给入站生成的 Xray tag，格式是 in-<端口>-<网络>。
// streamSettings 在不同接口下有时是 JSON 对象、有时是被编码过的字符串。
func inboundTag(port int, streamSettings json.RawMessage) string {
	network := "tcp"
	if len(streamSettings) > 0 {
		raw := streamSettings
		var asString string
		if json.Unmarshal(raw, &asString) == nil {
			raw = json.RawMessage(asString)
		}
		var st struct {
			Network string `json:"network"`
		}
		if json.Unmarshal(raw, &st) == nil && st.Network != "" {
			network = st.Network
		}
	}
	return fmt.Sprintf("in-%d-%s", port, network)
}

// resolvedInboundTag uses the authoritative tag returned by 3x-ui and only
// reconstructs it for compatibility with older API responses that omit tag.
func resolvedInboundTag(apiTag string, port int, streamSettings json.RawMessage) string {
	if apiTag != "" {
		return apiTag
	}
	return inboundTag(port, streamSettings)
}

// 出站与路由规则统一带这个前缀，便于识别与清理，不碰用户手工加的条目。
const xuiTagPrefix = "home-broadband-"

// tunnelTag 用节点主机名而非槽位号做标识：槽位在 home-broadband 重启后会重新分配，
// 用它做 tag 会让已有的入站绑定悄悄串到别的节点上。
func tunnelTag(t *tunnel.Tunnel) string {
	return xuiTagPrefix + tunnel.SanitizeTag(t.Node.HostName)
}

// boundInbounds 返回 inboundTag -> 隧道槽位 的当前绑定关系。
func (x *XUI) boundInbounds() (map[string]string, error) {
	setting, _, err := x.loadXray()
	if err != nil {
		return nil, err
	}
	bound := map[string]string{}
	routing, _ := setting["routing"].(map[string]any)
	if routing == nil {
		return bound, nil
	}
	rules, _ := routing["rules"].([]any)
	for _, r := range rules {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := m["outboundTag"].(string)
		if !strings.HasPrefix(tag, xuiTagPrefix) {
			continue
		}
		host := strings.TrimPrefix(tag, xuiTagPrefix)
		// 早期版本用槽位号做 tag，这类规则在重启后必然指向错误的节点，直接忽略
		if isAllDigits(host) {
			continue
		}
		for _, it := range toStringSlice(m["inboundTag"]) {
			bound[it] = host
		}
	}
	return bound, nil
}

func toStringSlice(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// Bind 把某个入站的流量导向指定隧道。slot 传 0 表示解绑，恢复直连。
//
// 只动 home-broadband- 前缀的出站与规则，用户手工配置的条目原样保留。
func (x *XUI) Bind(inboundTag string, hostname string, tunnels []*tunnel.Tunnel) error {
	var target *tunnel.Tunnel
	if hostname != "" {
		for _, t := range tunnels {
			if t.Node.HostName == hostname {
				target = t
				break
			}
		}
		if target == nil {
			return fmt.Errorf("节点 %s 没有运行中的隧道", hostname)
		}
		if target.Status != "up" {
			return fmt.Errorf("节点 %s 的隧道还没连通（当前 %s）", hostname, target.Status)
		}
	}

	live := map[string]bool{}
	for _, t := range tunnels {
		if t.Status == "up" {
			live[tunnel.SanitizeTag(t.Node.HostName)] = true
		}
	}
	current, err := x.Inbounds(live)
	if err != nil {
		return err
	}
	knownTags := map[string]bool{}
	for _, ib := range current {
		knownTags[ib.Tag] = true
	}

	setting, testURL, err := x.loadXray()
	if err != nil {
		return err
	}

	x.syncOutbounds(setting, tunnels)

	routing, _ := setting["routing"].(map[string]any)
	if routing == nil {
		routing = map[string]any{}
	}
	rules, _ := routing["rules"].([]any)

	// 先摘掉这个入站现有的 home-broadband 绑定，再按需要重新加一条
	cleaned := make([]any, 0, len(rules)+1)
	for _, r := range rules {
		m, ok := r.(map[string]any)
		if !ok {
			cleaned = append(cleaned, r)
			continue
		}
		outTag, _ := m["outboundTag"].(string)
		if !strings.HasPrefix(outTag, xuiTagPrefix) {
			cleaned = append(cleaned, r)
			continue
		}
		// 顺便丢掉不再存在的入站标签（如换过端口后残留的旧规则）
		remain := []any{}
		for _, it := range toStringSlice(m["inboundTag"]) {
			if it != inboundTag && knownTags[it] {
				remain = append(remain, it)
			}
		}
		if len(remain) > 0 {
			m["inboundTag"] = remain
			cleaned = append(cleaned, m)
		}
	}

	if target != nil {
		cleaned = append(cleaned, map[string]any{
			"type":        "field",
			"inboundTag":  []any{inboundTag},
			"outboundTag": tunnelTag(target),
		})
	}

	routing["rules"] = cleaned
	setting["routing"] = routing
	return x.saveXray(setting, testURL)
}

// syncOutbounds 让 home-broadband- 出站与当前已连通的隧道保持一致。
func (x *XUI) syncOutbounds(setting map[string]any, tunnels []*tunnel.Tunnel) {
	outbounds, _ := setting["outbounds"].([]any)
	kept := make([]any, 0, len(outbounds))
	for _, ob := range outbounds {
		m, ok := ob.(map[string]any)
		if !ok {
			kept = append(kept, ob)
			continue
		}
		tag, _ := m["tag"].(string)
		if !strings.HasPrefix(tag, xuiTagPrefix) {
			forceIPv4(m)
			kept = append(kept, ob)
		}
	}
	for _, t := range tunnels {
		if t.Status != "up" {
			continue
		}
		kept = append(kept, map[string]any{
			"tag":      tunnelTag(t),
			"protocol": "socks",
			"settings": map[string]any{
				"servers": []any{tunnel.SocksServerJSON(t)},
			},
		})
	}
	setting["outbounds"] = kept
}

// CloneToTunnels 以某个入站为模板，为每条指定隧道复制一个入站并绑定到对应出口。
//
// 复制时必须换掉端口、备注，以及客户端的 id/email —— 这些在面板里要求唯一。
// 返回新建入站的端口列表。
func (x *XUI) CloneToTunnels(templateID int, hosts []string, tunnels []*tunnel.Tunnel) ([]int, error) {
	raw, err := x.rawInbound(templateID)
	if err != nil {
		return nil, err
	}

	byHost := map[string]*tunnel.Tunnel{}
	for _, t := range tunnels {
		byHost[t.Node.HostName] = t
	}

	used, err := x.usedPorts()
	if err != nil {
		return nil, err
	}

	emails, err := clientEmails(raw)
	if err != nil {
		return nil, err
	}

	// 别名撞了客户端会丢节点，先把面板里已有的备注收进来避让
	takenRemarks := map[string]bool{}
	if list, lerr := x.Inbounds(nil); lerr == nil {
		for _, ib := range list {
			takenRemarks[strings.TrimSpace(ib.Remark)] = true
		}
	}

	created := []int{}
	for _, host := range hosts {
		t := byHost[host]
		if t == nil || t.Status != "up" {
			continue
		}

		port, err := tunnel.FreeRandomPort(used)
		if err != nil {
			return created, err
		}
		used[port] = true

		clone, err := cloneInboundPayload(raw, port, t)
		if err != nil {
			return created, err
		}
		if remark, ok := clone["remark"].(string); ok {
			unique := vpngate.UniqueRemark(remark, takenRemarks)
			clone["remark"] = unique
			takenRemarks[unique] = true
		}
		newID, err := x.addInbound(clone)
		if err != nil {
			return created, fmt.Errorf("复制到端口 %d 失败: %w", port, err)
		}
		if len(emails) > 0 {
			if err := x.attachClients(emails, newID); err != nil {
				return created, err
			}
		}
		created = append(created, port)

		// Read the tag assigned by 3x-ui instead of guessing it from the
		// template transport. This matters for WS inbounds whose persisted tag
		// may still use the "tcp" suffix.
		newRaw, err := x.rawInbound(newID)
		if err != nil {
			return created, fmt.Errorf("读取端口 %d 的入站标签失败: %w", port, err)
		}
		newTag, _ := newRaw["tag"].(string)
		if newTag == "" {
			newTag = inboundTagOf(port, raw)
		}
		if err := x.Bind(newTag, t.Node.HostName, tunnels); err != nil {
			return created, fmt.Errorf("端口 %d 绑定失败: %w", port, err)
		}
	}
	return created, nil
}

// rawInbound 取回某个入站的原始 JSON，用作复制模板。
func (x *XUI) rawInbound(id int) (map[string]any, error) {
	obj, err := x.get("panel/api/inbounds/list")
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := json.Unmarshal(obj, &list); err != nil {
		return nil, fmt.Errorf("解析入站列表失败: %w", err)
	}
	for _, m := range list {
		if int(toFloat(m["id"])) == id {
			return m, nil
		}
	}
	return nil, fmt.Errorf("入站 %d 不存在", id)
}

func toFloat(v any) float64 {
	f, _ := v.(float64)
	return f
}

// usedPorts 收集面板里已占用的入站端口。
func (x *XUI) usedPorts() (map[int]bool, error) {
	list, err := x.Inbounds(nil)
	if err != nil {
		return nil, err
	}
	used := map[int]bool{}
	for _, ib := range list {
		used[ib.Port] = true
	}
	return used, nil
}

func inboundTagOf(port int, template map[string]any) string {
	stream, _ := json.Marshal(template["streamSettings"])
	return inboundTag(port, stream)
}

// cloneInboundPayload 按模板构造一个新入站的提交体。
func cloneInboundPayload(tpl map[string]any, port int, t *tunnel.Tunnel) (map[string]any, error) {
	settings, err := asObject(tpl["settings"])
	if err != nil {
		return nil, fmt.Errorf("解析模板 settings 失败: %w", err)
	}

	label := exitLabel(t)

	// 客户端不重新生成：建成空入站后用 attach 把模板的客户端挂过来，
	// 这样同一套 UUID 能走所有出口，客户端那边只改端口即可。
	settings["clients"] = []any{}

	stream, err := asObject(tpl["streamSettings"])
	if err != nil {
		return nil, fmt.Errorf("解析模板 streamSettings 失败: %w", err)
	}
	sniff, err := asObject(tpl["sniffing"])
	if err != nil {
		sniff = map[string]any{"enabled": true, "destOverride": []any{"http", "tls"}}
	}

	return map[string]any{
		"enable":         true,
		"remark":         label,
		"listen":         fmt.Sprint(orEmpty(tpl["listen"])),
		"port":           port,
		"protocol":       fmt.Sprint(tpl["protocol"]),
		"expiryTime":     0,
		"total":          0,
		"settings":       mustJSON(settings),
		"streamSettings": mustJSON(stream),
		"sniffing":       mustJSON(sniff),
		"allocate":       mustJSON(map[string]any{}),
	}, nil
}

// exitLabel 给复制出来的入站起个好认的名字：国旗 + 中文国名 + 出口 IP 末段。
// 同一地区可能有多条隧道，带上末段才能区分。
func exitLabel(t *tunnel.Tunnel) string {
	place := vpngate.NodeLabel(t.Node)

	suffix := t.Node.HostName
	if t.ExitIP != "" {
		if i := strings.LastIndex(t.ExitIP, "."); i >= 0 {
			suffix = t.ExitIP[i+1:]
		} else {
			suffix = t.ExitIP
		}
	}

	if place == "" {
		return suffix
	}
	// 末段留着做区分：同一个国家可能开好几条出口，名字重了客户端会认不清，
	// mihomo 那边甚至直接要求节点名唯一
	return place + " " + suffix
}

// asObject 兼容字段是对象或是被编码成字符串的两种情况。
func asObject(v any) (map[string]any, error) {
	switch t := v.(type) {
	case map[string]any:
		return t, nil
	case string:
		var m map[string]any
		if err := json.Unmarshal([]byte(t), &m); err != nil {
			return nil, err
		}
		return m, nil
	}
	return nil, fmt.Errorf("无法解析为对象")
}

func orEmpty(v any) any {
	if v == nil {
		return ""
	}
	return v
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// addInbound 通过面板 API 新建一个入站，返回新入站的 id。这个端点收 JSON 体。
func (x *XUI) addInbound(payload map[string]any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	endpoint := x.base() + "/panel/api/inbounds/add"
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+x.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			ID int `json:"id"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("解析响应失败: %s", strings.TrimSpace(string(raw)))
	}
	if !envelope.Success {
		return 0, fmt.Errorf("%s", envelope.Msg)
	}
	return envelope.Obj.ID, nil
}

// 面板的 del 只动 inbounds，残留的规则会让后续绑定读到不存在的入站标签。
func (x *XUI) DeleteInbounds(ids []int, tunnels []*tunnel.Tunnel) error {
	for _, id := range ids {
		if _, err := x.post(fmt.Sprintf("panel/api/inbounds/del/%d", id), nil); err != nil {
			return fmt.Errorf("删除入站 %d 失败: %w", id, err)
		}
	}

	setting, testURL, err := x.loadXray()
	if err != nil {
		return err
	}
	x.syncOutbounds(setting, tunnels)

	remain, err := x.Inbounds(nil)
	if err != nil {
		return err
	}
	alive := map[string]bool{}
	for _, ib := range remain {
		alive[ib.Tag] = true
	}

	routing, _ := setting["routing"].(map[string]any)
	if routing == nil {
		routing = map[string]any{}
	}
	rules, _ := routing["rules"].([]any)
	cleaned := make([]any, 0, len(rules))
	for _, r := range rules {
		m, ok := r.(map[string]any)
		if !ok {
			cleaned = append(cleaned, r)
			continue
		}
		outTag, _ := m["outboundTag"].(string)
		if !strings.HasPrefix(outTag, xuiTagPrefix) {
			cleaned = append(cleaned, r)
			continue
		}
		kept := []any{}
		for _, it := range toStringSlice(m["inboundTag"]) {
			if alive[it] {
				kept = append(kept, it)
			}
		}
		if len(kept) > 0 {
			m["inboundTag"] = kept
			cleaned = append(cleaned, m)
		}
	}
	routing["rules"] = cleaned
	setting["routing"] = routing
	return x.saveXray(setting, testURL)
}

// Rebind 把原本绑到 oldHost 的入站改绑到新节点上。
// 隧道换节点后出站 tag 会变，需要同步路由规则。
func (x *XUI) Rebind(oldHost string, target *tunnel.Tunnel, tunnels []*tunnel.Tunnel) error {
	list, err := x.Inbounds(nil)
	if err != nil {
		return err
	}
	oldTag := tunnel.SanitizeTag(oldHost)
	newLabel := exitLabel(target)
	for _, ib := range list {
		if ib.BoundTo != oldTag {
			continue
		}
		if err := x.Bind(ib.Tag, target.Node.HostName, tunnels); err != nil {
			return err
		}
		// 备注里带着旧出口的地区和 IP 尾段，换了节点要跟着改，否则名不副实
		if renamed := renameExitLabel(ib.Remark, newLabel); renamed != ib.Remark {
			if err := x.renameInbound(ib.ID, renamed); err != nil {
				return err
			}
		}
	}
	return nil
}

// renameExitLabel 在换节点之后把别名改成新出口的。
//
// 只改 home-broadband 自己起的名字（开头是国旗那种）。以前是按 "-" 切段替换末两段，
// 既会把用户自己的备注切坏，又会在反复复制时叠成 "a-JP-243-VN-165" 这种。
// 现在整体替换，用户手工改过的名字一律不碰。
func renameExitLabel(remark, newLabel string) string {
	if newLabel == "" || !vpngate.IsGeneratedLabel(remark) {
		return remark
	}
	return newLabel
}

// postJSON 向面板发一个 JSON 体的请求，并解析它统一的 success/msg 信封。
// 面板的 update 是整体覆盖，没带上的字段会被清掉，所以要原样回填。
func inboundPayload(raw map[string]any) map[string]any {
	return map[string]any{
		"enable":         raw["enable"],
		"remark":         fmt.Sprint(orEmpty(raw["remark"])),
		"listen":         fmt.Sprint(orEmpty(raw["listen"])),
		"port":           int(toFloat(raw["port"])),
		"protocol":       fmt.Sprint(raw["protocol"]),
		"expiryTime":     0,
		"total":          0,
		"settings":       mustJSONField(raw["settings"]),
		"streamSettings": mustJSONField(raw["streamSettings"]),
		"sniffing":       mustJSONField(raw["sniffing"]),
		"allocate":       mustJSON(map[string]any{}),
	}
}

// updateInboundRaw 读出入站、让 mutate 改 payload，再整体写回。
func (x *XUI) updateInboundRaw(id int, what string, mutate func(payload map[string]any, raw map[string]any) error) error {
	raw, err := x.rawInbound(id)
	if err != nil {
		return err
	}
	payload := inboundPayload(raw)
	if mutate != nil {
		if err := mutate(payload, raw); err != nil {
			return err
		}
	}
	return x.postJSON(fmt.Sprintf("panel/api/inbounds/update/%d", id), payload, what)
}

// renameInbound 只改备注，其余配置原样写回。
func (x *XUI) renameInbound(id int, remark string) error {
	return x.updateInboundRaw(id, "改名", func(p, _ map[string]any) error {
		p["remark"] = remark
		return nil
	})
}

// UpdateInbound 改端口、备注与启停，其余配置原样写回。
//
// 改端口会同时改掉 inboundTag（面板用 in-<端口>-<网络> 命名），
// 所以绑定关系要跟着迁移，否则路由规则会指向一个不存在的入站。
func (x *XUI) UpdateInbound(id int, patch InboundPatch, tunnels []*tunnel.Tunnel) error {
	if patch.Port != nil {
		used, err := x.usedPorts()
		if err != nil {
			return err
		}
		cur, err := x.rawInbound(id)
		if err != nil {
			return err
		}
		if p := *patch.Port; p != int(toFloat(cur["port"])) && used[p] {
			return fmt.Errorf("端口 %d 已被别的入站占用", p)
		}
	}

	// 改端口前记下旧 tag 与它的绑定，改完再按新 tag 绑回去
	var oldTag, boundTo string
	if patch.Port != nil {
		list, err := x.Inbounds(nil)
		if err != nil {
			return err
		}
		for _, ib := range list {
			if ib.ID == id {
				oldTag, boundTo = ib.Tag, ib.BoundTo
				break
			}
		}
	}

	if err := x.updateInboundRaw(id, "改入站", func(p, _ map[string]any) error {
		if patch.Port != nil {
			p["port"] = *patch.Port
		}
		if patch.Remark != nil {
			p["remark"] = *patch.Remark
		}
		if patch.Enable != nil {
			p["enable"] = *patch.Enable
		}
		return nil
	}); err != nil {
		return err
	}

	if oldTag == "" || boundTo == "" {
		return nil
	}
	// 端口变了 tag 也变了，把绑定迁到新 tag 上
	list, err := x.Inbounds(nil)
	if err != nil {
		return err
	}
	for _, ib := range list {
		if ib.ID != id || ib.Tag == oldTag {
			continue
		}
		var host string
		for _, t := range tunnels {
			if tunnel.SanitizeTag(t.Node.HostName) == boundTo {
				host = t.Node.HostName
				break
			}
		}
		if host == "" {
			return nil
		}
		return x.Bind(ib.Tag, host, tunnels)
	}
	return nil
}

// settings/streamSettings/sniffing 要编码成字符串，这是面板 API 的要求。
func (x *XUI) CreateInbound(spec NewInboundSpec, tunnels []*tunnel.Tunnel) (*CreatedInbound, error) {
	used, err := x.usedPorts()
	if err != nil {
		return nil, err
	}
	ns, err := normalizeInboundSpec(spec, used)
	if err != nil {
		return nil, err
	}

	// 借用自建模式那套入站描述来生成 streamSettings：TLS 自签证书、
	// REALITY 密钥的生成逻辑两种后端完全一样，没必要写第二份。
	ib := &nativeInbound{
		Port:     ns.Port,
		Protocol: ns.Protocol,
		Network:  ns.Network,
		Path:     ns.Path,
		Host:     ns.Host,
		Security: ns.Security,
		Remark:   ns.Remark,
		Enable:   true,
	}
	switch ns.Security {
	case "tls":
		conf, err := buildTLS(x.workDir, spec)
		if err != nil {
			return nil, err
		}
		ib.TLS = conf
	case "reality":
		bin, err := findXray(x.workDir)
		if err != nil {
			return nil, fmt.Errorf("REALITY 需要 xray 生成密钥: %w", err)
		}
		conf, err := buildReality(bin, spec)
		if err != nil {
			return nil, err
		}
		ib.Reality = conf
	}

	client := newClientEntry(ns.Protocol, fmt.Sprintf("%s-%d", ns.Protocol, ns.Port))
	if ns.Protocol == "vless" {
		client["flow"] = ns.Flow
	}
	settings := map[string]any{
		"clients":   []any{client},
		"fallbacks": []any{},
	}
	if ns.Protocol == "vless" {
		settings["decryption"] = "none"
	}

	payload := map[string]any{
		"enable":         true,
		"remark":         ns.Remark,
		"listen":         "",
		"port":           ns.Port,
		"protocol":       ns.Protocol,
		"expiryTime":     0,
		"total":          0,
		"settings":       mustJSON(settings),
		"streamSettings": mustJSON(xuiStreamSettings(ib)),
		"sniffing":       mustJSON(map[string]any{"enabled": true, "destOverride": []any{"http", "tls"}}),
		"allocate":       mustJSON(map[string]any{}),
	}

	id, err := x.addInbound(payload)
	if err != nil {
		return nil, fmt.Errorf("面板新建入站失败: %w", err)
	}
	return &CreatedInbound{
		ID:       id,
		Port:     ns.Port,
		Protocol: ns.Protocol,
		Remark:   ns.Remark,
		Network:  ns.Network,
		Security: ns.Security,
	}, nil
}

// xuiStreamSettings 在自建模式的 streamSettings 基础上补面板要的字段。
//
// 面板生成分享链接时要读 realitySettings.settings 里的 publicKey / fingerprint，
// Xray 自己不用这些，但缺了面板给出的链接客户端连不上。
func xuiStreamSettings(ib *nativeInbound) map[string]any {
	stream := streamSettingsJSON(ib)
	if ib.securityOrNone() == "reality" && ib.Reality != nil {
		r, _ := stream["realitySettings"].(map[string]any)
		if r != nil {
			r["show"] = false
			r["xver"] = 0
			r["settings"] = map[string]any{
				"publicKey":   ib.Reality.PublicKey,
				"fingerprint": ib.Reality.Fingerprint,
				"spiderX":     "/",
			}
		}
	}
	return stream
}
