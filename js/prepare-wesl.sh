#!/usr/bin/env bash
set -eu

cd "$(dirname "$0")"

npm ci

npx esbuild index.js \
  --bundle \
  --target=es2022 \
  --format=cjs \
  --tree-shaking=true \
  --minify \
  --outfile=dist/index.js

cp dist/index.js ../wesl.js

cd ..
go test -v ./...
