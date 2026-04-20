# Plex Cover Manager Runtime

Es gibt zwei Windows-Builds. Linux wird als natives Binary gebaut; der portable Mesa-Launcher ist Windows-spezifisch.

Unabhängig vom Build sind die GUI-Funktionen wie Cover-Backup und Fehllisten-Ansicht gleich verfügbar.

## Normal

`PlexCoverManager-v<version>.exe` startet die Fyne-App direkt. Alle Go- und Fyne-Abhängigkeiten sind in der EXE enthalten. Das System muss aber einen funktionierenden Windows-Grafiktreiber mit mindestens OpenGL 2.1 bereitstellen.

Wenn OpenGL fehlt, zeigt die App vor dem GUI-Start eine Fehlermeldung mit Link zur Microsoft-Treiberhilfe.

## Portable

`PlexCoverManager-v<version>-portable.exe` ist ein Single-EXE-Launcher. Er enthält:

- die eigentliche Fyne-App
- Mesa/llvmpipe als Software-OpenGL-Fallback

Startablauf:

1. Der Launcher prüft schnell, ob das System mindestens OpenGL 2.1 per WGL bereitstellt.
2. Wenn ja, wird nur die App-EXE nach `%LOCALAPPDATA%\PlexCoverManager\runtime\...` entpackt und nativ gestartet.
3. Wenn nein, wird zusätzlich die eingebettete Mesa-Runtime entpackt und die App mit `llvmpipe` gestartet.

Der erste Start der portablen Variante kann wegen des Entpackens etwas länger dauern. Danach wird die gecachte Runtime wiederverwendet. Mit `PCM_FORCE_MESA=1` kann der Mesa-Modus erzwungen werden.

## Logs

- Launcher: `%APPDATA%\PlexCoverManager\launcher.log`
- App: `%APPDATA%\PlexCoverManager\app.log`

## Build

```powershell
powershell -ExecutionPolicy Bypass -File .\build-normal.ps1
powershell -ExecutionPolicy Bypass -File .\build-portable.ps1
```

Die Release-Dateien landen unter `dist/`.

Linux:

```bash
sh ./build-linux.sh
```

Der Linux-Build benötigt CGO sowie die Fyne/GLFW-Systembibliotheken der jeweiligen Distribution.
