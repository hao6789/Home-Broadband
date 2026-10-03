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

装完敲 `h` 进管理菜单，会打印 Web 面板地址和随机口令。面板里点「新建出口」，选地区和数量，十几秒自动拉起隧道并配好节点链接。

## 🖥️ 日常管理

`h` 命令搞定一切：启停服务、看日志、隧道列表、改端口/口令/访问路径、更新、卸载。

常用操作也可以直接带参数：`h info` 看连接信息，`h list` 看隧道，`h update` 更新，`h uninstall` 卸载。

## ❓ 常见问题

**直连 vpngate.net 失败？**
内置 Cloudflare Worker 反代兜底，默认已启用。想用自己的见 [worker/worker.js](worker/worker.js)，设 `HOMEBROADBAND_VPNGATE_MIRROR` 环境变量即可。

**节点连不上？**
VPN Gate 是志愿者节点，下线/满员是常态，连不上会自动顺着同地区候选往下试。

**面板放公网安全吗？**
默认只有随机路径 + 口令，没有 HTTPS，公网使用建议前面套一层反代。

## 📄 许可

[MIT](LICENSE)。节点来自筑波大学的 [VPN Gate](https://www.vpngate.net/) 学术实验项目，请遵守其条款和当地法律。
