package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plexcovermanager/models"
)

func TestScanLibrarySeriesSeasonFolders(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Example Show (2024)")
	seasonDir := filepath.Join(showDir, "Season 01")
	specialsDir := filepath.Join(showDir, "Specials")
	mustMkdir(t, seasonDir)
	mustMkdir(t, specialsDir)
	mustWrite(t, filepath.Join(showDir, "poster.jpg"))
	mustWrite(t, filepath.Join(seasonDir, "Example.Show.S01E01.mkv"))
	mustWrite(t, filepath.Join(specialsDir, "Example.Show.S00E01.mkv"))

	items, warnings := ScanLibrary(t.Context(), models.AppConfig{
		MediaPaths: []models.MediaPath{{Path: root, Type: models.MediaTypeSeries}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d", len(items))
	}
	item := items[0]
	if item.Type != models.MediaTypeSeries || item.Title != "Example Show (2024)" {
		t.Fatalf("item = %#v", item)
	}
	if len(item.Seasons) != 2 {
		t.Fatalf("len(seasons) = %d", len(item.Seasons))
	}
	if item.Status != models.CoverStatusPartial {
		t.Fatalf("status = %q, want partial", item.Status)
	}
	if len(item.CoverSlots) != 3 {
		t.Fatalf("len(slots) = %d", len(item.CoverSlots))
	}
}

func TestScanLibraryFlatMovie(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "Example Movie (2020).mkv"))
	mustWrite(t, filepath.Join(root, "Example Movie (2020).jpg"))

	items, warnings := ScanLibrary(t.Context(), models.AppConfig{
		MediaPaths: []models.MediaPath{{Path: root, Type: models.MediaTypeMovie}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d", len(items))
	}
	item := items[0]
	if !item.FlatStructure || item.Status != models.CoverStatusComplete {
		t.Fatalf("item = %#v", item)
	}
	if got := filepath.Base(item.CoverSlots[0].TargetPath); got != "Example Movie (2020).jpg" {
		t.Fatalf("target = %q", got)
	}
}

func TestJellyfinSeasonFolderUsesPosterInSeasonFolder(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Example Show (2024)")
	seasonDir := filepath.Join(showDir, "Season 01")
	mustMkdir(t, seasonDir)
	mustWrite(t, filepath.Join(seasonDir, "Example.Show.S01E01.mkv"))
	mustWrite(t, filepath.Join(seasonDir, "poster.jpg"))

	items, warnings := ScanLibrary(t.Context(), models.AppConfig{
		ServerMode: models.ServerModeJellyfin,
		MediaPaths: []models.MediaPath{{Path: root, Type: models.MediaTypeSeries}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d", len(items))
	}
	var seasonSlot models.CoverSlot
	for _, slot := range items[0].CoverSlots {
		if slot.Kind == models.CoverKindSeason && slot.SeasonNumber == 1 {
			seasonSlot = slot
		}
	}
	if !seasonSlot.Exists {
		t.Fatalf("season slot not detected: %#v", items[0].CoverSlots)
	}
	if got, want := seasonSlot.TargetPath, filepath.Join(seasonDir, "poster.jpg"); got != want {
		t.Fatalf("TargetPath = %q, want %q", got, want)
	}
}

func TestJellyfinFlatSeriesKeepsPlexSeasonPosterName(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Example Show (2024)")
	mustMkdir(t, showDir)
	mustWrite(t, filepath.Join(showDir, "Example.Show.S01E01.mkv"))
	mustWrite(t, filepath.Join(showDir, "season01-poster.jpg"))

	items, warnings := ScanLibrary(t.Context(), models.AppConfig{
		ServerMode: models.ServerModeJellyfin,
		MediaPaths: []models.MediaPath{{Path: root, Type: models.MediaTypeSeries}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d", len(items))
	}
	if !items[0].FlatStructure {
		t.Fatalf("expected flat structure: %#v", items[0])
	}
	var seasonSlot models.CoverSlot
	for _, slot := range items[0].CoverSlots {
		if slot.Kind == models.CoverKindSeason && slot.SeasonNumber == 1 {
			seasonSlot = slot
		}
	}
	if got, want := seasonSlot.TargetPath, filepath.Join(showDir, "season01-poster.jpg"); got != want {
		t.Fatalf("TargetPath = %q, want %q", got, want)
	}
}

func TestScanDetectsUnoptimizedWebPCover(t *testing.T) {
	root := t.TempDir()
	movieDir := filepath.Join(root, "Example Movie (2020)")
	mustMkdir(t, movieDir)
	mustWrite(t, filepath.Join(movieDir, "poster.webp"))

	items, warnings := ScanLibrary(t.Context(), models.AppConfig{
		MediaPaths: []models.MediaPath{{Path: root, Type: models.MediaTypeMovie}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d", len(items))
	}
	slot := items[0].CoverSlots[0]
	if !slot.Exists {
		t.Fatalf("webp cover not detected: %#v", slot)
	}
	if slot.IsOptimized || !strings.Contains(slot.OptimizeHint, "WEBP") {
		t.Fatalf("optimization state = optimized %v, hint %q", slot.IsOptimized, slot.OptimizeHint)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
