#!/bin/bash
# 启动 benchmark 专用的记忆服务
# 使用隔离的 benchmark_memory 目录和独立端口

export MEMSVC_MEMORY_DIR="$HOME/.oxencode/benchmark_memory"
export MEMSVC_DATA_DIR="$HOME/.oxencode/benchmark_memory"
export MEMSVC_PORT=8766

echo "启动 benchmark 记忆服务..."
echo "  Memory Dir: $MEMSVC_MEMORY_DIR"
echo "  Port: $MEMSVC_PORT"

cd "$(dirname "$0")"
uv run python -m memsvc.main --port 8766