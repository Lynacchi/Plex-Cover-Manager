package cover

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"plexcovermanager/models"
)

type ImportStatus string

const (
	ImportStatusReady         ImportStatus = "Match gefunden"
	ImportStatusOverwrite     ImportStatus = "Überschreibt bestehendes Cover"
	ImportStatusNoMatch       ImportStatus = "Kein Match gefunden"
	ImportStatusSeasonMissing ImportStatus = "Staffelordner existiert nicht"
	ImportStatusUnsupported   ImportStatus = "Nicht unterstütztes Format"
	ImportStatusInvalid       ImportStatus = "Ungültiger Dateiname"
)

type ImportPlan struct {
	SourcePath   string
	SourceFile   string
	ItemID       string
	ItemTitle    string
	ItemType     models.MediaType
	SlotKey      string
	SlotLabel    string
	TargetPath   string
	ExistingPath string
	Status       ImportStatus
	Message      string
	CanApply     bool
	Overwrites   bool
	Parsed       ParsedCover
	MatchedScore float64
}

func PlanImports(paths []string, items []models.MediaItem) []ImportPlan {
	plans := make([]ImportPlan, 0, len(paths))
	for _, path := range paths {
		parsed, err := ParseCoverFile(path)
		if err != nil {
			plans = append(plans, invalidPlan(path, ImportStatusUnsupported, err.Error()))
			continue
		}
		plans = append(plans, planParsedImport(parsed, items, false))
	}
	return plans
}

func PlanImportsForItem(paths []string, item models.MediaItem) []ImportPlan {
	plans := make([]ImportPlan, 0, len(paths))
	for _, path := range paths {
		parsed, err := ParseCoverFile(path)
		if err != nil {
			plans = append(plans, invalidPlan(path, ImportStatusUnsupported, err.Error()))
			continue
		}
		plans = append(plans, planParsedImport(parsed, []models.MediaItem{item}, true))
	}
	return plans
}

func PlanForSlot(path string, item models.MediaItem, slot models.CoverSlot) ImportPlan {
	parsed, err := ParseCoverFile(path)
	if err != nil {
		return invalidPlan(path, ImportStatusUnsupported, err.Error())
	}
	return buildPlan(parsed, item, slot, 1)
}

func planParsedImport(parsed ParsedCover, items []models.MediaItem, allowSingleCandidate bool) ImportPlan {
	item, score, ok := BestMatch(parsed, items, allowSingleCandidate)
	if !ok {
		return ImportPlan{
			SourcePath: parsed.SourcePath,
			SourceFile: parsed.FileName,
			Status:     ImportStatusNoMatch,
			Message:    "Kein passender Titel in der Medienbibliothek gefunden.",
			Parsed:     parsed,
		}
	}
	slot, ok := slotForParsedCover(item, parsed)
	if !ok {
		return ImportPlan{
			SourcePath:   parsed.SourcePath,
			SourceFile:   parsed.FileName,
			ItemID:       item.ID,
			ItemTitle:    item.Title,
			ItemType:     item.Type,
			Status:       ImportStatusSeasonMissing,
			Message:      missingSlotMessage(item, parsed),
			Parsed:       parsed,
			MatchedScore: score,
		}
	}
	return buildPlan(parsed, item, slot, score)
}

func buildPlan(parsed ParsedCover, item models.MediaItem, slot models.CoverSlot, score float64) ImportPlan {
	status := ImportStatusReady
	message := "Kann übernommen werden."
	overwrites := slot.Exists
	if overwrites {
		status = ImportStatusOverwrite
		message = "Ein vorhandenes Cover wird ersetzt."
	}
	return ImportPlan{
		SourcePath:   parsed.SourcePath,
		SourceFile:   parsed.FileName,
		ItemID:       item.ID,
		ItemTitle:    item.Title,
		ItemType:     item.Type,
		SlotKey:      slot.Key,
		SlotLabel:    slot.Label,
		TargetPath:   slot.TargetPath,
		ExistingPath: slot.ExistingPath,
		Status:       status,
		Message:      message,
		CanApply:     true,
		Overwrites:   overwrites,
		Parsed:       parsed,
		MatchedScore: score,
	}
}

func invalidPlan(path string, status ImportStatus, message string) ImportPlan {
	return ImportPlan{
		SourcePath: path,
		SourceFile: filepath.Base(path),
		Status:     status,
		Message:    message,
	}
}

func BestMatch(parsed ParsedCover, items []models.MediaItem, allowSingleCandidate bool) (models.MediaItem, float64, bool) {
	candidates := make([]matchCandidate, 0, len(items))
	for _, item := range items {
		if parsed.IsSeason && item.Type != models.MediaTypeSeries {
			continue
		}
		score := matchScore(parsed, item)
		candidates = append(candidates, matchCandidate{Item: item, Score: score})
	}
	if len(candidates) == 0 {
		return models.MediaItem{}, 0, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	threshold := 0.72
	if allowSingleCandidate && len(candidates) == 1 {
		threshold = 0.55
	}
	if candidates[0].Score < threshold {
		return models.MediaItem{}, candidates[0].Score, false
	}
	return candidates[0].Item, candidates[0].Score, true
}

type matchCandidate struct {
	Item  models.MediaItem
	Score float64
}

func matchScore(parsed ParsedCover, item models.MediaItem) float64 {
	a := NormalizeTitle(parsed.Title)
	b := NormalizeTitle(stripTrailingYear(item.Title))
	if a == "" || b == "" {
		return 0
	}
	score := similarity(a, b)
	if a == b {
		score += 0.2
	}
	if parsed.Year != "" && item.Year != "" {
		if parsed.Year == item.Year {
			score += 0.12
		} else {
			score -= 0.18
		}
	}
	if tokenContained(a, b) {
		score += 0.08
	}
	if score > 1 {
		return 1
	}
	if score < 0 {
		return 0
	}
	return score
}

func slotForParsedCover(item models.MediaItem, parsed ParsedCover) (models.CoverSlot, bool) {
	if !parsed.IsSeason {
		for _, slot := range item.CoverSlots {
			if slot.Kind == models.CoverKindMain {
				return slot, true
			}
		}
		return models.CoverSlot{}, false
	}
	if item.Type != models.MediaTypeSeries {
		return models.CoverSlot{}, false
	}
	for _, slot := range item.CoverSlots {
		if slot.Kind == models.CoverKindSeason && slot.SeasonNumber == parsed.SeasonNumber {
			return slot, true
		}
	}
	return models.CoverSlot{}, false
}

func missingSlotMessage(item models.MediaItem, parsed ParsedCover) string {
	if parsed.IsSeason {
		if parsed.SeasonNumber == 0 {
			return fmt.Sprintf("%s hat keinen erkannten Specials-Ordner bzw. keine S00-Medien.", item.Title)
		}
		return fmt.Sprintf("%s hat keine erkannte Staffel S%02d mit Medien.", item.Title, parsed.SeasonNumber)
	}
	return "Kein Zielslot für dieses Cover gefunden."
}

func NormalizeTitle(input string) string {
	withoutYear := stripTrailingYear(input)
	var builder strings.Builder
	lastSpace := true
	for _, r := range strings.ToLower(withoutYear) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func stripTrailingYear(input string) string {
	input = strings.TrimSpace(input)
	if matches := trailingYearPattern.FindStringSubmatch(input); len(matches) == 3 {
		return strings.TrimSpace(matches[1])
	}
	return input
}

func tokenContained(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) < 4 || len(b) < 4 {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

func similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	ra := []rune(a)
	rb := []rune(b)
	maxLen := len(ra)
	if len(rb) > maxLen {
		maxLen = len(rb)
	}
	if maxLen == 0 {
		return 1
	}
	distance := levenshtein(ra, rb)
	return 1 - float64(distance)/float64(maxLen)
}

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = minInt(
				curr[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func minInt(values ...int) int {
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}
