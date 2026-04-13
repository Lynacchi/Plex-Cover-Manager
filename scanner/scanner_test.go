package scanner

import (
	"os"
	"path/filepath"
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
