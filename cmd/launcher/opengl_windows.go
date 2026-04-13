//go:build launcher && windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	wsPopup          = 0x80000000
	pfdDoubleBuffer  = 0x00000001
	pfdDrawToWindow  = 0x00000004
	pfdSupportOpenGL = 0x00000020
	pfdTypeRGBA      = 0
	pfdMainPlane     = 0
)

type pixelFormatDescriptor struct {
	NSize           uint16
	NVersion        uint16
	DwFlags         uint32
	IPixelType      byte
	CColorBits      byte
	CRedBits        byte
	CRedShift       byte
	CGreenBits      byte
	CGreenShift     byte
	CBlueBits       byte
	CBlueShift      byte
	CAlphaBits      byte
	CAlphaShift     byte
	CAccumBits      byte
	CAccumRedBits   byte
	CAccumGreenBits byte
	CAccumBlueBits  byte
	CAccumAlphaBits byte
	CDepthBits      byte
	CStencilBits    byte
	CAuxBuffers     byte
	ILayerType      byte
	BReserved       byte
	DwLayerMask     uint32
	DwVisibleMask   uint32
	DwDamageMask    uint32
}

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procCreateWindowExW = user32.NewProc("CreateWindowExW")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
	procGetDC           = user32.NewProc("GetDC")
	procReleaseDC       = user32.NewProc("ReleaseDC")

	procChoosePixelFormat = gdi32.NewProc("ChoosePixelFormat")
	procSetPixelFormat    = gdi32.NewProc("SetPixelFormat")
)

func hasSystemOpenGL() (bool, error) {
	opengl := syscall.NewLazyDLL(systemOpenGLPath())
	procWGLCreateContext := opengl.NewProc("wglCreateContext")
	procWGLMakeCurrent := opengl.NewProc("wglMakeCurrent")
	procWGLDeleteContext := opengl.NewProc("wglDeleteContext")
	procGLGetString := opengl.NewProc("glGetString")

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("STATIC"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(""))),
		wsPopup,
		0, 0, 16, 16,
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		return false, fmt.Errorf("Testfenster konnte nicht erstellt werden: %w", err)
	}
	defer procDestroyWindow.Call(hwnd)

	hdc, _, err := procGetDC.Call(hwnd)
	if hdc == 0 {
		return false, fmt.Errorf("Device Context konnte nicht gelesen werden: %w", err)
	}
	defer procReleaseDC.Call(hwnd, hdc)

	pfd := pixelFormatDescriptor{
		NSize:      uint16(unsafe.Sizeof(pixelFormatDescriptor{})),
		NVersion:   1,
		DwFlags:    pfdDrawToWindow | pfdSupportOpenGL | pfdDoubleBuffer,
		IPixelType: pfdTypeRGBA,
		CColorBits: 24,
		CDepthBits: 24,
		ILayerType: pfdMainPlane,
	}
	format, _, err := procChoosePixelFormat.Call(hdc, uintptr(unsafe.Pointer(&pfd)))
	if format == 0 {
		return false, fmt.Errorf("kein OpenGL-Pixelformat gefunden: %w", err)
	}
	ok, _, err := procSetPixelFormat.Call(hdc, format, uintptr(unsafe.Pointer(&pfd)))
	if ok == 0 {
		return false, fmt.Errorf("OpenGL-Pixelformat konnte nicht gesetzt werden: %w", err)
	}

	context, _, err := procWGLCreateContext.Call(hdc)
	if context == 0 {
		return false, fmt.Errorf("WGL-Kontext konnte nicht erstellt werden: %w", err)
	}
	defer procWGLDeleteContext.Call(context)

	ok, _, err = procWGLMakeCurrent.Call(hdc, context)
	if ok == 0 {
		return false, fmt.Errorf("WGL-Kontext konnte nicht aktiviert werden: %w", err)
	}
	defer procWGLMakeCurrent.Call(0, 0)

	versionPtr, _, err := procGLGetString.Call(0x1F02) // GL_VERSION
	if versionPtr == 0 {
		return false, fmt.Errorf("OpenGL-Version konnte nicht gelesen werden: %w", err)
	}
	version := cString(versionPtr)
	if !supportsOpenGL21(version) {
		return false, fmt.Errorf("OpenGL %s gefunden, benötigt wird mindestens OpenGL 2.1", version)
	}
	return true, nil
}

func systemOpenGLPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "opengl32.dll")
}

func cString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	bytes := make([]byte, 0, 32)
	for i := uintptr(0); ; i++ {
		b := *(*byte)(unsafe.Pointer(ptr + i))
		if b == 0 {
			break
		}
		bytes = append(bytes, b)
	}
	return string(bytes)
}

func supportsOpenGL21(version string) bool {
	fields := strings.Fields(version)
	if len(fields) == 0 {
		return false
	}
	parts := strings.Split(fields[0], ".")
	if len(parts) < 2 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minorText := parts[1]
	for i, r := range minorText {
		if r < '0' || r > '9' {
			minorText = minorText[:i]
			break
		}
	}
	minor, err := strconv.Atoi(minorText)
	if err != nil {
		return false
	}
	return major > 2 || major == 2 && minor >= 1
}
