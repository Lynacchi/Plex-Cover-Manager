//go:build !windows

package diagnostics

func ShowGraphicsError(err error) {
	Log("graphics startup error: %v", err)
}

func OpenURL(url string) {
	_ = url
}
