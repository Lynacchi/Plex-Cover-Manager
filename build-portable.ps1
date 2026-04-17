$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$mingwBin = "C:\msys64\mingw64\bin"
$gcc = Join-Path $mingwBin "gcc.exe"
$objdump = Join-Path $mingwBin "objdump.exe"
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
if (-not (Test-Path $objdump)) {
    Write-Error "objdump wurde nicht gefunden: $objdump"
}
if (-not (Test-Path $windres)) {
    Write-Error "windres wurde nicht gefunden: $windres"
}
if (Test-Path $pacman) {
    & $pacman -S --noconfirm --needed mingw-w64-x86_64-mesa | Out-Host
}
if (-not (Test-Path (Join-Path $mingwBin "opengl32.dll"))) {
    Write-Error "Mesa OpenGL wurde nicht gefunden. Installiere: pacman -S --needed mingw-w64-x86_64-mesa"
}

$env:Path = "$mingwBin;$env:Path"
$env:CC = $gcc
$env:CGO_ENABLED = "1"

$appVersion = (Get-Content (Join-Path $root "VERSION") -Raw).Trim()
if ($appVersion -notmatch '^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$') {
    Write-Error "VERSION muss semantisch aussehen, z.B. 0.2.0. Aktuell: $appVersion"
}

function Write-WindowsResource {
    param(
        [Parameter(Mandatory = $true)] [string]$RcPath,
        [Parameter(Mandatory = $true)] [string]$IconPath,
        [Parameter(Mandatory = $true)] [string]$Version,
        [Parameter(Mandatory = $true)] [string]$OriginalFilename
    )
    $numeric = ($Version -replace '-.*$', '') -split '\.'
    $fileVersionNumeric = "$([int]$numeric[0]),$([int]$numeric[1]),$([int]$numeric[2]),0"
    $year = (Get-Date).Year
    $escapedIcon = $IconPath.Replace("\", "\\")
    $content = @"
#pragma code_page(65001)

1 ICON "$escapedIcon"

1 VERSIONINFO
FILEVERSION     $fileVersionNumeric
PRODUCTVERSION  $fileVersionNumeric
FILEOS          0x40004L
FILETYPE        0x1L
BEGIN
    BLOCK "StringFileInfo"
    BEGIN
        BLOCK "040904b0"
        BEGIN
            VALUE "CompanyName",      "Nakama Network"
            VALUE "FileDescription",  "Verwaltet Cover/Poster für Plex- und Jellyfin-Medienbibliotheken"
            VALUE "FileVersion",      "$Version"
            VALUE "InternalName",     "PlexCoverManager"
            VALUE "LegalCopyright",   "© $year Lynacchi / Nakama Network"
            VALUE "OriginalFilename", "$OriginalFilename"
            VALUE "ProductName",      "Plex Cover Manager"
            VALUE "ProductVersion",   "$Version"
        END
    END
    BLOCK "VarFileInfo"
    BEGIN
        VALUE "Translation", 0x409, 1200
    END
END
"@
    Set-Content -Path $RcPath -Value $content -Encoding UTF8
}

go run .\tools\icongen

$appSyso = Join-Path $root "app_icon_windows.syso"
$launcherSyso = Join-Path $root "cmd\launcher\launcher_icon_windows.syso"
$appRc = Join-Path $root "icon_windows.rc"
$launcherRc = Join-Path $root "cmd\launcher\launcher_icon_windows.rc"
$iconPath = Join-Path $root "assets\app.ico"
Remove-Item -Force $appSyso,$launcherSyso,$appRc,$launcherRc -ErrorAction SilentlyContinue
Write-WindowsResource -RcPath $appRc -IconPath $iconPath -Version $appVersion -OriginalFilename "PlexCoverManager.app.exe"
Write-WindowsResource -RcPath $launcherRc -IconPath $iconPath -Version $appVersion -OriginalFilename "PlexCoverManager-v$appVersion-portable.exe"
& $windres --codepage=65001 -i $appRc -O coff -o $appSyso
& $windres --codepage=65001 -i $launcherRc -O coff -o $launcherSyso

go test -mod=vendor ./...

$payloadDir = Join-Path $root "cmd\launcher\payload"
Remove-Item -Recurse -Force $payloadDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $payloadDir | Out-Null

$appPayload = Join-Path $payloadDir "PlexCoverManager.app.exe"
$appLdflags = "-H windowsgui -s -w -X plexcovermanager/appversion.Version=$appVersion"
go build -mod=vendor -trimpath -buildvcs=false -ldflags $appLdflags -o $appPayload .

$queue = [System.Collections.Generic.Queue[string]]::new()
$seen = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
$queue.Enqueue("opengl32.dll")
$queue.Enqueue("libgallium_wgl.dll")

while ($queue.Count -gt 0) {
    $name = $queue.Dequeue()
    if (-not $seen.Add($name)) {
        continue
    }

    $source = Join-Path $mingwBin $name
    if (-not (Test-Path $source)) {
        continue
    }

    Copy-Item $source (Join-Path $payloadDir $name) -Force

    $deps = & $objdump -p $source |
        Select-String -Pattern '^\s*DLL Name:' |
        ForEach-Object { ($_.Line -replace '^\s*DLL Name:\s*', '').Trim() }

    foreach ($dep in $deps) {
        if (Test-Path (Join-Path $mingwBin $dep)) {
            $queue.Enqueue($dep)
        }
    }
}

$hashInput = New-Object System.Text.StringBuilder
Get-ChildItem $payloadDir -File |
    Sort-Object Name |
    ForEach-Object {
        $fileHash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash
        [void]$hashInput.AppendLine("$($_.Name):$($_.Length):$fileHash")
    }

$sha = [System.Security.Cryptography.SHA256]::Create()
$bytes = [System.Text.Encoding]::UTF8.GetBytes($hashInput.ToString())
$payloadVersion = ([System.BitConverter]::ToString($sha.ComputeHash($bytes)) -replace '-', '').Substring(0, 16).ToLowerInvariant()

$distDir = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null
Get-ChildItem $distDir -Filter "PlexCoverManager-v*-portable.exe" -ErrorAction SilentlyContinue |
    Remove-Item -Force
Remove-Item -Force (Join-Path $root "PlexCoverManager.exe") -ErrorAction SilentlyContinue

$output = Join-Path $distDir "PlexCoverManager-v$appVersion-portable.exe"
$ldflags = "-H windowsgui -s -w -X main.payloadVersion=$payloadVersion -X main.appVersion=$appVersion"
go build -mod=vendor -tags launcher -trimpath -buildvcs=false -ldflags $ldflags -o $output .\cmd\launcher

Remove-Item -Recurse -Force $payloadDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $payloadDir | Out-Null
Set-Content -Path (Join-Path $payloadDir "placeholder.txt") -Value "generated by build-portable.ps1" -Encoding ascii
Remove-Item -Force $appSyso,$launcherSyso,$appRc,$launcherRc -ErrorAction SilentlyContinue

Write-Host "Fertig: $output"
Write-Host "Variante: portable"
Write-Host "App-Version: $appVersion"
Write-Host "Payload-Version: $payloadVersion"
