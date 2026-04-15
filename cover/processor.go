package cover

import (
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"

	"plexcovermanager/models"
)

type ProcessResult struct {
	Width     int
	Height    int
	SizeBytes int64
}

func ApplyImportPlan(plan ImportPlan, compression models.CompressionConfig) (ProcessResult, error) {
	if !plan.CanApply {
		return ProcessResult{}, fmt.Errorf("Import nicht anwendbar: %s", plan.Status)
	}
	targetPath := FinalImportTargetPath(plan, compression)
	result, err := ProcessCover(plan.SourcePath, targetPath, compression)
	if err != nil {
		return ProcessResult{}, err
	}
	if plan.ExistingPath != "" && !samePath(plan.ExistingPath, targetPath) {
		_ = os.Remove(plan.ExistingPath)
	}
	return result, nil
}

func FinalImportTargetPath(plan ImportPlan, compression models.CompressionConfig) string {
	targetPath := plan.TargetPath
	if compression.Disabled && targetPath != "" {
		sourceExt := strings.ToLower(filepath.Ext(plan.SourcePath))
		targetPath = strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + sourceExt
	}
	return targetPath
}

func ProcessCover(sourcePath, targetPath string, compression models.CompressionConfig) (ProcessResult, error) {
	if compression.Disabled {
		return copyFile(sourcePath, targetPath)
	}
	compressionConfig := normalizeCompression(compression)
	img, err := decodeImage(sourcePath)
	if err != nil {
		return ProcessResult{}, err
	}
	processed := resizeAndFlatten(img, compressionConfig.MaxWidth, compressionConfig.MaxHeight)

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return ProcessResult{}, err
	}
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".plex-cover-*.jpg")
	if err != nil {
		return ProcessResult{}, err
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := jpeg.Encode(tempFile, processed, &jpeg.Options{Quality: compressionConfig.JPEGQuality}); err != nil {
		_ = tempFile.Close()
		return ProcessResult{}, err
	}
	if err := tempFile.Close(); err != nil {
		return ProcessResult{}, err
	}

	if err := replaceFile(tempPath, targetPath); err != nil {
		return ProcessResult{}, err
	}
	cleanupTemp = false

	info, err := os.Stat(targetPath)
	if err != nil {
		return ProcessResult{}, err
	}
	bounds := processed.Bounds()
	return ProcessResult{
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
		SizeBytes: info.Size(),
	}, nil
}

func copyFile(sourcePath, targetPath string) (ProcessResult, error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return ProcessResult{}, err
	}
	src, err := os.Open(sourcePath)
	if err != nil {
		return ProcessResult{}, err
	}
	defer src.Close()

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".plex-cover-*"+filepath.Ext(sourcePath))
	if err != nil {
		return ProcessResult{}, err
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := io.Copy(tempFile, src); err != nil {
		_ = tempFile.Close()
		return ProcessResult{}, err
	}
	if err := tempFile.Close(); err != nil {
		return ProcessResult{}, err
	}

	if err := replaceFile(tempPath, targetPath); err != nil {
		return ProcessResult{}, err
	}
	cleanupTemp = false

	info, err := os.Stat(targetPath)
	if err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{SizeBytes: info.Size()}, nil
}

// CompressCover compresses an existing cover to JPEG, backing up the original first.
func CompressCover(slot models.CoverSlot, itemTitle string, compression models.CompressionConfig, normalizeName bool) (ProcessResult, error) {
	if !slot.Exists || slot.ExistingPath == "" {
		return ProcessResult{}, fmt.Errorf("kein Cover vorhanden")
	}
	backupDir, err := OriginalBackupDir()
	if err != nil {
		return ProcessResult{}, fmt.Errorf("Backup-Ordner nicht ermittelbar: %w", err)
	}
	if err := backupFile(slot.ExistingPath, itemTitle, slot.Label, backupDir); err != nil {
		return ProcessResult{}, fmt.Errorf("Backup fehlgeschlagen: %w", err)
	}
	comp := compression
	comp.Disabled = false
	targetPath := slot.TargetPath
	if !normalizeName {
		targetPath = optimizedSiblingPath(slot.ExistingPath)
	}
	result, err := ProcessCover(slot.ExistingPath, targetPath, comp)
	if err != nil {
		return ProcessResult{}, err
	}
	if !samePath(slot.ExistingPath, targetPath) {
		_ = os.Remove(slot.ExistingPath)
	}
	return result, nil
}

// RenameCoverToTargetName moves a detected cover to the current server-mode naming without recompressing it.
func RenameCoverToTargetName(slot models.CoverSlot) (string, error) {
	if !slot.Exists || slot.ExistingPath == "" {
		return "", fmt.Errorf("kein Cover vorhanden")
	}
	targetPath := RenameTargetPath(slot)
	if samePath(slot.ExistingPath, targetPath) {
		return targetPath, nil
	}
	if _, err := os.Stat(targetPath); err == nil {
		return "", fmt.Errorf("Zieldatei existiert bereits: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(slot.ExistingPath, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func RenameTargetPath(slot models.CoverSlot) string {
	return targetPathWithExistingExtension(slot)
}

func optimizedSiblingPath(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".jpg"
}

func targetPathWithExistingExtension(slot models.CoverSlot) string {
	ext := strings.ToLower(filepath.Ext(slot.ExistingPath))
	if ext == "" {
		ext = filepath.Ext(slot.TargetPath)
	}
	return strings.TrimSuffix(slot.TargetPath, filepath.Ext(slot.TargetPath)) + ext
}

// OriginalBackupDir returns the directory next to the executable for storing originals.
func OriginalBackupDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("PCM_ORIGINALS_DIR")); dir != "" {
		return filepath.Clean(dir), nil
	}
	if runtime.GOOS != "windows" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "PlexCoverManager", "originals"), nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "originals"), nil
}

func backupFile(existingPath, itemTitle, slotLabel, backupDir string) error {
	safeName := sanitizeFileName(itemTitle)
	destDir := filepath.Join(backupDir, safeName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	fileName := filepath.Base(existingPath)
	safeSlotLabel := sanitizeFileName(slotLabel)
	if safeSlotLabel == "" {
		safeSlotLabel = "Cover"
	}
	destPath := filepath.Join(destDir, safeSlotLabel+" - "+fileName)
	if _, err := os.Stat(destPath); err == nil {
		base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		ext := filepath.Ext(fileName)
		for i := 2; i < 100; i++ {
			destPath = filepath.Join(destDir, fmt.Sprintf("%s - %s_%d%s", safeSlotLabel, base, i, ext))
			if _, err := os.Stat(destPath); err != nil {
				break
			}
		}
	}
	src, err := os.Open(existingPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer(
		"<", "_", ">", "_", ":", "_", "\"", "_",
		"/", "_", "\\", "_", "|", "_", "?", "_", "*", "_",
	)
	result := replacer.Replace(name)
	if len(result) > 120 {
		result = result[:120]
	}
	return strings.TrimSpace(result)
}

// RenameCoversForModeSwitch renames season cover files to match the target server mode.
func RenameCoversForModeSwitch(items []models.MediaItem, toMode models.ServerMode) (int, []string) {
	renamed := 0
	var errs []string
	for _, item := range items {
		if item.Type != models.MediaTypeSeries || item.FlatStructure {
			continue
		}
		for _, slot := range item.CoverSlots {
			if slot.Kind != models.CoverKindSeason || !slot.Exists || slot.ExistingPath == "" {
				continue
			}
			dir := filepath.Dir(slot.ExistingPath)
			ext := filepath.Ext(slot.ExistingPath)
			var newBase string
			if toMode == models.ServerModeJellyfin {
				newBase = "poster"
			} else {
				newBase = fmt.Sprintf("season%02d-poster", slot.SeasonNumber)
			}
			newPath := filepath.Join(dir, newBase+ext)
			if samePath(slot.ExistingPath, newPath) {
				continue
			}
			if _, err := os.Stat(newPath); err == nil {
				errs = append(errs, fmt.Sprintf("%s → %s: Zieldatei existiert bereits", filepath.Base(slot.ExistingPath), filepath.Base(newPath)))
				continue
			}
			if err := os.Rename(slot.ExistingPath, newPath); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", slot.ExistingPath, err))
				continue
			}
			renamed++
		}
	}
	return renamed, errs
}

func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		img, err := jpeg.Decode(file)
		if err != nil {
			return nil, err
		}
		return img, nil
	case ".png":
		img, err := png.Decode(file)
		if err != nil {
			return nil, err
		}
		return img, nil
	case ".webp":
		img, err := webp.Decode(file)
		if err != nil {
			return nil, err
		}
		return img, nil
	default:
		return nil, fmt.Errorf("nicht unterstütztes Format: %s", filepath.Ext(path))
	}
}

func resizeAndFlatten(src image.Image, maxWidth, maxHeight int) *image.RGBA {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	targetWidth, targetHeight := width, height
	if width > maxWidth || height > maxHeight {
		scale := math.Min(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
		targetWidth = int(math.Round(float64(width) * scale))
		targetHeight = int(math.Round(float64(height) * scale))
		if targetWidth < 1 {
			targetWidth = 1
		}
		if targetHeight < 1 {
			targetHeight = 1
		}
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)
	if targetWidth == width && targetHeight == height {
		stddraw.Draw(dst, dst.Bounds(), src, bounds.Min, stddraw.Over)
		return dst
	}
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, xdraw.Over, nil)
	return dst
}

func replaceFile(tempPath, targetPath string) error {
	backupPath := ""
	if _, err := os.Stat(targetPath); err == nil {
		backupPath = targetPath + ".bak"
		_ = os.Remove(backupPath)
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, targetPath)
		}
		return err
	}
	if backupPath != "" {
		_ = os.Remove(backupPath)
	}
	return nil
}

func normalizeCompression(compression models.CompressionConfig) models.CompressionConfig {
	cfg := models.AppConfig{Compression: compression}
	cfg.Normalize()
	return cfg.Compression
}

func samePath(a, b string) bool {
	left := filepath.Clean(a)
	right := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
