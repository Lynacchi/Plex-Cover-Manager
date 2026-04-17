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

- `PlexCoverManager-v0.3.3.exe`
- `PlexCoverManager-v0.3.3-portable.exe`

Der Linux-Build erzeugt z.B.:

- `PlexCoverManager-v0.3.3-linux-amd64`

Die normale Variante ist fuer normale Windows-PCs gedacht. Die portable Variante ist fuer VMs, RDP, KVM-Server und Systeme ohne verlaessliches OpenGL gedacht.
