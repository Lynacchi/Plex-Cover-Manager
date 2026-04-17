package models

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type MediaType string

const (
	MediaTypeSeries MediaType = "series"
	MediaTypeMovie  MediaType = "movie"
)

type ServerMode string

const (
	ServerModePlex     ServerMode = "plex"
	ServerModeJellyfin ServerMode = "jellyfin"
)

type CoverStatus string

const (
	CoverStatusNone     CoverStatus = "none"
	CoverStatusPartial  CoverStatus = "partial"
	CoverStatusComplete CoverStatus = "complete"
)

type CoverKind string

const (
	CoverKindMain   CoverKind = "main"
	CoverKindSeason CoverKind = "season"
)

type MediaPath struct {
	Path string    `json:"path"`
	Type MediaType `json:"type"`
}

type CompressionConfig struct {
	Disabled    bool `json:"disabled"`
	JPEGQuality int  `json:"jpeg_quality"`
	MaxWidth    int  `json:"max_width"`
	MaxHeight   int  `json:"max_height"`
}

type AppConfig struct {
	ServerMode            ServerMode        `json:"server_mode"`
	MediaPaths            []MediaPath       `json:"media_paths"`
	Compression           CompressionConfig `json:"compression"`
	OptimizeThresholdKB   int               `json:"optimize_threshold_kb"`
	PosterDBSearchEnabled bool              `json:"posterdb_search_enabled"`
}

type CoverSlot struct {
	Key          string
	Label        string
	Kind         CoverKind
	SeasonNumber int
	TargetPath   string
	ExistingPath string
	Exists       bool
	SizeBytes    int64
	NamingOK     bool
	NamingHint   string
	IsOptimized  bool
	OptimizeHint string
}

type SeasonInfo struct {
	Number   int
	Label    string
	Path     string
	HasMedia bool
}

type MediaItem struct {
	ID            string
	Title         string
	Year          string
	Type          MediaType
	LibraryPath   string
	Path          string
	MediaFilePath string
	FlatStructure bool
	Seasons       []SeasonInfo
	CoverSlots    []CoverSlot
	Status        CoverStatus
	Missing       []string
	Warnings      []string
}

type ScanWarning struct {
	Path    string
	Message string
}

func DefaultConfig() AppConfig {
	return AppConfig{
		ServerMode: ServerModePlex,
		MediaPaths: []MediaPath{},
		Compression: CompressionConfig{
			JPEGQuality: 85,
			MaxWidth:    1000,
			MaxHeight:   1500,
		},
		OptimizeThresholdKB:   300,
		PosterDBSearchEnabled: true,
	}
}

func (c *AppConfig) Normalize() {
	if c.ServerMode != ServerModeJellyfin {
		c.ServerMode = ServerModePlex
	}
	if c.Compression.JPEGQuality == 0 {
		c.Compression.JPEGQuality = 85
	}
	if c.Compression.MaxWidth == 0 {
		c.Compression.MaxWidth = 1000
	}
	if c.Compression.MaxHeight == 0 {
		c.Compression.MaxHeight = 1500
	}
	if c.Compression.JPEGQuality < 70 {
		c.Compression.JPEGQuality = 70
	}
	if c.Compression.JPEGQuality > 100 {
		c.Compression.JPEGQuality = 100
	}
	if c.Compression.MaxWidth < 1 {
		c.Compression.MaxWidth = 1000
	}
	if c.Compression.MaxHeight < 1 {
		c.Compression.MaxHeight = 1500
	}
	if c.OptimizeThresholdKB <= 0 {
		c.OptimizeThresholdKB = 300
	}
	for i := range c.MediaPaths {
		if c.MediaPaths[i].Type != MediaTypeMovie {
			c.MediaPaths[i].Type = MediaTypeSeries
		}
		path := strings.TrimSpace(c.MediaPaths[i].Path)
		if path != "" {
			path = filepath.Clean(path)
		}
		c.MediaPaths[i].Path = path
	}
}

func (m *MediaItem) RecalculateStatus() {
	total := len(m.CoverSlots)
	present := 0
	missing := make([]string, 0)
	for _, slot := range m.CoverSlots {
		if slot.Exists {
			present++
			continue
		}
		missing = append(missing, slot.Label)
	}
	m.Missing = missing
	switch {
	case total == 0 || present == 0:
		m.Status = CoverStatusNone
	case present == total:
		m.Status = CoverStatusComplete
	default:
		m.Status = CoverStatusPartial
	}
}

func (m MediaItem) TypeLabel() string {
	if m.Type == MediaTypeMovie {
		return "Film"
	}
	return "Serie"
}

func (m MediaItem) StatusLabel() string {
	switch m.Status {
	case CoverStatusComplete:
		return "Alle Cover vorhanden"
	case CoverStatusPartial:
		if len(m.Missing) == 0 {
			return "Teilweise vorhanden"
		}
		if len(m.Missing) == 1 {
			return fmt.Sprintf("%s fehlt", m.Missing[0])
		}
		if len(m.Missing) <= 3 {
			return fmt.Sprintf("%s fehlen", strings.Join(m.Missing, ", "))
		}
		return fmt.Sprintf("%s fehlen und %d weitere", strings.Join(m.Missing[:3], ", "), len(m.Missing)-3)
	default:
		return "Kein Cover vorhanden"
	}
}

func (m MediaItem) StatusIcon() string {
	switch m.Status {
	case CoverStatusComplete:
		return "complete"
	case CoverStatusPartial:
		return "partial"
	default:
		return "none"
	}
}

func (m MediaItem) SortedSlots() []CoverSlot {
	slots := append([]CoverSlot(nil), m.CoverSlots...)
	sort.SliceStable(slots, func(i, j int) bool {
		a, b := slots[i], slots[j]
		if a.Kind != b.Kind {
			return a.Kind == CoverKindMain
		}
		return a.SeasonNumber < b.SeasonNumber
	})
	return slots
}

func (s SeasonInfo) DisplayLabel() string {
	if s.Number == 0 {
		return "Specials"
	}
	return fmt.Sprintf("S%02d", s.Number)
}

func SeasonSlotKey(season int) string {
	return fmt.Sprintf("season:%02d", season)
}

func MainSlotKey() string {
	return "main"
}
