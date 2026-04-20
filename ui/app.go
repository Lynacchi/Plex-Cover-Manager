package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"plexcovermanager/appversion"
	"plexcovermanager/assets"
	"plexcovermanager/backend"
	"plexcovermanager/config"
	"plexcovermanager/cover"
	"plexcovermanager/diagnostics"
	"plexcovermanager/models"
	"plexcovermanager/scanner"
)

const (
	filterAll        = "Alle"
	filterMissing    = "Fehlende Cover"
	filterComplete   = "Cover vollständig"
	filterPartial    = "Cover teilweise"
	typeFilterAll    = "Alle"
	typeFilterMovies = "Filme"
	typeFilterSeries = "Serien"
)

var (
	statusCompleteColor = color.NRGBA{R: 46, G: 204, B: 113, A: 255}
	statusPartialColor  = color.NRGBA{R: 241, G: 196, B: 15, A: 255}
	statusMissingColor  = color.NRGBA{R: 231, G: 76, B: 60, A: 255}
	statusOptimizeColor = color.NRGBA{R: 52, G: 152, B: 219, A: 255}
	statusUnknownColor  = color.NRGBA{R: 149, G: 165, B: 166, A: 255}
)

type visualStatus struct {
	fill    color.NRGBA
	tooltip string
}

type Application struct {
	app    fyne.App
	window fyne.Window
	config *config.Manager

	mu       sync.RWMutex
	items    []models.MediaItem
	filtered []models.MediaItem

	searchText      string
	typeFilter      string
	statusFilter    string
	statusLabel     *widget.Label
	list            *widget.List
	currentDetailID string
	detailDropSlots []detailDropSlot

	scanCancel context.CancelFunc
}

type detailDropSlot struct {
	itemID string
	slot   models.CoverSlot
	object fyne.CanvasObject
}

type statusIndicator struct {
	widget.BaseWidget

	dot      *canvas.Circle
	tooltip  string
	popup    *widget.PopUp
	timer    *time.Timer
	lastPos  fyne.Position
	hovering bool
}

func newStatusIndicator(status visualStatus) *statusIndicator {
	indicator := &statusIndicator{
		dot: canvas.NewCircle(status.fill),
	}
	indicator.dot.StrokeColor = color.NRGBA{R: 10, G: 10, B: 10, A: 180}
	indicator.dot.StrokeWidth = 1
	indicator.ExtendBaseWidget(indicator)
	indicator.SetStatus(status)
	return indicator
}

func (s *statusIndicator) SetStatus(status visualStatus) {
	s.tooltip = status.tooltip
	s.dot.FillColor = status.fill
	s.dot.Refresh()
	if s.popup != nil {
		s.popup.Hide()
		s.popup = nil
	}
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.Refresh()
}

func (s *statusIndicator) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewCenter(container.NewGridWrap(fyne.NewSize(14, 14), s.dot)))
}

func (s *statusIndicator) MouseIn(event *desktop.MouseEvent) {
	if strings.TrimSpace(s.tooltip) == "" {
		return
	}
	if event == nil {
		return
	}
	if s.popup != nil {
		return
	}
	s.hovering = true
	s.lastPos = event.AbsolutePosition
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(450*time.Millisecond, func() {
		fyne.Do(func() {
			if !s.hovering || s.popup != nil || strings.TrimSpace(s.tooltip) == "" {
				return
			}
			c := fyne.CurrentApp().Driver().CanvasForObject(s)
			if c == nil {
				return
			}
			label := widget.NewLabel(s.tooltip)
			label.Wrapping = fyne.TextWrapWord
			content := container.New(layout.NewCustomPaddedLayout(6, 6, 8, 8), label)
			s.popup = widget.NewPopUp(content, c)
			s.popup.Resize(fyne.NewSize(260, content.MinSize().Height))
			s.popup.ShowAtPosition(s.lastPos.Add(fyne.NewPos(14, 14)))
		})
	})
}

func (s *statusIndicator) MouseMoved(event *desktop.MouseEvent) {
	// Tooltips stay fixed at their first position until the pointer leaves the dot.
}

func (s *statusIndicator) MouseOut() {
	s.hovering = false
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if s.popup != nil {
		s.popup.Hide()
		s.popup = nil
	}
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
		typeFilter:   typeFilterAll,
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

func (a *Application) handleFileDrop(pos fyne.Position, uris []fyne.URI) {
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
		if slot, ok := a.detailSlotAt(pos); ok {
			if len(paths) > 1 {
				dialog.ShowInformation("Eine Datei pro Position", "Lege auf eine konkrete Cover-Position bitte nur eine Datei ab. Für mehrere Dateien nutze die automatische Zuordnung.", a.window)
				return
			}
			plan := cover.PlanForSlot(paths[0], item, slot)
			title := "Cover hinzufügen (Drag & Drop)"
			if slot.Exists {
				title = "Cover ersetzen (Drag & Drop)"
			}
			a.showImportPreview(title, []cover.ImportPlan{plan}, func() {
				a.showDetail(detailID)
			})
			return
		}
		plans := cover.PlanImportsForItem(paths, item)
		a.showImportPreview("Cover automatisch zuordnen (Drag & Drop)", plans, func() {
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

func (a *Application) detailSlotAt(pos fyne.Position) (models.CoverSlot, bool) {
	if a.currentDetailID == "" {
		return models.CoverSlot{}, false
	}
	driver := fyne.CurrentApp().Driver()
	for _, target := range a.detailDropSlots {
		if target.itemID != a.currentDetailID || target.object == nil || !target.object.Visible() {
			continue
		}
		topLeft := driver.AbsolutePositionForObject(target.object)
		size := target.object.Size()
		if pos.X >= topLeft.X && pos.X <= topLeft.X+size.Width && pos.Y >= topLeft.Y && pos.Y <= topLeft.Y+size.Height {
			return target.slot, true
		}
	}
	return models.CoverSlot{}, false
}

func (a *Application) showMainList() {
	diagnostics.Log("ui: show main list")
	a.currentDetailID = ""
	a.detailDropSlots = nil
	cfg := a.config.Get()
	posterDBEnabled := cfg.PosterDBSearchEnabled
	title := widget.NewLabelWithStyle("Plex Cover Manager", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	search := widget.NewEntry()
	search.SetPlaceHolder("Suchen...")
	search.SetText(a.searchText)
	search.OnChanged = func(value string) {
		a.searchText = value
		a.applyFilters()
	}

	typeSelect := widget.NewSelect([]string{typeFilterAll, typeFilterMovies, typeFilterSeries}, func(value string) {
		if value == "" {
			value = typeFilterAll
		}
		a.typeFilter = value
		a.applyFilters()
	})
	if a.typeFilter == "" {
		a.typeFilter = typeFilterAll
	}
	typeSelect.SetSelected(a.typeFilter)

	filterSelect := widget.NewSelect([]string{filterAll, filterMissing, filterComplete, filterPartial}, func(value string) {
		if value == "" {
			value = filterAll
		}
		a.statusFilter = value
		a.applyFilters()
	})
	filterSelect.SetSelected(a.statusFilter)

	rescanButton := widget.NewButton("Neu scannen", func() {
		a.refreshData("Scan läuft ...", nil)
	})
	batchButton := widget.NewButton("Batch-Import", a.startBatchImport)
	settingsButton := widget.NewButton("Einstellungen", a.showSettings)

	topLine := container.NewBorder(nil, nil, title, container.NewHBox(typeSelect, filterSelect, rescanButton, batchButton, settingsButton), search)
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
			typeLabel := widget.NewLabel("Serie")
			indicator := newStatusIndicator(visualStatusForCover(models.CoverStatusNone, 0, 0, "Kein Cover vorhanden"))
			searchButton := widget.NewButtonWithIcon("", theme.SearchIcon(), nil)
			searchButton.Hide()
			row := container.NewBorder(nil, nil, container.NewHBox(indicator, typeLabel), searchButton, titleLabel)
			return container.New(layout.NewCustomPaddedLayout(2, 2, 8, 8), row)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			a.mu.RLock()
			if id < 0 || id >= len(a.filtered) {
				a.mu.RUnlock()
				return
			}
			item := a.filtered[id]
			a.mu.RUnlock()
			padded := obj.(*fyne.Container)
			row := padded.Objects[0].(*fyne.Container)
			titleLabel := row.Objects[0].(*widget.Label)
			leftBox := row.Objects[1].(*fyne.Container)
			indicator := leftBox.Objects[0].(*statusIndicator)
			typeLabel := leftBox.Objects[1].(*widget.Label)
			searchButton := row.Objects[2].(*widget.Button)
			indicator.SetStatus(visualStatusForItem(item))
			typeLabel.SetText(item.TypeLabel())
			titleLabel.SetText(item.Title)
			if posterDBEnabled && shouldOfferPosterDBSearch(item) {
				itemCopy := item
				searchButton.OnTapped = func() { a.openPosterDBSearch(itemCopy) }
				searchButton.Show()
			} else {
				searchButton.OnTapped = nil
				searchButton.Hide()
			}
		},
	)
	a.list.HideSeparators = true
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
		switch a.typeFilter {
		case typeFilterMovies:
			if item.Type != models.MediaTypeMovie {
				continue
			}
		case typeFilterSeries:
			if item.Type != models.MediaTypeSeries {
				continue
			}
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
		return left < right
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

func (a *Application) refreshDetailItem(itemID string) {
	item, ok := a.findItem(itemID)
	if !ok {
		dialog.ShowInformation("Titel nicht gefunden", "Der Titel ist nach dem letzten Scan nicht mehr vorhanden.", a.window)
		return
	}
	cfg := a.config.Get()
	go func() {
		refreshed, warnings := scanner.RescanItem(context.Background(), cfg, item)
		fyne.Do(func() {
			if refreshed.ID == "" {
				if len(warnings) > 0 {
					dialog.ShowInformation("Aktualisieren", scanWarningText(warnings), a.window)
				} else {
					dialog.ShowInformation("Aktualisieren", "Der Titel konnte nicht neu geladen werden.", a.window)
				}
				return
			}
			a.mu.Lock()
			replaced := false
			for i := range a.items {
				if a.items[i].ID == itemID {
					a.items[i] = refreshed
					replaced = true
					break
				}
			}
			if !replaced {
				a.items = append(a.items, refreshed)
			}
			a.mu.Unlock()
			a.applyFilters()
			if len(warnings) > 0 {
				dialog.ShowInformation("Aktualisieren", scanWarningText(warnings), a.window)
			}
			a.showDetail(refreshed.ID)
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
	a.detailDropSlots = nil
	cfg := a.config.Get()

	backButton := widget.NewButton("Zurück", a.showMainList)
	title := widget.NewLabelWithStyle(item.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Truncation = fyne.TextTruncateEllipsis
	typeLabel := widget.NewLabel(item.TypeLabel())
	headerLine := container.NewBorder(nil, nil, container.NewHBox(newStatusIndicator(visualStatusForItem(item)), typeLabel), nil, title)
	refreshButton := widget.NewButton("Neu laden", func() {
		a.refreshDetailItem(item.ID)
	})
	headerActions := container.NewHBox()
	if cfg.PosterDBSearchEnabled && shouldOfferPosterDBSearch(item) {
		itemCopy := item
		posterDBButton := widget.NewButtonWithIcon("PosterDB", theme.SearchIcon(), func() {
			a.openPosterDBSearch(itemCopy)
		})
		headerActions.Add(posterDBButton)
	}
	headerActions.Add(refreshButton)
	header := container.NewBorder(nil, nil, backButton, headerActions, headerLine)

	structure := widget.NewLabel(itemStructureText(item, cfg.ServerMode))
	structure.Wrapping = fyne.TextWrapWord

	slotRows := container.NewVBox()
	for _, slot := range item.SortedSlots() {
		slotCopy := slot
		row := a.coverSlotRow(item, slotCopy, cfg)
		a.detailDropSlots = append(a.detailDropSlots, detailDropSlot{itemID: item.ID, slot: slotCopy, object: row})
		slotRows.Add(row)
	}

	addButton := widget.NewButton("Mehrere Cover automatisch zuordnen", func() {
		a.selectAndPreviewForItem(item.ID)
	})
	pathLabel := widget.NewLabel(fmt.Sprintf("Pfad: %s", displayItemPath(item)))
	pathLabel.Wrapping = fyne.TextWrapWord
	pathLabel.Selectable = true
	openPathButton := widget.NewButton("Ordner öffnen", func() {
		target := itemFolderPath(item)
		if err := openFolderInExplorer(target); err != nil {
			dialog.ShowError(err, a.window)
		}
	})
	pathRow := container.NewBorder(nil, nil, nil, openPathButton, pathLabel)

	body := container.New(layout.NewCustomPaddedVBoxLayout(4),
		structure,
		addButton,
		slotRows,
		pathRow,
	)
	a.window.SetContent(container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
}

func (a *Application) coverSlotRow(item models.MediaItem, slot models.CoverSlot, cfg models.AppConfig) fyne.CanvasObject {
	preview := slotPreview(slot)
	targetName := targetDisplayName(item, slot, cfg.ServerMode)
	infoText := fmt.Sprintf("%s\nZiel: %s\nStatus: Fehlt", slot.Label, targetName)
	if slot.Exists {
		compressionStatus := "OK"
		if cfg.Compression.Disabled {
			compressionStatus = "Deaktiviert"
		} else if !slot.IsOptimized && slot.OptimizeHint != "" {
			compressionStatus = slot.OptimizeHint
		}
		namingStatus := "Korrekt"
		if !slot.NamingOK && slot.NamingHint != "" {
			namingStatus = slot.NamingHint
		}
		infoText = fmt.Sprintf("%s\nDatei: %s\nGröße: %s\nStatus: Vorhanden\nBenennung: %s\nKomprimierung: %s", slot.Label, filepath.Base(slot.ExistingPath), formatBytes(slot.SizeBytes), namingStatus, compressionStatus)
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
	coverButtonText := "Hinzufügen"
	if slot.Exists {
		coverButtonText = "Ersetzen"
	}
	replaceButton := widget.NewButton(coverButtonText, func() {
		a.selectAndPreviewForSlot(item.ID, slot)
	})
	renameButton := widget.NewButton("Umbenennen", func() {
		a.confirmRenameCover(item.ID, slot)
	})
	if !slot.Exists || slot.NamingOK {
		renameButton.Disable()
	}
	slotCopy := slot
	itemID := item.ID
	itemTitle := item.Title
	compressButton := widget.NewButton("Komprimieren", func() {
		a.compressSingleCover(itemID, itemTitle, slotCopy)
	})
	if cfg.Compression.Disabled || !slot.Exists || slot.IsOptimized {
		compressButton.Disable()
	}
	actionButtons := []fyne.CanvasObject{replaceButton, deleteButton, renameButton}
	if !cfg.Compression.Disabled {
		actionButtons = append(actionButtons, compressButton)
	}
	actions := container.NewVBox(actionButtons...)
	return container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), container.NewBorder(nil, nil, preview, actions, info))
}

func (a *Application) compressSingleCover(itemID, itemTitle string, slot models.CoverSlot) {
	cfg := a.config.Get()
	if cfg.Compression.Disabled {
		dialog.ShowInformation("Komprimierung deaktiviert", "Aktiviere die Komprimierung in den Einstellungen, um Cover zu komprimieren.", a.window)
		return
	}
	normalizeName := slot.NamingOK
	go func() {
		_, err := cover.CompressCover(slot, itemTitle, cfg.Compression, normalizeName, cfg.OriginalsPath)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			msg := fmt.Sprintf("%s wurde komprimiert. Original gesichert.", slot.Label)
			if !normalizeName {
				msg += "\n\nDer erkannte Aliasname wurde beibehalten. Nutze \"Umbenennen\", um auf den Zielnamen zu wechseln."
			}
			dialog.ShowInformation("Komprimierung", msg, a.window)
			a.refreshData("Scan läuft ...", func() { a.showDetail(itemID) })
		})
	}()
}

func (a *Application) confirmRenameCover(itemID string, slot models.CoverSlot) {
	if !slot.Exists || slot.ExistingPath == "" {
		return
	}
	targetPath := cover.RenameTargetPath(slot)
	ext := filepath.Ext(slot.ExistingPath)
	if ext == "" {
		ext = filepath.Ext(targetPath)
	}
	dialog.NewConfirm("Cover umbenennen",
		fmt.Sprintf("%s wird auf den Zielnamen umbenannt.\n\nVon: %s\nNach: %s\n\nNur der Dateiname wird geändert. Das Format bleibt %s.", slot.Label, filepath.Base(slot.ExistingPath), filepath.Base(targetPath), ext),
		func(ok bool) {
			if !ok {
				return
			}
			if _, err := cover.RenameCoverToTargetName(slot); err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			a.refreshData("Scan läuft ...", func() { a.showDetail(itemID) })
		}, a.window).Show()
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
			title := "Cover hinzufügen"
			if slot.Exists {
				title = "Cover ersetzen"
			}
			a.showImportPreview(title, []cover.ImportPlan{plan}, func() {
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
			a.showImportPreview("Cover automatisch zuordnen", plans, func() {
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
	cfg := a.config.Get()
	applyCount := 0
	for _, plan := range plans {
		if plan.CanApply {
			applyCount++
		}
	}
	summary := widget.NewLabel(fmt.Sprintf("%d Datei(en) ausgewählt, %d werden übernommen.", len(plans), applyCount))
	summary.Wrapping = fyne.TextWrapWord

	rows := container.NewVBox()
	for _, plan := range plans {
		label := widget.NewLabel(previewText(plan, cfg.Compression))
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

func previewText(plan cover.ImportPlan, compression models.CompressionConfig) string {
	icon := "[FEHLER]"
	if plan.CanApply {
		icon = "[OK]"
	}
	if plan.Overwrites {
		icon = "[WARNUNG]"
	}
	target := cover.FinalImportTargetPath(plan, compression)
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
	return fmt.Sprintf("%s %s\nTitel: %s\nPosition: %s\nAktion: %s\nZiel: %s\n%s",
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
	a.detailDropSlots = nil
	cfg := a.config.Get()
	backButton := widget.NewButton("Zurück", func() {
		a.showMainList()
		a.refreshData("Scan läuft ...", nil)
	})
	title := widget.NewLabelWithStyle("Einstellungen", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header := container.NewBorder(nil, nil, backButton, nil, title)

	// --- Server mode ---
	modeSelect := widget.NewSelect([]string{"Plex", "Jellyfin"}, nil)
	if cfg.ServerMode == models.ServerModeJellyfin {
		modeSelect.SetSelected("Jellyfin")
	} else {
		modeSelect.SetSelected("Plex")
	}
	modeSelect.OnChanged = func(value string) {
		newMode := models.ServerModePlex
		if value == "Jellyfin" {
			newMode = models.ServerModeJellyfin
		}
		oldCfg := a.config.Get()
		if oldCfg.ServerMode == newMode {
			return
		}
		a.confirmModeSwitch(newMode)
	}

	// --- Media paths ---
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

	// --- Compression ---
	compressionControls := container.NewVBox()

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

	thresholdEntry := widget.NewEntry()
	thresholdEntry.SetText(strconv.Itoa(cfg.OptimizeThresholdKB))
	thresholdEntry.OnChanged = func(value string) {
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
			a.saveConfig(func(cfg *models.AppConfig) {
				cfg.OptimizeThresholdKB = parsed
			})
		}
	}

	compressionControls.Add(container.NewBorder(nil, nil, widget.NewLabel("JPEG-Qualität"), qualityValue, qualitySlider))
	compressionControls.Add(container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewLabel("Max. Breite"), nil, widthEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Max. Höhe"), nil, heightEntry),
	))
	compressionControls.Add(container.NewBorder(nil, nil, widget.NewLabel("Komprimierungs-Schwellwert (KB)"), nil, thresholdEntry))

	compressionCheck := widget.NewCheck("Komprimierung aktiviert", nil)
	compressionCheck.SetChecked(!cfg.Compression.Disabled)
	compressionCheck.OnChanged = func(enabled bool) {
		a.saveConfig(func(cfg *models.AppConfig) {
			cfg.Compression.Disabled = !enabled
		})
		a.showSettings()
	}
	if cfg.Compression.Disabled {
		compressionControls.Hide()
	}

	compressionBatchBox := container.NewVBox()
	if cfg.Compression.Disabled {
		info := widget.NewLabel("Komprimierung ist deaktiviert. Imports werden unverändert kopiert; bestehende Cover können hier nicht komprimiert werden.")
		info.Wrapping = fyne.TextWrapWord
		compressionBatchBox.Add(info)
	} else {
		uncompressedCount := a.countCompressibleCovers()
		compressLabel := widget.NewLabel(fmt.Sprintf("%d Cover können komprimiert werden.", uncompressedCount))
		batchCompressButton := widget.NewButton("Alle komprimieren", func() {
			a.batchCompress()
		})
		if uncompressedCount == 0 {
			batchCompressButton.Disable()
		}
		compressionBatchBox.Add(compressLabel)
		compressionBatchBox.Add(batchCompressButton)
	}

	// --- PosterDB search ---
	posterDBCheck := widget.NewCheck("Suchbutton für theposterdb.com anzeigen (bei fehlenden Covern)", nil)
	posterDBCheck.SetChecked(cfg.PosterDBSearchEnabled)
	posterDBCheck.OnChanged = func(enabled bool) {
		a.saveConfig(func(cfg *models.AppConfig) {
			cfg.PosterDBSearchEnabled = enabled
		})
	}

	// --- Backup and missing covers ---
	backupReportBox := container.NewVBox()
	backupCount := a.countExistingCovers()
	backupLabel := widget.NewLabel(fmt.Sprintf("%d gefundene Cover/Poster können gesichert werden.", backupCount))
	backupLabel.Wrapping = fyne.TextWrapWord
	backupButton := widget.NewButton("Cover-Backup erstellen", a.startCoverBackup)
	if backupCount == 0 {
		backupButton.Disable()
	}
	missingSlots, missingItems := a.missingCoverStats()
	missingLabel := widget.NewLabel(fmt.Sprintf("%d fehlende Cover in %d Titeln.", missingSlots, missingItems))
	missingLabel.Wrapping = fyne.TextWrapWord
	missingButton := widget.NewButton("Fehlende Cover anzeigen", a.showMissingCoverReport)
	if missingSlots == 0 {
		missingButton.Disable()
	}
	backupReportBox.Add(backupLabel)
	backupReportBox.Add(backupButton)
	backupReportBox.Add(missingLabel)
	backupReportBox.Add(missingButton)

	// --- Config file ---
	configPath := widget.NewLabel(fmt.Sprintf("Config: %s", a.config.Path()))
	configPath.Wrapping = fyne.TextWrapWord
	configPath.Selectable = true
	openConfigFolderButton := widget.NewButton("Ordner öffnen", func() {
		if err := openFolderInExplorer(filepath.Dir(a.config.Path())); err != nil {
			dialog.ShowError(err, a.window)
		}
	})
	configRow := container.NewBorder(nil, nil, nil, openConfigFolderButton, configPath)

	// --- Original backups (part of compression) ---
	effectiveBackupsDir, effectiveBackupsErr := cover.OriginalBackupDir(cfg.OriginalsPath)

	backupsHeader := widget.NewLabelWithStyle("Originale (Backups bei Komprimierung)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	effectiveText := ""
	if effectiveBackupsErr != nil {
		effectiveText = fmt.Sprintf("Aktueller Pfad: nicht ermittelbar (%s)", effectiveBackupsErr.Error())
	} else {
		effectiveText = fmt.Sprintf("Aktueller Pfad: %s", effectiveBackupsDir)
	}
	effectiveLabel := widget.NewLabel(effectiveText)
	effectiveLabel.Wrapping = fyne.TextWrapWord
	effectiveLabel.Selectable = true

	openBackupsFolderButton := widget.NewButton("Ordner öffnen", func() {
		if effectiveBackupsErr != nil {
			dialog.ShowError(effectiveBackupsErr, a.window)
			return
		}
		if err := os.MkdirAll(effectiveBackupsDir, 0o755); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		if err := openFolderInExplorer(effectiveBackupsDir); err != nil {
			dialog.ShowError(err, a.window)
		}
	})
	if effectiveBackupsErr != nil {
		openBackupsFolderButton.Disable()
	}
	effectiveRow := container.NewBorder(nil, nil, nil, openBackupsFolderButton, effectiveLabel)

	originalsEntry := widget.NewEntry()
	originalsEntry.SetPlaceHolder("Leer = Standard verwenden")
	originalsEntry.SetText(cfg.OriginalsPath)
	originalsEntry.OnChanged = func(value string) {
		trimmed := strings.TrimSpace(value)
		a.saveConfig(func(cfg *models.AppConfig) {
			cfg.OriginalsPath = trimmed
		})
	}
	browseOriginalsButton := widget.NewButton("Durchsuchen...", func() {
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
				originalsEntry.SetText(path)
				a.showSettings()
			})
		}()
	})
	resetOriginalsButton := widget.NewButton("Standard verwenden", func() {
		originalsEntry.SetText("")
		a.saveConfig(func(cfg *models.AppConfig) {
			cfg.OriginalsPath = ""
		})
		a.showSettings()
	})
	if strings.TrimSpace(cfg.OriginalsPath) == "" {
		resetOriginalsButton.Disable()
	}

	pathEntryRow := container.NewBorder(nil, nil, nil, container.NewHBox(browseOriginalsButton, resetOriginalsButton), originalsEntry)
	if cfg.Compression.Disabled {
		pathEntryRow.Hide()
	}
	backupsBlock := container.NewVBox(backupsHeader, effectiveRow, pathEntryRow)

	body := container.NewVBox(
		widget.NewLabelWithStyle("Server-Typ", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Modus"), nil, modeSelect),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Medienpfade", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		pathsBox,
		addPathButton,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Poster-Suche", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		posterDBCheck,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Backup und Fehlliste", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		backupReportBox,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Komprimierung", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		compressionCheck,
		compressionControls,
		compressionBatchBox,
		backupsBlock,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Konfiguration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		configRow,
	)
	a.window.SetContent(container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
}

func (a *Application) confirmModeSwitch(newMode models.ServerMode) {
	modeLabel := "Plex"
	if newMode == models.ServerModeJellyfin {
		modeLabel = "Jellyfin"
	}
	a.mu.RLock()
	items := append([]models.MediaItem(nil), a.items...)
	a.mu.RUnlock()

	content := widget.NewLabel(fmt.Sprintf("Möchtest du bestehende Staffel-Cover umbenennen, damit sie dem %s-Namensschema entsprechen?\n\nBetrifft nur Serien mit Staffelordnern.", modeLabel))
	content.Wrapping = fyne.TextWrapWord
	modeDialog := dialog.NewCustomWithoutButtons("Server-Typ wechseln", content, a.window)

	switchMode := func(rename bool) {
		modeDialog.Hide()
		if rename {
			go func() {
				renamed, errs := cover.RenameCoversForModeSwitch(items, newMode)
				fyne.Do(func() {
					a.saveConfig(func(cfg *models.AppConfig) {
						cfg.ServerMode = newMode
					})
					msg := fmt.Sprintf("%d Cover umbenannt.", renamed)
					if len(errs) > 0 {
						msg += fmt.Sprintf("\n\nFehler:\n%s", strings.Join(errs, "\n"))
					}
					dialog.ShowInformation("Modus gewechselt", msg, a.window)
					a.refreshData("Scan läuft ...", func() { a.showSettings() })
				})
			}()
			return
		}
		a.saveConfig(func(cfg *models.AppConfig) {
			cfg.ServerMode = newMode
		})
		a.refreshData("Scan läuft ...", func() { a.showSettings() })
	}

	cancelButton := widget.NewButton("Abbrechen", func() {
		modeDialog.Hide()
		a.showSettings()
	})
	withoutRenameButton := widget.NewButton("Ohne Umbenennen", func() {
		switchMode(false)
	})
	renameButton := widget.NewButton("Umbenennen", func() {
		switchMode(true)
	})
	renameButton.Importance = widget.HighImportance
	modeDialog.SetButtons([]fyne.CanvasObject{cancelButton, withoutRenameButton, renameButton})
	modeDialog.Show()
}

func (a *Application) countCompressibleCovers() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	count := 0
	for _, item := range a.items {
		for _, slot := range item.CoverSlots {
			if slot.Exists && !slot.IsOptimized {
				count++
			}
		}
	}
	return count
}

func (a *Application) batchCompress() {
	cfg := a.config.Get()
	if cfg.Compression.Disabled {
		dialog.ShowInformation("Komprimierung deaktiviert", "Aktiviere die Komprimierung in den Einstellungen, um bestehende Cover zu komprimieren.", a.window)
		return
	}
	a.mu.RLock()
	type compressJob struct {
		itemTitle string
		slot      models.CoverSlot
	}
	var jobs []compressJob
	for _, item := range a.items {
		for _, slot := range item.CoverSlots {
			if slot.Exists && !slot.IsOptimized {
				jobs = append(jobs, compressJob{itemTitle: item.Title, slot: slot})
			}
		}
	}
	a.mu.RUnlock()

	if len(jobs) == 0 {
		dialog.ShowInformation("Komprimierung", "Keine Cover zum Komprimieren gefunden.", a.window)
		return
	}

	dialog.NewConfirm("Batch-Komprimierung",
		fmt.Sprintf("%d Cover werden komprimiert. Originale werden gesichert.\nFortfahren?", len(jobs)),
		func(ok bool) {
			if !ok {
				return
			}
			go func() {
				compressed := 0
				var failures []string
				for _, job := range jobs {
					if _, err := cover.CompressCover(job.slot, job.itemTitle, cfg.Compression, job.slot.NamingOK, cfg.OriginalsPath); err != nil {
						failures = append(failures, fmt.Sprintf("%s (%s): %s", job.itemTitle, job.slot.Label, err.Error()))
						continue
					}
					compressed++
				}
				fyne.Do(func() {
					msg := fmt.Sprintf("%d Cover komprimiert.", compressed)
					if len(failures) > 0 {
						msg += fmt.Sprintf("\n\nFehler:\n%s", strings.Join(failures, "\n"))
					}
					dialog.ShowInformation("Batch-Komprimierung", msg, a.window)
					a.refreshData("Scan läuft ...", func() { a.showSettings() })
				})
			}()
		}, a.window).Show()
}

func (a *Application) startCoverBackup() {
	items := a.itemsSnapshot()
	if countExistingCoversInItems(items) == 0 {
		dialog.ShowInformation("Cover-Backup", "Keine vorhandenen Cover zum Sichern gefunden.", a.window)
		return
	}
	go func() {
		path, err := selectFolderWithTitle("Backup-Zielordner auswählen")
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if strings.TrimSpace(path) == "" {
				return
			}
			a.exportCoverBackup(items, path)
		})
	}()
}

func (a *Application) exportCoverBackup(items []models.MediaItem, destinationRoot string) {
	progress := widget.NewProgressBarInfinite()
	progress.Start()
	progressLabel := widget.NewLabel("Cover werden kopiert ...")
	progressDialog := dialog.NewCustomWithoutButtons("Cover-Backup", container.NewVBox(progressLabel, progress), a.window)
	progressDialog.Resize(fyne.NewSize(420, 120))
	progressDialog.Show()

	go func() {
		result, err := backend.ExportExistingCovers(items, destinationRoot)
		fyne.Do(func() {
			progress.Stop()
			progressDialog.Hide()
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			a.showCoverBackupResult(result)
		})
	}()
}

func (a *Application) showCoverBackupResult(result backend.CoverExportResult) {
	message := fmt.Sprintf("%d Cover kopiert.\n\nOrdner: %s", result.Copied, result.OutputDir)
	if len(result.Errors) > 0 {
		message += fmt.Sprintf("\n\nFehler (%d):\n%s", len(result.Errors), formatErrorList(result.Errors, 8))
	}
	content := widget.NewLabel(message)
	content.Wrapping = fyne.TextWrapWord
	content.Selectable = true

	resultDialog := dialog.NewCustomWithoutButtons("Cover-Backup abgeschlossen", content, a.window)
	openButton := widget.NewButton("Ordner öffnen", func() {
		if err := openFolderInExplorer(result.OutputDir); err != nil {
			dialog.ShowError(err, a.window)
		}
	})
	closeButton := widget.NewButton("Schließen", func() {
		resultDialog.Hide()
		a.refreshData("Scan läuft ...", func() { a.showSettings() })
	})
	resultDialog.SetButtons([]fyne.CanvasObject{openButton, closeButton})
	resultDialog.Resize(fyne.NewSize(720, 320))
	resultDialog.Show()
}

func (a *Application) showMissingCoverReport() {
	items := a.itemsSnapshot()
	report, missingSlots, missingItems := backend.BuildMissingCoverReport(items)
	if missingSlots == 0 {
		dialog.ShowInformation("Fehlende Cover", "Alle Cover sind vorhanden.", a.window)
		return
	}

	summary := widget.NewLabel(fmt.Sprintf("%d fehlende Cover in %d Titeln.", missingSlots, missingItems))
	summary.Wrapping = fyne.TextWrapWord
	reportGrid := widget.NewTextGridFromString(report)
	scroll := container.NewScroll(reportGrid)
	scroll.SetMinSize(fyne.NewSize(760, 420))
	copyStatus := widget.NewLabel("")
	content := container.NewBorder(summary, copyStatus, nil, nil, scroll)

	reportDialog := dialog.NewCustomWithoutButtons("Fehlende Cover", content, a.window)
	copyButton := widget.NewButton("Kopieren", func() {
		fyne.CurrentApp().Clipboard().SetContent(report)
		copyStatus.SetText("In die Zwischenablage kopiert.")
	})
	closeButton := widget.NewButton("Schließen", func() {
		reportDialog.Hide()
	})
	reportDialog.SetButtons([]fyne.CanvasObject{copyButton, closeButton})
	reportDialog.Resize(fyne.NewSize(840, 560))
	reportDialog.Show()
}

func (a *Application) countExistingCovers() int {
	return countExistingCoversInItems(a.itemsSnapshot())
}

func (a *Application) missingCoverStats() (int, int) {
	_, missingSlots, missingItems := backend.BuildMissingCoverReport(a.itemsSnapshot())
	return missingSlots, missingItems
}

func (a *Application) itemsSnapshot() []models.MediaItem {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]models.MediaItem(nil), a.items...)
}

func countExistingCoversInItems(items []models.MediaItem) int {
	count := 0
	for _, item := range items {
		for _, slot := range item.CoverSlots {
			if strings.TrimSpace(slot.ExistingPath) != "" {
				count++
			}
		}
	}
	return count
}

func formatErrorList(errs []error, maxShown int) string {
	lines := make([]string, 0, len(errs))
	for i, err := range errs {
		if i >= maxShown {
			lines = append(lines, fmt.Sprintf("... und %d weitere Fehler", len(errs)-maxShown))
			break
		}
		lines = append(lines, "- "+err.Error())
	}
	return strings.Join(lines, "\n")
}

func (a *Application) settingsPathRow(index int, mediaPath models.MediaPath) fyne.CanvasObject {
	pathLabel := widget.NewLabel(mediaPath.Path)
	pathLabel.Wrapping = fyne.TextWrapWord
	pathLabel.Selectable = true
	statusLabel := widget.NewLabel("prüfe...")
	reachability := newStatusIndicator(visualStatus{fill: statusUnknownColor, tooltip: "Erreichbarkeit wird geprüft"})

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
	row := container.NewBorder(nil, widget.NewSeparator(), nil, container.NewHBox(typeSelect, reachability, statusLabel, deleteButton), pathLabel)
	go func(path string) {
		_, err := os.Stat(path)
		reachable := err == nil
		fyne.Do(func() {
			if reachable {
				statusLabel.SetText("erreichbar")
			} else {
				statusLabel.SetText("nicht erreichbar")
			}
			reachability.SetStatus(reachabilityStatus(reachable))
		})
	}(mediaPath.Path)
	return row
}

func (a *Application) addMediaPath() {
	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder(`\\Server\Share\Media oder C:\Media`)
	typeSelect := widget.NewSelect([]string{"Serie", "Film"}, nil)
	typeSelect.SetSelected("Serie")
	hint := widget.NewLabel("Netzwerkpfade am schnellsten direkt einfügen. Durchsuchen ist nur optional.")
	hint.Wrapping = fyne.TextWrapWord

	browseButton := widget.NewButton("Durchsuchen...", func() {
		go func() {
			path, err := selectFolder()
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, a.window)
					return
				}
				if strings.TrimSpace(path) != "" {
					pathEntry.SetText(path)
				}
			})
		}()
	})

	content := container.NewVBox(
		widget.NewLabel("Medienpfad"),
		pathEntry,
		browseButton,
		widget.NewLabel("Typ"),
		typeSelect,
		hint,
	)
	confirm := dialog.NewCustomConfirm("Pfad hinzufügen", "Hinzufügen", "Abbrechen", content, func(ok bool) {
		if !ok {
			return
		}
		path := strings.TrimSpace(pathEntry.Text)
		if path == "" {
			dialog.ShowInformation("Pfad fehlt", "Bitte gib einen Medienpfad ein oder wähle einen Ordner aus.", a.window)
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
	}, a.window)
	confirm.Resize(fyne.NewSize(640, 260))
	confirm.Show()
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

func itemStructureText(item models.MediaItem, mode models.ServerMode) string {
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
		text := fmt.Sprintf("Flache Struktur: %d Staffel(n) aus Dateinamen erkannt: %s", len(labels), strings.Join(labels, ", "))
		if mode == models.ServerModeJellyfin {
			text += ". Jellyfin-Flat-Fallback: Staffelcover bleiben seasonXX-poster.jpg im Serienordner, weil ohne Staffelordner kein eindeutiges poster.jpg pro Staffel möglich ist."
		}
		return text
	}
	return fmt.Sprintf("%d Staffelordner gefunden: %s", len(labels), strings.Join(labels, ", "))
}

func targetDisplayName(item models.MediaItem, slot models.CoverSlot, mode models.ServerMode) string {
	name := filepath.Base(slot.TargetPath)
	if mode == models.ServerModeJellyfin && item.Type == models.MediaTypeSeries && item.FlatStructure && slot.Kind == models.CoverKindSeason {
		return name + " (Jellyfin-Flat-Fallback)"
	}
	return name
}

func displayItemPath(item models.MediaItem) string {
	if item.MediaFilePath != "" {
		return item.MediaFilePath
	}
	return item.Path
}

func itemFolderPath(item models.MediaItem) string {
	if item.Path != "" {
		return item.Path
	}
	if item.MediaFilePath != "" {
		return filepath.Dir(item.MediaFilePath)
	}
	return displayItemPath(item)
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

func visualStatusForItem(item models.MediaItem) visualStatus {
	return visualStatusForCover(item.Status, compressibleCoverCount(item), renameableCoverCount(item), item.StatusLabel())
}

func visualStatusForCover(status models.CoverStatus, compressCount, renameCount int, statusText string) visualStatus {
	if compressCount > 0 || renameCount > 0 {
		lines := []string{statusText}
		if compressCount > 0 {
			lines = append(lines, fmt.Sprintf("Komprimierung möglich: %d Cover", compressCount))
		}
		if renameCount > 0 {
			lines = append(lines, fmt.Sprintf("Umbenennung möglich: %d Cover", renameCount))
		}
		return visualStatus{
			fill:    statusOptimizeColor,
			tooltip: strings.Join(lines, "\n"),
		}
	}
	switch status {
	case models.CoverStatusComplete:
		return visualStatus{fill: statusCompleteColor, tooltip: statusText}
	case models.CoverStatusPartial:
		return visualStatus{fill: statusPartialColor, tooltip: statusText}
	default:
		return visualStatus{fill: statusMissingColor, tooltip: statusText}
	}
}

func compressibleCoverCount(item models.MediaItem) int {
	count := 0
	for _, slot := range item.CoverSlots {
		if slot.Exists && !slot.IsOptimized {
			count++
		}
	}
	return count
}

func renameableCoverCount(item models.MediaItem) int {
	count := 0
	for _, slot := range item.CoverSlots {
		if slot.Exists && !slot.NamingOK {
			count++
		}
	}
	return count
}

func posterDBSearchURL(item models.MediaItem) string {
	section := "shows"
	if item.Type == models.MediaTypeMovie {
		section = "movies"
	}
	params := url.Values{}
	params.Set("term", item.Title)
	params.Set("section", section)
	return "https://theposterdb.com/search?" + params.Encode()
}

func shouldOfferPosterDBSearch(item models.MediaItem) bool {
	return item.Status != models.CoverStatusComplete
}

func (a *Application) openPosterDBSearch(item models.MediaItem) {
	if strings.TrimSpace(item.Title) == "" {
		return
	}
	if err := openURLInBrowser(posterDBSearchURL(item)); err != nil {
		dialog.ShowError(err, a.window)
	}
}

func reachabilityStatus(reachable bool) visualStatus {
	if reachable {
		return visualStatus{fill: statusCompleteColor, tooltip: "Pfad erreichbar"}
	}
	return visualStatus{fill: statusMissingColor, tooltip: "Pfad nicht erreichbar"}
}
