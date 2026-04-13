# Release erstellen

Dieses Projekt nutzt `VERSION` als zentrale Versionsquelle. Fuer den ersten Release steht dort:

```text
0.0.1
```

## Build

```powershell
powershell -ExecutionPolicy Bypass -File .\build.ps1
```

Der Build erzeugt:

- `PlexCoverManager.exe` im Projektroot fuer lokale Tests
- `dist/PlexCoverManager-v0.0.1.exe` als Datei fuer den GitHub Release

## GitHub Release per Weboberflaeche

1. Alle Quellcode-Aenderungen committen und pushen.
2. Auf GitHub im Repository auf `Releases` gehen.
3. `Create a new release` waehlen.
4. Tag `v0.0.1` eintragen und erstellen lassen.
5. Release title: `Plex Cover Manager v0.0.1`
6. Unter Assets die Datei `dist/PlexCoverManager-v0.0.1.exe` hochladen.
7. Release notes kurz halten, z.B.:

```text
Initial release.

- Native Windows GUI
- Single-EXE launcher
- Mesa/OpenGL fallback for VMs and servers
- Plex poster naming and import preview
```

8. `Publish release` klicken.

## GitHub Release per CLI

Wenn GitHub CLI installiert ist:

```powershell
git tag v0.0.1
git push origin main
git push origin v0.0.1
gh release create v0.0.1 .\dist\PlexCoverManager-v0.0.1.exe --title "Plex Cover Manager v0.0.1" --notes "Initial release."
```

Die `.exe` sollte nicht ins Repository committed werden. Sie gehoert als Release Asset in GitHub Releases.
