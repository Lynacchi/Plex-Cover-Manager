package cover

import (
	"bytes"
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

func TestApplyImportPlanDisabledCopiesSourceAndPreservesExtension(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	target := filepath.Join(dir, "poster.jpg")
	content := []byte("not decoded when compression is disabled")
	if err := os.WriteFile(source, content, 0o644); err != nil {
		t.Fatal(err)
	}

	plan := ImportPlan{
		SourcePath: source,
		TargetPath: target,
		CanApply:   true,
	}
	if _, err := ApplyImportPlan(plan, models.CompressionConfig{Disabled: true}); err != nil {
		t.Fatalf("ApplyImportPlan() error = %v", err)
	}

	preservedTarget := filepath.Join(dir, "poster.png")
	data, err := os.ReadFile(preservedTarget)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("copied content = %q, want %q", data, content)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("unexpected compressed target at %s", target)
	}
}

func TestRenameCoversForModeSwitch(t *testing.T) {
	dir := t.TempDir()
	seasonDir := filepath.Join(dir, "Season 01")
	if err := os.MkdirAll(seasonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plexCover := filepath.Join(seasonDir, "season01-poster.jpg")
	if err := os.WriteFile(plexCover, []byte("cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := models.MediaItem{
		Type: models.MediaTypeSeries,
		CoverSlots: []models.CoverSlot{
			{
				Kind:         models.CoverKindSeason,
				SeasonNumber: 1,
				ExistingPath: plexCover,
				Exists:       true,
			},
		},
	}

	renamed, errs := RenameCoversForModeSwitch([]models.MediaItem{item}, models.ServerModeJellyfin)
	if renamed != 1 || len(errs) != 0 {
		t.Fatalf("renamed = %d, errs = %#v", renamed, errs)
	}
	jellyfinCover := filepath.Join(seasonDir, "poster.jpg")
	if _, err := os.Stat(jellyfinCover); err != nil {
		t.Fatalf("jellyfin cover missing: %v", err)
	}

	item.CoverSlots[0].ExistingPath = jellyfinCover
	renamed, errs = RenameCoversForModeSwitch([]models.MediaItem{item}, models.ServerModePlex)
	if renamed != 1 || len(errs) != 0 {
		t.Fatalf("renamed = %d, errs = %#v", renamed, errs)
	}
	if _, err := os.Stat(plexCover); err != nil {
		t.Fatalf("plex cover missing: %v", err)
	}
}

func TestOriginalBackupDirCanBeOverriddenByLauncher(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PCM_ORIGINALS_DIR", dir)

	got, err := OriginalBackupDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(dir) {
		t.Fatalf("OriginalBackupDir() = %q, want %q", got, filepath.Clean(dir))
	}
}
