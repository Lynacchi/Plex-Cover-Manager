#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$root"

export CGO_ENABLED=1

goos=${GOOS:-$(go env GOOS)}
goarch=${GOARCH:-$(go env GOARCH)}

if [ "$goos" != "linux" ]; then
  echo "Linux-Build bitte auf Linux ausfuehren oder GOOS=linux mit passendem Linux-C-Toolchain setzen." >&2
  exit 1
fi

version=$(tr -d '\r\n' < VERSION | sed 's/^\xef\xbb\xbf//')
if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  echo "VERSION muss semantisch aussehen, z.B. 0.2.0. Aktuell: $version" >&2
  exit 1
fi

go test -mod=vendor ./...

mkdir -p dist
output="dist/PlexCoverManager-v${version}-linux-${goarch}"
go build -mod=vendor -trimpath -buildvcs=false \
  -ldflags "-s -w -X plexcovermanager/appversion.Version=$version" \
  -o "$output" .

echo "Fertig: $output"
