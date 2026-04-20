package backend

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"plexcovermanager/models"
)

type CoverExportResult struct {
	OutputDir string
	Copied    int
	Errors    []error
}

// ExportExistingCovers copies all existing cover/poster slots into a new
// timestamped subdirectory below destinationRoot. Only slots with ExistingPath
// are copied, and sources are never modified.
func ExportExistingCovers(items []models.MediaItem, destinationRoot string) (CoverExportResult, error) {
	destinationRoot = strings.TrimSpace(destinationRoot)
	if destinationRoot == "" {
		return CoverExportResult{}, fmt.Errorf("Zielordner fehlt")
	}
	if err := os.MkdirAll(destinationRoot, 0o755); err != nil {
		return CoverExportResult{}, err
	}

	runDir, err := createRunDir(destinationRoot)
	if err != nil {
		return CoverExportResult{}, err
	}

	sortedItems := append([]models.MediaItem(nil), items...)
	sort.SliceStable(sortedItems, func(i, j int) bool {
		if sortedItems[i].Type != sortedItems[j].Type {
			return sortedItems[i].Type < sortedItems[j].Type
		}
		left := strings.ToLower(sortedItems[i].Title)
		right := strings.ToLower(sortedItems[j].Title)
		if left == right {
			return sortedItems[i].ID < sortedItems[j].ID
		}
		return left < right
	})

	result := CoverExportResult{OutputDir: runDir}
	for _, item := range sortedItems {
		slots := existingCoverSlots(item)
		if len(slots) == 0 {
			continue
		}
		itemDir, err := ensureItemDir(runDir, item)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", item.Title, err))
			continue
		}
		for _, slot := range slots {
			destName := exportFileName(slot)
			destPath := uniquePath(itemDir, destName)
			if err := copyFile(slot.ExistingPath, destPath); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("%s / %s: %w", item.Title, slot.Label, err))
				continue
			}
			result.Copied++
		}
	}

	return result, nil
}

func existingCoverSlots(item models.MediaItem) []models.CoverSlot {
	slots := item.SortedSlots()
	existing := make([]models.CoverSlot, 0, len(slots))
	for _, slot := range slots {
		if strings.TrimSpace(slot.ExistingPath) != "" {
			existing = append(existing, slot)
		}
	}
	return existing
}

func createRunDir(destinationRoot string) (string, error) {
	base := "cover-export-" + time.Now().UTC().Format("20060102-150405")
	runDir := filepath.Join(destinationRoot, base)
	for i := 0; i < 1000; i++ {
		candidate := runDir
		if i > 0 {
			candidate = filepath.Join(destinationRoot, fmt.Sprintf("%s-%02d", base, i+1))
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			if err := os.MkdirAll(candidate, 0o755); err != nil {
				return "", err
			}
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("konnte keinen freien Export-Ordner anlegen")
}

func ensureItemDir(runDir string, item models.MediaItem) (string, error) {
	label := strings.TrimSpace(item.Title)
	if label == "" {
		label = "Untitled"
	}
	typeLabel := strings.TrimSpace(item.TypeLabel())
	if typeLabel != "" {
		label = typeLabel + " - " + label
	}
	if item.Year != "" && !strings.Contains(label, "("+item.Year+")") {
		label += " (" + item.Year + ")"
	}
	stable := shortHash(item.ID)
	if stable != "" {
		label += " [" + stable + "]"
	}
	return createUniqueDir(runDir, sanitizeName(label))
}

func createUniqueDir(parent, base string) (string, error) {
	if base == "" {
		base = "item"
	}
	candidate := filepath.Join(parent, base)
	for i := 0; i < 1000; i++ {
		path := candidate
		if i > 0 {
			path = filepath.Join(parent, fmt.Sprintf("%s-%02d", base, i+1))
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return "", err
			}
			return path, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("konnte keinen freien Ordnernamen erzeugen")
}

func exportFileName(slot models.CoverSlot) string {
	slotLabel := sanitizeName(slotExportLabel(slot))
	if slotLabel == "" {
		slotLabel = "slot"
	}
	baseName := strings.TrimSuffix(filepath.Base(slot.ExistingPath), filepath.Ext(slot.ExistingPath))
	baseName = sanitizeName(baseName)
	if baseName == "" {
		baseName = "cover"
	}
	hash := shortHash(slot.ExistingPath)
	ext := filepath.Ext(slot.ExistingPath)
	if ext == "" {
		ext = ".bin"
	}
	if hash == "" {
		return fmt.Sprintf("%s - %s%s", slotLabel, baseName, ext)
	}
	return fmt.Sprintf("%s - %s [%s]%s", slotLabel, baseName, hash, ext)
}

func slotExportLabel(slot models.CoverSlot) string {
	switch slot.Kind {
	case models.CoverKindMain:
		return "Hauptcover"
	case models.CoverKindSeason:
		if slot.SeasonNumber == 0 {
			return "Specials"
		}
		if slot.Label != "" {
			return slot.Label
		}
		return fmt.Sprintf("S%02d", slot.SeasonNumber)
	default:
		if slot.Label != "" {
			return slot.Label
		}
		return "Slot"
	}
}

func copyFile(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	src, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".plex-cover-export-*"+filepath.Ext(targetPath))
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := io.Copy(tempFile, src); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func uniquePath(dir, name string) string {
	candidate := filepath.Join(dir, name)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	ext := filepath.Ext(name)
	for i := 2; i < 1000; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", base, shortHash(name), ext))
}

func sanitizeName(name string) string {
	replacer := strings.NewReplacer(
		"<", "_", ">", "_", ":", "_", "\"", "_",
		"/", "_", "\\", "_", "|", "_", "?", "_", "*", "_",
	)
	name = replacer.Replace(strings.TrimSpace(name))
	if len(name) > 120 {
		name = name[:120]
	}
	return strings.TrimSpace(name)
}

func shortHash(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:])[:8]
}
