package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"plexcovermanager/appversion"
	"plexcovermanager/assets"
	"plexcovermanager/config"
	"plexcovermanager/cover"
	"plexcovermanager/diagnostics"
	"plexcovermanager/models"
	"plexcovermanager/scanner"
)

const (
	filterAll      = "Alle"
	filterMissing  = "Nur fehlende Cover"
	filterComplete = "Nur vollständige"
	filterPartial  = "Nur teilweise"
)

type Application struct {
	app    fyne.App
	window fyne.Window
	config *config.Manager

	mu       sync.RWMutex
	items    []models.MediaItem
	filtered []models.MediaItem

	searchText      string
	sortAsc         bool
	statusFilter    string
	statusLabel     *widget.Label
	list            *widget.List
	currentDetailID string

	scanCancel context.CancelFunc
}

func NewApplication(configManager *config.Manager) *Application {
	diagnostics.Log("ui: create fyne app")
	fyneApp := fyneapp.NewWithID("de.plexcovermanager.app")
	fyneApp.SetIcon(assets.AppIcon())
	fyneApp.Settings().SetTheme(theme.DarkTheme())
	window := fyneApp.NewWindow(appversion.DisplayName())
	window.SetIcon(assets.AppIcon())
	window.SetMaster()
	window.Resize(fyne.NewSize(1024, 768))
	window.SetFixedSize(false)
	return &Application{
		app:          fyneApp,
		window:       window,
		config:       configManager,
		sortAsc:      true,
		statusFilter: filterAll,
	}
}

func (a *Application) Run() {
	diagnostics.Log("ui: run begin")
	a.window.SetOnDropped(a.handleFileDrop)
	a.showMainList()
	diagnostics.Log("ui: showing window")
	a.window.Show()
	a.window.CenterOnScreen()
	a.window.RequestFocus()
	diagnostics.Log("ui: window show returned")
	a.refreshData("Scan läuft ...", nil)
	diagnostics.Log("ui: app run begin")
	a.app.Run()
	diagnostics.Log("ui: app run returned")
}

func (a *Application) handleFileDrop(_ fyne.Position, uris []fyne.URI) {
	var paths []string
	for _, uri := range uris {
		p := uri.Path()
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return
	}
	detailID := a.currentDetailID
	if detailID != "" {
		item, ok := a.findItem(detailID)
		if !ok {
			return
		}
		plans := cover.PlanImportsForItem(paths, item)
		a.showImportPreview("Cover hinzufügen (Drag & Drop)", plans, func() {
			a.showDetail(detailID)
		})
	} else {
		a.mu.RLock()
		items := append([]models.MediaItem(nil), a.items...)
		a.mu.RUnlock()
		plans := cover.PlanImports(paths, items)
		a.showImportPreview("Batch-Import (Drag & Drop)", plans, nil)
	}
}

func (a *Application) showMainList() {
	diagnostics.Log("ui: show main list")
	a.currentDetailID = ""
	title := widget.NewLabelWithStyle("Plex Cover Manager", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	search := widget.NewEntry()
	search.SetPlaceHolder("Suchen...")
	search.SetText(a.searchText)
	search.OnChanged = func(value string) {
		a.searchText = value
		a.applyFilters()
	}

	sortSelect := widget.NewSelect([]string{"A-Z", "Z-A"}, func(value string) {
		a.sortAsc = value != "Z-A"
		a.applyFilters()
	})
	if a.sortAsc {
		sortSelect.SetSelected("A-Z")
	} else {
		sortSelect.SetSelected("Z-A")
	}

	filterSelect := widget.NewSelect([]string{filterAll, filterMissing, filterComplete, filterPartial}, func(value string) {
		if value == "" {
			value = filterAll
		}
		a.statusFilter = value
		a.applyFilters()
	})
	filterSelect.SetSelected(a.statusFilter)

	rescanButton := widget.NewButton("Rescan", func() {
		a.refreshData("Scan läuft ...", nil)
	})
	batchButton := widget.NewButton("Batch-Import", a.startBatchImport)
	settingsButton := widget.NewButton("Einstellungen", a.showSettings)

	topLine := container.NewBorder(nil, nil, title, container.NewHBox(sortSelect, filterSelect, rescanButton, batchButton, settingsButton), search)
	a.statusLabel = widget.NewLabel("Bereit")

	a.list = widget.NewList(
		func() int {
			a.mu.RLock()
			defer a.mu.RUnlock()
			return len(a.filtered)
		},
		func() fyne.CanvasObject {
			titleLabel := widget.NewLabelWithStyle("Titel", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			titleLabel.Truncation = fyne.TextTruncateEllipsis
			statusLabel := widget.NewLabel("Status")
			statusLabel.Importance = widget.LowImportance
			statusLabel.Truncation = fyne.TextTruncateEllipsis
			typeLabel := widget.NewLabel("[Serie]")
			typeLabel.Importance = widget.LowImportance
			topLine := container.NewBorder(nil, nil, typeLabel, nil, titleLabel)
			textBox := container.NewVBox(topLine, statusLabel)
			return container.NewBorder(nil, nil, statusDot(models.CoverStatusNone), nil, textBox)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			a.mu.RLock()
			defer a.mu.RUnlock()
			if id < 0 || id >= len(a.filtered) {
				return
			}
			item := a.filtered[id]
			row := obj.(*fyne.Container)
			textBox := row.Objects[0].(*fyne.Container)
			topLine := textBox.Objects[0].(*fyne.Container)
			titleLabel := topLine.Objects[0].(*widget.Label)
			typeLabel := topLine.Objects[1].(*widget.Label)
			statusLabel := textBox.Objects[1].(*widget.Label)
			dotCenter := row.Objects[1].(*fyne.Container)
			dotGrid := dotCenter.Objects[0].(*fyne.Container)
			dot := dotGrid.Objects[0].(*canvas.Circle)
			applyStatusDot(dot, item.Status)
			typeLabel.SetText(fmt.Sprintf("[%s]", item.TypeLabel()))
			titleLabel.SetText(item.Title)
			statusLabel.SetText(item.StatusLabel())
		},
	)
	a.list.OnSelected = func(id widget.ListItemID) {
		a.mu.RLock()
		if id < 0 || id >= len(a.filtered) {
			a.mu.RUnlock()
			return
		}
		item := a.filtered[id]
		a.mu.RUnlock()
		a.list.UnselectAll()
		a.showDetail(item.ID)
	}

	content := container.NewBorder(topLine, a.statusLabel, nil, nil, a.list)
	a.window.SetContent(content)
	a.applyFilters()
}

func (a *Application) applyFilters() {
	a.mu.Lock()
	filtered := make([]models.MediaItem, 0, len(a.items))
	query := strings.TrimSpace(strings.ToLower(a.searchText))
	for _, item := range a.items {
		if query != "" && !strings.Contains(strings.ToLower(item.Title), query) {
			continue
		}
		switch a.statusFilter {
		case filterMissing:
			if item.Status != models.CoverStatusNone {
				continue
			}
		case filterComplete:
			if item.Status != models.CoverStatusComplete {
				continue
			}
		case filterPartial:
			if item.Status != models.CoverStatusPartial {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := strings.ToLower(filtered[i].Title), strings.ToLower(filtered[j].Title)
		if left == right {
			return filtered[i].Path < filtered[j].Path
		}
		if a.sortAsc {
			return left < right
		}
		return left > right
	})
	a.filtered = filtered
	a.mu.Unlock()

	if a.list != nil {
		a.list.Refresh()
	}
	if a.statusLabel != nil {
		a.statusLabel.SetText(a.listStatusText())
	}
}

func (a *Application) listStatusText() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	total := len(a.items)
	shown := len(a.filtered)
	if total == 0 {
		return "Keine Titel gefunden. Prüfe die Medienpfade in den Einstellungen."
	}
	if shown == total {
		return fmt.Sprintf("%d Titel", total)
	}
	return fmt.Sprintf("%d von %d Titeln", shown, total)
}

func (a *Application) refreshData(message string, after func()) {
	diagnostics.Log("scan: requested")
	if a.scanCancel != nil {
		a.scanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.scanCancel = cancel
	if a.statusLabel != nil {
		a.statusLabel.SetText(message)
	}
	cfg := a.config.Get()
	go func() {
		diagnostics.Log("scan: start paths=%d", len(cfg.MediaPaths))
		items, warnings := scanner.ScanLibrary(ctx, cfg)
		if ctx.Err() != nil {
			diagnostics.Log("scan: canceled")
			return
		}
		diagnostics.Log("scan: done items=%d warnings=%d", len(items), len(warnings))
		fyne.Do(func() {
			diagnostics.Log("scan: applying ui update")
			a.mu.Lock()
			a.items = items
			a.mu.Unlock()
			a.applyFilters()
			if len(warnings) > 0 {
				dialog.ShowInformation("Scan-Hinweise", scanWarningText(warnings), a.window)
			}
			if after != nil {
				after()
			}
		})
	}()
}

func scanWarningText(warnings []models.ScanWarning) string {
	const maxShown = 8
	lines := make([]string, 0, len(warnings)+1)
	for i, warning := range warnings {
		if i >= maxShown {
			lines = append(lines, fmt.Sprintf("... und %d weitere Hinweise", len(warnings)-maxShown))
			break
		}
		lines = append(lines, fmt.Sprintf("%s: %s", warning.Path, warning.Message))
	}
	return strings.Join(lines, "\n")
}

func (a *Application) showDetail(itemID string) {
	item, ok := a.findItem(itemID)
	if !ok {
		a.showMainList()
		return
	}
	a.currentDetailID = itemID

	backButton := widget.NewButton("Zurück", a.showMainList)
	title := widget.NewLabelWithStyle(fmt.Sprintf("%s [%s]", item.Title, item.TypeLabel()), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	status := widget.NewLabel(item.StatusLabel())
	header := container.NewBorder(nil, nil, backButton, nil, container.NewVBox(title, container.NewHBox(statusDot(item.Status), status)))

	structure := widget.NewLabel(itemStructureText(item))
	structure.Wrapping = fyne.TextWrapWord

	slotRows := container.NewVBox()
	for _, slot := range item.SortedSlots() {
		slotCopy := slot
		slotRows.Add(a.coverSlotRow(item, slotCopy))
	}

	addButton := widget.NewButton("Cover hinzufügen", func() {
		a.selectAndPreviewForItem(item.ID)
	})
	pathLabel := widget.NewLabel(fmt.Sprintf("Pfad: %s", displayItemPath(item)))
	pathLabel.Wrapping = fyne.TextWrapWord
	pathLabel.Selectable = true
	openPathButton := widget.NewButton("Ordner öffnen", func() {
		target := displayItemPath(item)
		if err := openFolderInExplorer(target); err != nil {
			dialog.ShowError(err, a.window)
		}
	})
	pathRow := container.NewBorder(nil, nil, nil, openPathButton, pathLabel)

	body := container.NewVBox(
		structure,
		addButton,
		widget.NewSeparator(),
		slotRows,
		widget.NewSeparator(),
		pathRow,
	)
	a.window.SetContent(container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
}

func (a *Application) coverSlotRow(item models.MediaItem, slot models.CoverSlot) fyne.CanvasObject {
	preview := slotPreview(slot)
	infoText := fmt.Sprintf("%s\nZiel: %s\nStatus: Fehlt", slot.Label, filepath.Base(slot.TargetPath))
	if slot.Exists {
		infoText = fmt.Sprintf("%s\nDatei: %s\nGröße: %s\nStatus: Vorhanden", slot.Label, filepath.Base(slot.ExistingPath), formatBytes(slot.SizeBytes))
	}
	info := widget.NewLabel(infoText)
	info.Wrapping = fyne.TextWrapWord
	info.Selectable = true

	deleteButton := widget.NewButton("Löschen", func() {
		a.confirmDeleteCover(item.ID, slot)
	})
	deleteButton.Disable()
	if slot.Exists {
		deleteButton.Enable()
	}
	replaceButton := widget.NewButton("Ersetzen", func() {
		a.selectAndPreviewForSlot(item.ID, slot)
	})
	actions := container.NewVBox(replaceButton, deleteButton)
	return container.NewBorder(nil, widget.NewSeparator(), preview, actions, info)
}

func slotPreview(slot models.CoverSlot) fyne.CanvasObject {
	if slot.Exists && slot.ExistingPath != "" {
		img := canvas.NewImageFromFile(slot.ExistingPath)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(88, 132))
		return container.NewCenter(img)
	}
	label := widget.NewLabel("Kein\nCover")
	label.Alignment = fyne.TextAlignCenter
	return container.NewCenter(container.NewPadded(label))
}

func (a *Application) confirmDeleteCover(itemID string, slot models.CoverSlot) {
	if !slot.Exists || slot.ExistingPath == "" {
		return
	}
	dialog.NewConfirm("Cover löschen", fmt.Sprintf("%s wirklich löschen?\n\n%s", slot.Label, slot.ExistingPath), func(ok bool) {
		if !ok {
			return
		}
		if err := os.Remove(slot.ExistingPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			dialog.ShowError(err, a.window)
			return
		}
		a.refreshData("Scan läuft ...", func() { a.showDetail(itemID) })
	}, a.window).Show()
}

func (a *Application) selectAndPreviewForSlot(itemID string, slot models.CoverSlot) {
	go func() {
		paths, err := selectCoverFiles(false)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if len(paths) == 0 {
				return
			}
			item, ok := a.findItem(itemID)
			if !ok {
				dialog.ShowInformation("Titel nicht gefunden", "Der Titel ist nach dem letzten Scan nicht mehr vorhanden.", a.window)
				return
			}
			plan := cover.PlanForSlot(paths[0], item, slot)
			a.showImportPreview("Cover ersetzen", []cover.ImportPlan{plan}, func() {
				a.showDetail(itemID)
			})
		})
	}()
}

func (a *Application) selectAndPreviewForItem(itemID string) {
	go func() {
		paths, err := selectCoverFiles(true)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if len(paths) == 0 {
				return
			}
			item, ok := a.findItem(itemID)
			if !ok {
				dialog.ShowInformation("Titel nicht gefunden", "Der Titel ist nach dem letzten Scan nicht mehr vorhanden.", a.window)
				return
			}
			plans := cover.PlanImportsForItem(paths, item)
			a.showImportPreview("Cover hinzufügen", plans, func() {
				a.showDetail(itemID)
			})
		})
	}()
}

func (a *Application) startBatchImport() {
	go func() {
		paths, err := selectCoverFiles(true)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if len(paths) == 0 {
				return
			}
			a.mu.RLock()
			items := append([]models.MediaItem(nil), a.items...)
			a.mu.RUnlock()
			plans := cover.PlanImports(paths, items)
			a.showImportPreview("Batch-Import", plans, nil)
		})
	}()
}

func (a *Application) showImportPreview(title string, plans []cover.ImportPlan, after func()) {
	applyCount := 0
	for _, plan := range plans {
		if plan.CanApply {
			applyCount++
		}
	}
	summary := widget.NewLabel(fmt.Sprintf("%d Datei(en), %d werden übernommen.", len(plans), applyCount))
	summary.Wrapping = fyne.TextWrapWord

	rows := container.NewVBox()
	for _, plan := range plans {
		label := widget.NewLabel(previewText(plan))
		label.Wrapping = fyne.TextWrapWord
		label.Selectable = true
		rows.Add(label)
		rows.Add(widget.NewSeparator())
	}
	scroll := container.NewVScroll(rows)
	scroll.SetMinSize(fyne.NewSize(860, 420))
	content := container.NewBorder(summary, nil, nil, nil, scroll)

	confirm := dialog.NewCustomConfirm(title, "Übernehmen", "Abbrechen", content, func(ok bool) {
		if !ok {
			return
		}
		a.applyPlans(plans, after)
	}, a.window)
	confirm.Resize(fyne.NewSize(920, 540))
	confirm.Show()
}

func previewText(plan cover.ImportPlan) string {
	icon := "[FEHLER]"
	if plan.CanApply {
		icon = "[OK]"
	}
	if plan.Overwrites {
		icon = "[WARNUNG]"
	}
	target := plan.TargetPath
	if target == "" {
		target = "-"
	}
	title := plan.ItemTitle
	if title == "" {
		title = "-"
	}
	slot := plan.SlotLabel
	if slot == "" {
		slot = "-"
	}
	return fmt.Sprintf("%s %s\nTitel: %s | Slot: %s | Status: %s\nZiel: %s\n%s",
		icon, plan.SourceFile, title, slot, plan.Status, target, plan.Message)
}

func (a *Application) applyPlans(plans []cover.ImportPlan, after func()) {
	cfg := a.config.Get()
	go func() {
		applied := 0
		var failures []string
		for _, plan := range plans {
			if !plan.CanApply {
				continue
			}
			if _, err := cover.ApplyImportPlan(plan, cfg.Compression); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %s", plan.SourceFile, err.Error()))
				continue
			}
			applied++
		}
		fyne.Do(func() {
			if len(failures) > 0 {
				dialog.ShowInformation("Import abgeschlossen", fmt.Sprintf("%d Cover übernommen.\n\nFehler:\n%s", applied, strings.Join(failures, "\n")), a.window)
			} else {
				dialog.ShowInformation("Import abgeschlossen", fmt.Sprintf("%d Cover übernommen.", applied), a.window)
			}
			a.refreshData("Scan läuft ...", after)
		})
	}()
}

func (a *Application) showSettings() {
	a.currentDetailID = ""
	cfg := a.config.Get()
	backButton := widget.NewButton("Zurück", func() {
		a.showMainList()
		a.refreshData("Scan läuft ...", nil)
	})
	title := widget.NewLabelWithStyle("Einstellungen", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header := container.NewBorder(nil, nil, backButton, nil, title)

	pathsBox := container.NewVBox()
	if len(cfg.MediaPaths) == 0 {
		hint := widget.NewLabel("Noch keine Medienpfade konfiguriert.")
		hint.Wrapping = fyne.TextWrapWord
		pathsBox.Add(hint)
	}
	for idx, mediaPath := range cfg.MediaPaths {
		index := idx
		pathCopy := mediaPath
		pathsBox.Add(a.settingsPathRow(index, pathCopy))
	}
	addPathButton := widget.NewButton("Pfad hinzufügen", a.addMediaPath)

	qualityValue := widget.NewLabel(fmt.Sprintf("%d", cfg.Compression.JPEGQuality))
	qualitySlider := widget.NewSlider(70, 100)
	qualitySlider.Step = 1
	qualitySlider.Value = float64(cfg.Compression.JPEGQuality)
	qualitySlider.OnChanged = func(value float64) {
		quality := int(value + 0.5)
		qualityValue.SetText(fmt.Sprintf("%d", quality))
		a.saveConfig(func(cfg *models.AppConfig) {
			cfg.Compression.JPEGQuality = quality
		})
	}

	widthEntry := widget.NewEntry()
	widthEntry.SetText(strconv.Itoa(cfg.Compression.MaxWidth))
	widthEntry.OnChanged = func(value string) {
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
			a.saveConfig(func(cfg *models.AppConfig) {
				cfg.Compression.MaxWidth = parsed
			})
		}
	}
	heightEntry := widget.NewEntry()
	heightEntry.SetText(strconv.Itoa(cfg.Compression.MaxHeight))
	heightEntry.OnChanged = func(value string) {
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
			a.saveConfig(func(cfg *models.AppConfig) {
				cfg.Compression.MaxHeight = parsed
			})
		}
	}

	configPath := widget.NewLabel(fmt.Sprintf("Config: %s", a.config.Path()))
	configPath.Wrapping = fyne.TextWrapWord
	configPath.Selectable = true
	openConfigFolderButton := widget.NewButton("Ordner öffnen", func() {
		if err := openFileInExplorer(a.config.Path()); err != nil {
			dialog.ShowError(err, a.window)
		}
	})
	openConfigFileButton := widget.NewButton("Datei öffnen", func() {
		if err := openFileWithDefault(a.config.Path()); err != nil {
			dialog.ShowError(err, a.window)
		}
	})
	configRow := container.NewBorder(nil, nil, nil, container.NewHBox(openConfigFolderButton, openConfigFileButton), configPath)

	body := container.NewVBox(
		widget.NewLabelWithStyle("Medienpfade", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		pathsBox,
		addPathButton,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Komprimierung", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("JPEG-Qualität"), qualityValue, qualitySlider),
		container.NewGridWithColumns(2,
			container.NewBorder(nil, nil, widget.NewLabel("Max. Breite"), nil, widthEntry),
			container.NewBorder(nil, nil, widget.NewLabel("Max. Höhe"), nil, heightEntry),
		),
		widget.NewSeparator(),
		configRow,
	)
	a.window.SetContent(container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
}

func (a *Application) settingsPathRow(index int, mediaPath models.MediaPath) fyne.CanvasObject {
	pathLabel := widget.NewLabel(mediaPath.Path)
	pathLabel.Wrapping = fyne.TextWrapWord
	pathLabel.Selectable = true
	reachable := isPathReachable(mediaPath.Path)
	reachabilityLabel := "erreichbar"
	if !reachable {
		reachabilityLabel = "nicht erreichbar"
	}
	statusLabel := widget.NewLabel(reachabilityLabel)

	typeSelect := widget.NewSelect([]string{"Serie", "Film"}, func(value string) {
		mediaType := models.MediaTypeSeries
		if value == "Film" {
			mediaType = models.MediaTypeMovie
		}
		a.saveConfig(func(cfg *models.AppConfig) {
			if index >= 0 && index < len(cfg.MediaPaths) {
				cfg.MediaPaths[index].Type = mediaType
			}
		})
	})
	if mediaPath.Type == models.MediaTypeMovie {
		typeSelect.SetSelected("Film")
	} else {
		typeSelect.SetSelected("Serie")
	}

	deleteButton := widget.NewButton("Löschen", func() {
		dialog.NewConfirm("Pfad löschen", fmt.Sprintf("%s aus der Konfiguration entfernen?", mediaPath.Path), func(ok bool) {
			if !ok {
				return
			}
			a.saveConfig(func(cfg *models.AppConfig) {
				if index >= 0 && index < len(cfg.MediaPaths) {
					cfg.MediaPaths = append(cfg.MediaPaths[:index], cfg.MediaPaths[index+1:]...)
				}
			})
			a.showSettings()
		}, a.window).Show()
	})
	return container.NewBorder(nil, widget.NewSeparator(), nil, container.NewHBox(typeSelect, reachabilityDot(reachable), statusLabel, deleteButton), pathLabel)
}

func (a *Application) addMediaPath() {
	go func() {
		path, err := selectFolder()
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if strings.TrimSpace(path) == "" {
				return
			}
			typeSelect := widget.NewSelect([]string{"Serie", "Film"}, nil)
			typeSelect.SetSelected("Serie")
			content := container.NewVBox(widget.NewLabel(path), typeSelect)
			dialog.NewCustomConfirm("Pfad hinzufügen", "Hinzufügen", "Abbrechen", content, func(ok bool) {
				if !ok {
					return
				}
				mediaType := models.MediaTypeSeries
				if typeSelect.Selected == "Film" {
					mediaType = models.MediaTypeMovie
				}
				a.saveConfig(func(cfg *models.AppConfig) {
					cfg.MediaPaths = append(cfg.MediaPaths, models.MediaPath{Path: path, Type: mediaType})
				})
				a.showSettings()
			}, a.window).Show()
		})
	}()
}

func (a *Application) saveConfig(update func(*models.AppConfig)) {
	if err := a.config.Update(update); err != nil {
		dialog.ShowError(err, a.window)
	}
}

func (a *Application) findItem(itemID string) (models.MediaItem, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, item := range a.items {
		if item.ID == itemID {
			return item, true
		}
	}
	return models.MediaItem{}, false
}

func itemStructureText(item models.MediaItem) string {
	if item.Type == models.MediaTypeMovie {
		if item.FlatStructure {
			return "Filmdatei liegt direkt im Movies-Root."
		}
		return "Filmordner mit lokalem poster.jpg."
	}
	if len(item.Seasons) == 0 {
		return "Keine Staffelordner oder SxxExx-Mediendateien erkannt. Nur Main-Poster wird verwaltet."
	}
	labels := make([]string, 0, len(item.Seasons))
	for _, season := range item.Seasons {
		labels = append(labels, season.DisplayLabel())
	}
	if item.FlatStructure {
		return fmt.Sprintf("Flat Structure: %d Staffel(n) aus Dateinamen erkannt: %s", len(labels), strings.Join(labels, ", "))
	}
	return fmt.Sprintf("%d Staffelordner gefunden: %s", len(labels), strings.Join(labels, ", "))
}

func displayItemPath(item models.MediaItem) string {
	if item.MediaFilePath != "" {
		return item.MediaFilePath
	}
	return item.Path
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func isPathReachable(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func statusDot(status models.CoverStatus) fyne.CanvasObject {
	dot := canvas.NewCircle(color.NRGBA{})
	dot.StrokeColor = color.NRGBA{R: 10, G: 10, B: 10, A: 180}
	dot.StrokeWidth = 1
	applyStatusDot(dot, status)
	return container.NewCenter(container.NewGridWrap(fyne.NewSize(14, 14), dot))
}

func applyStatusDot(dot *canvas.Circle, status models.CoverStatus) {
	switch status {
	case models.CoverStatusComplete:
		dot.FillColor = color.NRGBA{R: 46, G: 204, B: 113, A: 255}
	case models.CoverStatusPartial:
		dot.FillColor = color.NRGBA{R: 241, G: 196, B: 15, A: 255}
	default:
		dot.FillColor = color.NRGBA{R: 231, G: 76, B: 60, A: 255}
	}
	dot.Refresh()
}

func reachabilityDot(reachable bool) fyne.CanvasObject {
	dot := canvas.NewCircle(color.NRGBA{R: 231, G: 76, B: 60, A: 255})
	if reachable {
		dot.FillColor = color.NRGBA{R: 46, G: 204, B: 113, A: 255}
	}
	dot.StrokeColor = color.NRGBA{R: 10, G: 10, B: 10, A: 180}
	dot.StrokeWidth = 1
	return container.NewCenter(container.NewGridWrap(fyne.NewSize(12, 12), dot))
}
