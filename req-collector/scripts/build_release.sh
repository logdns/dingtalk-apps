#!/usr/bin/env bash
# 一键构建发布包：编译前端 -> 编译 Go 二进制 -> 打包到 dist/
#
# 用法：
#   ./scripts/build_release.sh            # 用 config.go 里的 AppVersion
#   ./scripts/build_release.sh 2.3.3      # 指定版本（请与 config.go 的 AppVersion 保持一致）
#
# 产物：dist/dingtalk-req-collector-<ver>-<os>-<arch>.{tar.gz,zip}
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-$(grep -m1 'AppVersion' config.go | sed -E 's/.*"([0-9.]+)".*/\1/')}"
echo "==> 版本: $VERSION"

# 1) 前端 JSX 预编译
echo "==> 编译前端 src/*.jsx -> static/*.js"
if command -v node >/dev/null 2>&1; then
  node scripts/build_frontend.js
else
  echo "未找到 node，跳过前端编译（请确保 static/*.js 已是最新）"
fi

# 2) 打包目录准备
rm -rf dist/build
mkdir -p dist/build dist

PKG_NAME="dingtalk-req-collector"

build_one() {
  local goos=$1 goarch=$2 bin=$3 pkg=$4
  echo "==> 编译 $goos/$goarch"

  rm -rf "dist/build/$PKG_NAME"
  mkdir -p "dist/build/$PKG_NAME"
  GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w" -o "dist/build/$PKG_NAME/$bin" .

  # 默认打包不含密钥的模板；需要「开箱即用」时用 WITH_SECRETS=1 打包本机 config.json
  if [ "${WITH_SECRETS:-0}" = "1" ] && [ -f config.json ]; then
    cp config.json "dist/build/$PKG_NAME/config.json"
    echo "    (已打包本机 config.json，含钉钉密钥与数据库口令，注意分发范围)"
  else
    cp config.example.json "dist/build/$PKG_NAME/config.json"
  fi
  cp README.md "dist/build/$PKG_NAME/"
  cp -r deploy "dist/build/$PKG_NAME/"
  if [ "$goos" = "windows" ]; then
    (cd dist/build && zip -qr "../${pkg}.zip" "$PKG_NAME")
  else
    (cd dist/build && tar czf "../${pkg}.tar.gz" "$PKG_NAME")
  fi
  rm -rf "dist/build/$PKG_NAME"
}

build_one linux   amd64 dingtalk-req-collector      "dingtalk-req-collector-$VERSION-linux-amd64"
build_one linux   arm64 dingtalk-req-collector      "dingtalk-req-collector-$VERSION-linux-arm64"
build_one darwin  arm64 dingtalk-req-collector      "dingtalk-req-collector-$VERSION-macos-arm64"
build_one windows amd64 dingtalk-req-collector.exe  "dingtalk-req-collector-$VERSION-windows"

rm -rf dist/build
echo "==> 完成，产物在 dist/:"
ls -lh dist | tail -n +2
