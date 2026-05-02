#!/usr/bin/env bash
# 启动 / 停止 due 框架所需的本地开发中间件
#
# 用法:
#   ./dev.sh          启动（默认：redis + consul + nats）
#   ./dev.sh start    同上
#   ./dev.sh stop     停止
#   ./dev.sh status   查看运行状态
#   ./dev.sh etcd     用 etcd 替代 consul 启动
#
# 依赖: ~/workspace/env/my-docker-config

set -euo pipefail

INFRA_SCRIPT="$HOME/workspace/env/my-docker-config/infra/scripts/infra.sh"

if [[ ! -f "$INFRA_SCRIPT" ]]; then
  echo "错误: 公共基础设施脚本不存在: $INFRA_SCRIPT"
  exit 1
fi

# 检测 Docker 是否可用
if ! docker info &>/dev/null; then
  echo "错误: Docker 未运行，请先启动 OrbStack 或 Docker Desktop"
  echo "  macOS: open -a OrbStack"
  exit 1
fi

# shellcheck source=/dev/null
source "$INFRA_SCRIPT" 2>/dev/null

# due 框架各模块依赖说明:
#   redis  → locate（用户位置）/ cache（缓存）/ lock（分布式锁）
#   consul → registry（服务发现）/ config（动态配置）
#   nats   → eventbus（跨节点事件总线）

DEFAULT_PROFILES=(redis consul nats)
ETCD_PROFILES=(redis etcd nats)

CMD="${1:-start}"

case "$CMD" in
  start)
    echo "启动 due 开发环境（redis + consul + nats）..."
    infra-up "${DEFAULT_PROFILES[@]}"
    echo ""
    infra-ps
    ;;
  etcd)
    echo "启动 due 开发环境（redis + etcd + nats）..."
    infra-up "${ETCD_PROFILES[@]}"
    echo ""
    infra-ps
    ;;
  stop)
    echo "停止 due 开发环境..."
    infra-down redis consul etcd nats
    ;;
  status)
    infra-ps
    ;;
  *)
    echo "用法: $0 [start|stop|status|etcd]"
    echo ""
    echo "  start   启动 redis + consul + nats（默认）"
    echo "  etcd    启动 redis + etcd + nats（用 etcd 替代 consul）"
    echo "  stop    停止所有 due 相关服务"
    echo "  status  查看服务运行状态"
    exit 1
    ;;
esac
