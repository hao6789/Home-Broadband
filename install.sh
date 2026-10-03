#!/usr/bin/env bash
# fanout 安装脚本：装二进制、装服务（systemd 或 OpenRC）、开机自启。
#
# Alpine 默认不带 bash，先装再跑：
#   apk add bash && bash <(curl -fsSL .../install.sh)

set -euo pipefail

# 记下用户是否显式给了 WEB_PORT：重装时只有显式指定才覆盖已保存的端口
WEB_PORT_EXPLICIT="${WEB_PORT:+1}"
WEB_PORT="${WEB_PORT:-8899}"
WORK_DIR="${WORK_DIR:-/var/lib/fanout}"
BIN=/usr/local/bin/fanout

if [[ $EUID -ne 0 ]]; then
  echo "需要 root 权限（要创建 netns 和改 iptables）" >&2
  exit 1
fi

# ── init 系统抽象：systemd 与 OpenRC 两套 ────────────────
INIT_SYS=""
if command -v systemctl >/dev/null 2>&1 && [[ -d /run/systemd/system ]]; then
  INIT_SYS=systemd
elif command -v rc-service >/dev/null 2>&1; then
  INIT_SYS=openrc
else
  echo "不认识的 init 系统（需要 systemd 或 OpenRC）" >&2
  exit 1
fi

# seed_settings 把端口落进 settings.json —— 程序、f 菜单、Web 界面都以它为准。
#
# 重装时不覆盖用户已经改过的端口：除非这次显式指定了 WEB_PORT，
# 否则沿用原值，免得重装一次把人家改好的端口打回默认。
seed_settings() {
  local f="${WORK_DIR}/settings.json"
  if [[ -f "$f" ]] && [[ -z "${WEB_PORT_EXPLICIT:-}" ]]; then
    local cur
    cur=$(sed -n 's/.*"port"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' "$f" | head -1)
    [[ -n $cur ]] && { WEB_PORT="$cur"; return; }
  fi
  printf '{\n  "port": %s,\n  "listen_addr": ""\n}\n' "$WEB_PORT" > "$f"
  chmod 600 "$f"
}

svc_install() {
  if [[ "$INIT_SYS" == systemd ]]; then
    # 端口不写进服务文件：它由 ${WORK_DIR}/settings.json 决定（见 seed_settings），
    # 两处都写会互相拽回旧值——界面改完重启失效，或 f 改完被配置覆盖。
    # 老版本模板里可能还带 -web，一并去掉。
    sed "s#-web [0-9]* ##; s#-dir /var/lib/fanout#-dir ${WORK_DIR}#" fanout.service \
      > /etc/systemd/system/fanout.service
    systemctl daemon-reload
  else
    # OpenRC 没有 systemd 那套单元文件，直接写 init script。
    # supervise-daemon 负责守护与重启，等价于 Restart=on-failure。
    cat > /etc/init.d/fanout <<INITEOF
#!/sbin/openrc-run
name="fanout"
description="fanout - VPN Gate 出口扇出网关"
command="${BIN}"
command_args="-dir ${WORK_DIR}"
command_background=true
pidfile="/run/fanout.pid"
output_log="/var/log/fanout.log"
error_log="/var/log/fanout.log"
respawn_delay=5
respawn_max=0
supervisor=supervise-daemon
depend() { need net; after firewall; }
INITEOF
    chmod +x /etc/init.d/fanout
  fi
}

svc_enable_start() {
  if [[ "$INIT_SYS" == systemd ]]; then
    systemctl enable --now fanout
  else
    rc-update add fanout default >/dev/null 2>&1 || true
    rc-service fanout restart
  fi
}

svc_is_active() {
  if [[ "$INIT_SYS" == systemd ]]; then
    systemctl is-active --quiet fanout
  else
    rc-service fanout status >/dev/null 2>&1
  fi
}

svc_logs_hint() {
  [[ "$INIT_SYS" == systemd ]] && echo "journalctl -u fanout -n 30" || echo "cat /var/log/fanout.log"
}

echo "[1/6] 检查依赖"

# 同一个命令在各发行版里的包名并不一致，按包管理器分别给出。
pkg_for() {
  local cmd="$1" mgr="$2"
  case "$cmd" in
    openvpn)  echo openvpn ;;
    curl)     echo curl ;;
    openssl)  echo openssl ;;
    tar)      echo tar ;;
    ip)       case "$mgr" in apk) echo iproute2 ;; pacman) echo iproute2 ;; *) echo iproute ;; esac ;;
    iptables) echo iptables ;;
    unzip)    echo unzip ;;
  esac
}

detect_mgr() {
  for m in apt-get dnf yum pacman apk zypper; do
    command -v "$m" >/dev/null && { echo "$m"; return; }
  done
  echo ""
}

install_pkgs() {
  local mgr="$1"; shift
  case "$mgr" in
    apt-get)
      apt-get update -qq
      DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$@"
      ;;
    dnf)    dnf install -y -q "$@" ;;
    yum)    yum install -y -q "$@" ;;
    pacman) pacman -Sy --noconfirm --needed "$@" ;;
    apk)    apk add --no-cache "$@" ;;
    zypper) zypper --non-interactive install -y "$@" ;;
  esac
}

MGR=$(detect_mgr)
# Debian/Ubuntu 的 iproute2 与 RHEL 系的 iproute 是同一个东西，名字不同
[[ "$MGR" == "apt-get" ]] && iproute_pkg=iproute2 || iproute_pkg=iproute

need_cmd=()
for c in openvpn curl openssl tar iptables; do
  command -v "$c" >/dev/null || need_cmd+=("$c")
done
command -v ip >/dev/null || need_cmd+=(ip)

if [[ ${#need_cmd[@]} -gt 0 ]]; then
  echo "      缺少: ${need_cmd[*]}"
  if [[ -z "$MGR" ]]; then
    echo "      不认识的包管理器，请手动安装后重试" >&2
    exit 1
  fi
  pkgs=()
  for c in "${need_cmd[@]}"; do
    if [[ "$c" == "ip" ]]; then pkgs+=("$iproute_pkg"); else pkgs+=("$(pkg_for "$c" "$MGR")"); fi
  done
  echo "      安装: ${pkgs[*]}"
  install_pkgs "$MGR" "${pkgs[@]}" || {
    echo "      自动安装失败，请手动安装: ${pkgs[*]}" >&2
    exit 1
  }
fi

echo "[2/6] 获取程序"
REPO="${REPO:-byJoey/fanout}"
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) echo "      不支持的架构: $ARCH" >&2; exit 1 ;;
esac

if [[ -f main.go ]] && command -v go >/dev/null; then
  echo "      从源码编译"
  go build -trimpath -ldflags "-s -w" -o "$BIN" .
else
  echo "      下载预编译版本 (${GOARCH})"
  TMP=$(mktemp -d)
  URL="https://github.com/${REPO}/releases/latest/download/fanout-linux-${GOARCH}.tar.gz"
  if ! curl -fsSL "$URL" -o "$TMP/f.tar.gz"; then
    echo "      下载失败: $URL" >&2
    echo "      也可以 clone 仓库后在源码目录运行本脚本" >&2
    exit 1
  fi
  tar xzf "$TMP/f.tar.gz" -C "$TMP"
  install -m 755 "$TMP/fanout" "$BIN"
  [[ -f fanout.service ]] || cp "$TMP/fanout.service" .
  [[ -f "$TMP/f.sh" ]] && install -m 755 "$TMP/f.sh" /usr/local/bin/f
  rm -rf "$TMP"
fi

echo "[3/6] 准备 Xray"
# 没有现成面板接管时 fanout 自己跑 Xray，需要一份二进制。
# 装到 WORK_DIR/bin 下而不是 /usr/local/bin，避免和机器上别人的 xray 抢版本。
mkdir -p "${WORK_DIR}/bin"
if command -v /usr/local/x-ui/x-ui >/dev/null 2>&1 || [[ -x /usr/bin/x-ui ]]; then
  echo "      检测到 3x-ui，入站交给面板管，跳过"
elif [[ -d /etc/xray-cf-lite && -f /usr/local/etc/xray/config.json ]]; then
  echo "      检测到 xray-cf-lite，入站交给它管，跳过"
elif [[ -x "${WORK_DIR}/bin/xray" ]]; then
  echo "      已有 $("${WORK_DIR}/bin/xray" version 2>/dev/null | head -1)"
else
  case "$GOARCH" in
    amd64) XRAY_ASSET=Xray-linux-64.zip ;;
    arm64) XRAY_ASSET=Xray-linux-arm64-v8a.zip ;;
  esac
  echo "      下载 Xray (${XRAY_ASSET})"
  XT=$(mktemp -d)
  XURL="https://github.com/XTLS/Xray-core/releases/latest/download/${XRAY_ASSET}"
  if curl -fsSL "$XURL" -o "$XT/x.zip"; then
    # 只为解一个 zip 装 unzip 有点重，busybox 环境常自带
    if command -v unzip >/dev/null; then
      unzip -qo "$XT/x.zip" -d "$XT"
    elif command -v busybox >/dev/null && busybox unzip -h >/dev/null 2>&1; then
      busybox unzip -qo "$XT/x.zip" -d "$XT"
    else
      [[ -n "$MGR" ]] && install_pkgs "$MGR" unzip >/dev/null 2>&1 || true
      command -v unzip >/dev/null && unzip -qo "$XT/x.zip" -d "$XT"
    fi
    if [[ -f "$XT/xray" ]]; then
      install -m 755 "$XT/xray" "${WORK_DIR}/bin/xray"
      echo "      $("${WORK_DIR}/bin/xray" version 2>/dev/null | head -1)"
    else
      echo "      解压失败，自建模式不可用（装了 3x-ui 则不受影响）" >&2
    fi
  else
    echo "      下载失败，自建模式不可用（装了 3x-ui 则不受影响）" >&2
  fi
  rm -rf "$XT"
fi

echo "[4/6] 放行转发"
sysctl -qw net.ipv4.ip_forward=1
grep -q '^net.ipv4.ip_forward=1' /etc/sysctl.conf 2>/dev/null \
  || echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
# FORWARD 链常有兜底 REJECT，fanout 用的网段要插到最前面
if ! iptables -C FORWARD -s 10.99.0.0/16 -j ACCEPT 2>/dev/null; then
  iptables -I FORWARD 1 -s 10.99.0.0/16 -j ACCEPT
fi
if ! iptables -C FORWARD -d 10.99.0.0/16 -j ACCEPT 2>/dev/null; then
  iptables -I FORWARD 1 -d 10.99.0.0/16 -j ACCEPT
fi
command -v netfilter-persistent >/dev/null && netfilter-persistent save >/dev/null 2>&1 || true

echo "[5/6] 安装服务"
# 管理菜单
if [[ -f f.sh ]]; then
  install -m 755 f.sh /usr/local/bin/f
elif [[ -n "${TMP:-}" && -f "${TMP}/f.sh" ]]; then
  install -m 755 "${TMP}/f.sh" /usr/local/bin/f
else
  curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/f.sh" -o /usr/local/bin/f \
    && chmod 755 /usr/local/bin/f
fi
mkdir -p "$WORK_DIR"
chmod 700 "$WORK_DIR"
seed_settings
svc_install
svc_enable_start

echo "[6/6] 就绪"
sleep 3
svc_is_active && echo "      服务运行中（${INIT_SYS}）" || {
  echo "      服务启动失败，看 $(svc_logs_hint)" >&2
  exit 1
}

# 口令与访问路径由 fanout 首次启动时生成，等它写出来
for _ in $(seq 1 10); do
  [[ -s "${WORK_DIR}/password" && -s "${WORK_DIR}/basepath" ]] && break
  sleep 1
done

IP=$(curl -s --max-time 8 http://api.ipify.org || echo "<本机IP>")
BP=$(cat "${WORK_DIR}/basepath" 2>/dev/null || true)
# 以配置里的实际端口为准报地址，别让提示和真实监听对不上
ACTUAL_PORT=$(sed -n 's/.*"port"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' \
  "${WORK_DIR}/settings.json" 2>/dev/null | head -1)
[[ -n $ACTUAL_PORT ]] && WEB_PORT="$ACTUAL_PORT"
echo
echo "  管理界面  http://${IP}:${WEB_PORT}/${BP}/"
echo "  访问口令  $(cat "${WORK_DIR}/password" 2>/dev/null || echo "见 ${WORK_DIR}/password")"
echo
echo "  路径和口令都是随机生成的，也可以随时查看："
echo "    cat ${WORK_DIR}/basepath"
echo "    cat ${WORK_DIR}/password"
echo
echo "  输入 f 打开管理菜单"
echo
echo "  ────────────────────────────────"
echo "  交流群  https://t.me/+ft-zI76oovgwNmRh"
echo "  油管    https://youtube.com/@joeyblog"
echo "  博客    https://joeyblog.net"
echo "  项目    https://github.com/byJoey/fanout"
echo
