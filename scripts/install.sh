#!/usr/bin/env bash
# home-broadband 安装脚本：装二进制、装服务（systemd 或 OpenRC）、开机自启。
#
# Alpine 默认不带 bash，先装再跑：
#   apk add bash && bash <(curl -fsSL .../install.sh)

set -euo pipefail

# 记下用户是否显式给了 WEB_PORT：重装时只有显式指定才覆盖已保存的端口
WEB_PORT_EXPLICIT="${WEB_PORT:+1}"
WEB_PORT="${WEB_PORT:-8899}"
WORK_DIR="${WORK_DIR:-/var/lib/home-broadband}"
BIN=/usr/local/bin/home-broadband

# WEB_PORT 必须是 1-65535 的数字
if ! [[ $WEB_PORT =~ ^[0-9]+$ ]] || (( WEB_PORT < 1 || WEB_PORT > 65535 )); then
  echo "WEB_PORT 不合法（须为 1-65535）: ${WEB_PORT}" >&2
  exit 1
fi

# WORK_DIR 校验：非空、绝对路径、不含换行（后面要拼进 python/sed/heredoc）
if [[ -z $WORK_DIR || $WORK_DIR != /* || $WORK_DIR == *$'\n'* ]]; then
  echo "WORK_DIR 不合法（须为非空绝对路径且不含换行）: ${WORK_DIR}" >&2
  exit 1
fi

# 临时目录统一走 EXIT trap 清理，set -e 中途退出也不残留
TMP=""; XT=""
trap 'rm -rf "$TMP" "$XT"' EXIT

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

# seed_settings 把端口落进配置 —— 程序、h 菜单、Web 界面都以它为准。
#
# 重装时不覆盖用户已经改过的端口：除非这次显式指定了 WEB_PORT，
# 否则沿用原值，免得重装一次把人家改好的端口打回默认。
# 配置统一后权威来源是 config.json；老散文件 settings.json 只在首次启动时被迁移。
seed_settings() {
  # 已有 config.json：直接沿用里面的端口（除非显式指定了新的）
  if [[ -f "${WORK_DIR}/config.json" && -z "${WEB_PORT_EXPLICIT:-}" ]]; then
    local cur
    cur=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]+'/config.json')).get('web',{}).get('port',''))" "$WORK_DIR" 2>/dev/null)
    [[ -n $cur ]] && { WEB_PORT="$cur"; return; }
  fi
  # 全新安装：写 settings.json，首次启动时自动迁移进 config.json
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
    # 端口不写进服务文件：它由 ${WORK_DIR}/config.json 决定（见 seed_settings），
    # 两处都写会互相拽回旧值——界面改完重启失效，或 f 改完被配置覆盖。
    # 老版本模板里可能还带 -web，一并去掉。
    # WORK_DIR 转义 sed 特殊字符（& # \），防止自定义路径写坏服务文件
    esc_dir=$(printf '%s' "$WORK_DIR" | sed 's/[#&\\/]/\\&/g')
    sed "s#-web [0-9]* ##; s#-dir /var/lib/home-broadband#-dir ${esc_dir}#" deploy/home-broadband.service \
      > /etc/systemd/system/home-broadband.service
    systemctl daemon-reload
  else
    # OpenRC 没有 systemd 那套单元文件，直接写 init script。
    # supervise-daemon 负责守护与重启，等价于 Restart=on-failure。
    cat > /etc/init.d/home-broadband <<INITEOF
#!/sbin/openrc-run
name="home-broadband"
description="home-broadband - VPN Gate 多出口网关"
command="${BIN}"
command_args="-dir \"${WORK_DIR}\""
command_background=true
pidfile="/run/home-broadband.pid"
output_log="/var/log/home-broadband.log"
error_log="/var/log/home-broadband.log"
respawn_delay=5
respawn_max=0
supervisor=supervise-daemon
depend() { need net; after firewall; }
INITEOF
    chmod +x /etc/init.d/home-broadband
  fi
}

svc_enable_start() {
  if [[ "$INIT_SYS" == systemd ]]; then
    systemctl enable home-broadband >/dev/null 2>&1 || true
    # enable --now 不会重启已在跑的旧进程；重装后必须 restart 才能换上新二进制
    if systemctl is-active --quiet home-broadband 2>/dev/null; then
      systemctl restart home-broadband
    else
      systemctl start home-broadband
    fi
  else
    rc-update add home-broadband default >/dev/null 2>&1 || true
    rc-service home-broadband restart
  fi
}

svc_is_active() {
  if [[ "$INIT_SYS" == systemd ]]; then
    systemctl is-active --quiet home-broadband
  else
    rc-service home-broadband status >/dev/null 2>&1
  fi
}

svc_logs_hint() {
  [[ "$INIT_SYS" == systemd ]] && echo "journalctl -u home-broadband -n 30" || echo "cat /var/log/home-broadband.log"
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
    python3)  case "$mgr" in apk) echo python3 ;; *) echo python3 ;; esac ;;
    ip)       case "$mgr" in apk|pacman|zypper) echo iproute2 ;; *) echo iproute ;; esac ;;
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

need_cmd=()
for c in openvpn curl openssl tar iptables python3; do
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
    pkgs+=("$(pkg_for "$c" "$MGR")")
  done
  echo "      安装: ${pkgs[*]}"
  install_pkgs "$MGR" "${pkgs[@]}" || {
    echo "      自动安装失败，请手动安装: ${pkgs[*]}" >&2
    exit 1
  }
fi

echo "[2/6] 获取程序"
REPO="${REPO:-hao6789/Home-Broadband}"
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) echo "      不支持的架构: $ARCH" >&2; exit 1 ;;
esac

if [[ -f main.go ]] && command -v go >/dev/null; then
  echo "      从源码编译"
  # 先编到临时文件再 install：直接 -o 到 $BIN 时若旧进程在跑会 ETXTBSY
  TMP_BUILD=$(mktemp -d)
  trap 'rm -rf "$TMP" "$XT" "$TMP_BUILD"' EXIT
  go build -trimpath -ldflags "-s -w" -o "$TMP_BUILD/home-broadband" .
  install -m 755 "$TMP_BUILD/home-broadband" "$BIN"
else
  echo "      下载预编译版本 (${GOARCH})"
  TMP=$(mktemp -d)
  URL="https://github.com/${REPO}/releases/latest/download/home-broadband-linux-${GOARCH}.tar.gz"
  if ! curl -fsSL "$URL" -o "$TMP/pkg.tar.gz"; then
    echo "      下载失败: $URL" >&2
    echo "      也可以 clone 仓库后在源码目录运行本脚本" >&2
    exit 1
  fi
  # 校验 SHA-256（与 h.sh do_update 对齐，防篡改/损坏）
  if curl -fsSL "https://github.com/${REPO}/releases/latest/download/checksums.txt" -o "$TMP/checksums.txt" 2>/dev/null; then
    want=$(grep "home-broadband-linux-${GOARCH}.tar.gz" "$TMP/checksums.txt" | awk '{print $1}')
    if [[ -n $want ]]; then
      got=$(sha256sum "$TMP/pkg.tar.gz" | awk '{print $1}')
      if [[ $got != "$want" ]]; then
        echo "      SHA-256 校验失败，安装包可能被篡改" >&2
        exit 1
      fi
      echo "      SHA-256 校验通过"
    fi
  else
    echo "      警告: 无法下载 checksums.txt，跳过校验" >&2
  fi
  tar xzf "$TMP/pkg.tar.gz" -C "$TMP" || { echo "解压安装包失败，可能是下载不完整" >&2; exit 1; }
  install -m 755 "$TMP/home-broadband" "$BIN"
  [[ -f deploy/home-broadband.service ]] || { mkdir -p deploy && cp "$TMP/deploy/home-broadband.service" deploy/; }
  [[ -f "$TMP/scripts/h.sh" ]] && install -m 755 "$TMP/scripts/h.sh" /usr/local/bin/h
  # TMP 由 EXIT trap 统一清理，这里不删：后面步骤 5 还要用它找 h.sh
fi

echo "[3/6] 准备 Xray"
# 没有现成面板接管时 home-broadband 自己跑 Xray，需要一份二进制。
# 装到 WORK_DIR/bin 下而不是 /usr/local/bin，避免和机器上别人的 xray 抢版本。
mkdir -p "${WORK_DIR}/bin"
if command -v /usr/local/x-ui/x-ui >/dev/null 2>&1 || [[ -x /usr/bin/x-ui ]]; then
  echo "      检测到 3x-ui，入站交给面板管，跳过"
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
    # 基本完整性检查：zip 文件头魔数
    if ! head -c 2 "$XT/x.zip" | grep -q "^PK"; then
      echo "      Xray 下载文件损坏（非 zip 格式）" >&2
    else
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
    fi
  else
    echo "      下载失败，自建模式不可用（装了 3x-ui 则不受影响）" >&2
  fi
  # XT 由 EXIT trap 统一清理
fi

echo "[4/6] 放行转发"
sysctl -qw net.ipv4.ip_forward=1 2>/dev/null || echo "警告: 无法设置 ip_forward（容器环境常见），隧道功能可能受限" >&2
grep -q '^net.ipv4.ip_forward=1' /etc/sysctl.conf 2>/dev/null \
  || echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
# FORWARD 链常有兜底 REJECT，home-broadband 用的网段要插到最前面
# （容器/无特权环境可能失败，给警告不中断安装）
if ! iptables -C FORWARD -s 10.99.0.0/16 -j ACCEPT 2>/dev/null; then
  iptables -I FORWARD 1 -s 10.99.0.0/16 -j ACCEPT 2>/dev/null || echo "警告: iptables 规则添加失败（容器环境常见）" >&2
fi
if ! iptables -C FORWARD -d 10.99.0.0/16 -j ACCEPT 2>/dev/null; then
  iptables -I FORWARD 1 -d 10.99.0.0/16 -j ACCEPT 2>/dev/null || echo "警告: iptables 规则添加失败（容器环境常见）" >&2
fi
command -v netfilter-persistent >/dev/null && netfilter-persistent save >/dev/null 2>&1 || true
# RHEL 系用 iptables-services 持久化；都没有就告警，重启后规则会丢
if ! command -v netfilter-persistent >/dev/null 2>&1; then
  if command -v service >/dev/null 2>&1 && service iptables save >/dev/null 2>&1; then
    : # RHEL/CentOS 已保存
  elif [[ -d /etc/iptables ]]; then
    iptables-save > /etc/iptables/rules.v4 2>/dev/null || true
  else
    echo "      警告：本机无 iptables 持久化工具，重启后 FORWARD 规则会丢失" >&2
  fi
fi

echo "[5/6] 安装服务"
# 管理菜单
if [[ -f scripts/h.sh ]]; then
  install -m 755 scripts/h.sh /usr/local/bin/h
elif [[ -n "${TMP:-}" && -f "${TMP}/scripts/h.sh" ]]; then
  install -m 755 "${TMP}/scripts/h.sh" /usr/local/bin/h
else
  curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/scripts/h.sh" -o /usr/local/bin/h \
    && chmod 755 /usr/local/bin/h
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

# 口令、访问路径、端口都在 config.json 里（配置统一后不再写散文件）。
# 等 home-broadband 首次启动写出来：光文件存在不够，
# 口令和路径是程序启动后才生成的，得等到它们非空。
# 路径走 sys.argv 传参，不拼进单引号（WORK_DIR 含引号也不炸）。
BP=""; PW=""
for _ in $(seq 1 30); do
  [[ -s "${WORK_DIR}/config.json" ]] || { sleep 1; continue; }
  BP=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get('basepath',''))" "${WORK_DIR}/config.json" 2>/dev/null)
  PW=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get('password',''))" "${WORK_DIR}/config.json" 2>/dev/null)
  [[ -n "$BP" && -n "$PW" ]] && break
  sleep 1
done

IP=$(curl -s --max-time 8 http://api.ipify.org || echo "<本机IP>")
# 用 python3 从 config.json 里取，免得再被散文件改名搞乱
WEB_PORT=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get('web',{}).get('port',''))" "${WORK_DIR}/config.json" 2>/dev/null)
[[ -n "$WEB_PORT" ]] || WEB_PORT=8899
echo
echo "  管理界面  http://${IP}:${WEB_PORT}${BP}/"
echo "  访问口令  ${PW:-见 ${WORK_DIR}/config.json 的 password 字段}"
echo
echo "  路径和口令都是随机生成的，存在 ${WORK_DIR}/config.json 里："
echo "    python3 -c \"import json,sys; d=json.load(open(sys.argv[1])); print(d['basepath'], d['password'])\" \"${WORK_DIR}/config.json\""
echo
echo "  输入 h 打开管理菜单"
echo
echo "  安全提示：面板当前是 HTTP 明文传输，公网访问建议开启 HTTPS。"
echo "  输入 h 选择「证书管理」→「申请证书」，按提示办一张免费证书。"
echo
echo "  ────────────────────────────────"
echo "  项目    https://github.com/hao6789/Home-Broadband"
echo
