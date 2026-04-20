package backend

import (
	"fmt"
	"sort"
	"strings"

	"plexcovermanager/models"
)

// BuildMissingCoverReport formats a textual list of missing covers and returns
// the rendered report together with the number of missing slots and affected items.
func BuildMissingCoverReport(items []models.MediaItem) (string, int, int) {
	type reportEntry struct {
		item         models.MediaItem
		missing      []string
		fullyMissing bool
	}

	entries := make([]reportEntry, 0)
	missingSlots := 0
	missingItems := 0

	for _, item := range items {
		missing := missingLabels(item)
		if len(missing) == 0 {
			continue
		}
		missingSlots += len(missing)
		missingItems++
		fullyMissing := len(item.CoverSlots) == 0 || len(missing) == len(item.CoverSlots)
		entries = append(entries, reportEntry{item: item, missing: missing, fullyMissing: fullyMissing})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].fullyMissing != entries[j].fullyMissing {
			return entries[i].fullyMissing
		}
		if entries[i].item.Type != entries[j].item.Type {
			return entries[i].item.Type < entries[j].item.Type
		}
		left := strings.ToLower(entries[i].item.Title)
		right := strings.ToLower(entries[j].item.Title)
		if left == right {
			return entries[i].item.ID < entries[j].item.ID
		}
		return left < right
	})

	var builder strings.Builder
	builder.WriteString("Fehlliste\n")
	builder.WriteString(fmt.Sprintf("Fehlende Cover: %d\n", missingSlots))
	builder.WriteString(fmt.Sprintf("Betroffene Titel: %d\n", missingItems))
	builder.WriteString("\n")

	writeSection := func(title string, complete bool) {
		builder.WriteString(title + "\n")
		written := false
		for _, entry := range entries {
			if entry.fullyMissing != complete {
				continue
			}
			if !written {
				written = true
			}
			builder.WriteString(formatEntry(entry.item, entry.missing))
			builder.WriteString("\n")
		}
		if !written {
			builder.WriteString("- keine Einträge\n")
		}
		builder.WriteString("\n")
	}

	writeSection("Komplett fehlende Einträge", true)
	writeSection("Teilweise fehlende Einträge", false)

	return strings.TrimSpace(builder.String()), missingSlots, missingItems
}

func missingLabels(item models.MediaItem) []string {
	if len(item.CoverSlots) == 0 {
		if len(item.Missing) == 0 {
			return nil
		}
		return dedupeAndNormalize(item.Missing, nil)
	}

	labelByKey := make(map[string]string, len(item.CoverSlots))
	for _, slot := range item.CoverSlots {
		key := normalizeKey(slot.Label)
		if key != "" {
			labelByKey[key] = slotExportLabel(slot)
		}
		if slot.Kind == models.CoverKindMain {
			labelByKey[normalizeKey(models.MainSlotKey())] = slotExportLabel(slot)
		}
		if slot.Kind == models.CoverKindSeason {
			labelByKey[normalizeKey(fmt.Sprintf("S%02d", slot.SeasonNumber))] = slotExportLabel(slot)
			if slot.SeasonNumber == 0 {
				labelByKey[normalizeKey("Specials")] = slotExportLabel(slot)
			}
		}
	}

	if len(item.Missing) > 0 {
		return dedupeAndNormalize(item.Missing, labelByKey)
	}

	missing := make([]string, 0)
	for _, slot := range item.CoverSlots {
		if slot.Exists {
			continue
		}
		missing = append(missing, slotExportLabel(slot))
	}
	return missing
}

func dedupeAndNormalize(labels []string, labelByKey map[string]string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		key := normalizeKey(label)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if labelByKey != nil {
			if friendly, ok := labelByKey[key]; ok && friendly != "" {
				out = append(out, friendly)
				continue
			}
		}
		out = append(out, strings.TrimSpace(label))
	}
	return out
}

func normalizeKey(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func formatEntry(item models.MediaItem, missing []string) string {
	if len(missing) == 0 {
		return fmt.Sprintf("- %s: %s", item.TypeLabel(), item.Title)
	}
	if len(item.CoverSlots) > 0 && len(missing) == len(item.CoverSlots) {
		if len(missing) == 1 {
			return fmt.Sprintf("- %s: %s: komplett betroffen (1 Cover fehlt)", item.TypeLabel(), item.Title)
		}
		return fmt.Sprintf("- %s: %s: komplett betroffen (%d Cover fehlen)", item.TypeLabel(), item.Title, len(missing))
	}
	label := "fehlen"
	if len(missing) == 1 {
		label = "fehlt"
	}
	extra := joinHumanList(missing)
	return fmt.Sprintf("- %s: %s: %s %s", item.TypeLabel(), item.Title, extra, label)
}

func joinHumanList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return values[0]
	}
	if len(values) == 2 {
		return values[0] + " und " + values[1]
	}
	return strings.Join(values[:len(values)-1], ", ") + " und " + values[len(values)-1]
}
