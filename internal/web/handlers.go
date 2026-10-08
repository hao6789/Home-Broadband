package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"home-broadband/internal/config"
	"home-broadband/internal/panel"
	"home-broadband/internal/tunnel"
	"home-broadband/internal/vpngate"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// requirePost 拒绝非 POST 请求，防 CSRF（SameSite=Lax 挡不住顶层导航 GET）。
func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "请用 POST 请求"})
		return false
	}
	return true
}

// writeServerError 记详细日志，给前端返回通用文案，避免内部细节外泄。
func writeServerError(w http.ResponseWriter, code int, err error, publicMsg string) {
	log.Printf("API %d: %v", code, err)
	writeJSON(w, code, map[string]string{"error": publicMsg})
}

func apiNodes(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes, fetched := m.Nodes()
		total := len(nodes)
		// 默认跟挑节点的口径一致：开了"只用家宽"就不列机房节点。
		// 带 all=1 能看到完整列表，用来确认过滤掉了多少。
		if config.ResidentialOnly() && r.URL.Query().Get("all") != "1" {
			kept := make([]vpngate.Node, 0, len(nodes))
			for _, n := range nodes {
				if n.Residential {
					kept = append(kept, n)
				}
			}
			nodes = kept
		}
		shown := len(nodes)
		if len(nodes) > 200 {
			nodes = nodes[:200]
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"nodes":            nodes,
			"fetched":          fetched,
			"total":            total,
			"available":        shown,
			"residential_only": config.ResidentialOnly(),
		})
	}
}

func apiTunnels(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, m.Tunnels())
	}
}

func apiStart(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		host := r.URL.Query().Get("host")
		if host == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 host 参数"})
			return
		}
		nodes, _ := m.Nodes()
		for _, n := range nodes {
			if n.HostName == host {
				t, err := m.Start(n)
				if err != nil {
					writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, t)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "节点不存在，可能列表已过期"})
	}
}

func apiStop(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		slot, err := strconv.Atoi(r.URL.Query().Get("slot"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slot 参数无效"})
			return
		}
		if err := m.Stop(slot); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "已停止"})
	}
}

func apiRefresh(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		n, err := m.RefreshNodes()
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"count": n})
	}
}

// apiSwap 就地把一个出口换到别的节点，端口不变。
func apiSwap(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		slot, err := strconv.Atoi(r.URL.Query().Get("slot"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slot 参数无效"})
			return
		}
		if err := m.Swap(slot); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "正在换节点"})
	}
}

// apiRegions 给新建向导用：各地区还剩多少空闲节点。
func apiRegions(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, m.Regions())
	}
}

// apiCred 改一个出口的 SOCKS5 用户名口令。两个参数都留空表示随机重置。
func apiCred(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		q := r.URL.Query()
		slot, err := strconv.Atoi(q.Get("slot"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slot 参数无效"})
			return
		}
		cred, err := m.SetCred(slot, tunnel.SocksCred{
			User: strings.TrimSpace(q.Get("user")),
			Pass: strings.TrimSpace(q.Get("pass")),
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"user": cred.User,
			"pass": cred.Pass,
		})
	}
}

// apiSettings 管理界面自身的设置：改密码 / 改路径 / 改端口 / 改本地监听。
// GET 返回当前值（不含明文口令）；POST 按传入的字段逐项应用，任一项失败即整体回报。
func apiSettings(auth *Auth, srv *webServer) http.HandlerFunc {
	type settingsReq struct {
		Password        *string `json:"password"`         // 非空则改口令
		BasePath        *string `json:"base_path"`        // 提供即改访问路径（空串=去掉前缀）
		Port            *int    `json:"port"`             // 提供即改监听端口
		ListenAddr      *string `json:"listen_addr"`      // 提供即改监听地址
		ResidentialOnly *bool   `json:"residential_only"` // 提供即改"只用家宽"
		TLSCert         *string `json:"tls_cert"`         // 提供即改证书路径（空串=关 HTTPS）
		TLSKey          *string `json:"tls_key"`          // 提供即改私钥路径（空串=关 HTTPS）
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			var in settingsReq
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
				return
			}

			// 改口令
			if in.Password != nil && *in.Password != "" {
				if err := auth.SetPassword(*in.Password); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
			}
			// 改访问路径
			if in.BasePath != nil {
				if _, err := config.SetBasePath(*in.BasePath); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
			}
			// 改"只用家宽"。放在改端口之前：ApplyWebSettings 会整份覆盖设置，
			// 顺序颠倒会把这个开关写回旧值。
			if in.ResidentialOnly != nil {
				if err := config.SetResidentialOnly(*in.ResidentialOnly); err != nil {
					writeServerError(w, http.StatusInternalServerError, err, "服务器内部错误")
					return
				}
			}
			// 改端口 / 监听地址 / 证书：合成一份新的 config.WebSettings 一起应用，避免绑两次
			if in.Port != nil || in.ListenAddr != nil || in.TLSCert != nil || in.TLSKey != nil {
				next := config.GetWebSettings()
				if in.Port != nil {
					next.Port = *in.Port
				}
				if in.ListenAddr != nil {
					next.ListenAddr = *in.ListenAddr
				}
				if in.TLSCert != nil {
					next.TLSCert = strings.TrimSpace(*in.TLSCert)
				}
				if in.TLSKey != nil {
					next.TLSKey = strings.TrimSpace(*in.TLSKey)
				}
				if err := srv.ApplyWebSettings(next); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
			}
		}

		cfg := config.GetWebSettings()
		listen := cfg.ListenAddr
		if listen == "" {
			listen = "0.0.0.0"
		}
		var tlsInfo map[string]any
		if cfg.TlsEnabled() {
			if cn, notAfter, ok := TlsCertInfo(cfg.TLSCert); ok {
				days := int(time.Until(notAfter).Hours() / 24)
				tlsInfo = map[string]any{
					"cn":        cn,
					"not_after": notAfter.Format("2006-01-02"),
					"days_left": days,
				}
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"base_path":        config.CurrentBasePath(),
			"port":             cfg.Port,
			"listen_addr":      listen,
			"has_password":     true,
			"residential_only": config.ResidentialOnly(),
			"tls_cert":         cfg.TLSCert,
			"tls_key":          cfg.TLSKey,
			"tls_info":         tlsInfo,
			"version":          Version,
		})
	}
}

// apiUpdateCheck 问 GitHub 最新 release，回报当前/最新版本与更新内容。
func apiUpdateCheck(w http.ResponseWriter, r *http.Request) {
	st, err := checkUpdate()
	if err != nil {
		writeServerError(w, http.StatusBadGateway, err, "检查更新失败，请稍后重试")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// apiUpdateApply 下载最新版替换二进制并重启服务。成功后进程会被拉起成新版本。
func apiUpdateApply(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	st, err := checkUpdate()
	if err != nil {
		writeServerError(w, http.StatusBadGateway, err, "检查更新失败，请稍后重试")
		return
	}
	if !st.HasUpdate {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restarting": false, "message": "已经是最新版"})
		return
	}
	// 把 checkUpdate 拿到的 release 传进去，避免 applyUpdate 再调一次 API
	//（省配额，也避免两次查询之间 release 变化的 TOCTOU）
	if err := applyUpdateWithRelease(st.Release); err != nil {
		writeServerError(w, http.StatusInternalServerError, err, "服务器内部错误")
		return
	}
	// 先把响应发回去，restartSelf 已排在延迟后触发
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restarting": true, "latest": st.Latest})
}

// apiRestart 重启面板服务：先把响应发回去，再延迟触发 restartSelf，
// 复用更新流程的重启路径（systemd/openrc/自我 exec）。
func apiRestart(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "正在重启"})
	go func() {
		time.Sleep(800 * time.Millisecond)
		restartSelf()
	}()
}

// apiExits 返回主界面需要的一切：出口以及挂在它上面的入站。
func apiExits(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, m.ExitsOf())
	}
}

// apiProvision 接收"开 N 个某地区的出口"这个意图，返回作业 id 供轮询。
func apiProvision(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		q := r.URL.Query()
		count, err := strconv.Atoi(q.Get("count"))
		if err != nil || count < 1 || count > 50 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "count 参数无效（1-50）"})
			return
		}
		tpl := 0
		if s := q.Get("template"); s != "" {
			if tpl, err = strconv.Atoi(s); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "template 参数无效"})
				return
			}
		}
		job, err := m.Provision(tunnel.ProvisionRequest{
			Region: q.Get("region"), Count: count, TemplateID: tpl,
			EveryRegion: q.Get("every") == "1",
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"job": job.ID()})
	}
}

func apiJobs(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, m.Jobs.Views())
	}
}

func apiJobDismiss(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		m.Jobs.Dismiss(r.URL.Query().Get("id"))
		writeJSON(w, http.StatusOK, map[string]string{"ok": "已关闭"})
	}
}

func apiJobCancel(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		if m.Jobs.Cancel(r.URL.Query().Get("id")) {
			writeJSON(w, http.StatusOK, map[string]string{"ok": "已取消"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "作业不存在或已结束"})
		}
	}
}

// apiXUIStatus 报告当前的节点链接后端：接管的 3x-ui，或 home-broadband 自己跑的 Xray。
func apiXUIStatus(w http.ResponseWriter, r *http.Request) {
	p, err := panel.OpenPanel()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"available": false,
			"reason":    err.Error(),
		})
		return
	}
	resp := map[string]any{
		"available":  true,
		"kind":       p.Kind(),
		"describe":   p.Describe(),
		"can_create": true,
	}
	if x, ok := p.(*panel.XUI); ok {
		resp["port"] = x.Port
		resp["base_path"] = x.BasePath
		resp["scheme"] = x.Scheme
		resp["host"] = x.Host
	}
	writeJSON(w, http.StatusOK, resp)
}

// apiPanelMode 读取/切换节点链接后端。
// GET 返回当前模式与本机可选模式；POST {"mode":"..."} 运行时切换，空 mode = 恢复自动探测。
func apiPanelMode(m *tunnel.Manager, workDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
			var in struct {
				Mode string `json:"mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
				return
			}
			p, err := panel.SwitchPanelMode(in.Mode)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			m.SetBackend(p)
			tunnel.InvalidateInbounds()
			writeJSON(w, http.StatusOK, map[string]any{
				"mode":     panel.CurrentPanelMode(),
				"kind":     p.Kind(),
				"describe": p.Describe(),
			})
			return
		}
		resp := map[string]any{
			"mode":  panel.CurrentPanelMode(),
			"modes": panel.AvailablePanelModes(workDir),
		}
		if p, err := panel.OpenPanel(); err == nil {
			resp["kind"] = p.Kind()
			resp["describe"] = p.Describe()
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// apiXUIInbounds 列出面板里已有的入站及其绑定状态。
func apiXUIInbounds(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := tunnel.CachedInbounds(m.Backend(), liveHosts(m))
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// liveHosts 返回当前有连通隧道的节点标识集合。
func liveHosts(m *tunnel.Manager) map[string]bool {
	live := map[string]bool{}
	for _, t := range m.Tunnels() {
		if t.GetStatus() == "up" {
			live[tunnel.SanitizeTag(t.GetNode().HostName)] = true
		}
	}
	return live
}

// apiXUIBind 把某个入站绑定到某条隧道，slot=0 表示解绑。
func apiXUIBind(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		tag := r.URL.Query().Get("tag")
		if tag == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 tag 参数"})
			return
		}
		host := r.URL.Query().Get("host")
		x, err := panel.OpenPanel()
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		if err := x.Bind(tag, host, m.Tunnels()); err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		// 3x-ui 内部异步应用：等它真的生效再返回，像 3x-ui 自己的体验一样。
		// 每 300ms 查一次，最多等 6 秒；超时也返回成功，前端轮询会兜底。
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			tunnel.InvalidateInbounds()
			list, lerr := tunnel.CachedInbounds(m.Backend(), liveHosts(m))
			if lerr == nil {
				for _, ib := range list {
					if ib.Tag == tag && ib.BoundTo == host {
						goto confirmed
					}
				}
			}
			time.Sleep(300 * time.Millisecond)
		}
	confirmed:
		tunnel.InvalidateInbounds()
		writeJSON(w, http.StatusOK, map[string]string{"ok": "已更新"})
	}
}

// apiXUIClone 以某个入站为模板，为所有已连通的隧道各复制一个入站并绑好出口。
func apiXUIClone(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id 参数无效"})
			return
		}

		tunnels := m.Tunnels()
		// 用节点主机名而非槽位号：槽位在重启后会重排，指代会错位
		var hosts []string
		if raw := r.URL.Query().Get("hosts"); raw != "" {
			for _, part := range strings.Split(raw, ",") {
				if h := strings.TrimSpace(part); h != "" {
					hosts = append(hosts, h)
				}
			}
		} else {
			for _, t := range tunnels {
				if t.GetStatus() == "up" {
					hosts = append(hosts, t.GetNode().HostName)
				}
			}
		}
		if len(hosts) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "没有可用的隧道"})
			return
		}

		x, err := panel.OpenPanel()
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		ports, err := x.CloneToTunnels(id, hosts, tunnels)
		tunnel.InvalidateInbounds()
		if err != nil {
			log.Printf("API 502: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "部分克隆失败", "created": ports})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"created": ports})
	}
}

// apiXUIDetail 返回某个入站的详情，含客户端与可直接复制的分享链接。
func apiXUIDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id 参数无效"})
		return
	}
	x, err := panel.OpenPanel()
	if err != nil {
		writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
		return
	}
	host := r.URL.Query().Get("host")
	if host == "" {
		host = publicHost(r)
	}
	detail, err := x.InboundDetail(id, host)
	if err != nil {
		writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// publicHost 决定分享链接里的连接地址。母机公网 IPv4 才是客户端真正能连上
// 的地址，所以优先用它；探测不到（比如纯内网）再退回访问 home-broadband 时用的主机名。
func publicHost(r *http.Request) string {
	if ip := tunnel.HostPublicIP(); ip != "" {
		return ip
	}
	host := r.Host
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	if host == "" || host == "127.0.0.1" || host == "localhost" {
		return "<服务器IP>"
	}
	return host
}

// apiXUILinks 批量导出多个入站的分享链接。
func apiXUILinks(w http.ResponseWriter, r *http.Request) {
	x, err := panel.OpenPanel()
	if err != nil {
		writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
		return
	}

	var ids []int
	if raw := r.URL.Query().Get("ids"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				ids = append(ids, n)
			}
		}
	} else {
		list, err := x.Inbounds(nil)
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		for _, ib := range list {
			ids = append(ids, ib.ID)
		}
	}
	if len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "没有可导出的入站"})
		return
	}

	host := r.URL.Query().Get("host")
	if host == "" {
		host = publicHost(r)
	}
	links, err := x.InboundLinks(ids, host)
	if err != nil {
		log.Printf("API 502: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "获取链接失败", "links": links})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

// apiXUIDelete 删除入站。停掉出口后它的入站会留下来，用户需要一个清理入口。
func apiXUIDelete(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		var ids []int
		for _, part := range strings.Split(r.URL.Query().Get("ids"), ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				ids = append(ids, n)
			}
		}
		if len(ids) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "没有指定要删除的入站"})
			return
		}
		x, err := panel.OpenPanel()
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		err = x.DeleteInbounds(ids, m.Tunnels())
		tunnel.InvalidateInbounds()
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"deleted": len(ids)})
	}
}

// apiInboundCreate 新建一个入站。两种后端都支持：自建模式写自己的库，
// 接管 3x-ui 时走面板的 inbounds/add API。
// apiInboundUpdate 改入站的端口、备注与启停。两种后端都支持。
func apiInboundUpdate(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		p, err := panel.OpenPanel()
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		q := r.URL.Query()
		id, err := strconv.Atoi(q.Get("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id 参数无效"})
			return
		}

		// 只有真正传了的参数才改，没传的保持原样
		var patch panel.InboundPatch
		if v := q.Get("port"); v != "" {
			port, err := strconv.Atoi(v)
			if err != nil || port < 1 || port > 65535 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "端口无效（1-65535）"})
				return
			}
			patch.Port = &port
		}
		if q.Has("remark") {
			remark := q.Get("remark")
			if len(remark) > 200 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "备注太长（最多200字符）"})
				return
			}
			patch.Remark = &remark
		}
		if v := q.Get("enable"); v != "" {
			enable := v == "1"
			patch.Enable = &enable
		}

		err = p.UpdateInbound(id, patch, m.Tunnels())
		tunnel.InvalidateInbounds()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "已保存"})
	}
}

// clientAction 把三个客户端操作的公共部分收拢：解析 id/email 再调后端。
func clientAction(m *tunnel.Manager, what string,
	do func(p panel.Panel, id int, email string, tunnels []*tunnel.Tunnel) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		p, err := panel.OpenPanel()
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id 参数无效"})
			return
		}
		email := r.URL.Query().Get("email")
		if len(email) > 200 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email 太长（最多200字符）"})
			return
		}
		err = do(p, id, email, m.Tunnels())
		tunnel.InvalidateInbounds()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": what})
	}
}

func apiClientAdd(m *tunnel.Manager) http.HandlerFunc {
	return clientAction(m, "已添加", func(p panel.Panel, id int, email string, t []*tunnel.Tunnel) error {
		return p.AddClient(id, email, t)
	})
}

func apiClientDelete(m *tunnel.Manager) http.HandlerFunc {
	return clientAction(m, "已删除", func(p panel.Panel, id int, email string, t []*tunnel.Tunnel) error {
		return p.DeleteClient(id, email, t)
	})
}

func apiClientReset(m *tunnel.Manager) http.HandlerFunc {
	return clientAction(m, "已重置", func(p panel.Panel, id int, email string, t []*tunnel.Tunnel) error {
		return p.ResetClient(id, email, t)
	})
}

func apiInboundCreate(m *tunnel.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		p, err := panel.OpenPanel()
		if err != nil {
			writeServerError(w, http.StatusBadGateway, err, "上游服务异常，请稍后重试")
			return
		}

		q := r.URL.Query()
		port, _ := strconv.Atoi(q.Get("port"))
		ib, err := p.CreateInbound(panel.NewInboundSpec{
			Protocol: q.Get("protocol"),
			Network:  q.Get("network"),
			Port:     port,
			Remark:   q.Get("remark"),
			Path:     q.Get("path"),
			Host:     q.Get("host"),
			Security: q.Get("security"),
			Vision:   q.Get("vision") == "1",

			ServerName: q.Get("sni"),
			CertFile:   q.Get("cert"),
			KeyFile:    q.Get("key"),

			Dest:        q.Get("dest"),
			ServerNames: q.Get("server_names"),
			ShortID:     q.Get("sid"),
			Fingerprint: q.Get("fp"),
		}, m.Tunnels())
		tunnel.InvalidateInbounds()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":       ib.ID,
			"port":     ib.Port,
			"protocol": ib.Protocol,
			"remark":   ib.Remark,
			"network":  ib.Network,
			"security": ib.Security,
		})
	}
}
