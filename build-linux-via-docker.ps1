$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

docker info | Out-Null

$image = "plex-cover-manager-linux-builder:go1.26.1"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("pcm-linux-builder-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempDir | Out-Null

try {
    $dockerfile = @"
FROM golang:1.26.1-bookworm
ENV PATH="/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc pkg-config libgl1-mesa-dev xorg-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
"@
    Set-Content -Path (Join-Path $tempDir "Dockerfile") -Value $dockerfile -Encoding ascii
    docker build -t $image $tempDir
}
finally {
    Remove-Item -Recurse -Force $tempDir -ErrorAction SilentlyContinue
}

docker run --rm `
    -v "${root}:/src" `
    -v "plex-cover-manager-go-build-cache:/root/.cache/go-build" `
    -w /src `
    $image `
    sh ./build-linux.sh
