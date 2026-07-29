#!/usr/bin/env bash
set -euo pipefail

# 发布检查在阶段 7 扩展为双架构容器 smoke test。
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

make check
make build
