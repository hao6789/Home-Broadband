#!/usr/bin/env bash
# home-broadband 管理菜单
set -uo pipefail

WORK_DIR="${WORK_DIR:-/var/lib/home-broadband}"
SERVICE=home-broadband
BIN=/usr/local/bin/home-broadband
REPO="${REPO:-hao6789/Home-Broadband}"

# WORK_DIR 校验：非空、绝对路径、不含换行（do_uninstall 有 rm -rf，必须拦）
if [[ -z $WORK_DIR || $WORK_DIR != /* || $WORK_DIR == *$'\n'* ]]; then
  echo "WORK_DIR 不合法（须为非空绝对路径且不含换行）: ${WORK_DIR}" >&2
  exit 1
fi

G='\033[0;32m'; R='\033[0;31m'; Y='\033[0;33m'; B='\033[0;36m'; D='\033[2m'; N='\033[0m'

# ── config.json 读写：配置统一后唯一的权威来源 ──────────
# 散文件（settings.json/password/basepath）已废弃，只在迁移时读一次。
CFG="$WORK_DIR/config.json"

cfg_get() { # cfg_get <key>：读顶层字段，失败返回非零
  local out rc
  out=$(python3 -c "import json; print(json.load(open('$CFG')).get('$1',''))" 2>&1); rc=$?
  if (( rc != 0 )); then
    echo "  读配置失败 (${CFG}): $(tail -n1 <<<"$out")" >&2
    return 1
  fi
  printf '%s\n' "$out"
}
cfg_get_web() { # cfg_get_web <key>：读 web 段字段，失败返回非零
  local out rc
  out=$(python3 -c "import json; print(json.load(open('$CFG')).get('web',{}).get('$1',''))" 2>&1); rc=$?
  if (( rc != 0 )); then
    echo "  读配置失败 (${CFG}): $(tail -n1 <<<"$out")" >&2
    return 1
  fi
  printf '%s\n' "$out"
}
cfg_set() { # cfg_set <key> <value>：写顶层字段（字符串），失败返回非零
  local out rc
  out=$(python3 - "$CFG" "$1" "$2" <<'PY' 2>&1
import json, sys
f, k, v = sys.argv[1], sys.argv[2], sys.argv[3]
d = json.load(open(f))
d[k] = v
json.dump(d, open(f, 'w'), indent=2)
PY
); rc=$?
  if (( rc != 0 )); then
    echo "  写配置失败 (${CFG}): $(tail -n1 <<<"$out")" >&2
    return 1
  fi
  chmod 600 "$CFG"
}
cfg_set_web() { # cfg_set_web <key> <value>：写 web 段字段，失败返回非零
  local k="$1" v="$2" out rc
  if [[ $v =~ ^[0-9]+$ ]]; then
    out=$(python3 - "$CFG" "$k" "$v" <<'PY' 2>&1
import json, sys
f, k, v = sys.argv[1], sys.argv[2], int(sys.argv[3])
d = json.load(open(f))
d.setdefault('web', {})[k] = v
json.dump(d, open(f, 'w'), indent=2)
PY
); rc=$?
  else
    out=$(python3 - "$CFG" "$k" "$v" <<'PY' 2>&1
import json, sys
f, k, v = sys.argv[1], sys.argv[2], sys.argv[3]
d = json.load(open(f))
d.setdefault('web', {})[k] = v
json.dump(d, open(f, 'w'), indent=2)
PY
); rc=$?
  fi
  if (( rc != 0 )); then
    echo "  写配置失败 (${CFG}): $(tail -n1 <<<"$out")" >&2
    return 1
  fi
  chmod 600 "$CFG"
}

need_root() {
  [[ $EUID -eq 0 ]] || { echo -e "${R}需要 root${N}"; exit 1; }
}

# ── init 系统抽象：systemd 与 OpenRC ────────────────────
if command -v systemctl >/dev/null 2>&1 && [[ -d /run/systemd/system ]]; then
  INIT_SYS=systemd
  UNIT=/etc/systemd/system/${SERVICE}.service
elif command -v rc-service >/dev/null 2>&1; then
  INIT_SYS=openrc
  UNIT=/etc/init.d/${SERVICE}
else
  echo -e "${R}不认识的 init 系统（需要 systemd 或 OpenRC）${N}" >&2
  exit 1
fi

svc_start()   { [[ $INIT_SYS == systemd ]] && systemctl start "$SERVICE"   || rc-service "$SERVICE" start; }
svc_stop()    { [[ $INIT_SYS == systemd ]] && systemctl stop "$SERVICE"    || rc-service "$SERVICE" stop; }
svc_restart() { [[ $INIT_SYS == systemd ]] && systemctl restart "$SERVICE" || rc-service "$SERVICE" restart; }
svc_reload()  { [[ $INIT_SYS == systemd ]] && systemctl daemon-reload || true; }
svc_enable()  { [[ $INIT_SYS == systemd ]] && systemctl enable "$SERVICE" >/dev/null 2>&1 || rc-update add "$SERVICE" default >/dev/null 2>&1; }
svc_disable() { [[ $INIT_SYS == systemd ]] && systemctl disable "$SERVICE" >/dev/null 2>&1 || rc-update del "$SERVICE" default >/dev/null 2>&1; }

svc_is_enabled() {
  if [[ $INIT_SYS == systemd ]]; then
    systemctl is-enabled --quiet "$SERVICE"
  else
    rc-update show default 2>/dev/null | grep -q "^ *${SERVICE} "
  fi
}

svc_enabled_text() {
  svc_is_enabled && echo enabled || echo disabled
}

svc_status_page() {
  if [[ $INIT_SYS == systemd ]]; then
    systemctl status "$SERVICE" --no-pager
  else
    rc-service "$SERVICE" status
  fi
}

svc_logs() {
  if [[ $INIT_SYS == systemd ]]; then
    journalctl -u "$SERVICE" -n "${1:-50}" --no-pager
  else
    tail -n "${1:-50}" /var/log/${SERVICE}.log 2>/dev/null || echo "  暂无日志"
  fi
}

svc_logs_follow() {
  if [[ $INIT_SYS == systemd ]]; then
    journalctl -u "$SERVICE" -f
  else
    tail -f /var/log/${SERVICE}.log
  fi
}

svc_state() {
  if [[ $INIT_SYS == systemd ]]; then
    systemctl is-active --quiet "$SERVICE" && echo running || echo stopped
  else
    rc-service "$SERVICE" status >/dev/null 2>&1 && echo running || echo stopped
  fi
}

# 端口以 config.json 的 web.port 为准。老版本把 -web 写死在服务文件里，
# 两处各改各的会互相拽回旧值，所以这里只认工作目录下的配置。
web_port() {
  local p
  p=$(cfg_get_web port)
  [[ -n $p ]] && { echo "$p"; return; }
  # 兼容老安装：config.json 还没生成时退回读服务文件
  grep -oE '\-web [0-9]+' "$UNIT" 2>/dev/null \
    | grep -oE '[0-9]+' | head -1 || echo 8899
}

public_ip() {
  curl -s --max-time 6 http://api.ipify.org 2>/dev/null || echo "<本机IP>"
}

pause() {
  echo
  read -rp "回车返回菜单..." _
}

show_info() {
  local state port bp pw ip ver n autostart
  state=$(svc_state); port=$(web_port)
  bp=$(cfg_get basepath); [[ -z $bp ]] && bp="-"
  pw=$(cfg_get password); [[ -z $pw ]] && pw="-"
  ip=$(public_ip)
  ver=$("$BIN" -version 2>/dev/null || echo '-')
  n=$(ls -d /var/run/netns/hb* 2>/dev/null | wc -l | tr -d ' ')
  if svc_is_enabled; then autostart="${G}已开启${N}"; else autostart="${Y}已关闭${N}"; fi

  echo
  if [[ $state == running ]]; then
    echo -e "  状态  ${G}运行中${N}    版本  ${ver}"
  else
    echo -e "  状态  ${R}已停止${N}    版本  ${ver}"
  fi
  echo -e "  自启  ${autostart}    隧道  ${n} 条"
  echo
  echo -e "  ${B}管理地址  http://${ip}:${port}${bp}/${N}"
  echo -e "  ${B}访问口令  ${pw}${N}"
}

list_tunnels() {
  local port bp pw ck pf
  port=$(web_port)
  bp=$(cfg_get basepath)
  pw=$(cfg_get password)
  ck=$(mktemp)
  pf=$(mktemp)
  # Ctrl-C 也清理临时文件
  trap 'rm -f "$ck" "$pf" "$ck.json"' RETURN INT TERM
  # 口令写临时文件再 --data @file，避免进命令行被 ps 看到
  printf 'password=%s' "$pw" > "$pf"
  curl -s --max-time 10 -c "$ck" -X POST --data @"$pf" \
    "http://127.0.0.1:${port}${bp}/login" -o /dev/null
  rm -f "$pf"
  echo
  curl -s --max-time 10 -b "$ck" "http://127.0.0.1:${port}${bp}/api/tunnels" \
    > "$ck.json" 2>/dev/null
  rm -f "$ck"

  # 用 sed/awk 解析而不是 python3/jq：Alpine 最小安装两者都没有，
  # 为了一条列表命令再拉依赖不值当。字段固定，按对象拆行足够稳。
  if [[ ! -s "$ck.json" ]] || ! grep -q '"port"' "$ck.json" 2>/dev/null; then
    echo "  还没有隧道，去网页里添加"
  else
    printf "  %-10s%-11s%-18s%s\n" "端口" "状态" "出口 IP" "节点"
    # 按 {"slot" 切分而不是按 }：node 是嵌套对象，按 } 切会把一条记录劈成两半
    sed 's/{"slot"/\n{"slot"/g' "$ck.json" | while IFS= read -r line; do
      case "$line" in *'"slot"'*) ;; *) continue ;; esac
      p=$(echo "$line"  | sed -n 's/.*"port":\([0-9]*\).*/\1/p')
      st=$(echo "$line" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')
      ip=$(echo "$line" | sed -n 's/.*"exit_ip":"\([^"]*\)".*/\1/p')
      hn=$(echo "$line" | sed -n 's/.*"hostname":"\([^"]*\)".*/\1/p')
      [[ -z $p ]] && continue
      printf "  %-10s%-11s%-18s%s\n" "$p" "${st:--}" "${ip:--}" "${hn:--}"
    done
  fi
  rm -f "$ck.json"
}

change_port() {
  local cur new
  cur=$(web_port)
  echo
  read -rp "  新端口 (当前 ${cur}): " new
  [[ -z $new ]] && { echo "  未修改"; return; }
  if ! [[ $new =~ ^[0-9]+$ ]] || (( new < 1 || new > 65535 )); then
    echo -e "  ${R}端口不合法${N}"; return
  fi
  if ss -tln 2>/dev/null | grep -q ":${new} "; then
    echo -e "  ${R}端口 ${new} 已被占用${N}"; return
  fi
  # 写 config.json 的 web.port（权威来源），并把服务文件里可能残留的 -web 一并同步，
  # 免得老安装重启后又被写死的旧端口拽回去。
  cfg_set_web port "$new"
  # sed 直接改服务文件：先备份，失败恢复
  if [[ -n ${UNIT:-} && -f $UNIT ]]; then
    cp -f "$UNIT" "$UNIT.bak" 2>/dev/null
    if sed -i "s/-web ${cur}/-web ${new}/" "$UNIT" 2>/dev/null; then
      rm -f "$UNIT.bak"
    else
      mv -f "$UNIT.bak" "$UNIT" 2>/dev/null
      echo -e "  ${Y}服务文件更新失败，已恢复原文件${N}"
    fi
  fi
  svc_reload
  svc_restart
  echo -e "  ${G}已改为 ${new} 并重启${N}"
}

reset_password() {
  local pw
  echo
  read -rsp "  新口令 (留空则随机生成): " pw
  echo
  if [[ -z $pw ]]; then
    pw=$(head -c 9 /dev/urandom | od -An -tx1 | tr -d ' \n')
  fi
  cfg_set password "$pw"
  # 清掉已登录的会话，强制所有人用新口令重新登录
  python3 - "$CFG" <<'PY' 2>/dev/null
import json, sys
f = sys.argv[1]
d = json.load(open(f))
d['sessions'] = {}
json.dump(d, open(f, 'w'), indent=2)
PY
  chmod 600 "$CFG"
  svc_restart
  echo -e "  ${G}新口令: ${pw}${N}"
}

reset_basepath() {
  local bp
  echo
  read -rp "  新访问路径 (留空则随机生成): " bp
  if [[ -z $bp ]]; then
    bp=$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n' | head -c 12)
  else
    bp=${bp#/}; bp=${bp%/}
  fi
  # 统一存成 /xxx 格式，和程序里的 normalizeBasePath 一致
  [[ -n $bp ]] && bp="/${bp}"
  cfg_set basepath "$bp"
  svc_restart
  echo -e "  ${G}新路径: ${bp:-/}/${N}"
}

ipv6_state() {
  local a d
  a=$(sysctl -n net.ipv6.conf.all.disable_ipv6 2>/dev/null || echo 0)
  d=$(sysctl -n net.ipv6.conf.default.disable_ipv6 2>/dev/null || echo 0)
  [[ "$a" == 1 && "$d" == 1 ]] && echo disabled || echo enabled
}

toggle_ipv6() {
  local conf=/etc/sysctl.d/99-home-broadband-ipv6.conf
  echo
  if [[ $(ipv6_state) == disabled ]]; then
    read -rp "  当前已禁用 IPv6，要重新启用吗？[y/N]: " yes
    [[ ${yes,,} == y ]] || { echo "  已取消"; return; }
    rm -f "$conf"
    sysctl -qw net.ipv6.conf.all.disable_ipv6=0
    sysctl -qw net.ipv6.conf.default.disable_ipv6=0
    sysctl -qw net.ipv6.conf.lo.disable_ipv6=0
    echo -e "  ${G}已重新启用 IPv6${N}"
    return
  fi

  echo -e "  ${D}母机有全局 IPv6 时，没走隧道的流量可能从 IPv6 出去，暴露真实地址。${N}"
  read -rp "  确认禁用整机 IPv6？[y/N]: " yes
  [[ ${yes,,} == y ]] || { echo "  已取消"; return; }

  cat > "$conf" <<EOF
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1
net.ipv6.conf.lo.disable_ipv6 = 1
EOF
  sysctl -qw net.ipv6.conf.all.disable_ipv6=1
  sysctl -qw net.ipv6.conf.default.disable_ipv6=1
  sysctl -qw net.ipv6.conf.lo.disable_ipv6=1
  svc_restart >/dev/null 2>&1
  echo -e "  ${G}已禁用 IPv6（重启后依然生效）${N}"
}

show_links() {
  echo
  echo -e "  项目    ${B}https://github.com/hao6789/Home-Broadband${N}"
  echo
  echo -e "  ${D}用着有问题、或者想要什么功能，提 issue。${N}"
}

# ── 证书管理：给面板办 HTTPS 证书 ──────────────────────
ACME_BIN=/root/.acme.sh/acme.sh
CERT_BASE=/root/cert

# 找 acme.sh：3x-ui 装过就直接复用，没有才自己装
ensure_acme() {
  if [[ -x $ACME_BIN ]]; then
    echo "  找到已有的 acme.sh，直接复用"
    "$ACME_BIN" --upgrade >/dev/null 2>&1 || true
    return 0
  fi
  echo "  正在安装 acme.sh..."
  curl -s https://get.acme.sh | sh || { echo -e "  ${R}acme.sh 安装失败${N}"; return 1; }
  [[ -x $ACME_BIN ]] || { echo -e "  ${R}acme.sh 安装失败${N}"; return 1; }
  echo "  acme.sh 安装成功"
}

ensure_socat() {
  command -v socat >/dev/null && return 0
  echo "  正在安装 socat..."
  if command -v apt-get >/dev/null; then
    apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq socat
  elif command -v dnf >/dev/null; then dnf install -y -q socat
  elif command -v yum >/dev/null; then yum install -y -q socat
  elif command -v apk >/dev/null; then apk add --no-cache socat
  else return 1; fi
}

# 从 3x-ui 现有证书里读域名，当申请时的默认值
default_domain() {
  local crt
  crt=$(ls "$CERT_BASE"/*/fullchain.pem 2>/dev/null | head -1)
  [[ -n $crt ]] || return 0
  openssl x509 -noout -subject -in "$crt" 2>/dev/null | sed -n 's/.*CN *= *\([^/,]*\).*/\1/p'
}

cert_domains() {
  find "$CERT_BASE" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; 2>/dev/null
}

cert_apply() {
  ensure_acme || return
  ensure_socat || { echo -e "  ${R}socat 安装失败，请手动安装后重试${N}"; return; }

  local domain dflt
  dflt=$(default_domain)
  echo
  if [[ -n $dflt ]]; then
    read -rp "  域名 (回车用 ${dflt}): " domain
    [[ -z $domain ]] && domain=$dflt
  else
    read -rp "  域名: " domain
  fi
  [[ -z $domain ]] && { echo "  已取消"; return; }
  # domain 拼进 mkdir 路径，严格校验防止路径逃逸（与 cert_revoke 一致）
  if [[ $domain == *".."* || $domain == /* || $domain == *~* || ! $domain =~ ^[A-Za-z0-9.-]+$ ]]; then
    echo -e "  ${R}域名不合法${N}"; return
  fi

  # 预检：域名解析到本机？（getent 在 Alpine/musl 下可能没有，失败就跳过这项检查）
  local myip dip
  myip=$(curl -s --max-time 6 http://api.ipify.org 2>/dev/null)
  if command -v getent >/dev/null 2>&1; then
    dip=$(getent hosts "$domain" 2>/dev/null | awk '{print $1}' | head -1)
  elif command -v nslookup >/dev/null 2>&1; then
    dip=$(nslookup "$domain" 2>/dev/null | awk '/^Address: /{print $2}' | tail -1)
  fi
  if [[ -n $myip && -n $dip && $myip != "$dip" ]]; then
    echo -e "  ${Y}警告：域名解析到 ${dip}，本机 IP 是 ${myip}，签发会失败${N}"
    read -rp "  还是继续吗？[y/N]: " yes
    [[ ${yes,,} == y ]] || { echo "  已取消"; return; }
  fi
  # 预检：80 端口空闲？
  if ss -tln 2>/dev/null | grep -q ':80 '; then
    echo -e "  ${Y}警告：80 端口被占用，签发需要空出 80 端口${N}"
    ss -tlnp 2>/dev/null | grep ':80 ' | head -3 | sed 's/^/    /'
    read -rp "  还是继续吗？[y/N]: " yes
    [[ ${yes,,} == y ]] || { echo "  已取消"; return; }
  fi

  echo "  正在签发证书..."
  if ! "$ACME_BIN" --issue -d "$domain" --standalone; then
    echo -e "  ${R}签发失败${N}"; return
  fi

  local cdir="${CERT_BASE}/${domain}"
  mkdir -p "$cdir"
  # 续期后自动重启面板；两个面板用不同域名，各自独立续期互不干扰
  local reload="systemctl restart $SERVICE 2>/dev/null || rc-service $SERVICE restart"
  if ! "$ACME_BIN" --install-cert -d "$domain" \
      --key-file "$cdir/privkey.pem" \
      --fullchain-file "$cdir/fullchain.pem" \
      --reloadcmd "$reload"; then
    echo -e "  ${R}证书安装失败${N}"; return
  fi
  chmod 600 "$cdir/privkey.pem"

  cfg_set_web tls_cert "$cdir/fullchain.pem"
  cfg_set_web tls_key "$cdir/privkey.pem"
  svc_restart
  local port bp
  port=$(web_port); bp=$(cfg_get basepath)
  echo -e "  ${G}证书已启用${N}"
  echo -e "  面板地址  ${B}https://$(public_ip):${port}${bp}/${N}"
}

cert_show() {
  local found=0 d crt key domain end
  echo
  for d in "$CERT_BASE"/*/; do
    [[ -d $d ]] || continue
    crt="${d}fullchain.pem"; key="${d}privkey.pem"
    [[ -f $crt ]] || continue
    found=1
    domain=$(basename "$d")
    end=$(openssl x509 -noout -enddate -in "$crt" 2>/dev/null | cut -d= -f2)
    echo -e "  域名      ${domain}"
    echo -e "  证书      ${crt}"
    if [[ -f $key ]]; then echo -e "  私钥      ${key}"
    else echo -e "  ${R}私钥缺失${N}"; fi
    echo -e "  有效期至  ${end}"
    echo
  done
  [[ $found == 0 ]] && echo "  还没有证书，用「申请证书」办一张"
}

cert_renew() {
  ensure_acme || return
  local domains domain
  domains=$(cert_domains)
  [[ -z $domains ]] && { echo "  还没有证书"; return; }
  echo "  已有域名："; echo "$domains" | sed 's/^/    /'
  read -rp "  输入要续期的域名: " domain
  [[ -z $domain ]] && { echo "  已取消"; return; }
  if "$ACME_BIN" --renew -d "$domain" --force; then
    echo -e "  ${G}续期成功${N}"
  else
    echo -e "  ${R}续期失败${N}"
  fi
}

cert_revoke() {
  ensure_acme || return
  local domains domain yes
  domains=$(cert_domains)
  [[ -z $domains ]] && { echo "  还没有证书"; return; }
  echo "  已有域名："; echo "$domains" | sed 's/^/    /'
  read -rp "  输入要吊销的域名: " domain
  [[ -z $domain ]] && { echo "  已取消"; return; }
  # domain 直接拼进 rm -rf 路径，严格校验防止路径逃逸
  if [[ $domain == *".."* || $domain == /* || $domain == *~* || ! $domain =~ ^[A-Za-z0-9.-]+$ ]]; then
    echo -e "  ${R}域名不合法${N}"; return
  fi
  read -rp "  确认吊销 ${domain} 的证书？[y/N]: " yes
  [[ ${yes,,} == y ]] || { echo "  已取消"; return; }
  "$ACME_BIN" --revoke -d "$domain" 2>/dev/null
  rm -rf "${CERT_BASE}/${domain}"
  # 面板切回 HTTP
  cfg_set_web tls_cert ""
  cfg_set_web tls_key ""
  svc_restart
  echo -e "  ${G}已吊销，面板切回 HTTP${N}"
}

cert_menu() {
  while true; do
    clear
    echo -e "${B}  证书管理${N}  ${D}给面板办 HTTPS 证书${N}"
    echo
    echo "   1 申请证书"
    echo "   2 查看证书"
    echo "   3 强制续期"
    echo "   4 吊销证书"
    echo
    echo "   0 返回"
    echo
    read -rp "  选择: " c
    case "$c" in
      1) cert_apply; pause ;;
      2) cert_show; pause ;;
      3) cert_renew; pause ;;
      4) cert_revoke; pause ;;
      0) return ;;
    esac
  done
}

# 老版本把 -web 写死在服务文件里，和配置互相拽回旧值。
# 更新时把端口搬进 config.json 再从服务文件里摘掉，之后只认一处。
migrate_port_to_settings() {
  local unit_port
  unit_port=$(grep -oE '\-web [0-9]+' "$UNIT" 2>/dev/null | grep -oE '[0-9]+' | head -1)
  [[ -z $unit_port ]] && return

  if [[ ! -f $CFG ]]; then
    # config.json 还不存在（极老版本），先建一个最小的
    printf '{\n  "version": 1,\n  "web": {\n    "port": %s,\n    "listen_addr": ""\n  }\n}\n' "$unit_port" > "$CFG"
    chmod 600 "$CFG"
  else
    cfg_set_web port "$unit_port"
  fi
  sed -i "s/-web ${unit_port} //" "$UNIT"
  svc_reload
  echo "  已把端口 ${unit_port} 迁移到 config.json"
}

do_update() {
  local arch goarch tmp
  arch=$(uname -m)
  case "$arch" in
    x86_64) goarch=amd64 ;;
    aarch64|arm64) goarch=arm64 ;;
    *) echo -e "  ${R}不支持的架构 ${arch}${N}"; return ;;
  esac

  # 新版 h.sh 读写 config.json 依赖 python3，老机器可能没有
  if ! command -v python3 >/dev/null; then
    echo "  正在安装 python3..."
    if command -v apt-get >/dev/null; then
      apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq python3
    elif command -v dnf >/dev/null; then dnf install -y -q python3
    elif command -v yum >/dev/null; then yum install -y -q python3
    elif command -v apk >/dev/null; then apk add --no-cache python3
    elif command -v pacman >/dev/null; then pacman -Sy --noconfirm --needed python3
    else echo -e "  ${Y}装不上 python3，菜单可能显示不全${N}"; fi
  fi

  echo -e "\n  当前 $("$BIN" -version 2>/dev/null || echo '-')"
  # 版本比较：已是最新就跳过，避免无意义的重装重启。
  # 走 releases/latest/download 的 302 跳转取版本号，不调 api.github.com（未认证 60 次/小时限额）。
  latest_ver=$(curl -fsSIL --max-time 15 -o /dev/null -w '%{redirect_url}' \
    "https://github.com/${REPO}/releases/latest/download/home-broadband-linux-${goarch}.tar.gz" 2>/dev/null \
    | sed -n 's#.*/releases/download/\([^/]*\)/.*#\1#p')
  cur_ver=$("$BIN" -version 2>/dev/null | awk '{print $2}')
  if [[ -n $latest_ver && -n $cur_ver && $cur_ver == "$latest_ver" ]]; then
    echo -e "  ${G}已是最新版 ${latest_ver}，无需更新${N}"; return
  fi
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' RETURN
  echo "  正在下载最新版..."
  arch="home-broadband-linux-${goarch}.tar.gz"
  if ! curl -fsSL "https://github.com/${REPO}/releases/latest/download/${arch}" \
       -o "$tmp/f.tar.gz"; then
    echo -e "  ${R}下载失败${N}"; return
  fi
  # 校验 checksums.txt，防止装上损坏/被篡改的包
  if ! curl -fsSL "https://github.com/${REPO}/releases/latest/download/checksums.txt" \
       -o "$tmp/checksums.txt"; then
    echo -e "  ${R}下载校验文件失败，拒绝安装${N}"; return
  fi
  want=$(grep "  ${arch}$" "$tmp/checksums.txt" | awk '{print $1}')
  got=$(sha256sum "$tmp/f.tar.gz" | awk '{print $1}')
  if [ -z "$want" ] || [ "$want" != "$got" ]; then
    echo -e "  ${R}校验失败，拒绝安装${N}"; return
  fi
  if ! tar xzf "$tmp/f.tar.gz" -C "$tmp"; then
    echo -e "  ${R}解压失败${N}"; return
  fi
  svc_stop
  # 从这里起若被 Ctrl-C 中断，保证把服务拉起来，不留停机状态
  trap 'rm -rf "$tmp"; svc_start >/dev/null 2>&1; echo -e "  ${Y}更新被中断，已恢复服务${N}"' INT TERM
  # 备份旧二进制，出问题可手动恢复
  [[ -x $BIN ]] && cp -f "$BIN" "$BIN.bak"
  if ! install -m 755 "$tmp/home-broadband" "$BIN"; then
    echo -e "  ${R}安装新二进制失败${N}"
    svc_start
    trap - INT TERM; trap - RETURN; rm -rf "$tmp"
    return 1
  fi
  migrate_port_to_settings
  svc_start
  trap - INT TERM
  trap - RETURN
  rm -rf "$tmp"
  echo -e "  ${G}已更新到 $("$BIN" -version 2>/dev/null)${N}"
}

do_uninstall() {
  local yes
  echo
  read -rp "  确认卸载？隧道和配置都会删除 [y/N]: " yes
  [[ ${yes,,} == y ]] || { echo "  已取消"; return; }

  svc_stop >/dev/null 2>&1
  svc_disable
  # 清掉残留的 netns 与 veth
  # 命名规则：默认实例 hb1/hb2…，自定义 WORK_DIR 时是 hb<4位hex>（如 hba3f2）
  for ns in $(ip netns list 2>/dev/null | awk '{print $1}' | grep -E '^hb[0-9a-f]+$'); do
    ip netns del "$ns" 2>/dev/null
  done
  # ip -o 输出的 veth 名带 @ifN 后缀（如 hbv3@if12），要先 strip 掉再删
  for l in $(ip -o link show 2>/dev/null | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -E '^hbv[0-9a-f]+$'); do
    ip link del "$l" 2>/dev/null
  done
  # 删掉安装时加的 iptables FORWARD 规则（persist 的那份也清）
  for dir in "-s" "-d"; do
    while iptables -C FORWARD $dir 10.99.0.0/16 -j ACCEPT 2>/dev/null; do
      iptables -D FORWARD $dir 10.99.0.0/16 -j ACCEPT 2>/dev/null
    done
  done
  command -v netfilter-persistent >/dev/null && netfilter-persistent save >/dev/null 2>&1 || true
  # 证书、IPv6 禁用配置、日志一并清理；acme.sh 本体保留（可能是 3x-ui 在用）
  rm -rf "${CERT_BASE}"
  rm -f /etc/sysctl.d/99-home-broadband-ipv6.conf /var/log/${SERVICE}.log
  rm -f "$UNIT" "$BIN" "$BIN.bak" /usr/local/bin/h
  rm -rf "$WORK_DIR"
  svc_reload
  echo -e "  ${G}已卸载${N}"
  echo -e "  ${D}注意：/etc/sysctl.conf 里的 net.ipv4.ip_forward=1 未动（系统级改动，留着无害）${N}"
  exit 0
}

menu() {
  while true; do
    clear
    echo -e "${B}  ★ home-broadband${N}  ${D}VPN Gate 出口网关${N}"
    show_info
    echo
    echo -e "  ${D}[ 服务 ]${N}"
    echo "   1 启动    2 停止    3 重启    4 日志"
    echo
    echo -e "  ${D}[ 查看 ]${N}"
    echo "   5 隧道列表    6 连接信息"
    echo
    echo -e "  ${D}[ 配置 ]${N}"
    echo "   7 改端口    8 改口令    9 改访问路径    10 开机自启"
    echo
    echo -e "  ${D}[ 其他 ]${N}"
    echo "  11 更新    12 卸载"
    echo "  13 反馈    14 证书管理"
    echo
    echo "   0 退出"
    echo
    read -rp "  选择: " choice

    case "$choice" in
      1) svc_start   && echo -e "\n  ${G}已启动${N}"; pause ;;
      2) svc_stop    && echo -e "\n  ${Y}已停止${N}"; pause ;;
      3) svc_restart && echo -e "\n  ${G}已重启${N}"; pause ;;
      4) echo; svc_logs 40; pause ;;
      5) list_tunnels; pause ;;
      6) show_info; pause ;;
      7) change_port; pause ;;
      8) reset_password; pause ;;
      9) reset_basepath; pause ;;
      10)
        if svc_is_enabled; then
          svc_disable
          echo -e "\n  ${Y}已关闭开机自启${N}"
        else
          svc_enable
          echo -e "\n  ${G}已开启开机自启${N}"
        fi
        pause ;;
      11) do_update; pause ;;
      13) show_links; pause ;;
      14) cert_menu ;;
      12) do_uninstall; pause ;;
      0) exit 0 ;;
      *) ;;
    esac
  done
}

need_root

# 带参数时当普通命令用，不进菜单
case "${1:-}" in
  start)    svc_start ;;
  stop)     svc_stop ;;
  restart)  svc_restart ;;
  status)   svc_status_page ;;
  log)      svc_logs_follow ;;
  info)     show_info ;;
  list)     list_tunnels ;;
  update)   do_update ;;
  uninstall) do_uninstall ;;
  "")       menu ;;
  *)
    echo "用法: h [start|stop|restart|status|log|info|list|update|uninstall]"
    echo "不带参数进入交互菜单"
    ;;
esac
