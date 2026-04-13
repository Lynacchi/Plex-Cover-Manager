//go:build launcher && !windows

package main

func smoothStartupWindow(pid uint32) {
	_ = pid
}
