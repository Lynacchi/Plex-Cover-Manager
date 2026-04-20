//go:build windows

package diagnostics

import (
	"syscall"
	"unsafe"
)

const (
	mbIconError = 0x00000010
	mbYesNo     = 0x00000004
	idYes       = 6
	swShow      = 5
)

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")

	procMessageBoxW   = user32.NewProc("MessageBoxW")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

func ShowGraphicsError(err error) {
	text := "Plex Cover Manager kann die grafische Oberfläche nicht starten.\n\n" +
		"Windows meldet, dass OpenGL nicht verfügbar ist oder der Grafiktreiber es nicht unterstützt.\n\n" +
		"Bitte installiere oder aktualisiere den Grafiktreiber. Auf Remote-Desktops oder sehr reduzierten Windows-Installationen kann OpenGL fehlen.\n\n" +
		"Details: " + err.Error() + "\n\n" +
		"Soll die Microsoft-Hilfeseite zum Aktualisieren von Treibern geöffnet werden?"
	title := "Plex Cover Manager - OpenGL fehlt"
	result, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))),
		mbIconError|mbYesNo,
	)
	if result == idYes {
		OpenURL("https://support.microsoft.com/windows/update-drivers-manually-in-windows-ec62f46c-ff14-c91d-eead-d7126dc1f7b6")
	}
}

func OpenURL(url string) {
	procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("open"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(url))),
		0,
		0,
		swShow,
	)
}
