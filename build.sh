#!/usr/bin/env bash
# ah系统优化工具箱（ahtool）跨平台构建脚本
# 用法：./build.sh [release]
set -euo pipefail
cd "$(dirname "$0")"
APPVER=1.0.1
BUILDDATE=$(date +%F)
GO=${GO:-go}
if [ -x _toolchain/go/bin/go ]; then GO=_toolchain/go/bin/go; fi
if [ "${1:-}" = "release" ]; then VER="$APPVER"; EXE="bin/ahtool_v${APPVER}.exe"; else VER="${APPVER}-dev"; EXE="bin/ahtool_${APPVER}-dev.exe"; fi
mkdir -p bin
export GOOS=windows GOARCH=amd64 CGO_ENABLED=0
"$GO" build -buildvcs=false -trimpath -tags desktop,production \
  -ldflags "-s -w -H windowsgui -X main.appVersion=${VER} -X main.buildDate=${BUILDDATE}" \
  -o "$EXE" .
echo "OK: $EXE"
