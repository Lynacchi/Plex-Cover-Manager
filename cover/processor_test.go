package cover

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"plexcovermanager/models"
)

func TestProcessCoverResizesAndWritesJPEG(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	target := filepath.Join(dir, "poster.jpg")

	img := image.NewRGBA(image.Rect(0, 0, 2000, 3000))
	for y := 0; y < 3000; y++ {
		for x := 0; x < 2000; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 40, B: 200, A: 255})
		}
	}
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := ProcessCover(source, target, models.CompressionConfig{JPEGQuality: 85, MaxWidth: 1000, MaxHeight: 1500})
	if err != nil {
		t.Fatalf("ProcessCover() error = %v", err)
	}
	if result.Width != 1000 || result.Height != 1500 {
		t.Fatalf("size = %dx%d, want 1000x1500", result.Width, result.Height)
	}
	out, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	decoded, err := jpeg.Decode(out)
	if err != nil {
		t.Fatalf("target is not a JPEG: %v", err)
	}
	if decoded.Bounds().Dx() != 1000 || decoded.Bounds().Dy() != 1500 {
		t.Fatalf("decoded size = %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}
