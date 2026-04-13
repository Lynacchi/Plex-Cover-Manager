package cover

import (
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
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
	result, err := ProcessCover(plan.SourcePath, plan.TargetPath, compression)
	if err != nil {
		return ProcessResult{}, err
	}
	if plan.ExistingPath != "" && !samePath(plan.ExistingPath, plan.TargetPath) {
		_ = os.Remove(plan.ExistingPath)
	}
	return result, nil
}

func ProcessCover(sourcePath, targetPath string, compression models.CompressionConfig) (ProcessResult, error) {
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
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
