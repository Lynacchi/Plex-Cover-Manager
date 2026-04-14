$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$mingwBin = "C:\msys64\mingw64\bin"
$gcc = Join-Path $mingwBin "gcc.exe"
$windres = Join-Path $mingwBin "windres.exe"
$pacman = "C:\msys64\usr\bin\pacman.exe"

if (-not (Test-Path $gcc)) {
    if (Test-Path $pacman) {
        & $pacman -S --noconfirm --needed mingw-w64-x86_64-gcc
    }
}
if (-not (Test-Path $gcc)) {
    Write-Error "MinGW64 GCC wurde nicht gefunden: $gcc. Installiere MSYS2 und dann: pacman -S --needed mingw-w64-x86_64-gcc"
}
if (-not (Test-Path $windres)) {
    Write-Error "windres wurde nicht gefunden: $windres"
}

$env:Path = "$mingwBin;$env:Path"
$env:CC = $gcc
$env:CGO_ENABLED = "1"

$appVersion = (Get-Content (Join-Path $root "VERSION") -Raw).Trim()
if ($appVersion -notmatch '^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$') {
    Write-Error "VERSION muss semantisch aussehen, z.B. 0.0.6. Aktuell: $appVersion"
}

go run .\tools\icongen

$appSyso = Join-Path $root "app_icon_windows.syso"
$iconRc = Join-Path $root "icon_windows.rc"
$iconPath = (Join-Path $root "assets\app.ico").Replace("\", "\\")
Remove-Item -Force $appSyso,$iconRc -ErrorAction SilentlyContinue
Set-Content -Path $iconRc -Value "1 ICON `"$iconPath`"" -Encoding ascii
& $windres -i $iconRc -O coff -o $appSyso

go test -mod=vendor ./...

$distDir = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null
Get-ChildItem $distDir -Filter "PlexCoverManager-v*.exe" -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -notlike "*-portable.exe" } |
    Remove-Item -Force
Remove-Item -Force (Join-Path $root "PlexCoverManager.exe") -ErrorAction SilentlyContinue

$output = Join-Path $distDir "PlexCoverManager-v$appVersion.exe"
$ldflags = "-H windowsgui -s -w -X plexcovermanager/appversion.Version=$appVersion"
go build -mod=vendor -trimpath -buildvcs=false -ldflags $ldflags -o $output .

Remove-Item -Force $appSyso,$iconRc -ErrorAction SilentlyContinue

Write-Host "Fertig: $output"
Write-Host "Variante: normal"
Write-Host "App-Version: $appVersion"
