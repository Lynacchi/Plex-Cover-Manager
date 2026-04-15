//go:build !windows

package ui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type nativeCommand struct {
	name string
	args []string
}

func openFolderInExplorer(path string) error {
	target := filepath.Clean(path)
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		target = filepath.Dir(target)
	}
	return startFirstAvailable(
		nativeCommand{name: "xdg-open", args: []string{target}},
		nativeCommand{name: "gio", args: []string{"open", target}},
	)
}

func openFileInExplorer(filePath string) error {
	return openFolderInExplorer(filepath.Dir(filePath))
}

func openFileWithDefault(filePath string) error {
	target := filepath.Clean(filePath)
	return startFirstAvailable(
		nativeCommand{name: "xdg-open", args: []string{target}},
		nativeCommand{name: "gio", args: []string{"open", target}},
	)
}

func selectCoverFiles(multi bool) ([]string, error) {
	zenityArgs := []string{
		"--file-selection",
		"--title=Cover auswaehlen",
		"--file-filter=Bilder | *.png *.jpg *.jpeg *.webp",
	}
	if multi {
		zenityArgs = append(zenityArgs, "--multiple", "--separator=\n")
	}
	if paths, handled, err := runDialogCommand("zenity", zenityArgs...); handled {
		return paths, err
	}

	kdialogArgs := []string{"--getopenfilename", ".", "Bilder (*.png *.jpg *.jpeg *.webp)"}
	if multi {
		kdialogArgs = []string{"--multiple", "--separate-output", "--getopenfilename", ".", "Bilder (*.png *.jpg *.jpeg *.webp)"}
	}
	if paths, handled, err := runDialogCommand("kdialog", kdialogArgs...); handled {
		return paths, err
	}

	return nil, fmt.Errorf("kein Dateidialog gefunden; installiere zenity oder kdialog")
}

func selectFolder() (string, error) {
	if paths, handled, err := runDialogCommand("zenity",
		"--file-selection",
		"--directory",
		"--title=Medienpfad auswaehlen",
	); handled {
		if err != nil || len(paths) == 0 {
			return "", err
		}
		return paths[0], nil
	}

	if paths, handled, err := runDialogCommand("kdialog", "--getexistingdirectory", "."); handled {
		if err != nil || len(paths) == 0 {
			return "", err
		}
		return paths[0], nil
	}

	return "", fmt.Errorf("kein Ordnerdialog gefunden; installiere zenity oder kdialog")
}

func startFirstAvailable(commands ...nativeCommand) error {
	var missing []string
	for _, command := range commands {
		path, err := exec.LookPath(command.name)
		if err != nil {
			missing = append(missing, command.name)
			continue
		}
		cmd := exec.Command(path, command.args...)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("%s konnte nicht gestartet werden: %w", command.name, err)
		}
		return nil
	}
	return fmt.Errorf("keines der benoetigten Tools wurde gefunden: %s", strings.Join(missing, ", "))
}

func runDialogCommand(name string, args ...string) ([]string, bool, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, false, nil
	}
	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(string(output)) == "" {
			return nil, true, nil
		}
		return nil, true, fmt.Errorf("%s fehlgeschlagen: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return splitDialogOutput(output), true, nil
}

func splitDialogOutput(output []byte) []string {
	lines := bytes.Split(output, []byte{'\n'})
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		path := strings.TrimSpace(string(line))
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
