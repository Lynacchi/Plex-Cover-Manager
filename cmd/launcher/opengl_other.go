//go:build launcher && !windows

package main

func hasSystemOpenGL() (bool, error) {
	return true, nil
}
