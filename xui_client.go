package main

import (
	"encoding/json"
	"fmt"
	"io"
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
func (x *XUI) AddClient(id int, email string, tunnels []*Tunnel) error {
	raw, err := x.rawInbound(id)
	if err != nil {
		return err
	}
	proto := fmt.Sprint(raw["protocol"])
	if email == "" {
		email = fmt.Sprintf("%s-%d-%s", proto, int(toFloat(raw["port"])), randomHex(3))
	}
	return x.updateInboundRaw(id, "加客户端", func(p, raw map[string]any) error {
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
		settings["clients"] = append(clients, newClientEntry(proto, email))
		p["settings"] = mustJSON(settings)
		return nil
	})
}

// DeleteClient 摘掉入站上的一个客户端。
func (x *XUI) DeleteClient(id int, email string, tunnels []*Tunnel) error {
	return x.updateInboundRaw(id, "删客户端", func(p, raw map[string]any) error {
		settings, err := asObject(raw["settings"])
		if err != nil {
			return fmt.Errorf("解析 settings 失败: %w", err)
		}
		clients, _ := settings["clients"].([]any)
		kept := make([]any, 0, len(clients))
		for _, c := range clients {
			if cm, ok := c.(map[string]any); ok && fmt.Sprint(orEmpty(cm["email"])) == email {
				continue
			}
			kept = append(kept, c)
		}
		if len(kept) == len(clients) {
			return fmt.Errorf("客户端 %s 不存在", email)
		}
		if len(kept) == 0 {
			return fmt.Errorf("这是最后一个客户端，删掉入站就没人能连了")
		}
		settings["clients"] = kept
		p["settings"] = mustJSON(settings)
		return nil
	})
}

// ResetClient 换掉客户端凭据，已分发的旧链接随即失效。
func (x *XUI) ResetClient(id int, email string, tunnels []*Tunnel) error {
	return x.updateInboundRaw(id, "重置凭据", func(p, raw map[string]any) error {
		proto := fmt.Sprint(raw["protocol"])
		settings, err := asObject(raw["settings"])
		if err != nil {
			return fmt.Errorf("解析 settings 失败: %w", err)
		}
		clients, _ := settings["clients"].([]any)
		found := false
		for _, c := range clients {
			cm, ok := c.(map[string]any)
			if !ok || fmt.Sprint(orEmpty(cm["email"])) != email {
				continue
			}
			found = true
			if proto == "trojan" {
				cm["password"] = randomHex(8)
			} else {
				cm["id"] = newUUID()
			}
		}
		if !found {
			return fmt.Errorf("客户端 %s 不存在", email)
		}
		settings["clients"] = clients
		p["settings"] = mustJSON(settings)
		return nil
	})
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
