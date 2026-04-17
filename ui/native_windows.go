//go:build windows

package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const swShowNormal = 1

var (
	nativeShell32       = syscall.NewLazyDLL("shell32.dll")
	nativeShellExecuteW = nativeShell32.NewProc("ShellExecuteW")
)

func openFolderInExplorer(path string) error {
	target := filepath.FromSlash(path)
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		target = filepath.Dir(target)
	}
	return shellExecute("open", target, "", "")
}

func openFileInExplorer(filePath string) error {
	target := filepath.FromSlash(filePath)
	return shellExecute("open", "explorer.exe", fmt.Sprintf(`/select,"%s"`, target), "")
}

func openURLInBrowser(rawURL string) error {
	return shellExecute("open", rawURL, "", "")
}

func shellExecute(verb, file, params, dir string) error {
	result, _, err := nativeShellExecuteW.Call(
		0,
		utf16Arg(verb),
		utf16Arg(file),
		utf16Arg(params),
		utf16Arg(dir),
		swShowNormal,
	)
	if result <= 32 {
		if err != syscall.Errno(0) {
			return fmt.Errorf("Windows konnte %q nicht öffnen: %w", file, err)
		}
		return fmt.Errorf("Windows konnte %q nicht öffnen (ShellExecute-Code %d)", file, result)
	}
	return nil
}

func utf16Arg(value string) uintptr {
	if value == "" {
		return 0
	}
	return uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(value)))
}

func selectCoverFiles(multi bool) ([]string, error) {
	multiValue := "$false"
	if multi {
		multiValue = "$true"
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.OpenFileDialog
$dialog.Title = 'Cover auswählen'
$dialog.Filter = 'Bilder (*.png;*.jpg;*.jpeg;*.webp)|*.png;*.jpg;*.jpeg;*.webp'
$dialog.Multiselect = %s
if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  $dialog.FileNames | ForEach-Object { [Console]::WriteLine($_) }
}
`, multiValue)
	return runPowerShellDialog(script)
}

func selectFolder() (string, error) {
	script := `
Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.OpenFileDialog
$dialog.Title = 'Medienpfad auswählen'
$dialog.CheckFileExists = $false
$dialog.CheckPathExists = $true
$dialog.ValidateNames = $false
$dialog.FileName = 'Ordner auswählen'
$dialog.Filter = 'Ordner|*.'
if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  $selected = $dialog.FileName
  if ([System.IO.Directory]::Exists($selected)) {
    [Console]::WriteLine($selected)
  } else {
    [Console]::WriteLine([System.IO.Path]::GetDirectoryName($selected))
  }
}
`
	paths, err := runPowerShellDialog(script)
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func runPowerShellDialog(script string) ([]string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	var paths []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			paths = append(paths, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return paths, nil
}
