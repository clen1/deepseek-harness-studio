#!/usr/bin/env sh
set -eu

PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$PROJECT_ROOT"

if ! command -v wails >/dev/null 2>&1; then
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
  PATH="$(go env GOPATH)/bin:$PATH"
  export PATH
fi

go test ./...

if [ "$(uname -s)" = "Linux" ] && pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
  wails build -clean -tags webkit2_41
else
  wails build -clean
fi

echo "Build ready under: $PROJECT_ROOT/build/bin"
