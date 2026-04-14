# Plex Cover Manager

Plex Cover Manager ist eine kleine Windows-Desktop-App zum Verwalten lokaler Plex-Poster. Du kannst heruntergeladene Cover-Dateien importieren, vor dem Schreiben pruefen und Plex-konform in deiner Medienstruktur ablegen lassen.

## Features

- Serien- und Filmordner scannen
- fehlende, teilweise vorhandene und vollstaendige Cover anzeigen
- Batch-Import mehrerer Cover-Dateien
- Preview vor jedem Schreibvorgang
- Plex-konforme Dateinamen wie `poster.jpg` und `season01-poster.jpg`
- JPEG-Komprimierung mit konfigurierbarer Qualitaet und Maximalaufloesung
- SMB-/UNC-Pfade wie `\\Server\Share\Media`

## Download

Releases enthalten zwei Windows-EXEs:

- `PlexCoverManager-v0.0.5.exe`
  Normale Variante. Kleiner, weniger antivirus-anfaellig, nutzt den vorhandenen Windows-Grafiktreiber. Empfohlen fuer normale Desktop-PCs.

- `PlexCoverManager-v0.0.5-portable.exe`
  Portable Variante mit eingebettetem Mesa/OpenGL-Fallback. Groesser und eher antivirus-anfaellig, dafuer besser fuer VMs, RDP-Sitzungen, KVM-Server und Systeme ohne brauchbares OpenGL.

Die App benoetigt keine Installation. Einfach die passende EXE starten.

## Build

Voraussetzungen:

- Go
- MSYS2 mit MinGW64 GCC

Normale Release-EXE bauen:

```powershell
powershell -ExecutionPolicy Bypass -File .\build-normal.ps1
```

Portable Release-EXE mit Mesa-Fallback bauen:

```powershell
powershell -ExecutionPolicy Bypass -File .\build-portable.ps1
```

Die fertigen Dateien landen ausschliesslich in `dist/`.

## Version

Die aktuelle Version steht in:

```text
VERSION
```

Fuer neue Releases die Version dort erhoehen, beide Builds ausfuehren und die Dateien aus `dist/` als GitHub Release Assets hochladen.

## Speicherorte

Konfiguration:

```text
%APPDATA%\PlexCoverManager\config.json
```

Logs:

```text
%APPDATA%\PlexCoverManager\
```
