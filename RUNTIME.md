# Plex Cover Manager Runtime

`PlexCoverManager.exe` ist ein Single-EXE-Launcher. Der Launcher selbst hat keine direkte OpenGL- oder Fyne-Abhaengigkeit. Er enthaelt die eigentliche Fyne-App sowie eine Mesa/llvmpipe-Software-OpenGL-Runtime als eingebettetes Payload.

Die App-Version steht in `VERSION` und startet bei `0.0.1`.

Startablauf:

1. Der Launcher prueft schnell, ob das System mindestens OpenGL 2.1 per WGL bereitstellt.
2. Wenn ja, wird nur die App-EXE nach `%LOCALAPPDATA%\PlexCoverManager\runtime\...` entpackt und nativ gestartet.
3. Wenn nein, wird zusaetzlich die eingebettete Mesa-Runtime entpackt und die App mit `llvmpipe` gestartet.

Der erste Start kann wegen des Entpackens etwas laenger dauern. Danach wird die gecachte Runtime wiederverwendet. Mit `PCM_FORCE_MESA=1` kann der Mesa-Modus erzwungen werden.

Logs:

- Launcher: `%APPDATA%\PlexCoverManager\launcher.log`
- App: `%APPDATA%\PlexCoverManager\app.log`

Build:

```powershell
powershell -ExecutionPolicy Bypass -File .\build.ps1
```

Die Release-Datei landet unter `dist\PlexCoverManager-v<version>.exe`.
