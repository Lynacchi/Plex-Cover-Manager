//go:build launcher && windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

const (
	swHide   = 0
	swShowNA = 8
)

var (
	procEnumWindows              = messageUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessID = messageUser32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW           = messageUser32.NewProc("GetWindowTextW")
	procIsWindow                 = messageUser32.NewProc("IsWindow")
	procIsWindowVisible          = messageUser32.NewProc("IsWindowVisible")
	procShowWindow               = messageUser32.NewProc("ShowWindow")
)

func smoothStartupWindow(pid uint32) {
	const searchTimeout = 1600 * time.Millisecond
	deadline := time.Now().Add(searchTimeout)

	for time.Now().Before(deadline) {
		hwnd := findTopLevelWindow(pid)
		if hwnd == 0 {
			time.Sleep(4 * time.Millisecond)
			continue
		}

		procShowWindow.Call(hwnd, swHide)
		time.Sleep(220 * time.Millisecond)
		if isWindow(hwnd) {
			procShowWindow.Call(hwnd, swShowNA)
		}
		return
	}
}

func findTopLevelWindow(pid uint32) uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		var windowPID uint32
		procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if windowPID == pid && isWindow(hwnd) && windowTitle(hwnd) == "Plex Cover Manager" {
			found = hwnd
			return 0
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return found
}

func isWindow(hwnd uintptr) bool {
	ok, _, _ := procIsWindow.Call(hwnd)
	return ok != 0
}

func isWindowVisible(hwnd uintptr) bool {
	ok, _, _ := procIsWindowVisible.Call(hwnd)
	return ok != 0
}

func windowTitle(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
