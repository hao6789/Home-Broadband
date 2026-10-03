# home-broadband

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

把 VPN Gate 公共节点变成"一端口一个出口 IP"的 SOCKS5 出口。
每条隧道跑在独立 network namespace 里，VPN 路由劫持只影响自己的 netns，母机网络不受影响。

同机装了 3x-ui 就接管它的入站；没装则自己跑 Xray。建站、换节点、订阅、发链接都在同一个 Web 界面里完成。

## 安装

需要 root 的 Linux（依赖 netns 和 `/dev/net/tun`，LXC 小鸡没给 tun 权限的用不了）。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/hao6789/Home-Broadband/main/scripts/install.sh)
```

依赖（openvpn / curl / iproute / iptables 等）按发行版自动装。装完敲 `h` 打开管理菜单，会打印管理界面地址和随机口令。

## 使用

- **新建出口**：选地区和数量，一次拉起多条隧道并自动配好节点链接。一个端口对应一个国家出口。
- **换节点**：出口 IP 变、端口不变，已分发的客户端配置不用改。
- **订阅**：拿一条订阅地址填进客户端，加出口、删出口自动跟上。
- **只用家宽**：默认过滤 VPN Gate 机房节点，只用志愿者家宽出口。

## 运维

`h` 打开管理菜单：启停、日志、隧道列表、改端口/口令、更新、卸载。健康检查每 10 秒一次，隧道异常自动换节点重连，端口和槽位不变。

## 反代 Worker（可选）

直连 `vpngate.net` 失败时的兜底：一个 Cloudflare Worker，只转发节点列表接口，不是通用代理。默认已配好，开箱即用。

想用自己的：把 `worker/worker.js` 贴进 Cloudflare，新建 `ACCESS_KEY`，绑域名，然后设环境变量：

```
HOMEBROADBAND_VPNGATE_MIRROR=https://你的域名/vpngate
HOMEBROADBAND_VPNGATE_MIRROR_KEY=你的 ACCESS_KEY
```

设成空字符串则彻底关掉兜底，只走直连。key 只是防扫描白嫖，不是安全措施。

## 已知限制

- VPN Gate 是志愿者节点，不少已下线或满员，连不上会自动顺着同地区候选往下试。
- 管理界面只有随机路径 + 口令，没有 HTTPS，放公网建议套一层反代。

## 许可

[MIT](LICENSE)。节点来自 [VPN Gate](https://www.vpngate.net/)（筑波大学学术实验项目），请遵守其条款和你所在地的法律。

用着有问题、或者想要什么功能，提 issue。
