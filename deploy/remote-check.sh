#!/usr/bin/env bash
# zoek 部署自检（在服务器上运行）
#
# 用法：
#   bash remote-check.sh [ROOT] [SVC] [PORT] [DOMAIN]
#   默认： /opt/zoek  zoek-api  8090  （DOMAIN 留空则跳过 HTTPS 检查）
#
# 通常不用手动跑，本地执行 `make check-deploy` 或 `scripts/release.sh --check`
# 会把它上传到 /tmp 后执行。
set -u

ROOT="${1:-/opt/zoek}"
SVC="${2:-zoek-api}"
PORT="${3:-8090}"
DOMAIN="${4:-}"

fail=0
warn=0
ok()  { printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad() { printf '  \033[31m✗\033[0m %s\n' "$*"; fail=$((fail + 1)); }
meh() { printf '  \033[33m!\033[0m %s\n' "$*"; warn=$((warn + 1)); }
sec() { printf '\n\033[36m== %s ==\033[0m\n' "$*"; }

# 从 yaml 取值：cfg_get <file> <section> <key>
cfg_get() {
  awk -v sec="$2" -v key="$3" '
    $0 ~ "^" sec ":" { insec = 1; next }
    /^[^[:space:]#]/ { insec = 0 }
    insec {
      line = $0
      sub(/[[:space:]]*#.*$/, "", line)
      sub(/^[[:space:]]+/, "", line)
      if (line ~ "^" key ":") {
        sub("^" key ":[[:space:]]*", "", line)
        gsub(/"/, "", line)
        gsub(/[[:space:]]+$/, "", line)
        print line
        exit
      }
    }
  ' "$1" 2>/dev/null
}

printf '\033[1mzoek 部署自检\033[0m  root=%s  service=%s  port=%s%s\n' \
  "$ROOT" "$SVC" "$PORT" "${DOMAIN:+  domain=$DOMAIN}"

# ---------- 1. 文件 ----------
sec "1. 部署文件"
if [ -x "$ROOT/api" ]; then
  ok "$ROOT/api 存在且可执行（$(stat -c '%s' "$ROOT/api" 2>/dev/null | awk '{printf "%.1fMB", $1/1048576}')，$(stat -c '%y' "$ROOT/api" 2>/dev/null | cut -d. -f1)）"
else
  bad "缺少可执行文件 $ROOT/api"
fi

CFG="$ROOT/config.yaml"
CFG_PORT=""
DB_HOST=""
DB_PORT=""
DB_USER=""
DB_PASS=""
DB_NAME=""
if [ -f "$CFG" ]; then
  ok "$CFG 存在"
  CFG_PORT="$(cfg_get "$CFG" server port)"
  DB_HOST="$(cfg_get "$CFG" database host)"
  DB_PORT="$(cfg_get "$CFG" database port)"
  DB_USER="$(cfg_get "$CFG" database user)"
  DB_PASS="$(cfg_get "$CFG" database password)"
  DB_NAME="$(cfg_get "$CFG" database dbname)"
else
  bad "缺少 $CFG —— 服务会退回内置默认值（端口 8080、数据库密码为空、JWT 用开发密钥）"
fi

if [ -d "$ROOT/uploads/avatars" ]; then
  ok "$ROOT/uploads/avatars 存在（属主 $(stat -c '%U:%G' "$ROOT/uploads" 2>/dev/null)）"
else
  bad "缺少 $ROOT/uploads/avatars（头像上传会 404/500）"
fi

# ---------- 2. systemd ----------
sec "2. systemd 服务"
if systemctl list-unit-files "$SVC.service" >/dev/null 2>&1 && systemctl cat "$SVC" >/dev/null 2>&1; then
  ok "$SVC.service 已安装：$(systemctl show -p FragmentPath --value "$SVC" 2>/dev/null)"
else
  bad "$SVC.service 未安装（首次部署需把 deploy/zoek-api.service 放到 /etc/systemd/system/）"
fi

UNIT_TEXT="$(systemctl cat "$SVC" 2>/dev/null)"
if printf '%s' "$UNIT_TEXT" | grep -q -- '-config'; then
  ok "ExecStart 带 -config，会读取 $CFG"
else
  bad "ExecStart 没有 -config —— 服务不会读 $CFG，整台服务跑在默认值上"
fi

SVC_USER="$(systemctl show -p User --value "$SVC" 2>/dev/null)"
SVC_STATE="$(systemctl is-active "$SVC" 2>/dev/null)"
N_RESTART="$(systemctl show -p NRestarts --value "$SVC" 2>/dev/null)"
if [ "$SVC_STATE" = "active" ]; then
  ok "$SVC 正在运行（User=${SVC_USER:-未指定}）"
else
  bad "$SVC 状态：$SVC_STATE"
fi
if [ "${N_RESTART:-0}" -gt 5 ] 2>/dev/null; then
  meh "已重启 $N_RESTART 次 —— 处于崩溃循环，看日志：journalctl -u $SVC -n 50 --no-pager"
fi

if [ -n "${SVC_USER:-}" ] && [ -n "$(stat -c '%U' "$ROOT/uploads" 2>/dev/null)" ]; then
  if [ "$(stat -c '%U' "$ROOT/uploads" 2>/dev/null)" = "$SVC_USER" ]; then
    ok "uploads 属主与服务用户一致（$SVC_USER）"
  else
    meh "uploads 属主 $(stat -c '%U' "$ROOT/uploads") ≠ 服务用户 $SVC_USER，头像写盘可能失败"
  fi
fi

# ---------- 3. 配置项 ----------
sec "3. 配置项"
if [ -n "$CFG_PORT" ]; then
  if [ "$CFG_PORT" = "$PORT" ]; then
    ok "server.port=$CFG_PORT，与 Caddy/健康检查端口一致"
  else
    bad "server.port=$CFG_PORT ≠ $PORT —— Caddy 反代和健康检查都指向 $PORT"
  fi
else
  [ -f "$CFG" ] && bad "$CFG 里读不到 server.port"
fi

if [ -n "$DB_PASS" ]; then
  ok "database.password 已设置（长度 ${#DB_PASS}）"
else
  bad "database.password 为空 —— MySQL 会报 Access denied (using password: NO)"
fi
[ -n "$DB_USER" ] && ok "database.user=$DB_USER  dbname=${DB_NAME:-未填}  host=${DB_HOST:-未填}:${DB_PORT:-未填}"

if [ -f "$CFG" ]; then
  MODE="$(cfg_get "$CFG" server mode)"
  [ "$MODE" = "release" ] && ok "server.mode=release" || meh "server.mode=${MODE:-未填}，线上建议 release"
  JWT="$(cfg_get "$CFG" jwt secret)"
  case "$JWT" in
    ""|zoek-dev-secret-change-in-production) bad "jwt.secret 还是默认/空值，必须换成 make jwt-secret 生成的值" ;;
    *) ok "jwt.secret 已自定义" ;;
  esac
  WX_ID="$(cfg_get "$CFG" wechat appid)"
  WX_SECRET="$(cfg_get "$CFG" wechat app_secret)"
  if [ -n "$WX_ID" ] && [ -n "$WX_SECRET" ] && [ "$WX_ID" != "your_appid" ]; then
    ok "wechat.appid/app_secret 已填（$WX_ID）"
  else
    bad "wechat.appid/app_secret 未填 —— 小程序登录（code2session）会失败"
  fi
  TTS_ID="$(cfg_get "$CFG" tts secret_id)"
  [ -n "$TTS_ID" ] && ok "tts.secret_id 已填（粤语报分可用）" || meh "tts.secret_id 为空，粤语报分会回落微信同声传译"
fi

# ---------- 4. 数据库 ----------
sec "4. 数据库"
if ss -lnt 2>/dev/null | grep -qE "127\.0\.0\.1:${DB_PORT:-3306}|0\.0\.0\.0:${DB_PORT:-3306}|\[::\]:${DB_PORT:-3306}"; then
  ok "MySQL 端口 ${DB_PORT:-3306} 已监听"
else
  bad "MySQL 端口 ${DB_PORT:-3306} 未监听（systemctl status mysql / mariadb）"
fi

if command -v mysql >/dev/null 2>&1 && [ -n "$DB_USER" ]; then
  TMP_CNF="$(mktemp)"
  chmod 600 "$TMP_CNF"
  printf '[client]\nhost=%s\nport=%s\nuser=%s\npassword=%s\n' \
    "${DB_HOST:-127.0.0.1}" "${DB_PORT:-3306}" "$DB_USER" "$DB_PASS" >"$TMP_CNF"
  ERRTXT="$(mysql --defaults-extra-file="$TMP_CNF" -N -B -e 'SELECT 1' 2>&1 >/dev/null | head -1)"
  if [ -z "$ERRTXT" ]; then
    ok "用 config.yaml 里的账号能连上 MySQL"
    if mysql --defaults-extra-file="$TMP_CNF" -N -B -e "SHOW DATABASES LIKE '${DB_NAME:-zoek}'" 2>/dev/null | grep -q .; then
      ok "数据库 ${DB_NAME:-zoek} 已存在"
    else
      bad "数据库 ${DB_NAME:-zoek} 不存在：CREATE DATABASE ${DB_NAME:-zoek} DEFAULT CHARACTER SET utf8mb4;"
    fi
  else
    bad "MySQL 连接失败：$ERRTXT"
  fi
  rm -f "$TMP_CNF"
else
  meh "服务器没有 mysql 客户端或未填 database.user，跳过连通性检查"
fi

# ---------- 5. 服务端口与健康检查 ----------
sec "5. 服务与健康检查"
if ss -lnt 2>/dev/null | grep -q ":${PORT} "; then
  ok "端口 $PORT 已监听"
else
  bad "端口 $PORT 未监听（服务没起来或配置端口不对）"
fi
if [ "$PORT" != "8080" ] && ss -lnt 2>/dev/null | grep -q ":8080 "; then
  meh "8080 也在监听 —— 若是本服务跑成了默认端口，就是没读到 config.yaml"
fi
if curl -sf --max-time 3 "http://127.0.0.1:${PORT}/health" >/dev/null 2>&1; then
  ok "本机 http://127.0.0.1:${PORT}/health 返回 2xx"
else
  bad "本机 http://127.0.0.1:${PORT}/health 不通"
fi

# ---------- 6. Caddy ----------
sec "6. Caddy"
if systemctl is-active --quiet caddy; then
  ok "caddy 运行中"
else
  bad "caddy 未运行：systemctl status caddy"
fi
if grep -qs 'import conf.d/' /etc/caddy/Caddyfile; then
  ok "主 Caddyfile 有 import conf.d/*.caddy"
else
  meh "主 Caddyfile 没有 import conf.d/ —— 站点文件不会被加载"
fi
if grep -rqs "127.0.0.1:${PORT}" /etc/caddy/conf.d/ 2>/dev/null; then
  ok "conf.d 里有反代到 127.0.0.1:$PORT 的站点"
else
  bad "conf.d 里没有指向 127.0.0.1:$PORT 的站点"
fi
ss -lnt 2>/dev/null | grep -q ':443 ' && ok "443 已监听" || bad "443 未监听（Caddy 无法签发证书）"
ss -lnt 2>/dev/null | grep -q ':80 ' && ok "80 已监听" || meh "80 未监听（Let's Encrypt HTTP-01 验证会失败）"

if [ -n "$DOMAIN" ]; then
  CODE="$(curl -s -o /dev/null -w '%{http_code}' --max-time 8 "https://${DOMAIN}/health" 2>/dev/null || echo 000)"
  if [ "$CODE" = "200" ]; then
    ok "https://${DOMAIN}/health → 200"
  else
    bad "https://${DOMAIN}/health → $CODE（检查域名解析、证书、云安全组 80/443；证书见第 7 节）"
  fi
  CUR="$(curl -s --max-time 8 "https://${DOMAIN}/health" 2>/dev/null | head -c 200)"
  [ -n "$CUR" ] && printf '      响应：%s\n' "$CUR"
fi

# ---------- 7. 证书与 ACME ----------
sec "7. 证书与 ACME"
CERT_DIR="/var/lib/caddy/.local/share/caddy/certificates"
SELF_SIGNED=0
grep -rqs 'tls internal' /etc/caddy/conf.d/ 2>/dev/null && SELF_SIGNED=1

if [ -n "$DOMAIN" ]; then
  if [ "$SELF_SIGNED" = "1" ]; then
    meh "站点用 \`tls internal\`（自签源站证书）—— 仅当前置 CDN/ESA 不校验源站证书时可行"
  fi

  CRT="$(find "$CERT_DIR" -maxdepth 3 -name "${DOMAIN}.crt" 2>/dev/null | head -1)"
  if [ -n "$CRT" ]; then
    ISSUER="$(openssl x509 -in "$CRT" -noout -issuer 2>/dev/null | sed 's/^issuer=//')"
    EXP="$(openssl x509 -in "$CRT" -noout -enddate 2>/dev/null | cut -d= -f2)"
    case "$ISSUER" in
      *"Caddy Local Authority"*)
        meh "源站证书是 Caddy 自签（到期 $EXP）—— 依赖前置 CDN/ESA 不校验源站证书" ;;
      *)
        ok "源站 ACME 证书已签发（到期 $EXP，签发者 $ISSUER）" ;;
    esac
  elif [ "$SELF_SIGNED" = "1" ]; then
    meh "源站走自签证书，但没找到已签发的文件（Caddy 可能还没签）"
  else
    bad "$DOMAIN 源站没有证书 —— 前置 CDN/ESA 回源会报 525 Origin SSL Handshake Error"
  fi

  SNI_OUT="$(echo | openssl s_client -connect "127.0.0.1:443" -servername "$DOMAIN" 2>/dev/null | openssl x509 -noout -subject 2>/dev/null)"
  if [ -n "$SNI_OUT" ]; then
    ok "本机 443 以 SNI=$DOMAIN 可完成握手"
  else
    bad "本机 443 以 SNI=$DOMAIN 握手失败（Caddy 手上没有该域名证书）"
  fi
fi

acme_probe() {
  local url="$1" name="$2" code
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url" 2>/dev/null)"
  case "$code" in ""|000) code="000（连接超时/被挡）" ;; esac
  if [ "$code" = "200" ]; then
    ok "ACME «$name» 可达"
  else
    meh "ACME «$name» 不可达（HTTP $code）—— 不要把它写进 Caddy 的 tls.ca"
  fi
}
acme_probe "https://acme-v02.api.letsencrypt.org/directory" "Let's Encrypt（走 Cloudflare，国内常被挡）"
acme_probe "https://acme.zerossl.com/v2/DV90" "ZeroSSL（强制 EAB，需 API Key）"

# ---------- 汇总 ----------
sec "汇总"
if [ "$fail" -eq 0 ] && [ "$warn" -eq 0 ]; then
  printf '  \033[32m全部通过\033[0m\n'
elif [ "$fail" -eq 0 ]; then
  printf '  \033[33m%d 项提醒\033[0m，没有阻塞项\n' "$warn"
else
  printf '  \033[31m%d 项必须修\033[0m，%d 项提醒\n' "$fail" "$warn"
fi
exit "$([ "$fail" -eq 0 ] && echo 0 || echo 1)"
