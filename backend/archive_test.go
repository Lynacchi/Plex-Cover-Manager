package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plexcovermanager/models"
)

func TestExportExistingCoversCopiesAllExistingSlots(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "exports")
	sourceMain := filepath.Join(root, "source", "poster.jpg")
	sourceSeason := filepath.Join(root, "source", "season01.webp")
	writeBytes(t, sourceMain, []byte("main-cover"))
	writeBytes(t, sourceSeason, []byte("season-cover"))

	items := []models.MediaItem{
		{
			ID:    "series-1",
			Title: "Example Show (2024)",
			Year:  "2024",
			Type:  models.MediaTypeSeries,
			CoverSlots: []models.CoverSlot{
				{Key: models.MainSlotKey(), Label: "Main", Kind: models.CoverKindMain, ExistingPath: sourceMain, Exists: true},
				{Key: models.SeasonSlotKey(1), Label: "S01", Kind: models.CoverKindSeason, SeasonNumber: 1, ExistingPath: sourceSeason, Exists: true},
				{Key: models.SeasonSlotKey(2), Label: "S02", Kind: models.CoverKindSeason, SeasonNumber: 2},
			},
		},
		{
			ID:    "series-2",
			Title: "Missing Show",
			Type:  models.MediaTypeSeries,
			CoverSlots: []models.CoverSlot{
				{Key: models.MainSlotKey(), Label: "Main", Kind: models.CoverKindMain},
			},
		},
	}

	result, err := ExportExistingCovers(items, destination)
	if err != nil {
		t.Fatalf("ExportExistingCovers() error = %v", err)
	}
	if result.Copied != 2 {
		t.Fatalf("Copied = %d, want 2", result.Copied)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("Errors = %#v", result.Errors)
	}

	files := walkFiles(t, result.OutputDir)
	if len(files) != 2 {
		t.Fatalf("exported file count = %d, want 2; files=%v", len(files), files)
	}
	dirs := immediateDirs(t, result.OutputDir)
	if len(dirs) != 1 {
		t.Fatalf("exported item dir count = %d, want 1; dirs=%v", len(dirs), dirs)
	}

	joined := strings.Join(files, "\n")
	if !strings.Contains(joined, "Hauptcover") || !strings.Contains(joined, "S01") {
		t.Fatalf("exported names do not reflect slot labels: %v", files)
	}
	if strings.Contains(joined, "Missing Show") {
		t.Fatalf("export should not create folders for items without existing covers: %v", files)
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		switch filepath.Ext(path) {
		case ".jpg":
			if string(data) != "main-cover" {
				t.Fatalf("jpg content = %q, want main-cover", string(data))
			}
		case ".webp":
			if string(data) != "season-cover" {
				t.Fatalf("webp content = %q, want season-cover", string(data))
			}
		default:
			t.Fatalf("unexpected exported extension in %s", path)
		}
	}

	if got, want := mustReadFile(t, sourceMain), "main-cover"; string(got) != want {
		t.Fatalf("source main changed: %q", string(got))
	}
	if got, want := mustReadFile(t, sourceSeason), "season-cover"; string(got) != want {
		t.Fatalf("source season changed: %q", string(got))
	}
}

func TestBuildMissingCoverReportCountsAndFormats(t *testing.T) {
	items := []models.MediaItem{
		{
			ID:    "movie-1",
			Title: "Example Movie (2020)",
			Type:  models.MediaTypeMovie,
			CoverSlots: []models.CoverSlot{
				{Key: models.MainSlotKey(), Label: "Main", Kind: models.CoverKindMain},
			},
		},
		{
			ID:    "series-1",
			Title: "Example Show (2024)",
			Type:  models.MediaTypeSeries,
			CoverSlots: []models.CoverSlot{
				{Key: models.MainSlotKey(), Label: "Main", Kind: models.CoverKindMain, Exists: true, ExistingPath: "poster.jpg"},
				{Key: models.SeasonSlotKey(1), Label: "S01", Kind: models.CoverKindSeason, SeasonNumber: 1},
				{Key: models.SeasonSlotKey(2), Label: "S02", Kind: models.CoverKindSeason, SeasonNumber: 2, Exists: true, ExistingPath: "season02.jpg"},
				{Key: models.SeasonSlotKey(0), Label: "Specials", Kind: models.CoverKindSeason, SeasonNumber: 0},
			},
		},
	}

	report, missingSlots, missingItems := BuildMissingCoverReport(items)
	if missingSlots != 3 {
		t.Fatalf("missingSlots = %d, want 3", missingSlots)
	}
	if missingItems != 2 {
		t.Fatalf("missingItems = %d, want 2", missingItems)
	}
	if !strings.Contains(report, "Komplett fehlende Einträge") {
		t.Fatalf("report missing complete section: %s", report)
	}
	if !strings.Contains(report, "Teilweise fehlende Einträge") {
		t.Fatalf("report missing partial section: %s", report)
	}
	if !strings.Contains(report, "komplett betroffen") {
		t.Fatalf("report missing complete-item marker: %s", report)
	}
	if !strings.Contains(report, "S01") || !strings.Contains(report, "Specials") {
		t.Fatalf("report missing season/special labels: %s", report)
	}
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func walkFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func immediateDirs(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(root, entry.Name()))
		}
	}
	return dirs
}
