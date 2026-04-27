# Builds erstellen

Die Build-Skripte lesen die Versionsnummer aus `VERSION` und schreiben die fertigen Dateien nach `dist/`.

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

Oder aus Windows per Docker:

```powershell
powershell -ExecutionPolicy Bypass -File .\build-linux-via-docker.ps1
```

Die Windows-Builds erzeugen in `dist/`:

- `PlexCoverManager-v0.4.3.exe`
- `PlexCoverManager-v0.4.3-portable.exe`

Der Linux-Build erzeugt als Release-Asset:

- `PlexCoverManager-v0.4.3-linux-amd64.tar.gz`

Das Archiv enthält:

- `PlexCoverManager-v0.4.3-linux-amd64`
- `plex-cover-manager.png`
- `PlexCoverManager-v0.4.3-linux-amd64.desktop`
- `install-linux-desktop.sh`

Die normale Variante ist für normale Windows-PCs gedacht. Die portable Variante ist für VMs, RDP, KVM-Server und Systeme ohne verlässliches OpenGL gedacht.

Die aktuelle Oberfläche enthält außerdem ein Cover-Backup und eine Ansicht für fehlende Cover.
