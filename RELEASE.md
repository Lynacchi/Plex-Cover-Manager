# Release erstellen

Dieses Projekt nutzt `VERSION` als zentrale Versionsquelle. Aktuell:

```text
0.0.5
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

Danach liegen in `dist/` zwei Dateien:

- `PlexCoverManager-v0.0.5.exe`
- `PlexCoverManager-v0.0.5-portable.exe`

Die normale Variante ist fuer normale Windows-PCs gedacht. Die portable Variante ist fuer VMs, RDP, KVM-Server und Systeme ohne verlaessliches OpenGL gedacht.

## GitHub Release per Weboberflaeche

1. Alle Quellcode-Aenderungen committen und pushen.
2. Auf GitHub im Repository auf `Releases` gehen.
3. `Create a new release` waehlen.
4. Tag `v0.0.5` eintragen und erstellen lassen.
5. Release title: `Plex Cover Manager v0.0.5`
6. Beide Dateien aus `dist/` als Assets hochladen.
7. Release notes kurz halten, z.B.:

```text
Plex Cover Manager v0.0.5

- Normal build for regular Windows desktops
- Portable Mesa build for VM/RDP/server environments
- Plex poster import and preview workflow
```

8. `Publish release` klicken.

## GitHub Release per CLI

Wenn GitHub CLI installiert ist:

```powershell
git tag v0.0.5
git push origin main
git push origin v0.0.5
gh release create v0.0.5 `
  .\dist\PlexCoverManager-v0.0.5.exe `
  .\dist\PlexCoverManager-v0.0.5-portable.exe `
  --title "Plex Cover Manager v0.0.5" `
  --notes "Plex Cover Manager v0.0.5"
```

Die `.exe`-Dateien sollten nicht ins Repository committed werden. Sie gehoeren als Release Assets in GitHub Releases.
