package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"plexcovermanager/models"
)

var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".ts": true, ".m2ts": true,
	".wmv": true, ".flv": true, ".mov": true,
}

var localCoverExtensions = []string{".jpg", ".jpeg", ".png", ".webp"}

var seasonPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^s\s*0*(\d{1,2})$`),
	regexp.MustCompile(`(?i)^season[\s._-]*0*(\d{1,2})$`),
	regexp.MustCompile(`(?i)^staffel[\s._-]*0*(\d{1,2})$`),
}

var flatEpisodePattern = regexp.MustCompile(`(?i)\bs0*(\d{1,2})e\d{1,3}\b`)
var yearPattern = regexp.MustCompile(`\((\d{4})\)`)

func ScanLibrary(ctx context.Context, cfg models.AppConfig) ([]models.MediaItem, []models.ScanWarning) {
	cfg.Normalize()
	var items []models.MediaItem
	var warnings []models.ScanWarning

	for _, mediaPath := range cfg.MediaPaths {
		if ctx.Err() != nil {
			break
		}
		if mediaPath.Path == "" {
			continue
		}
		info, err := os.Stat(mediaPath.Path)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{Path: mediaPath.Path, Message: err.Error()})
			continue
		}
		if !info.IsDir() {
			warnings = append(warnings, models.ScanWarning{Path: mediaPath.Path, Message: "Pfad ist kein Ordner"})
			continue
		}
		switch mediaPath.Type {
		case models.MediaTypeMovie:
			scanned, scanWarnings := scanMoviePath(ctx, mediaPath.Path, cfg)
			items = append(items, scanned...)
			warnings = append(warnings, scanWarnings...)
		default:
			scanned, scanWarnings := scanSeriesPath(ctx, mediaPath.Path, cfg)
			items = append(items, scanned...)
			warnings = append(warnings, scanWarnings...)
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if strings.EqualFold(items[i].Title, items[j].Title) {
			return items[i].Path < items[j].Path
		}
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
	return items, warnings
}

func scanSeriesPath(ctx context.Context, root string, cfg models.AppConfig) ([]models.MediaItem, []models.ScanWarning) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, []models.ScanWarning{{Path: root, Message: err.Error()}}
	}
	items := make([]models.MediaItem, 0)
	warnings := make([]models.ScanWarning, 0)
	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}
		if !entry.IsDir() {
			continue
		}
		showPath := filepath.Join(root, entry.Name())
		item, err := scanSeriesItem(root, showPath, entry.Name(), cfg)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{Path: showPath, Message: err.Error()})
			continue
		}
		items = append(items, item)
	}
	return items, warnings
}

func scanSeriesItem(root, showPath, title string, cfg models.AppConfig) (models.MediaItem, error) {
	item := models.MediaItem{
		ID:          filepath.Clean(showPath),
		Title:       title,
		Year:        extractYear(title),
		Type:        models.MediaTypeSeries,
		LibraryPath: filepath.Clean(root),
		Path:        filepath.Clean(showPath),
	}

	seasons, err := seasonFoldersWithMedia(showPath)
	if err != nil {
		return item, err
	}
	if len(seasons) == 0 {
		seasons = flatSeriesSeasons(showPath)
		if len(seasons) > 0 {
			item.FlatStructure = true
		}
	}
	sort.SliceStable(seasons, func(i, j int) bool {
		return seasons[i].Number < seasons[j].Number
	})
	item.Seasons = seasons

	mainSlot := mainCoverSlot(showPath)
	checkSlotOptimization(&mainSlot, cfg)
	item.CoverSlots = append(item.CoverSlots, mainSlot)
	for _, season := range seasons {
		targetDir := showPath
		if !item.FlatStructure && season.Path != "" {
			targetDir = season.Path
		}
		isFlat := item.FlatStructure
		slot := seasonCoverSlot(targetDir, season.Number, cfg.ServerMode, isFlat)
		checkSlotOptimization(&slot, cfg)
		item.CoverSlots = append(item.CoverSlots, slot)
	}
	item.RecalculateStatus()
	return item, nil
}

func scanMoviePath(ctx context.Context, root string, cfg models.AppConfig) ([]models.MediaItem, []models.ScanWarning) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, []models.ScanWarning{{Path: root, Message: err.Error()}}
	}
	items := make([]models.MediaItem, 0)
	warnings := make([]models.ScanWarning, 0)

	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			item := scanMovieFolder(root, path, entry.Name(), cfg)
			items = append(items, item)
			continue
		}
		if isVideoFile(entry.Name()) {
			item := scanFlatMovie(root, path, entry.Name(), cfg)
			items = append(items, item)
		}
	}
	return items, warnings
}

func scanMovieFolder(root, moviePath, title string, cfg models.AppConfig) models.MediaItem {
	slot := mainCoverSlot(moviePath)
	checkSlotOptimization(&slot, cfg)
	item := models.MediaItem{
		ID:          filepath.Clean(moviePath),
		Title:       title,
		Year:        extractYear(title),
		Type:        models.MediaTypeMovie,
		LibraryPath: filepath.Clean(root),
		Path:        filepath.Clean(moviePath),
		CoverSlots:  []models.CoverSlot{slot},
	}
	item.RecalculateStatus()
	return item
}

func scanFlatMovie(root, mediaFilePath, fileName string, cfg models.AppConfig) models.MediaItem {
	title := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	target := filepath.Join(root, title+".jpg")
	existingPath, size, exists := findExistingCover(root, coverNameCandidates(title))
	slot := models.CoverSlot{
		Key:          models.MainSlotKey(),
		Label:        "Main",
		Kind:         models.CoverKindMain,
		SeasonNumber: -1,
		TargetPath:   target,
		ExistingPath: existingPath,
		Exists:       exists,
		SizeBytes:    size,
	}
	checkSlotOptimization(&slot, cfg)
	item := models.MediaItem{
		ID:            filepath.Clean(mediaFilePath),
		Title:         title,
		Year:          extractYear(title),
		Type:          models.MediaTypeMovie,
		LibraryPath:   filepath.Clean(root),
		Path:          filepath.Clean(root),
		MediaFilePath: filepath.Clean(mediaFilePath),
		FlatStructure: true,
		CoverSlots:    []models.CoverSlot{slot},
	}
	item.RecalculateStatus()
	return item
}

func seasonFoldersWithMedia(showPath string) ([]models.SeasonInfo, error) {
	entries, err := os.ReadDir(showPath)
	if err != nil {
		return nil, err
	}
	seasonsByNumber := map[int]models.SeasonInfo{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		number, ok := parseSeasonFolder(entry.Name())
		if !ok {
			continue
		}
		path := filepath.Join(showPath, entry.Name())
		if !containsVideoFile(path) {
			continue
		}
		seasonsByNumber[number] = models.SeasonInfo{
			Number:   number,
			Label:    seasonLabel(number),
			Path:     filepath.Clean(path),
			HasMedia: true,
		}
	}
	seasons := make([]models.SeasonInfo, 0, len(seasonsByNumber))
	for _, season := range seasonsByNumber {
		seasons = append(seasons, season)
	}
	return seasons, nil
}

func flatSeriesSeasons(showPath string) []models.SeasonInfo {
	entries, err := os.ReadDir(showPath)
	if err != nil {
		return nil
	}
	seasonsByNumber := map[int]models.SeasonInfo{}
	for _, entry := range entries {
		if entry.IsDir() || !isVideoFile(entry.Name()) {
			continue
		}
		matches := flatEpisodePattern.FindStringSubmatch(entry.Name())
		if len(matches) != 2 {
			continue
		}
		number, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}
		seasonsByNumber[number] = models.SeasonInfo{
			Number:   number,
			Label:    seasonLabel(number),
			Path:     filepath.Clean(showPath),
			HasMedia: true,
		}
	}
	seasons := make([]models.SeasonInfo, 0, len(seasonsByNumber))
	for _, season := range seasonsByNumber {
		seasons = append(seasons, season)
	}
	return seasons
}

func parseSeasonFolder(name string) (int, bool) {
	normalized := strings.TrimSpace(name)
	lower := strings.ToLower(normalized)
	switch lower {
	case "special", "specials", "sp":
		return 0, true
	}
	for _, pattern := range seasonPatterns {
		matches := pattern.FindStringSubmatch(normalized)
		if len(matches) != 2 {
			continue
		}
		number, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, false
		}
		return number, true
	}
	return 0, false
}

func mainCoverSlot(dir string) models.CoverSlot {
	existingPath, size, exists := findExistingCover(dir, coverNameCandidates("poster"))
	return models.CoverSlot{
		Key:          models.MainSlotKey(),
		Label:        "Main",
		Kind:         models.CoverKindMain,
		SeasonNumber: -1,
		TargetPath:   filepath.Join(dir, "poster.jpg"),
		ExistingPath: existingPath,
		Exists:       exists,
		SizeBytes:    size,
	}
}

func seasonCoverSlot(dir string, season int, mode models.ServerMode, isFlat bool) models.CoverSlot {
	if mode == models.ServerModeJellyfin && !isFlat {
		existingPath, size, exists := findExistingCover(dir, coverNameCandidates("poster"))
		return models.CoverSlot{
			Key:          models.SeasonSlotKey(season),
			Label:        seasonLabel(season),
			Kind:         models.CoverKindSeason,
			SeasonNumber: season,
			TargetPath:   filepath.Join(dir, "poster.jpg"),
			ExistingPath: existingPath,
			Exists:       exists,
			SizeBytes:    size,
		}
	}
	base := fmt.Sprintf("season%02d-poster", season)
	existingPath, size, exists := findExistingCover(dir, coverNameCandidates(base))
	return models.CoverSlot{
		Key:          models.SeasonSlotKey(season),
		Label:        seasonLabel(season),
		Kind:         models.CoverKindSeason,
		SeasonNumber: season,
		TargetPath:   filepath.Join(dir, base+".jpg"),
		ExistingPath: existingPath,
		Exists:       exists,
		SizeBytes:    size,
	}
}

func coverNameCandidates(base string) []string {
	candidates := make([]string, 0, len(localCoverExtensions))
	for _, ext := range localCoverExtensions {
		candidates = append(candidates, base+ext)
	}
	return candidates
}

func checkSlotOptimization(slot *models.CoverSlot, cfg models.AppConfig) {
	if cfg.Compression.Disabled || !slot.Exists || slot.ExistingPath == "" {
		slot.IsOptimized = true
		return
	}
	ext := strings.ToLower(filepath.Ext(slot.ExistingPath))
	if ext != ".jpg" && ext != ".jpeg" {
		slot.IsOptimized = false
		slot.OptimizeHint = fmt.Sprintf("Format: %s", strings.ToUpper(strings.TrimPrefix(ext, ".")))
		return
	}
	thresholdBytes := int64(cfg.OptimizeThresholdKB) * 1024
	if cfg.OptimizeThresholdKB > 0 && slot.SizeBytes > thresholdBytes {
		slot.IsOptimized = false
		slot.OptimizeHint = fmt.Sprintf("Zu groß (%s, Limit: %d KB)", formatSize(slot.SizeBytes), cfg.OptimizeThresholdKB)
		return
	}
	slot.IsOptimized = true
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func findExistingCover(dir string, candidates []string) (string, int64, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", 0, false
	}
	found := make(map[string]os.DirEntry, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		found[strings.ToLower(entry.Name())] = entry
	}
	for _, candidate := range candidates {
		entry, ok := found[strings.ToLower(candidate)]
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return filepath.Join(dir, entry.Name()), 0, true
		}
		return filepath.Join(dir, entry.Name()), info.Size(), true
	}
	return "", 0, false
}

func containsVideoFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isVideoFile(entry.Name()) {
			return true
		}
	}
	return false
}

func isVideoFile(name string) bool {
	return videoExtensions[strings.ToLower(filepath.Ext(name))]
}

func extractYear(name string) string {
	matches := yearPattern.FindStringSubmatch(name)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func seasonLabel(season int) string {
	if season == 0 {
		return "Specials"
	}
	return fmt.Sprintf("S%02d", season)
}
