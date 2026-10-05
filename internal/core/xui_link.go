package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// InboundDetail 是某个入站的完整信息，用于在 home-broadband 里直接查看而不必跳到面板。
type InboundDetail struct {
	Inbound
	Clients []ClientInfo `json:"clients"`
	Links   []string     `json:"links"`
	Listen  string       `json:"listen"`
	Network string       `json:"network"`
	TLS     string       `json:"tls"`
}

type ClientInfo struct {
	Email  string `json:"email"`
	ID     string `json:"id"`
	Enable bool   `json:"enable"`
}

// InboundDetail 取一个入站的详情，含客户端与分享链接。
// 分享链接里面板会写 localhost，这里换成实际可连的地址。
func (x *XUI) InboundDetail(id int, publicHost string) (*InboundDetail, error) {
	raw, err := x.rawInbound(id)
	if err != nil {
		return nil, err
	}

	settings, err := asObject(raw["settings"])
	if err != nil {
		return nil, fmt.Errorf("解析 settings 失败: %w", err)
	}
	stream, _ := asObject(raw["streamSettings"])

	bound, err := x.boundInbounds()
	if err != nil {
		return nil, err
	}

	streamJSON, _ := json.Marshal(raw["streamSettings"])
	port := int(toFloat(raw["port"]))
	apiTag, _ := raw["tag"].(string)
	tag := resolvedInboundTag(apiTag, port, streamJSON)

	detail := &InboundDetail{
		Inbound: Inbound{
			ID:       id,
			Port:     port,
			Protocol: fmt.Sprint(raw["protocol"]),
			Remark:   fmt.Sprint(raw["remark"]),
			Enable:   raw["enable"] == true,
			Tag:      tag,
			BoundTo:  bound[tag],
		},
		Listen: fmt.Sprint(orEmpty(raw["listen"])),
	}
	if stream != nil {
		detail.Network = fmt.Sprint(orEmpty(stream["network"]))
		detail.TLS = fmt.Sprint(orEmpty(stream["security"]))
	}

	clients, _ := settings["clients"].([]any)
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		info := ClientInfo{
			Email:  fmt.Sprint(orEmpty(cm["email"])),
			ID:     fmt.Sprint(orEmpty(cm["id"])),
			Enable: cm["enable"] != false,
		}
		detail.Clients = append(detail.Clients, info)

		if links, err := x.clientLinks(info.Email); err == nil {
			for _, l := range links {
				if fixed, ok := linkForPort(l, port, publicHost); ok {
					detail.Links = append(detail.Links, fixed)
				}
			}
		}
	}
	return detail, nil
}

// linkForPort 从一批分享链接里挑出属于指定端口的那条，并把面板写的
// localhost 换成实际可连的地址。
//
// vmess 的链接是 base64 编码的 JSON（vmess://<base64>），端口和地址都在里面，
// 按 URI 形式匹配 ":端口?" 一条也筛不出来，得先解码。
func linkForPort(link string, port int, publicHost string) (string, bool) {
	if strings.HasPrefix(link, "vmess://") {
		return fixVMessLink(link, port, publicHost)
	}
	if strings.Contains(link, fmt.Sprintf(":%d?", port)) || strings.Contains(link, fmt.Sprintf(":%d#", port)) {
		return strings.Replace(link, "@localhost:", "@"+publicHost+":", 1), true
	}
	return "", false
}

// fixVMessLink 解码 vmess 链接，确认端口后把 add 换成实际地址再编码回去。
// 解不开就按原样放行：宁可给一条地址还是 localhost 的链接，也别整条丢掉。
func fixVMessLink(link string, port int, publicHost string) (string, bool) {
	payload := strings.TrimPrefix(link, "vmess://")
	blob, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		// 有的面板版本用 URI 形式的 vmess，退回通用匹配
		return "", strings.Contains(link, fmt.Sprintf(":%d?", port)) ||
			strings.Contains(link, fmt.Sprintf(":%d#", port))
	}
	var conf map[string]any
	if err := json.Unmarshal(blob, &conf); err != nil {
		return "", false
	}
	if int(toFloat(conf["port"])) != port {
		return "", false
	}
	if fmt.Sprint(orEmpty(conf["add"])) == "localhost" {
		conf["add"] = publicHost
	}
	fixed, err := json.Marshal(conf)
	if err != nil {
		return link, true
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(fixed), true
}

// clientLinks 取某个客户端在所有入站上的分享链接。
func (x *XUI) clientLinks(email string) ([]string, error) {
	obj, err := x.get("panel/api/clients/links/" + url.PathEscape(email))
	if err != nil {
		return nil, err
	}
	var links []string
	if err := json.Unmarshal(obj, &links); err != nil {
		return nil, err
	}
	return links, nil
}

// InboundLinks 批量取多个入站的分享链接，用于一次性导出。
func (x *XUI) InboundLinks(ids []int, publicHost string) ([]string, error) {
	var out []string
	for _, id := range ids {
		detail, err := x.InboundDetail(id, publicHost)
		if err != nil {
			return out, err
		}
		out = append(out, detail.Links...)
	}
	return out, nil
}

// DeleteInbounds 删除入站，并顺手清掉指向它们的 home-broadband 路由规则。
