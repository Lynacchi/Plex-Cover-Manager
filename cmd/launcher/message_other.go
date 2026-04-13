//go:build launcher && !windows

package main

func showError(text string, offerDriverHelp bool) {
	_ = offerDriverHelp
	logLine("error: %s", text)
}
