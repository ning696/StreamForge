#!/usr/bin/env bash
# deploy.sh —— 在 Linux 服务器上一键部署 StreamForge
# 前置要求：
#   - 已安装 Docker Engine 20.10+
#   - 已安装 Docker Compose v2（docker compose 命令可用）
#   - 已把 .env.example 复制为 .env 并填写完毕
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
cd "${SCRIPT_DIR}"

log()  { echo -e "\033[1;32m[deploy]\033[0m $*"; }
warn() { echo -e "\033[1;33m[deploy]\033[0m $*"; }
err()  { echo -e "\033[1;31m[deploy]\033[0m $*" >&2; }

# ---------------- 0. 基础检查 ----------------
if ! command -v docker >/dev/null 2>&1; then
  err "未找到 docker，请先安装 Docker Engine。"
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
  err "未找到 'docker compose'（v2）。请安装 Docker Compose Plugin。"
  exit 1
fi
if [[ ! -f .env ]]; then
  err ".env 不存在。请执行：cp .env.example .env  然后按注释填写，再重跑。"
  exit 1
fi
if [[ ! -f user-service/user-service.jar ]]; then
  err "user-service/user-service.jar 缺失。请在开发机执行 build.sh 后再上传本目录。"
  exit 1
fi
if [[ ! -f media-service/media-service ]]; then
  err "media-service/media-service 二进制缺失。请在开发机执行 build.sh。"
  exit 1
fi
if [[ ! -d frontend/dist ]]; then
  err "frontend/dist 缺失。请在开发机执行 build.sh。"
  exit 1
fi

# ---------------- 1. 校验 .env 关键字段 ----------------
set -a
# shellcheck disable=SC1091
source .env
set +a

require_var() {
  local name="$1"
  local value="${!name:-}"
  if [[ -z "${value}" || "${value}" == "change-me" || "${value}" == your.* || "${value}" == your-* ]]; then
    err ".env 中的 ${name} 尚未填写有效值（当前：'${value}'）。"
    exit 1
  fi
}
require_var APP_DOMAIN
require_var LIVEKIT_DOMAIN
require_var FRONTEND_PORT
require_var MYSQL_HOST
require_var MYSQL_USERNAME
require_var MYSQL_PASSWORD
require_var REDIS_HOST
require_var LIVEKIT_API_KEY
require_var LIVEKIT_API_SECRET

if [[ "${LIVEKIT_API_SECRET}" == "please-change-to-a-long-random-secret" ]]; then
  err "LIVEKIT_API_SECRET 仍是默认值，请改成一个至少 32 字节的随机字符串。"
  exit 1
fi

if [[ "${FRONTEND_PORT}" == "80" || "${FRONTEND_PORT}" == "443" ]]; then
  err "FRONTEND_PORT 不能使用 80/443；这两个端口由 1Panel OpenResty 占用。建议使用 8088。"
  exit 1
fi

log "环境变量校验通过 (APP_DOMAIN=${APP_DOMAIN}, LIVEKIT_DOMAIN=${LIVEKIT_DOMAIN}, FRONTEND_PORT=${FRONTEND_PORT})"

# ---------------- 2. 构建镜像并启动 ----------------
log "构建 Docker 镜像 ..."
docker compose build

log "启动服务 ..."
docker compose up -d

# ---------------- 3. 健康检查 ----------------
log "等待服务就绪 ..."
sleep 5

check_http() {
  local name="$1"
  local url="$2"
  for i in {1..20}; do
    if curl -fsS -o /dev/null "${url}"; then
      log "  ${name} 就绪 (${url})"
      return 0
    fi
    sleep 2
  done
  warn "  ${name} 20 次探测未通过 (${url})，请查看容器日志。"
  return 1
}

check_http "frontend  " "http://127.0.0.1:${FRONTEND_PORT}/health" || true
# user-service 和 media-service 走前端 Nginx 反代
check_http "user-svc  " "http://127.0.0.1:${FRONTEND_PORT}/api/users/999999" || true

log ""
log "========================================================"
log "部署完成："
log "  前端回源端口 : ${FRONTEND_PORT}  (1Panel 请代理到服务器内网 IP 的该端口)"
log "  前端入口 : https://${APP_DOMAIN}"
log "  LiveKit  : wss://${LIVEKIT_DOMAIN}  (1Panel 反代到 7880)"
log "  RTC 端口 : TCP 7881 与 UDP 30000-30020"
log ""
log "常用命令："
log "  查看运行状态 : docker compose ps"
log "  查看某服务日志: docker compose logs -f media-service"
log "  停止         : docker compose down"
log "  重启         : docker compose restart"
log "========================================================"
