# Plex Cover Manager

Plex Cover Manager ist eine kleine Desktop-App zum Verwalten lokaler Plex- und Jellyfin-Poster. Du kannst heruntergeladene Cover-Dateien importieren, vor dem Schreiben pruefen und passend fuer Plex oder Jellyfin in deiner Medienstruktur ablegen lassen.

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
- konfigurierbarer Speicherort fuer Original-Backups bei Komprimierung
- smarte Erkennung vorhandener Alias-Cover wie `folder.jpg`, `cover.jpg` oder heruntergeladene Staffelcover
- optionales Umbenennen erkannter Alias-Cover auf den aktuellen Plex-/Jellyfin-Zielnamen
- optionaler theposterdb.com-Suchbutton fuer Titel mit fehlenden Covern
- Detail-Refresh fuer einzelne Titel ohne kompletten Bibliotheksscan
- Filter fuer alle Titel, Filme oder Serien
- SMB-/UNC-Pfade wie `\\Server\Share\Media`
- schnelle manuelle Pfadeingabe fuer Netzlaufwerke
- gezieltes Drag & Drop auf einzelne Cover-Positionen in der Detailansicht

## Download

Releases enthalten Windows- und Linux-Builds:

### Windows

- `PlexCoverManager-v0.3.3.exe`
  Normale Variante. Kleiner und nutzt den vorhandenen Windows-Grafiktreiber. Empfohlen fuer normale Desktop-PCs.

- `PlexCoverManager-v0.3.3-portable.exe`
  Portable Variante mit eingebettetem Mesa/OpenGL-Fallback. Groesser, dafuer besser fuer VMs, RDP-Sitzungen, KVM-Server und Systeme ohne brauchbares OpenGL.

Die Windows-App benoetigt keine Installation. Einfach die passende EXE starten.

### Linux

- `PlexCoverManager-v0.3.3-linux-amd64`
  Natives Linux-Binary fuer x86_64/amd64-Systeme.

Nach dem Download ausfuehrbar machen und starten:

```bash
chmod +x PlexCoverManager-v0.3.3-linux-amd64
./PlexCoverManager-v0.3.3-linux-amd64
```

Datei- und Ordnerdialoge nutzen unter Linux `zenity` oder `kdialog`; Datei-/Ordneroeffnen nutzt `xdg-open` oder `gio`.

## Build

### Windows

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

### Linux

Voraussetzungen:

- Go in der Version aus `go.mod`
- `gcc`, `pkg-config`
- Fyne/GLFW-Systembibliotheken, z.B. unter Debian/Ubuntu:

```bash
sudo apt install gcc pkg-config libgl1-mesa-dev xorg-dev
```

Build:

```bash
sh ./build-linux.sh
```

Linux-Build aus Windows per Docker:

```powershell
powershell -ExecutionPolicy Bypass -File .\build-linux-via-docker.ps1
```

Der Linux-Build erzeugt ein natives Binary in `dist/`. Datei- und Ordnerdialoge nutzen unter Linux `zenity` oder `kdialog`; Datei-/Ordneroeffnen nutzt `xdg-open` oder `gio`.

## Speicherorte

Konfiguration:

```text
%APPDATA%\PlexCoverManager\config.json
```

Logs:

```text
%APPDATA%\PlexCoverManager\
```

Unter Linux nutzt die App die Standardpfade von `os.UserConfigDir`, typischerweise:

```text
~/.config/PlexCoverManager/
```
