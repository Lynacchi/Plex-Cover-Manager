//go:build launcher

package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const appPayloadName = "PlexCoverManager.app.exe"

var payloadVersion = "dev"
var appVersion = "0.3.0"

//go:embed payload/*
var payload embed.FS

func main() {
	logLine("launcher start app_version=%s payload_version=%s", appVersion, payloadVersion)

	mode := "native"
	if strings.EqualFold(os.Getenv("PCM_FORCE_MESA"), "1") {
		mode = "mesa"
		logLine("mesa forced by environment")
	} else if ok, err := hasSystemOpenGL(); !ok {
		mode = "mesa"
		logLine("system opengl unavailable: %v", err)
	} else {
		logLine("system opengl ok")
	}

	dir, err := ensureRuntime(mode)
	if err != nil {
		logLine("runtime extraction failed: %v", err)
		showError("Plex Cover Manager konnte die portable Runtime nicht vorbereiten.\n\n"+err.Error(), false)
		return
	}
	if err := launchApp(dir, mode); err != nil {
		logLine("launch failed: %v", err)
		showError("Plex Cover Manager konnte nicht gestartet werden.\n\n"+err.Error(), false)
		return
	}
}

func ensureRuntime(mode string) (string, error) {
	base, err := runtimeBaseDir()
	if err != nil {
		return "", err
	}
	version := payloadVersion
	if version == "" || version == "dev" {
		version = payloadDigest()
	}
	target := filepath.Join(base, version, mode)

	names, err := requiredPayloadFiles(mode)
	if err != nil {
		return "", err
	}
	if runtimeReady(target, version, names) {
		return target, nil
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	for _, name := range names {
		if err := extractPayloadFile(name, filepath.Join(target, name)); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(target, ".payload-version"), []byte(version), 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func requiredPayloadFiles(mode string) ([]string, error) {
	entries, err := fs.ReadDir(payload, "payload")
	if err != nil {
		return nil, err
	}
	names := []string{appPayloadName}
	if mode == "mesa" {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.EqualFold(filepath.Ext(name), ".dll") {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

func runtimeReady(target, version string, names []string) bool {
	data, err := os.ReadFile(filepath.Join(target, ".payload-version"))
	if err != nil || string(data) != version {
		return false
	}
	for _, name := range names {
		entry, err := fs.Stat(payload, filepath.ToSlash(filepath.Join("payload", name)))
		if err != nil {
			return false
		}
		info, err := os.Stat(filepath.Join(target, name))
		if err != nil || info.Size() != entry.Size() {
			return false
		}
	}
	return true
}

func extractPayloadFile(name, target string) error {
	data, err := payload.ReadFile(filepath.ToSlash(filepath.Join("payload", name)))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(target)
		if err := os.Rename(tmp, target); err != nil {
			return err
		}
	}
	return nil
}

func launchApp(dir, mode string) error {
	appPath := filepath.Join(dir, appPayloadName)
	cmd := exec.Command(appPath)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if launcherPath, err := os.Executable(); err == nil {
		cmd.Env = append(cmd.Env, "PCM_ORIGINALS_DIR="+filepath.Join(filepath.Dir(launcherPath), "originals"))
	}
	if mode == "mesa" {
		cmd.Env = append(cmd.Env,
			"LIBGL_ALWAYS_SOFTWARE=true",
			"GALLIUM_DRIVER=llvmpipe",
			"MESA_LOADER_DRIVER_OVERRIDE=llvmpipe",
			"PCM_RUNTIME_MODE=mesa",
			"PCM_SKIP_APP_OPENGL_CHECK=1",
		)
	} else {
		cmd.Env = append(cmd.Env,
			"PCM_RUNTIME_MODE=native",
			"PCM_SKIP_APP_OPENGL_CHECK=1",
		)
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("die App wurde sofort beendet: %w", err)
		}
		return fmt.Errorf("die App wurde sofort wieder beendet")
	case <-time.After(900 * time.Millisecond):
		return nil
	}
}

func runtimeBaseDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		base = dir
	}
	return filepath.Join(base, "PlexCoverManager", "runtime"), nil
}

func payloadDigest() string {
	entries, err := fs.ReadDir(payload, "payload")
	if err != nil {
		return "dev"
	}
	hash := sha256.New()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := payload.ReadFile(filepath.ToSlash(filepath.Join("payload", name)))
		if err != nil {
			continue
		}
		hash.Write([]byte(name))
		hash.Write([]byte{0})
		sum := sha256.Sum256(data)
		hash.Write(sum[:])
	}
	return hex.EncodeToString(hash.Sum(nil))[:16]
}

func logLine(format string, args ...any) {
	path, err := launcherLogPath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func launcherLogPath() (string, error) {
	base := os.Getenv("APPDATA")
	if base == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = dir
	}
	return filepath.Join(base, "PlexCoverManager", "launcher.log"), nil
}
