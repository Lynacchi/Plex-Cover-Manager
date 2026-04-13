package cover

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var supportedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

var trailingYearPattern = regexp.MustCompile(`^(.*)\s+\((\d{4})\)$`)

type ParsedCover struct {
	SourcePath   string
	FileName     string
	Title        string
	Year         string
	IsSeason     bool
	SeasonNumber int
	Extension    string
}

func ParseCoverFile(path string) (ParsedCover, error) {
	name := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(name))
	if !supportedImageExtensions[ext] {
		return ParsedCover{}, fmt.Errorf("nicht unterstütztes Format: %s", ext)
	}

	base := strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name)))
	titlePart := base
	var seasonNumber int
	isSeason := false

	if before, tail, ok := splitSeasonTail(base); ok {
		titlePart = before
		seasonNumber = tail
		isSeason = true
	}

	year := ""
	if matches := trailingYearPattern.FindStringSubmatch(strings.TrimSpace(titlePart)); len(matches) == 3 {
		titlePart = strings.TrimSpace(matches[1])
		year = matches[2]
	}
	if titlePart == "" {
		return ParsedCover{}, fmt.Errorf("kein Titel im Dateinamen erkannt")
	}

	return ParsedCover{
		SourcePath:   path,
		FileName:     name,
		Title:        titlePart,
		Year:         year,
		IsSeason:     isSeason,
		SeasonNumber: seasonNumber,
		Extension:    ext,
	}, nil
}

func splitSeasonTail(base string) (string, int, bool) {
	idx := strings.LastIndex(base, " - ")
	if idx < 0 {
		return "", 0, false
	}
	before := strings.TrimSpace(base[:idx])
	tail := strings.TrimSpace(base[idx+3:])
	season, ok := ParseSeasonToken(tail)
	if !ok || before == "" {
		return "", 0, false
	}
	return before, season, true
}

func ParseSeasonToken(token string) (int, bool) {
	normalized := strings.TrimSpace(strings.ToLower(token))
	switch normalized {
	case "special", "specials", "sp":
		return 0, true
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^season\s*0*(\d{1,2})$`),
		regexp.MustCompile(`^s0*(\d{1,2})$`),
		regexp.MustCompile(`^staffel\s*0*(\d{1,2})$`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(normalized)
		if len(matches) != 2 {
			continue
		}
		season, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, false
		}
		return season, true
	}
	return 0, false
}
