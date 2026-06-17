#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$ROOT_DIR/web"
DEFAULT_WEB_DIR="$WEB_DIR/default"
DEV_COMPOSE_FILE="$ROOT_DIR/docker-compose.dev.yml"

BACKEND_PORT="${BACKEND_PORT:-3000}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"
HOST="${HOST:-0.0.0.0}"

START_BACKEND=1
START_FRONTEND=1
DETACH=0

usage() {
  cat <<EOF
用法:
  $(basename "$0") [选项]

选项:
  --backend-only     只启动后端 docker 开发环境
  --frontend-only    只启动前端 dev server
  --detach           后端后台启动；前端不启动时直接退出
  --frontend-port N  指定前端端口，默认: $FRONTEND_PORT
  --backend-port N   指定后端端口，默认: $BACKEND_PORT
  -h, --help         显示帮助

环境变量:
  HOST               前端监听地址，默认: $HOST
  FRONTEND_PORT      前端端口，默认: $FRONTEND_PORT
  BACKEND_PORT       后端端口，默认: $BACKEND_PORT

示例:
  $(basename "$0")
  $(basename "$0") --backend-only
  $(basename "$0") --frontend-only --frontend-port 5173
EOF
}

log() {
  printf '[new-api-dev] %s\n' "$*"
}

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf '缺少依赖命令: %s\n' "$1" >&2
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --backend-only)
      START_BACKEND=1
      START_FRONTEND=0
      shift
      ;;
    --frontend-only)
      START_BACKEND=0
      START_FRONTEND=1
      shift
      ;;
    --detach)
      DETACH=1
      shift
      ;;
    --frontend-port)
      FRONTEND_PORT="$2"
      shift 2
      ;;
    --backend-port)
      BACKEND_PORT="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf '未知参数: %s\n\n' "$1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$START_BACKEND" -eq 1 ]]; then
  need_cmd docker
fi

if [[ "$START_FRONTEND" -eq 1 ]]; then
  need_cmd bun
fi

compose_up() {
  log "启动并重新构建后端 docker 开发环境..."
  (
    cd "$ROOT_DIR"
    docker compose -f "$DEV_COMPOSE_FILE" up -d --build
  )
  log "后端已启动: http://localhost:$BACKEND_PORT"
}

frontend_install_if_needed() {
  if [[ ! -d "$WEB_DIR/node_modules" ]]; then
    log "检测到 $WEB_DIR/node_modules 不存在，先执行 bun install..."
    (
      cd "$WEB_DIR"
      bun install
    )
  fi

  if [[ ! -d "$DEFAULT_WEB_DIR/node_modules" && ! -d "$WEB_DIR/node_modules/.bin" ]]; then
    log "前端依赖似乎未正确安装，请检查 bun install 输出。"
  fi
}

start_frontend() {
  frontend_install_if_needed
  log "启动默认前端 dev server..."
  log "前端地址: http://localhost:$FRONTEND_PORT"
  log "后端 API: http://localhost:$BACKEND_PORT"
  cd "$DEFAULT_WEB_DIR"
  exec bun run dev -- --host "$HOST" --port "$FRONTEND_PORT"
}

if [[ "$START_BACKEND" -eq 1 ]]; then
  compose_up
fi

if [[ "$START_FRONTEND" -eq 1 ]]; then
  if [[ "$DETACH" -eq 1 && "$START_BACKEND" -eq 1 ]]; then
    log "--detach 已启用，但前端 dev server 需要前台运行；继续以前台方式启动前端。"
  fi
  start_frontend
fi

if [[ "$DETACH" -eq 1 ]]; then
  log "开发环境已后台启动。"
  exit 0
fi

log "没有需要启动的服务。"
