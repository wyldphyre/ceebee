#!/usr/bin/env bash
#
# Builds CeeBee for macOS and Windows, on both x64 and arm64, and zips each
# build into dist/, for example dist/CeeBee-1.0.0-macos-arm64.zip.
#
# Run it on a Mac from anywhere in the repository: macOS builds need Xcode's
# command line tools. Windows builds are cross-compiled.

set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname)" != "Darwin" ]]; then
    echo "This script must be run on macOS." >&2
    exit 1
fi

# Use wails3 from PATH, or where `go install` puts it.
WAILS=$(command -v wails3 || echo "$(go env GOPATH)/bin/wails3")
if [[ ! -x "$WAILS" ]]; then
    echo "wails3 not found. Install the version in go.mod with:" >&2
    echo "  go install github.com/wailsapp/wails/v3/cmd/wails3@$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)" >&2
    exit 1
fi
# The Wails tasks run wails3 by name, so make sure it's on PATH.
export PATH="$(dirname "$WAILS"):$PATH"

# The version comes from main.go, so it only needs changing in one place here.
VERSION=$(sed -n 's/^const version = "\(.*\)"$/\1/p' main.go)
if [[ -z "$VERSION" ]]; then
    echo "Couldn't read the version from main.go." >&2
    exit 1
fi

DIST=dist
mkdir -p "$DIST"

# Go and Wails call the architectures amd64 and arm64; the zip names use x64.
label() {
    if [[ "$1" == "amd64" ]]; then echo x64; else echo "$1"; fi
}

for arch in arm64 amd64; do
    zip="$DIST/CeeBee-$VERSION-macos-$(label "$arch").zip"
    echo "==> Building $zip"
    rm -rf bin/CeeBee.app
    "$WAILS" task darwin:package ARCH="$arch"
    # ditto keeps the app bundle's symlinks and metadata intact.
    rm -f "$zip"
    ditto -c -k --keepParent bin/CeeBee.app "$zip"
done

for arch in arm64 amd64; do
    zip="$DIST/CeeBee-$VERSION-windows-$(label "$arch").zip"
    echo "==> Building $zip"
    rm -f bin/CeeBee.exe
    "$WAILS" build GOOS=windows GOARCH="$arch"
    rm -f "$zip"
    zip -j -q "$zip" bin/CeeBee.exe
done

echo
echo "Done:"
ls -lh "$DIST"/CeeBee-"$VERSION"-*.zip
