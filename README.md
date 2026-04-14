# Plex Cover Manager

Plex Cover Manager ist eine kleine Windows-Desktop-App zum Verwalten lokaler Plex-Poster. Du kannst heruntergeladene Cover-Dateien importieren, vor dem Schreiben pruefen und Plex-konform in deiner Medienstruktur ablegen lassen.

## Features

- Serien- und Filmordner scannen
- fehlende, teilweise vorhandene und vollstaendige Cover anzeigen
- Batch-Import mehrerer Cover-Dateien
- Preview vor jedem Schreibvorgang
- Plex-konforme Dateinamen wie `poster.jpg` und `season01-poster.jpg`
- Jellyfin-Modus mit `poster.jpg` in Staffelordnern und `seasonXX-poster.jpg` als Flat-Fallback
- JPEG-Komprimierung mit konfigurierbarer Qualitaet und Maximalaufloesung
- optional deaktivierbare Komprimierung
- Komprimierungserkennung fuer zu grosse oder nicht als JPEG gespeicherte Cover
- smarte Erkennung vorhandener Alias-Cover wie `folder.jpg`, `cover.jpg` oder heruntergeladene Staffelcover
- optionales Umbenennen erkannter Alias-Cover auf den aktuellen Plex-/Jellyfin-Zielnamen
- Detail-Refresh fuer einzelne Titel ohne kompletten Bibliotheksscan
- Filter fuer alle Titel, Filme oder Serien
- SMB-/UNC-Pfade wie `\\Server\Share\Media`
- schnelle manuelle Pfadeingabe fuer Netzlaufwerke
- gezieltes Drag & Drop auf einzelne Cover-Positionen in der Detailansicht

## Download

Releases enthalten zwei Windows-EXEs:

- `PlexCoverManager-v0.2.0.exe`
  Normale Variante. Kleiner, weniger antivirus-anfaellig, nutzt den vorhandenen Windows-Grafiktreiber. Empfohlen fuer normale Desktop-PCs.

- `PlexCoverManager-v0.2.0-portable.exe`
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
