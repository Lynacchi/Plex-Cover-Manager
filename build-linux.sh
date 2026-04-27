#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$root"

export CGO_ENABLED=1

goos=${GOOS:-$(go env GOOS)}
goarch=${GOARCH:-$(go env GOARCH)}

if [ "$goos" != "linux" ]; then
  echo "Linux-Build bitte auf Linux ausführen oder GOOS=linux mit passender Linux-C-Toolchain setzen." >&2
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

icon_output="dist/plex-cover-manager.png"
desktop_output="dist/PlexCoverManager-v${version}-linux-${goarch}.desktop"
install_output="dist/install-linux-desktop.sh"
package_name="PlexCoverManager-v${version}-linux-${goarch}"
package_dir="dist/${package_name}"
package_output="dist/${package_name}.tar.gz"

cp assets/app.png "$icon_output"
cat > "$desktop_output" <<EOF
[Desktop Entry]
Type=Application
Name=Plex Cover Manager
Comment=Manage local Plex and Jellyfin poster files
Exec=PlexCoverManager-v${version}-linux-${goarch}
Icon=plex-cover-manager
Terminal=false
Categories=AudioVideo;Utility;
EOF

cat > "$install_output" <<EOF
#!/usr/bin/env sh
set -eu

script_dir=\$(CDPATH= cd -- "\$(dirname -- "\$0")" && pwd)
bin_dir="\$HOME/.local/bin"
icon_dir="\$HOME/.local/share/icons/hicolor/256x256/apps"
desktop_dir="\$HOME/.local/share/applications"

mkdir -p "\$bin_dir" "\$icon_dir" "\$desktop_dir"
cp "\$script_dir/PlexCoverManager-v${version}-linux-${goarch}" "\$bin_dir/PlexCoverManager"
cp "\$script_dir/plex-cover-manager.png" "\$icon_dir/plex-cover-manager.png"
cp "\$script_dir/PlexCoverManager-v${version}-linux-${goarch}.desktop" "\$desktop_dir/plex-cover-manager.desktop"
chmod +x "\$bin_dir/PlexCoverManager"
sed -i "s|^Exec=.*|Exec=\$bin_dir/PlexCoverManager|" "\$desktop_dir/plex-cover-manager.desktop"

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "\$desktop_dir" >/dev/null 2>&1 || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache "\$HOME/.local/share/icons/hicolor" >/dev/null 2>&1 || true
fi

echo "Installiert: \$desktop_dir/plex-cover-manager.desktop"
EOF
chmod +x "$install_output"

rm -rf "$package_dir" "$package_output"
mkdir -p "$package_dir"
cp "$output" "$package_dir/"
cp "$icon_output" "$package_dir/"
cp "$desktop_output" "$package_dir/"
cp "$install_output" "$package_dir/"
tar -C dist -czf "$package_output" "$package_name"
rm -rf "$package_dir"

echo "Fertig: $output"
echo "Linux-Paket: $package_output"
