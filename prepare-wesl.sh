#!/usr/bin/env bash

set -ex

npm install --include=dev

# need to copy the file out of node_modules for it to be transpiled
cp node_modules/wesl/dist/index.js wesl-raw.js
trap 'rm -f wesl-raw.js' EXIT

node_modules/.bin/babel wesl-raw.js > wesl.js

go test -v ./...
