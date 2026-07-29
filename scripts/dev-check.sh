#!/usr/bin/env bash
set -euo pipefail

# 本地统一检查入口；业务规则不能放进脚本。
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

make check
