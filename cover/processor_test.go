package cover

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
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

func TestProcessCoverCanReduceQualityToTargetSize(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	standardTarget := filepath.Join(dir, "standard.jpg")
	target := filepath.Join(dir, "poster.jpg")
	writeDetailedTestPNG(t, source)

	standard, err := ProcessCover(source, standardTarget, models.CompressionConfig{JPEGQuality: 95, MaxWidth: 800, MaxHeight: 800})
	if err != nil {
		t.Fatalf("standard ProcessCover() error = %v", err)
	}
	thresholdKB := int(standard.SizeBytes/1024) - 4
	if thresholdKB < 1 {
		t.Fatalf("standard output too small for target-size test: %d", standard.SizeBytes)
	}

	result, err := ProcessCover(source, target, models.CompressionConfig{
		JPEGQuality:           95,
		MaxWidth:              800,
		MaxHeight:             800,
		ReduceQualityToTarget: true,
		TargetSizeKB:          thresholdKB,
	})
	if err != nil {
		t.Fatalf("ProcessCover() error = %v", err)
	}
	if result.SizeBytes >= standard.SizeBytes {
		t.Fatalf("reduced size = %d, standard size = %d", result.SizeBytes, standard.SizeBytes)
	}
	if result.SizeBytes > int64(thresholdKB)*1024 {
		t.Fatalf("reduced size = %d, threshold = %d KB", result.SizeBytes, thresholdKB)
	}
}

func TestProcessCoverKeepsBestEffortWhenTargetSizeCannotBeReached(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	target := filepath.Join(dir, "poster.jpg")
	writeDetailedTestPNG(t, source)

	result, err := ProcessCover(source, target, models.CompressionConfig{
		JPEGQuality:           95,
		MaxWidth:              800,
		MaxHeight:             800,
		ReduceQualityToTarget: true,
		TargetSizeKB:          1,
	})
	if err != nil {
		t.Fatalf("ProcessCover() error = %v", err)
	}
	if result.SizeBytes <= 0 {
		t.Fatalf("SizeBytes = %d", result.SizeBytes)
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

func TestFinalImportTargetPathPreservesExtensionWhenCompressionDisabled(t *testing.T) {
	plan := ImportPlan{
		SourcePath: filepath.Join("covers", "source.webp"),
		TargetPath: filepath.Join("media", "poster.jpg"),
	}
	got := FinalImportTargetPath(plan, models.CompressionConfig{Disabled: true})
	want := filepath.Join("media", "poster.webp")
	if got != want {
		t.Fatalf("FinalImportTargetPath() = %q, want %q", got, want)
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

func TestRenameCoversForModeSwitchUsesSpecialsPosterNameForPlex(t *testing.T) {
	dir := t.TempDir()
	specialsDir := filepath.Join(dir, "Specials")
	if err := os.MkdirAll(specialsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jellyfinCover := filepath.Join(specialsDir, "poster.png")
	if err := os.WriteFile(jellyfinCover, []byte("cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := models.MediaItem{
		Type: models.MediaTypeSeries,
		CoverSlots: []models.CoverSlot{
			{
				Kind:         models.CoverKindSeason,
				SeasonNumber: 0,
				ExistingPath: jellyfinCover,
				Exists:       true,
			},
		},
	}

	renamed, errs := RenameCoversForModeSwitch([]models.MediaItem{item}, models.ServerModePlex)
	if renamed != 1 || len(errs) != 0 {
		t.Fatalf("renamed = %d, errs = %#v", renamed, errs)
	}
	plexCover := filepath.Join(specialsDir, "season-specials-poster.png")
	if _, err := os.Stat(plexCover); err != nil {
		t.Fatalf("plex specials cover missing: %v", err)
	}
}

func TestOriginalBackupDirCanBeOverriddenByLauncher(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PCM_ORIGINALS_DIR", dir)

	got, err := OriginalBackupDir("")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(dir) {
		t.Fatalf("OriginalBackupDir() = %q, want %q", got, filepath.Clean(dir))
	}
}

func TestOriginalBackupDirConfigOverrideWinsOverEnv(t *testing.T) {
	envDir := t.TempDir()
	cfgDir := t.TempDir()
	t.Setenv("PCM_ORIGINALS_DIR", envDir)

	got, err := OriginalBackupDir(cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(cfgDir) {
		t.Fatalf("OriginalBackupDir(%q) = %q, want %q", cfgDir, got, filepath.Clean(cfgDir))
	}
}

func TestSamePathUsesPlatformCaseRules(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "Poster.jpg")
	right := filepath.Join(dir, "poster.jpg")
	got := samePath(left, right)
	if runtime.GOOS == "windows" {
		if !got {
			t.Fatalf("samePath() = false on Windows, want true")
		}
		return
	}
	if got {
		t.Fatalf("samePath() = true on %s, want false", runtime.GOOS)
	}
}

func TestCompressCoverCanPreserveSmartAliasName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PCM_ORIGINALS_DIR", filepath.Join(dir, "originals"))
	source := filepath.Join(dir, "folder.png")
	target := filepath.Join(dir, "poster.jpg")
	writeTestPNG(t, source)

	slot := models.CoverSlot{
		Label:        "Main",
		TargetPath:   target,
		ExistingPath: source,
		Exists:       true,
		NamingOK:     false,
	}
	if _, err := CompressCover(slot, "Example Movie", models.CompressionConfig{JPEGQuality: 85, MaxWidth: 1000, MaxHeight: 1500}, false, ""); err != nil {
		t.Fatalf("CompressCover() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "folder.jpg")); err != nil {
		t.Fatalf("optimized alias missing: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("unexpected canonical target at %s", target)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("unexpected original source at %s", source)
	}
}

func TestRenameCoverToTargetNamePreservesExtension(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "folder.png")
	target := filepath.Join(dir, "poster.jpg")
	if err := os.WriteFile(source, []byte("cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	slot := models.CoverSlot{
		TargetPath:   target,
		ExistingPath: source,
		Exists:       true,
	}

	renamedPath, err := RenameCoverToTargetName(slot)
	if err != nil {
		t.Fatalf("RenameCoverToTargetName() error = %v", err)
	}
	if want := filepath.Join(dir, "poster.png"); renamedPath != want {
		t.Fatalf("renamedPath = %q, want %q", renamedPath, want)
	}
	if _, err := os.Stat(renamedPath); err != nil {
		t.Fatalf("renamed cover missing: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("unexpected source at %s", source)
	}
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 96))
	for y := 0; y < 96; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: 120, B: 180, A: 255})
		}
	}
	file, err := os.Create(path)
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
}

func writeDetailedTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 800, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x*y + y) % 256),
				G: uint8((x*3 + y*7) % 256),
				B: uint8((x*11 + y*5) % 256),
				A: 255,
			})
		}
	}
	file, err := os.Create(path)
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
}
