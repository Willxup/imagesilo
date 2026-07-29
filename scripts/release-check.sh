#!/usr/bin/env bash
set -euo pipefail

# 本地发布检查保持单进程和单 worker；双架构镜像由原生 GitHub runner 分别验证。
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

make check e2e
