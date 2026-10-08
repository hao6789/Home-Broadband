package panel

import (
	"encoding/json"
	"fmt"
	"home-broadband/internal/tunnel"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// attachClients 把模板入站上的客户端挂到新建的入站，实现一套凭据走多个出口。
func (x *XUI) attachClients(emails []string, inboundID int) error {
	for _, email := range emails {
		err := x.attachOnce(email, inboundID)
		if err == nil {
			continue
		}
		// 面板在 inbounds/add 里内联建的客户端会同步进 clients 表，
		// 却不写 client_inbounds 关联。attach 看到 email 已存在但查不到关联，
		// 就当成新建去插入，撞上 clients.email 的唯一索引报 Duplicate。
		// 先调一次 update 让面板把这条记录补全，再 attach 就能成功。
		if !isDuplicateEmail(err) {
			return err
		}
		if nerr := x.normalizeClient(email); nerr != nil {
			return fmt.Errorf("挂载客户端 %s 失败: %w", email, err)
		}
		if err := x.attachOnce(email, inboundID); err != nil {
			return err
		}
	}
	return nil
}

// attachOnce 调一次面板的 attach 接口。
func (x *XUI) attachOnce(email string, inboundID int) error {
	body, err := json.Marshal(map[string]any{"inboundIds": []int{inboundID}})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/panel/api/clients/%s/attach", x.base(), url.PathEscape(email))
	raw, err := x.jsonRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}

	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("解析 attach 响应失败: %s", strings.TrimSpace(string(raw)))
	}
	if !envelope.Success {
		return fmt.Errorf("挂载客户端 %s 失败: %s", email, envelope.Msg)
	}
	return nil
}

// normalizeClient 用一次空更新让面板把老格式的客户端记录补全。
func (x *XUI) normalizeClient(email string) error {
	body, err := json.Marshal(map[string]any{"email": email})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/panel/api/clients/update/%s", x.base(), url.PathEscape(email))
	raw, err := x.jsonRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("解析 update 响应失败: %s", strings.TrimSpace(string(raw)))
	}
	if !envelope.Success {
		return fmt.Errorf("%s", envelope.Msg)
	}
	return nil
}

// jsonRequest 发一个带 JSON 体的请求并读回响应。
func (x *XUI) jsonRequest(method, endpoint string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+x.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// checkEnvelope 解析面板统一的 success/msg 信封，失败时返回带 what 前缀的错误。
func checkEnvelope(body []byte, what string) error {
	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("解析%s响应失败: %s", what, strings.TrimSpace(string(body)))
	}
	if !envelope.Success {
		return fmt.Errorf("%s失败: %s", what, envelope.Msg)
	}
	return nil
}

func isDuplicateEmail(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Duplicate email")
}

// clientEmails 取出模板入站上所有客户端的 email，用于 attach。
func clientEmails(tpl map[string]any) ([]string, error) {
	settings, err := asObject(tpl["settings"])
	if err != nil {
		return nil, fmt.Errorf("解析模板 settings 失败: %w", err)
	}
	clients, _ := settings["clients"].([]any)
	out := []string{}
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if email, _ := cm["email"].(string); email != "" {
			out = append(out, email)
		}
	}
	return out, nil
}

// AddClient 给入站加一个客户端。
// v3.9.0 起 POST /panel/api/inbounds/update 不再改动客户端，改调 clients/add。
func (x *XUI) AddClient(id int, email string, tunnels []*tunnel.Tunnel) error {
	raw, err := x.rawInbound(id)
	if err != nil {
		return err
	}
	proto := fmt.Sprint(raw["protocol"])
	if email == "" {
		email = fmt.Sprintf("%s-%d-%s", proto, int(toFloat(raw["port"])), randomHex(3))
	}
	// 预检查邮箱是否已存在，给出友好错误（v3.9.0 前这步在 updateInboundRaw 里做）
	settings, err := asObject(raw["settings"])
	if err != nil {
		return fmt.Errorf("解析 settings 失败: %w", err)
	}
	clients, _ := settings["clients"].([]any)
	for _, c := range clients {
		if cm, ok := c.(map[string]any); ok && fmt.Sprint(orEmpty(cm["email"])) == email {
			return fmt.Errorf("客户端 %s 已存在", email)
		}
	}
	client := newClientEntry(proto, email)
	body, err := json.Marshal(map[string]any{
		"client":     client,
		"inboundIds": []int{id},
	})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/panel/api/clients/add", x.base())
	respBody, err := x.jsonRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return fmt.Errorf("加客户端失败: %w", err)
	}
	return checkEnvelope(respBody, "加客户端")
}

// DeleteClient 摘掉入站上的一个客户端。
// v3.9.0 起改调 clients/del/{email}，旧 update 端点会静默忽略。
func (x *XUI) DeleteClient(id int, email string, tunnels []*tunnel.Tunnel) error {
	// 保留原来的保护：不能删最后一个客户端
	raw, err := x.rawInbound(id)
	if err != nil {
		return err
	}
	settings, err := asObject(raw["settings"])
	if err != nil {
		return fmt.Errorf("解析 settings 失败: %w", err)
	}
	clients, _ := settings["clients"].([]any)
	count := 0
	for _, c := range clients {
		if cm, ok := c.(map[string]any); ok && fmt.Sprint(orEmpty(cm["email"])) == email {
			count++
		}
	}
	if count == 0 {
		return fmt.Errorf("客户端 %s 不存在", email)
	}
	if len(clients) <= 1 {
		return fmt.Errorf("这是最后一个客户端，删掉入站就没人能连了")
	}
	endpoint := fmt.Sprintf("%s/panel/api/clients/del/%s", x.base(), url.PathEscape(email))
	respBody, err := x.jsonRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("删客户端失败: %w", err)
	}
	if err := checkEnvelope(respBody, "删客户端"); err != nil {
		return err
	}
	// 防御性复查：删后确认入站没被删空（极端并发下预检查可能失效）
	if raw2, rerr := x.rawInbound(id); rerr == nil {
		if s2, serr := asObject(raw2["settings"]); serr == nil {
			if c2, _ := s2["clients"].([]any); len(c2) == 0 {
				log.Printf("警告：入站 %d 的客户端被删空，请尽快添加新客户端", id)
			}
		}
	}
	return nil
}

// ResetClient 换掉客户端凭据，已分发的旧链接随即失效。
// v3.9.0 起改调 clients/update/{email} 发完整客户端 payload（服务端做替换）。
func (x *XUI) ResetClient(id int, email string, tunnels []*tunnel.Tunnel) error {
	raw, err := x.rawInbound(id)
	if err != nil {
		return err
	}
	proto := fmt.Sprint(raw["protocol"])
	settings, err := asObject(raw["settings"])
	if err != nil {
		return fmt.Errorf("解析 settings 失败: %w", err)
	}
	clients, _ := settings["clients"].([]any)
	var target map[string]any
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok || fmt.Sprint(orEmpty(cm["email"])) != email {
			continue
		}
		target = cm
		break
	}
	if target == nil {
		return fmt.Errorf("客户端 %s 不存在", email)
	}
	// 换掉凭据，其他字段原样保留
	if proto == "trojan" {
		target["password"] = randomHex(8)
	} else {
		target["id"] = newUUID()
	}
	body, err := json.Marshal(target)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/panel/api/clients/update/%s", x.base(), url.PathEscape(email))
	respBody, err := x.jsonRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return fmt.Errorf("重置凭据失败: %w", err)
	}
	return checkEnvelope(respBody, "重置凭据")
}

// newClientEntry 按协议造一个新客户端条目。
func newClientEntry(proto, email string) map[string]any {
	// 字段与面板自己生成的客户端保持一致：tgId 是数字（给字符串面板会直接报
	// json unmarshal 错），subId 留空由面板按需分配。
	c := map[string]any{
		"email": email, "enable": true, "comment": "", "security": "",
		"expiryTime": 0, "totalGB": 0, "limitIp": 0,
		"tgId": 0, "subId": "", "reset": 0,
	}
	if proto == "trojan" {
		c["password"] = randomHex(8)
	} else {
		c["id"] = newUUID()
	}
	if proto == "vless" {
		c["flow"] = ""
	}
	return c
}

// mustJSONField 兼容字段是对象或已编码字符串两种情况，统一输出字符串。
func mustJSONField(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return mustJSON(v)
}

// isAllDigits 判断 tag 后缀是不是纯数字（早期用槽位号命名出站留下的遗留格式）。
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// OnTunnelsChanged 对 3x-ui 是空操作：出站在 Bind/CloneToTunnels 里已经顺带
