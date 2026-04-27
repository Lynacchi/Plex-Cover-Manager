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
	var specialsSlot models.CoverSlot
	for _, slot := range item.CoverSlots {
		if slot.Kind == models.CoverKindSeason && slot.SeasonNumber == 0 {
			specialsSlot = slot
		}
	}
	if got, want := specialsSlot.TargetPath, filepath.Join(specialsDir, "season-specials-poster.jpg"); got != want {
		t.Fatalf("specials TargetPath = %q, want %q", got, want)
	}
}

func TestScanLibrarySpecialsPosterName(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Example Show (2024)")
	specialsDir := filepath.Join(showDir, "Specials")
	mustMkdir(t, specialsDir)
	mustWrite(t, filepath.Join(specialsDir, "Example.Show.S00E01.mkv"))
	mustWrite(t, filepath.Join(specialsDir, "season-specials-poster.png"))

	items, warnings := ScanLibrary(t.Context(), models.AppConfig{
		MediaPaths: []models.MediaPath{{Path: root, Type: models.MediaTypeSeries}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d", len(items))
	}
	var specialsSlot models.CoverSlot
	for _, slot := range items[0].CoverSlots {
		if slot.Kind == models.CoverKindSeason && slot.SeasonNumber == 0 {
			specialsSlot = slot
		}
	}
	if !specialsSlot.Exists || !specialsSlot.NamingOK {
		t.Fatalf("specials slot = %#v", specialsSlot)
	}
	if got, want := specialsSlot.TargetPath, filepath.Join(specialsDir, "season-specials-poster.jpg"); got != want {
		t.Fatalf("TargetPath = %q, want %q", got, want)
	}
}

func TestScanLibraryLegacySeason00SpecialsPosterIsRenameCandidate(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Example Show (2024)")
	specialsDir := filepath.Join(showDir, "Specials")
	mustMkdir(t, specialsDir)
	mustWrite(t, filepath.Join(specialsDir, "Example.Show.S00E01.mkv"))
	mustWrite(t, filepath.Join(specialsDir, "season00-poster.png"))

	items, warnings := ScanLibrary(t.Context(), models.AppConfig{
		MediaPaths: []models.MediaPath{{Path: root, Type: models.MediaTypeSeries}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d", len(items))
	}
	var specialsSlot models.CoverSlot
	for _, slot := range items[0].CoverSlots {
		if slot.Kind == models.CoverKindSeason && slot.SeasonNumber == 0 {
			specialsSlot = slot
		}
	}
	if !specialsSlot.Exists || specialsSlot.NamingOK {
		t.Fatalf("specials slot = %#v", specialsSlot)
	}
	if got, want := filepath.Base(specialsSlot.ExistingPath), "season00-poster.png"; got != want {
		t.Fatalf("ExistingPath base = %q, want %q", got, want)
	}
	if !strings.Contains(specialsSlot.NamingHint, "season-specials-poster.png") {
		t.Fatalf("NamingHint = %q", specialsSlot.NamingHint)
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

func TestScanDetectsSmartMainCoverAlias(t *testing.T) {
	root := t.TempDir()
	movieDir := filepath.Join(root, "Example Movie (2020)")
	mustMkdir(t, movieDir)
	mustWrite(t, filepath.Join(movieDir, "Example.Movie.2020.mkv"))
	mustWrite(t, filepath.Join(movieDir, "folder.jpg"))

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
		t.Fatalf("smart cover alias not detected: %#v", slot)
	}
	if slot.NamingOK || !strings.Contains(slot.NamingHint, "folder.jpg") {
		t.Fatalf("naming state = ok %v, hint %q", slot.NamingOK, slot.NamingHint)
	}
	if items[0].Status != models.CoverStatusComplete {
		t.Fatalf("status = %q, want complete", items[0].Status)
	}
}

func TestScanDetectsDownloadedFlatSeasonCover(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Example Show (2024)")
	mustMkdir(t, showDir)
	mustWrite(t, filepath.Join(showDir, "Example.Show.S01E01.mkv"))
	mustWrite(t, filepath.Join(showDir, "Example Show (2024) - Season 1.png"))

	items, warnings := ScanLibrary(t.Context(), models.AppConfig{
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
		t.Fatalf("downloaded season cover not detected: %#v", items[0].CoverSlots)
	}
	if seasonSlot.NamingOK || filepath.Base(seasonSlot.ExistingPath) != "Example Show (2024) - Season 1.png" {
		t.Fatalf("season slot = %#v", seasonSlot)
	}
}

func TestRescanItemOnlyReloadsSelectedItem(t *testing.T) {
	root := t.TempDir()
	movieDir := filepath.Join(root, "Example Movie (2020)")
	mustMkdir(t, movieDir)
	mustWrite(t, filepath.Join(movieDir, "Example.Movie.2020.mkv"))

	item := models.MediaItem{
		ID:          filepath.Clean(movieDir),
		Title:       "Example Movie (2020)",
		Type:        models.MediaTypeMovie,
		LibraryPath: filepath.Clean(root),
		Path:        filepath.Clean(movieDir),
	}
	mustWrite(t, filepath.Join(movieDir, "poster.jpg"))

	rescanned, warnings := RescanItem(t.Context(), models.AppConfig{}, item)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if rescanned.ID != item.ID {
		t.Fatalf("ID = %q, want %q", rescanned.ID, item.ID)
	}
	if rescanned.Status != models.CoverStatusComplete {
		t.Fatalf("status = %q, want complete", rescanned.Status)
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
