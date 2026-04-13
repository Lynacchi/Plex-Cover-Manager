# Plex Cover Manager

Plex Cover Manager ist eine kleine Windows-Desktop-App zum Verwalten lokaler Plex-Poster. Du kannst heruntergeladene Cover-Dateien importieren, vor dem Schreiben prüfen und Plex-konform in deiner Medienstruktur ablegen lassen.

## Features

- Serien- und Filmordner scannen
- fehlende, teilweise vorhandene und vollständige Cover anzeigen
- Batch-Import mehrerer Cover-Dateien
- Preview vor jedem Schreibvorgang
- Plex-konforme Dateinamen wie `poster.jpg` und `season01-poster.jpg`
- JPEG-Komprimierung mit konfigurierbarer Qualität und Maximalauflösung
- Single-EXE für Windows
- eingebauter Mesa/OpenGL-Fallback für VMs, RDP- und Server-Umgebungen

## Download

Die fertige EXE wird über GitHub Releases bereitgestellt:

```text
PlexCoverManager-v0.0.1.exe
```

Die App benötigt keine Installation. Einfach die EXE starten.

## Build

Voraussetzungen:

- Go
- MSYS2 mit MinGW64 GCC

Build starten:

```powershell
powershell -ExecutionPolicy Bypass -File .\build.ps1
```

Der Build erzeugt:

- `PlexCoverManager.exe`
- `dist\PlexCoverManager-v0.0.1.exe`

## Version

Die aktuelle Version steht in:

```text
VERSION
```

Für neue Releases die Version dort erhöhen, neu bauen und die Datei aus `dist/` als GitHub Release Asset hochladen.

## Hinweise

Die App speichert ihre Konfiguration unter:

```text
%APPDATA%\PlexCoverManager\config.json
```

Diagnose-Logs liegen bei Bedarf unter:

```text
%APPDATA%\PlexCoverManager\
```
