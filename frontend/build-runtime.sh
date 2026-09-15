#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
tsc -p frontend/tsconfig.runtime.json
mkdir -p frontend/public/assets
cp web/assets/app.js frontend/public/assets/runtime-app.js
