//go:build launcher && windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	mbIconError     = 0x00000010
	mbYesNo         = 0x00000004
	mbSetForeground = 0x00010000
	idYes           = 6
	swShow          = 5
)

var (
	messageUser32  = syscall.NewLazyDLL("user32.dll")
	messageShell32 = syscall.NewLazyDLL("shell32.dll")

	procMessageBoxW   = messageUser32.NewProc("MessageBoxW")
	procShellExecuteW = messageShell32.NewProc("ShellExecuteW")
)

func showError(text string, offerDriverHelp bool) {
	if offerDriverHelp {
		text += "\n\nSoll die Microsoft-Hilfeseite zum Aktualisieren von Treibern geöffnet werden?"
	}
	flags := uintptr(mbIconError | mbSetForeground)
	if offerDriverHelp {
		flags |= mbYesNo
	}
	result, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Plex Cover Manager"))),
		flags,
	)
	if offerDriverHelp && result == idYes {
		openURL("https://support.microsoft.com/windows/update-drivers-manually-in-windows-ec62f46c-ff14-c91d-eead-d7126dc1f7b6")
	}
}

func openURL(url string) {
	procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("open"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(url))),
		0,
		0,
		swShow,
	)
}
