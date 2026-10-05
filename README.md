# Home-Broadband

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8.svg)](go.mod)

一键把 [VPN Gate](https://www.vpngate.net/) 公共节点变成独享出口 IP。每个出口独立隧道、独立 IP，互不干扰，母机网络零影响。

## ✨ 能做什么

- **一端口一 IP**：每个 SOCKS5 端口对应一个国家出口，客户端连哪个端口就从哪国出去
- **隧道隔离**：每条 OpenVPN 隧道跑在独立 network namespace，路由劫持不出 netns
- **Web 面板**：新建出口、换节点、订阅、节点链接管理，一个界面全搞定
- **双后端**：有 3x-ui 就接管它，没装就自建 Xray（VLESS / VMess / Trojan）
- **自动运维**：每 10 秒健康检查，隧道挂了自动换节点重连，端口不变
- **家宽优先**：默认过滤机房节点，只用志愿者家庭宽带出口

## 🚀 快速开始

需要 root 的 Linux，依赖 network namespace 和 `/dev/net/tun`。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/hao6789/Home-Broadband/main/scripts/install.sh)
```

自定义面板端口：`WEB_PORT=8888 bash <(curl -fsSL https://raw.githubusercontent.com/hao6789/Home-Broadband/main/scripts/install.sh)`

装完敲 `h` 进管理菜单，会打印 Web 面板地址和随机口令。面板里点「新建出口」，选地区和数量，十几秒自动拉起隧道并配好节点链接。

## 🖥️ 日常管理

`h` 命令搞定一切：启停服务、看日志、隧道列表、改端口/口令/访问路径、证书管理、更新、卸载。

常用操作也可以直接带参数：`h info` 看连接信息，`h list` 看隧道，`h update` 更新，`h uninstall` 卸载。

## 🔒 HTTPS

面板原生支持 HTTPS，两种方式二选一：

**h 菜单申请**：`h` → `14 证书管理` → `1 申请证书`，按提示输入域名。会自动复用 3x-ui 装过的 acme.sh（没有就自己装），standalone 模式签发免费证书，存到 `/root/cert/<域名>/`，到期自动续期并重启面板。

**面板设置页**：设置里直接填证书和私钥路径，保存即热切换（会提示改用 `https://` 打开），有效期显示在输入框下面。

注意：
- 签发需要域名解析到本机、80 端口空闲
- 和 3x-ui 共存时两个面板建议用不同域名，免得续期配置互相覆盖
- 证书管理子菜单里还能查看证书、强制续期、吊销

## ☁️ 反代 Worker

拉节点列表时直连失败的兜底，部署在 Cloudflare Workers 上，只转发 VPN Gate 的节点列表接口，不是通用代理。

默认用 `https://h.111678.xyz/vpngate`，开箱即用，不需要自己搭。

### 自己部署

1. Cloudflare 控制台建一个 Worker，把 `worker/worker.js` 的内容贴进去
2. Settings → Variables 加一个 Secret，名字 `ACCESS_KEY`，值随便一串随机字符
3. 绑一个自己的域名
4. 给 home-broadband 设两个环境变量：

```
HOMEBROADBAND_VPNGATE_MIRROR=https://你的域名/vpngate
HOMEBROADBAND_VPNGATE_MIRROR_KEY=你刚才设的 ACCESS_KEY
```

`HOMEBROADBAND_VPNGATE_MIRROR` 设成空字符串就是彻底关掉兜底，只走直连。

### 关于那个 key

只是让爬虫和端口扫描器扫到域名时看到 404，别把 Worker 当免费流量白嫖。home-broadband 是开源的，密钥就写在源码里，这不是安全措施，别指望它挡住有心人。

## ❓ 常见问题

**节点连不上？**
VPN Gate 是志愿者节点，下线/满员是常态，连不上会自动顺着同地区候选往下试。

**面板放公网安全吗？**
默认是 HTTP + 随机路径 + 口令。公网使用建议开 HTTPS：`h` → 证书管理 → 申请证书，或在面板设置页填证书路径。想再套一层反代也行。

## 📄 许可

[MIT](LICENSE)。节点来自筑波大学的 [VPN Gate](https://www.vpngate.net/) 学术实验项目，请遵守其条款和当地法律。
