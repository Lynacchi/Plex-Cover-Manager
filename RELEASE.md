# Release erstellen

Dieses Projekt nutzt `VERSION` als zentrale Versionsquelle. Aktuell:

```text
0.2.0
```

## Builds

Normale Variante:

```powershell
powershell -ExecutionPolicy Bypass -File .\build-normal.ps1
```

Portable Variante mit Mesa/OpenGL-Fallback:

```powershell
powershell -ExecutionPolicy Bypass -File .\build-portable.ps1
```

Linux-Binary:

```bash
sh ./build-linux.sh
```

Die Windows-Builds erzeugen in `dist/`:

- `PlexCoverManager-v0.2.0.exe`
- `PlexCoverManager-v0.2.0-portable.exe`

Der Linux-Build erzeugt z.B.:

- `PlexCoverManager-v0.2.0-linux-amd64` (wenn der Linux-Build ausgefuehrt wurde)

Die normale Variante ist fuer normale Windows-PCs gedacht. Die portable Variante ist fuer VMs, RDP, KVM-Server und Systeme ohne verlaessliches OpenGL gedacht.

## GitHub Release per Weboberflaeche

1. Alle Quellcode-Aenderungen committen und pushen.
2. Auf GitHub im Repository auf `Releases` gehen.
3. `Create a new release` waehlen.
4. Tag `v0.2.0` eintragen und erstellen lassen.
5. Release title: `Plex Cover Manager v0.2.0`
6. Beide Dateien aus `dist/` als Assets hochladen.
7. Release notes kurz halten, z.B.:

```text
Plex Cover Manager v0.2.0

- Normal build for regular Windows desktops
- Portable Mesa build for VM/RDP/server environments
- Plex/Jellyfin naming modes
- Compression checks and optional disabled compression
- Compact one-line list with status tooltips
- Compact one-line detail header
- Slot-targeted drag and drop in the detail view
- Delayed status tooltips
- Jellyfin flat-series fallback is now shown explicitly
- Smart detection for existing alias cover names
- Explicit rename action for alias covers
- Static status tooltips after hover delay
- Media-type filter for all/movie/series
- Detail refresh for the selected title
- Consistent compression wording and disabled compression actions
- Manual slot imports show manual assignment instead of a match
- Faster path adding for network shares
- Plex poster import and preview workflow
```

8. `Publish release` klicken.

## GitHub Release per CLI

Wenn GitHub CLI installiert ist:

```powershell
git tag v0.2.0
git push origin main
git push origin v0.2.0
gh release create v0.2.0 `
  .\dist\PlexCoverManager-v0.2.0.exe `
  .\dist\PlexCoverManager-v0.2.0-portable.exe `
  --title "Plex Cover Manager v0.2.0" `
  --notes "Plex Cover Manager v0.2.0"
```

Die `.exe`-Dateien sollten nicht ins Repository committed werden. Sie gehoeren als Release Assets in GitHub Releases.
