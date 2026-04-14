package cover

import (
	"path/filepath"
	"testing"

	"plexcovermanager/models"
)

func TestPlanImportsMatchesNormalizedSeriesSeason(t *testing.T) {
	seasonTarget := filepath.Join(`C:\Media\TV\Star Trek Strange New Worlds (2022)\Season 01`, "season01-poster.jpg")
	items := []models.MediaItem{
		{
			ID:    `C:\Media\TV\Star Trek Strange New Worlds (2022)`,
			Title: "Star Trek: Strange New Worlds (2022)",
			Year:  "2022",
			Type:  models.MediaTypeSeries,
			CoverSlots: []models.CoverSlot{
				{Key: models.MainSlotKey(), Label: "Main", Kind: models.CoverKindMain, TargetPath: filepath.Join(`C:\Media\TV\Star Trek Strange New Worlds (2022)`, "poster.jpg")},
				{Key: models.SeasonSlotKey(1), Label: "S01", Kind: models.CoverKindSeason, SeasonNumber: 1, TargetPath: seasonTarget},
			},
		},
	}

	plans := PlanImports([]string{`C:\covers\Star Trek - Strange New Worlds (2022) - Season 1.png`}, items)
	if len(plans) != 1 {
		t.Fatalf("len(plans) = %d", len(plans))
	}
	plan := plans[0]
	if !plan.CanApply {
		t.Fatalf("plan not applicable: %#v", plan)
	}
	if plan.TargetPath != seasonTarget {
		t.Fatalf("TargetPath = %q, want %q", plan.TargetPath, seasonTarget)
	}
}

func TestPlanImportsRejectsMissingSeason(t *testing.T) {
	items := []models.MediaItem{
		{
			ID:    `C:\Media\TV\The Show (2024)`,
			Title: "The Show (2024)",
			Year:  "2024",
			Type:  models.MediaTypeSeries,
			CoverSlots: []models.CoverSlot{
				{Key: models.MainSlotKey(), Label: "Main", Kind: models.CoverKindMain, TargetPath: filepath.Join(`C:\Media\TV\The Show (2024)`, "poster.jpg")},
			},
		},
	}

	plan := PlanImports([]string{`C:\covers\The Show (2024) - S02.png`}, items)[0]
	if plan.CanApply {
		t.Fatalf("plan should not apply: %#v", plan)
	}
	if plan.Status != ImportStatusSeasonMissing {
		t.Fatalf("Status = %q, want %q", plan.Status, ImportStatusSeasonMissing)
	}
}

func TestPlanForSlotUsesManualStatus(t *testing.T) {
	item := models.MediaItem{
		ID:    `C:\Media\Movies\365 Days to the Wedding`,
		Title: "365 Days to the Wedding",
		Type:  models.MediaTypeSeries,
	}
	slot := models.CoverSlot{
		Key:        models.MainSlotKey(),
		Label:      "Main",
		Kind:       models.CoverKindMain,
		TargetPath: filepath.Join(`C:\Media\Movies\365 Days to the Wedding`, "poster.jpg"),
	}

	plan := PlanForSlot(`C:\covers\100 METERS (2025).png`, item, slot)
	if !plan.CanApply {
		t.Fatalf("plan not applicable: %#v", plan)
	}
	if plan.Status != ImportStatusTargeted {
		t.Fatalf("Status = %q, want %q", plan.Status, ImportStatusTargeted)
	}
	if plan.MatchedScore != 0 {
		t.Fatalf("MatchedScore = %f, want 0", plan.MatchedScore)
	}
}
