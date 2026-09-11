#!/usr/bin/env bash
# 雀友记（zoek）发版：backend 二进制 → /opt/zoek
#
# 用法：
#   ./scripts/release.sh              # 构建 + 上传 + 重启
#   ./scripts/release.sh backend
#   ./scripts/release.sh --build-only backend   # 只构建不上传
#   ./scripts/release.sh --upload-only backend  # 只上传已有产物（需先 build）
#
# 配置：
#   cp deploy/release.env.example deploy/release.env
#   编辑 SSH_HOST、REMOTE_API_PORT 等；未配置 SSH_HOST 时仅本地构建
#
# 首次部署清单（详见 deploy/README.md）：
#   1. 服务器准备 /opt/zoek/config.yaml（参考 backend/config.yaml，改端口/数据库/密钥）
#   2. 建库：CREATE DATABASE zoek DEFAULT CHARACTER SET utf8mb4;
#   3. Caddy 站点：拷 deploy/caddy/conf.d/zoek.caddy 到 /etc/caddy/conf.d/ 并改域名
#   4. 小程序后台 request 合法域名配置为该域名

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${RELEASE_ENV_FILE:-$ROOT/deploy/release.env}"
BUILD_ONLY=0
UPLOAD_ONLY=0
DO_BACKEND=0
TARGETS=()

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
yellow(){ printf '\033[33m%s\033[0m\n' "$*"; }
info()  { printf '\033[36m==>\033[0m %s\n' "$*"; }
die()   { red "错误: $*"; exit 1; }

usage() {
  sed -n '2,16p' "$0"
}

# ---------- 参数 ----------
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help|help) usage; exit 0 ;;
    --build-only)   BUILD_ONLY=1; shift ;;
    --upload-only)  UPLOAD_ONLY=1; shift ;;
    all|backend|api) TARGETS+=(backend); shift ;;
    *)              die "未知参数: $1（见 --help）" ;;
  esac
done

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  TARGETS=(backend)
fi

for t in "${TARGETS[@]}"; do
  case "$t" in
    backend) DO_BACKEND=1 ;;
  esac
done

if [[ "$BUILD_ONLY" -eq 1 && "$UPLOAD_ONLY" -eq 1 ]]; then
  die "--build-only 与 --upload-only 不能同时使用"
fi

# ---------- 加载配置（白名单 + 兼容 macOS bash 3.2）----------
is_allowed_key() {
  case "$1" in
    SSH_HOST|SSH_PORT|SSH_KEY|SSH_OPTS|\
    REMOTE_ROOT|REMOTE_API_NAME|REMOTE_API_SERVICE|REMOTE_API_PORT|REMOTE_API_HEALTH_URL|\
    REMOTE_RELOAD_CADDY|UPLOAD_CONFIG|BACKEND_ARCH) return 0 ;;
    *) return 1 ;;
  esac
}

load_release_env() {
  local f="$1"
  [[ -f "$f" ]] || return 1
  local line key val
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line//$'\r'/}"
    case "$line" in
      ''|\#*) continue ;;
    esac
    case "$line" in
      *=*) ;;
      *) continue ;;
    esac
    key="${line%%=*}"
    val="${line#*=}"
    key="${key// /}"
    key="${key//$'\t'/}"
    val="${val#"${val%%[![:space:]]*}"}"
    val="${val%"${val##*[![:space:]]}"}"
    if [[ ${#val} -ge 2 ]]; then
      if [[ "${val:0:1}" == '"' && "${val: -1}" == '"' ]]; then
        val="${val:1:$((${#val} - 2))}"
      elif [[ "${val:0:1}" == "'" && "${val: -1}" == "'" ]]; then
        val="${val:1:$((${#val} - 2))}"
      fi
    fi
    case "$key" in
      *[!A-Za-z0-9_]*|'') continue ;;
    esac
    is_allowed_key "$key" || continue
    printf -v "$key" '%s' "$val"
    export "$key"
  done < "$f"
  return 0
}

if [[ -f "$ENV_FILE" ]]; then
  info "加载配置 $ENV_FILE"
  load_release_env "$ENV_FILE" || die "无法读取 $ENV_FILE"
else
  yellow "未找到 $ENV_FILE，使用默认值（仅本地构建；复制 deploy/release.env.example 可配置 SSH）"
fi

# ---------- 默认值 ----------
: "${SSH_HOST:=}"
: "${SSH_PORT:=22}"
: "${SSH_KEY:=}"
: "${SSH_OPTS:=}"
: "${REMOTE_ROOT:=/opt/zoek}"
: "${REMOTE_API_NAME:=api}"
: "${REMOTE_API_SERVICE:=zoek-api}"
: "${REMOTE_API_PORT:=8090}"
: "${REMOTE_API_HEALTH_URL:=}"
: "${REMOTE_RELOAD_CADDY:=0}"
: "${UPLOAD_CONFIG:=0}"
: "${BACKEND_ARCH:=amd64}"

SSH_HOST="${SSH_HOST// /}"
SSH_PORT="${SSH_PORT// /}"
REMOTE_API_PORT="${REMOTE_API_PORT// /}"
UPLOAD_CONFIG="${UPLOAD_CONFIG// /}"

if [[ -z "${REMOTE_API_HEALTH_URL}" ]]; then
  REMOTE_API_HEALTH_URL="http://127.0.0.1:${REMOTE_API_PORT}/health"
fi

API_BIN_LOCAL=""
case "$BACKEND_ARCH" in
  amd64|x86_64)  API_BIN_LOCAL="$ROOT/backend/bin/zoek-api-linux-amd64" ;;
  arm64|aarch64) API_BIN_LOCAL="$ROOT/backend/bin/zoek-api-linux-arm64" ;;
  *) die "BACKEND_ARCH 仅支持 amd64 或 arm64，当前=$BACKEND_ARCH" ;;
esac

run_ssh() {
  [[ -n "$SSH_HOST" ]] || die "未配置 SSH_HOST"
  local args=(ssh -p "$SSH_PORT" -o ConnectTimeout=15)
  if [[ -n "$SSH_KEY" ]]; then
    args+=(-i "${SSH_KEY/#\~/$HOME}")
  fi
  if [[ -n "$SSH_OPTS" ]]; then
    # shellcheck disable=SC2206
    local extra=($SSH_OPTS)
    args+=("${extra[@]}")
  fi
  args+=("$SSH_HOST" "$*")
  "${args[@]}"
}

upload_file() {
  local src="$1" dest="$2"
  local a=(scp -P "$SSH_PORT" -o ConnectTimeout=15)
  if [[ -n "$SSH_KEY" ]]; then
    a+=(-i "${SSH_KEY/#\~/$HOME}")
  fi
  if [[ -n "$SSH_OPTS" ]]; then
    # shellcheck disable=SC2206
    local extra=($SSH_OPTS)
    a+=("${extra[@]}")
  fi
  a+=("$src" "${SSH_HOST}:${dest}")
  "${a[@]}"
}

can_upload() {
  [[ -n "$SSH_HOST" && "$BUILD_ONLY" -eq 0 ]]
}

# ---------- 构建 ----------
build_backend() {
  info "构建 backend（linux/${BACKEND_ARCH}）…"
  (
    cd "$ROOT/backend"
    mkdir -p bin
    GOOS=linux GOARCH=$BACKEND_ARCH CGO_ENABLED=0 \
      go build -trimpath -ldflags "-s -w" \
      -o "bin/zoek-api-linux-${BACKEND_ARCH}" ./cmd/server
  )
  [[ -f "$API_BIN_LOCAL" ]] || die "未找到产物 $API_BIN_LOCAL"
  green "backend → $API_BIN_LOCAL"
}

# ---------- 上传 + 重启 ----------
# 若服务器尚无 unit，上传并 enable（与 zola 的 xingyue-api 不同名，可共存）
ensure_remote_api_service() {
  local svc="${REMOTE_API_SERVICE}"
  local unit_src="$ROOT/deploy/zoek-api.service"
  [[ -n "$svc" ]] || return 0
  [[ -f "$unit_src" ]] || { yellow "本地无 $unit_src，跳过自动安装 unit"; return 0; }

  if run_ssh "systemctl cat '${svc}.service' >/dev/null 2>&1"; then
    return 0
  fi

  info "服务器无 ${svc}.service，自动安装 systemd unit…"
  local rsh="ssh -p ${SSH_PORT} -o ConnectTimeout=15"
  [[ -n "$SSH_KEY" ]] && rsh+=" -i ${SSH_KEY/#\~/$HOME}"
  [[ -n "$SSH_OPTS" ]] && rsh+=" ${SSH_OPTS}"

  local tmp_unit
  tmp_unit="$(mktemp)"
  cat >"$tmp_unit" <<EOF
[Unit]
Description=Zoek API
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${REMOTE_ROOT}
ExecStart=${REMOTE_ROOT}/${REMOTE_API_NAME}
Restart=on-failure
RestartSec=3
# 配置：${REMOTE_ROOT}/config.yaml（或用 ZOEK_* 环境变量覆盖）

[Install]
WantedBy=multi-user.target
EOF
  # 若存在 zoek 用户则改用该用户
  upload_file "$tmp_unit" "/tmp/zoek-api.service"
  rm -f "$tmp_unit"
  run_ssh "if id zoek >/dev/null 2>&1; then sed -i 's/^User=root\$/User=zoek/' /tmp/zoek-api.service; fi; \
    sudo mv /tmp/zoek-api.service /etc/systemd/system/${svc}.service && \
    sudo systemctl daemon-reload && sudo systemctl enable '${svc}.service'"
  green "已安装并 enable ${svc}.service"
  yellow "请确认服务器上存在 ${REMOTE_ROOT}/config.yaml（首次需自备，参考 backend/config.yaml）"
}

upload_backend() {
  info "上传 backend → ${SSH_HOST}:${REMOTE_ROOT}/${REMOTE_API_NAME}"
  run_ssh "mkdir -p '${REMOTE_ROOT}/uploads/avatars'"
  # 先传临时文件再 mv，减少服务中断窗口
  local tmp_remote="${REMOTE_ROOT}/${REMOTE_API_NAME}.new"
  upload_file "$API_BIN_LOCAL" "${tmp_remote}"
  run_ssh "chmod +x '${tmp_remote}' && mv -f '${tmp_remote}' '${REMOTE_ROOT}/${REMOTE_API_NAME}'"

  # 可选：上传本地配置覆盖服务器（UPLOAD_CONFIG=1 时）
  if [[ "$UPLOAD_CONFIG" == "1" && -f "$ROOT/backend/config.yaml" ]]; then
    info "上传 config.yaml（UPLOAD_CONFIG=1）"
    upload_file "$ROOT/backend/config.yaml" "${REMOTE_ROOT}/config.yaml"
  else
    if ! run_ssh "test -f '${REMOTE_ROOT}/config.yaml'"; then
      yellow "警告: 服务器缺少 ${REMOTE_ROOT}/config.yaml（或设置 UPLOAD_CONFIG=1 上传本地配置）"
    fi
  fi

  green "backend 已上传"

  if [[ -n "$REMOTE_API_SERVICE" ]]; then
    ensure_remote_api_service
    info "重启 systemd: $REMOTE_API_SERVICE"
    if ! run_ssh "sudo systemctl restart '${REMOTE_API_SERVICE}'"; then
      yellow "重启命令失败。最近日志："
      run_ssh "journalctl -u '${REMOTE_API_SERVICE}' -n 40 --no-pager" 2>/dev/null || true
      return 1
    fi
    info "健康检查: ${REMOTE_API_HEALTH_URL}（端口 ${REMOTE_API_PORT}）"
    local ok=0 i
    for i in $(seq 1 15); do
      if run_ssh "curl -sf --connect-timeout 2 '${REMOTE_API_HEALTH_URL}'" >/dev/null 2>&1; then
        ok=1
        break
      fi
      sleep 1
    done
    if [[ "$ok" -eq 1 ]]; then
      green "健康检查 OK: ${REMOTE_API_HEALTH_URL}"
    else
      yellow "警告: 健康检查失败 (${REMOTE_API_HEALTH_URL})"
      yellow "  请确认服务器 config.yaml 的 server.port=${REMOTE_API_PORT} 与 release.env 的 REMOTE_API_PORT 一致"
      yellow "  最近日志："
      run_ssh "journalctl -u '${REMOTE_API_SERVICE}' -n 40 --no-pager" 2>/dev/null || true
    fi
  else
    yellow "REMOTE_API_SERVICE 为空，跳过重启"
  fi
}

reload_caddy_if_needed() {
  if [[ "$REMOTE_RELOAD_CADDY" == "1" ]] && can_upload; then
    info "reload caddy…"
    run_ssh "sudo systemctl reload caddy" || yellow "caddy reload 失败（可忽略若未用 caddy）"
  fi
}

# ---------- 主流程 ----------
info "目标: ${TARGETS[*]}"
if can_upload; then
  info "将上传到 SSH: ${SSH_HOST} 端口 ${SSH_PORT} → ${REMOTE_ROOT}"
  info "API 健康检查: ${REMOTE_API_HEALTH_URL}"
  if [[ -n "$SSH_KEY" && ! -f "${SSH_KEY/#\~/$HOME}" ]]; then
    yellow "警告: SSH_KEY 文件不存在: $SSH_KEY"
  fi
else
  if [[ "$BUILD_ONLY" -eq 1 ]]; then
    yellow "仅本地构建（--build-only）"
  elif [[ -z "$SSH_HOST" ]]; then
    yellow "仅本地构建（SSH_HOST 为空）。配置：cp deploy/release.env.example deploy/release.env 并填 SSH_HOST"
  else
    yellow "仅本地构建"
  fi
fi

if [[ "$UPLOAD_ONLY" -eq 0 ]]; then
  [[ "$DO_BACKEND" -eq 1 ]] && build_backend
else
  info "跳过构建（--upload-only）"
  [[ "$DO_BACKEND" -eq 1 && ! -f "$API_BIN_LOCAL" ]] && die "缺少 $API_BIN_LOCAL，请先构建"
fi

if can_upload; then
  command -v ssh >/dev/null 2>&1 || die "需要 ssh"
  command -v scp >/dev/null 2>&1 || die "需要 scp"
  [[ "$DO_BACKEND" -eq 1 ]] && upload_backend
  reload_caddy_if_needed
  green "发版完成（已上传${REMOTE_API_SERVICE:+并尝试重启 API}）"
else
  green "本地构建完成"
  [[ "$DO_BACKEND" -eq 1 ]] && echo "  backend: $API_BIN_LOCAL"
  if [[ -z "$SSH_HOST" ]]; then
    echo
    yellow "配置 SSH 后可自动上传："
    echo "  cp deploy/release.env.example deploy/release.env"
    echo "  # 填写 SSH_HOST=user@ip 后重新执行"
  fi
fi
